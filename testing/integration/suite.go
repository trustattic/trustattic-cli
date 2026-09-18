//go:build integration

package integration

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/trustattic/trustattic-cli/internal/client"
	"github.com/trustattic/trustattic-cli/testing/container"
	"github.com/trustattic/trustattic-cli/testing/integration/helper"
)

// BinaryPath is the path to the built trustattic binary, set once by
// TestMain before any suite runs.
var BinaryPath string

// BaseTestSuite starts the postgres+cloudapi+platform-api stack once per
// suite and bootstraps a real project and service-account token via the
// typed API client directly (never via the CLI - the CLI itself only ever
// uses the resulting service-account token, per the design spec).
type BaseTestSuite struct {
	suite.Suite
	stack *container.Stack

	// Token is the bootstrapped service-account token. Project is the slug
	// of the bootstrapped project. Both are set by SetupSuite.
	Token   string
	Project string

	// configHome is a throwaway directory RunCLI points the CLI's config
	// lookup at, so a developer's real ~/.config/trustattic/config.yaml can
	// never leak into a test run. See RunCLI.
	configHome string
}

func ptr[T any](v T) *T { return &v }

func (s *BaseTestSuite) SetupSuite() {
	ctx := context.Background()

	// Created from the suite-level T (not a per-test one) so it outlives
	// every test in the suite; RunCLI points the CLI at it.
	s.configHome = s.T().TempDir()

	jwtHelper, err := helper.NewJWT()
	s.Require().NoError(err)
	pubPEM, err := jwtHelper.PublicKeyPEM()
	s.Require().NoError(err)

	stack, err := container.Start(ctx, pubPEM)
	s.Require().NoError(err)
	s.stack = stack
	// If any Require() below this point fails, testify calls t.FailNow(),
	// which invokes runtime.Goexit() - SetupSuite never returns normally,
	// so suite.Run() never gets to register TearDownSuite's defer, and the
	// stack would otherwise leak. runtime.Goexit() still runs already-
	// registered defers in this goroutine before it exits (see `go doc
	// runtime.Goexit`), so this defer terminates the stack in that case. On
	// a normal, fully-successful return, s.T().Failed() is false, this is a
	// no-op, and TearDownSuite performs the real cleanup as before.
	defer func() {
		if s.T().Failed() {
			s.stack.Terminate(ctx)
		}
	}()

	apiClient, err := client.NewClientWithResponses(stack.PlatformURL)
	s.Require().NoError(err)

	userJWT, err := jwtHelper.Mint("bootstrap-user")
	s.Require().NoError(err)

	obtainResp, err := apiClient.AccountPutObtainWithResponse(ctx, &client.AccountPutObtainParams{
		XAuthToken: userJWT,
	}, client.AccountPutObtainJSONRequestBody{
		Email:  "bootstrap@example.com",
		Name:   "Bootstrap User",
		Issuer: "TEST",
	})
	s.Require().NoError(err)
	s.Require().Less(obtainResp.StatusCode(), 300, string(obtainResp.Body))

	projectResp, err := apiClient.ProjectPostWithResponse(ctx, &client.ProjectPostParams{
		XAuthToken: userJWT,
	}, client.ProjectPostJSONRequestBody{
		Name: ptr("Bootstrap Project"),
	})
	s.Require().NoError(err)
	s.Require().Equal(201, projectResp.StatusCode(), string(projectResp.Body))
	s.Project = projectResp.JSON201.Slug

	saResp, err := apiClient.AccountPostWithResponse(ctx, &client.AccountPostParams{
		XAuthToken: userJWT,
	}, client.AccountPostJSONRequestBody{
		Name: ptr("cli-service-account"),
		Scope: &client.PermissionsScopes{{
			ProjectId:   projectResp.JSON201.Id,
			Permissions: client.Permissions{"project/manage"},
		}},
	})
	s.Require().NoError(err)
	s.Require().Equal(201, saResp.StatusCode(), string(saResp.Body))

	tokenResp, err := apiClient.AccountTokensPostWithResponse(ctx, saResp.JSON201.Id, &client.AccountTokensPostParams{
		XAuthToken: userJWT,
	}, client.AccountTokensPostJSONRequestBody{
		Name: ptr("cli-integration-test"),
		// ExpireAt is a required field. Platform happens to tolerate the
		// zero value (treating a zero/past expiry as "use the default"),
		// but its own tests set it explicitly rather than lean on that, and
		// so should this one - nothing here needs a token whose expiry is
		// whatever the server decides.
		ExpireAt: time.Now().Add(time.Hour),
	})
	s.Require().NoError(err)
	s.Require().Equal(201, tokenResp.StatusCode(), string(tokenResp.Body))
	// MaskedKey carries the raw secret exactly once, at creation time
	// (see platform's internal/api/openapi/gin/gen.go doc comment on
	// Token.MaskedKey and internal/services/token/service/token.go
	// Generate, which returns the real issued key under that name) -
	// subsequent reads of the token only ever show it masked.
	s.Token = tokenResp.JSON201.MaskedKey
}

func (s *BaseTestSuite) TearDownSuite() {
	if s.stack != nil {
		s.stack.Terminate(context.Background())
	}
}

// RunCLI execs the built trustattic binary with args, pointed at the
// bootstrapped stack and authenticated as the bootstrapped service account.
// stdout and stderr are returned separately: callers parse stdout as JSON,
// and merging the two streams would let any warning the CLI writes to stderr
// corrupt that parse.
func (s *BaseTestSuite) RunCLI(args ...string) (stdout, stderr string, err error) {
	cmd := exec.Command(BinaryPath, args...)
	// TRUSTATTIC_TOKEN overrides the stored token, but nothing overrides
	// the stored current_project - so without redirecting the CLI's config
	// lookup, a developer's own `trustattic use <project>` would silently
	// become the default project for every project-scoped test. Both env
	// vars below are what os.UserConfigDir consults (XDG_CONFIG_HOME on
	// Linux, $HOME on macOS), so setting both isolates the run on either.
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("TRUSTATTIC_API_URL=%s", s.stack.PlatformURL),
		fmt.Sprintf("TRUSTATTIC_TOKEN=%s", s.Token),
		fmt.Sprintf("XDG_CONFIG_HOME=%s", s.configHome),
		fmt.Sprintf("HOME=%s", s.configHome),
	)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	return outBuf.String(), errBuf.String(), err
}

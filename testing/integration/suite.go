//go:build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"os/exec"

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
}

func ptr[T any](v T) *T { return &v }

func (s *BaseTestSuite) SetupSuite() {
	ctx := context.Background()

	jwtHelper, err := helper.NewJWT()
	s.Require().NoError(err)
	pubPEM, err := jwtHelper.PublicKeyPEM()
	s.Require().NoError(err)

	stack, err := container.Start(ctx, pubPEM)
	s.Require().NoError(err)
	s.stack = stack

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
func (s *BaseTestSuite) RunCLI(args ...string) (stdout string, err error) {
	cmd := exec.Command(BinaryPath, args...)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("TRUSTATTIC_API_URL=%s", s.stack.PlatformURL),
		fmt.Sprintf("TRUSTATTIC_TOKEN=%s", s.Token),
	)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

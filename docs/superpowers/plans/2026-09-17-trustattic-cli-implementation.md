# TrustAttic CLI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `trustattic`, a Go CLI for the TrustAttic Platform API whose command tree, flags, and request wiring are generated from the platform's OpenAPI spec, styled with the charm.land ecosystem, authenticating only as a service account.

**Architecture:** Two generation stages (oapi-codegen for a typed HTTP client, a small hand-written generator for the Cobra command tree) sit on top of a thin hand-written core (config, auth/transport, output formatting, Fang-wrapped root command). Everything resource-specific is generated; everything hand-written is generic.

**Tech Stack:** Go 1.26, Cobra + Fang + Lip Gloss + Bubbles + Huh (charmbracelet), oapi-codegen/v2, kin-openapi (spec parsing for the generator), testify, testcontainers-go, golang-jwt/v5.

**Spec:** `docs/superpowers/specs/2026-09-17-trustattic-cli-design.md`

## Global Constraints

- No CLI-specific business logic: command existence, names, flags, and validation must trace back to the OpenAPI spec, not be hand-coded per resource.
- Every operation in the spec gets a generated command, including ones that only make sense with a JWT (no endpoint filtering).
- The CLI authenticates only via `X-Auth-Token` with a service-account token — never JWT.
- Default API URL is hardcoded to `https://api.trustattic.com`; overridable via `TRUSTATTIC_API_URL`.
- Token stored in `~/.config/trustattic/config.yaml` (0600 perms), overridable via `TRUSTATTIC_TOKEN`.
- Output is styled/interactive by default, raw JSON when stdout isn't a TTY or `--output json` is passed.
- `--project` flags (bound to the `project_slug` path parameter specifically) are optional, falling back to a stored `current_project` set via `trustattic use`.
- Module path: `github.com/trustattic/trustattic-cli`. Go version: `1.26.3` (matches `platform`/`cloudapi`).
- oapi-codegen client generation config mirrors platform's (`package: client`, `models: true`, `client: true`, `response-type-suffix: Resp`), using `github.com/oapi-codegen/oapi-codegen/v2 v2.5.0` / `github.com/oapi-codegen/runtime v1.1.2` (same versions platform pins).
- Generator's own spec parsing uses `github.com/getkin/kin-openapi v0.132.0` (same version platform's dependency tree already resolves to).

---

## File Structure

```
trustattic-cli/
  go.mod
  Makefile
  spec/api.yaml                        # vendored copy of platform/api.yaml
  tools/
    tools.go                           # go:generate directive for oapi-codegen
    client.yaml                        # oapi-codegen config
  cmd/
    trustattic/main.go                  # entrypoint
    gen/main.go                         # generator CLI entrypoint (go run ./cmd/gen) -
                                          # a sibling of cmd/trustattic, not inside
                                          # internal/generator (already `package generator`)
  internal/
    client/client.gen.go               # generated: oapi-codegen typed client
    cli/
      config.go        config_test.go       # load/save config, env overrides
      transport.go      transport_test.go   # HTTP client, auth header injection
      output.go          output_test.go      # table/kv/json rendering
      errors.go           errors_test.go      # styled error rendering
      root.go                                 # Fang-wrapped root command
    commands/
      login.go           login_test.go       # trustattic login / logout
      use.go               use_test.go         # trustattic use
      generated/                              # generated: full Cobra command tree
        <tag>.gen.go       (one file per tag)
        register.gen.go                       # Register(root *cobra.Command, ...) wiring
    generator/
      spec.go            spec_test.go        # parse spec/api.yaml -> []Operation
      naming.go          naming_test.go      # Operation -> command path/verb/params
      flags.go            flags_test.go       # params/body -> Cobra flag/positional defs
      overrides.go     overrides_test.go   # operationId -> command name overrides
      overrides.yaml
      emit.go              emit_test.go        # builds Go source -> internal/commands/generated
  testing/
    container/containers.go                  # postgres + cloudapi + platform testcontainers
    integration/
      helper/jwt.go                           # RSA keypair + JWT minting (bootstrap only)
      main_test.go                            # TestMain: builds the trustattic binary
      suite.go                                # BaseTestSuite: containers + bootstrap
      project_test.go                         # list/create/list flow
```

---

### Task 1: Project scaffolding

**Files:**
- Create: `go.mod`
- Create: `cmd/trustattic/main.go`
- Create: `internal/cli/root.go`
- Test: `internal/cli/root_test.go`

**Interfaces:**
- Produces: `cli.NewRootCommand() *cobra.Command`, `cli.Execute() error` — used by `main.go` and by every later command-registration task.

- [ ] **Step 1: Initialize the module**

```bash
go mod init github.com/trustattic/trustattic-cli
go get github.com/spf13/cobra@latest
go get github.com/charmbracelet/fang@latest
```

- [ ] **Step 2: Write the failing test for the root command**

```go
// internal/cli/root_test.go
package cli_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/trustattic/trustattic-cli/internal/cli"
)

func TestNewRootCommand_HasNameAndOutputFlag(t *testing.T) {
	root := cli.NewRootCommand()
	require.Equal(t, "trustattic", root.Use)

	flag := root.PersistentFlags().Lookup("output")
	require.NotNil(t, flag, "expected a persistent --output flag")
}
```

Run `go get github.com/stretchr/testify@latest` first.

- [ ] **Step 2b: Run test to verify it fails**

Run: `go test ./internal/cli/... -run TestNewRootCommand -v`
Expected: FAIL (package `cli` / `NewRootCommand` does not exist yet)

- [ ] **Step 3: Implement the root command**

```go
// internal/cli/root.go
package cli

import (
	"context"

	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"
)

// NewRootCommand builds the trustattic root command. Resource subcommands are
// registered onto it by internal/commands (hand-written) and
// internal/commands/generated (generated) in later tasks.
func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:           "trustattic",
		Short:         "Command-line client for the TrustAttic Platform API",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().String("output", "", `output mode: "json", or empty for interactive (auto-detected)`)
	return root
}

// Execute runs the CLI, wrapping the root command with Fang's styled
// help/usage/error rendering.
func Execute() error {
	return fang.Execute(context.Background(), NewRootCommand())
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/cli/... -run TestNewRootCommand -v`
Expected: PASS

- [ ] **Step 5: Write main.go**

```go
// cmd/trustattic/main.go
package main

import (
	"os"

	"github.com/trustattic/trustattic-cli/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
```

- [ ] **Step 6: Build and smoke-test the binary**

Run: `go build -o /tmp/trustattic ./cmd/trustattic && /tmp/trustattic --help`
Expected: styled help output listing the `--output` flag, no error.

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum cmd internal/cli
git commit -m "Scaffold trustattic-cli module with a Fang-wrapped root command"
```

---

### Task 2: Vendor the spec and generate the typed API client

**Files:**
- Create: `spec/api.yaml`
- Create: `tools/tools.go`
- Create: `tools/client.yaml`
- Create: `Makefile` (first targets)
- Create: `internal/client/client.gen.go` (generated)
- Test: `internal/client/client_test.go`

**Interfaces:**
- Produces: package `github.com/trustattic/trustattic-cli/internal/client` with `NewClientWithResponses(server string, opts ...ClientOption) (*ClientWithResponses, error)` and one `<OperationID>WithResponse(...)` method per operation — consumed by the command generator (Task 12) and by integration tests (Task 16).

- [ ] **Step 1: Vendor the spec**

```bash
cp ../platform/api.yaml spec/api.yaml
```

- [ ] **Step 2: Add the oapi-codegen config and go:generate directive**

```yaml
# tools/client.yaml
package: client
generate:
  models: true
  client: true
output-options:
  response-type-suffix: Resp
```

```go
// tools/tools.go
//go:build tools

package tools

//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen --config=client.yaml -o ../internal/client/client.gen.go ../spec/api.yaml
```

- [ ] **Step 3: Add codegen dependencies and generate the client**

```bash
go get github.com/oapi-codegen/oapi-codegen/v2@v2.5.0
go get github.com/oapi-codegen/runtime@v1.1.2
cd tools && go generate -tags=tools ./... && cd ..
```

`tools.go` is gated by `//go:build tools` (a deliberate convention that
keeps codegen-only dependencies out of the main build) — plain
`go generate ./...` silently matches nothing under that tag, so the
`-tags=tools` flag is required every time this is invoked, including in the
Makefile below.

- [ ] **Step 4: Write a compile-level smoke test**

```go
// internal/client/client_test.go
package client_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/trustattic/trustattic-cli/internal/client"
)

func TestNewClientWithResponses_ConstructsAgainstAnyServer(t *testing.T) {
	c, err := client.NewClientWithResponses("http://example.invalid")
	require.NoError(t, err)
	require.NotNil(t, c)
}
```

- [ ] **Step 5: Run the test**

Run: `go test ./internal/client/... -v`
Expected: PASS. If `client.gen.go` failed to generate (missing `ProjectPostWithResponse` etc.), re-check `spec/api.yaml` was copied correctly and Step 3 ran without error.

- [ ] **Step 6: Start the Makefile**

```makefile
# Makefile
.PHONY: generate update-spec build test

update-spec:
	cp ../platform/api.yaml spec/api.yaml

generate:
	cd tools && go generate -tags=tools ./...

build:
	go build -o dist/trustattic ./cmd/trustattic

test:
	go test ./...
```

- [ ] **Step 7: Commit**

```bash
git add spec tools Makefile internal/client go.mod go.sum
git commit -m "Vendor platform OpenAPI spec and generate the typed API client"
```

---

### Task 3: Config file (load/save, current project, env overrides)

**Files:**
- Create: `internal/cli/config.go`
- Test: `internal/cli/config_test.go`

**Interfaces:**
- Produces: `cli.Config{Token, CurrentProject string}`, `cli.LoadConfig() (Config, error)`, `cli.SaveConfig(Config) error`, `cli.DefaultAPIURL string`, `cli.ResolveAPIURL() string`, `cli.ResolveToken(cfg Config) (string, error)`, `cli.ErrNotLoggedIn error` — consumed by Task 4 (transport), Task 7 (login/logout), Task 8 (use).

- [ ] **Step 1: Add the yaml dependency**

```bash
go get gopkg.in/yaml.v3@latest
```

- [ ] **Step 2: Write failing tests**

```go
// internal/cli/config_test.go
package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/trustattic/trustattic-cli/internal/cli"
)

func TestSaveConfig_ThenLoadConfig_RoundTrips(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	err := cli.SaveConfig(cli.Config{Token: "svc-abc", CurrentProject: "acme"})
	require.NoError(t, err)

	got, err := cli.LoadConfig()
	require.NoError(t, err)
	require.Equal(t, "svc-abc", got.Token)
	require.Equal(t, "acme", got.CurrentProject)
}

func TestSaveConfig_WritesWithOwnerOnlyPermissions(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	require.NoError(t, cli.SaveConfig(cli.Config{Token: "svc-abc"}))

	info, err := os.Stat(filepath.Join(dir, "trustattic", "config.yaml"))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestLoadConfig_MissingFile_ReturnsZeroValueNoError(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	got, err := cli.LoadConfig()
	require.NoError(t, err)
	require.Equal(t, cli.Config{}, got)
}

func TestResolveToken_PrefersEnvVarOverConfig(t *testing.T) {
	t.Setenv("TRUSTATTIC_TOKEN", "env-token")

	got, err := cli.ResolveToken(cli.Config{Token: "config-token"})
	require.NoError(t, err)
	require.Equal(t, "env-token", got)
}

func TestResolveToken_FallsBackToConfig(t *testing.T) {
	t.Setenv("TRUSTATTIC_TOKEN", "")

	got, err := cli.ResolveToken(cli.Config{Token: "config-token"})
	require.NoError(t, err)
	require.Equal(t, "config-token", got)
}

func TestResolveToken_ErrorsWhenNeitherIsSet(t *testing.T) {
	t.Setenv("TRUSTATTIC_TOKEN", "")

	_, err := cli.ResolveToken(cli.Config{})
	require.ErrorIs(t, err, cli.ErrNotLoggedIn)
}

func TestResolveAPIURL_DefaultsWhenEnvUnset(t *testing.T) {
	t.Setenv("TRUSTATTIC_API_URL", "")
	require.Equal(t, cli.DefaultAPIURL, cli.ResolveAPIURL())
}

func TestResolveAPIURL_PrefersEnvVar(t *testing.T) {
	t.Setenv("TRUSTATTIC_API_URL", "http://localhost:8080")
	require.Equal(t, "http://localhost:8080", cli.ResolveAPIURL())
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/cli/... -run TestSaveConfig -v`
Expected: FAIL (package doesn't compile — `Config`/`LoadConfig`/etc. don't exist)

- [ ] **Step 4: Implement config.go**

```go
// internal/cli/config.go
package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// DefaultAPIURL is used when TRUSTATTIC_API_URL is not set.
const DefaultAPIURL = "https://api.trustattic.com"

// ErrNotLoggedIn is returned by ResolveToken when no token is available from
// either TRUSTATTIC_TOKEN or the config file.
var ErrNotLoggedIn = errors.New("not logged in: run `trustattic login` or set TRUSTATTIC_TOKEN")

// Config is the on-disk shape of ~/.config/trustattic/config.yaml.
type Config struct {
	Token          string `yaml:"token,omitempty"`
	CurrentProject string `yaml:"current_project,omitempty"`
}

func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	return filepath.Join(dir, "trustattic", "config.yaml"), nil
}

// LoadConfig reads the config file. A missing file is not an error; it
// returns the zero-value Config instead.
func LoadConfig() (Config, error) {
	path, err := configPath()
	if err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}

// SaveConfig writes cfg to the config file, creating its parent directory as
// needed, with owner-only (0600) permissions.
func SaveConfig(cfg Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// ResolveToken returns the service-account token to authenticate with:
// TRUSTATTIC_TOKEN if set, else cfg.Token, else ErrNotLoggedIn.
func ResolveToken(cfg Config) (string, error) {
	if v := os.Getenv("TRUSTATTIC_TOKEN"); v != "" {
		return v, nil
	}
	if cfg.Token != "" {
		return cfg.Token, nil
	}
	return "", ErrNotLoggedIn
}

// ResolveAPIURL returns TRUSTATTIC_API_URL if set, else DefaultAPIURL.
func ResolveAPIURL() string {
	if v := os.Getenv("TRUSTATTIC_API_URL"); v != "" {
		return v
	}
	return DefaultAPIURL
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/cli/... -run 'TestSaveConfig|TestLoadConfig|TestResolveToken|TestResolveAPIURL' -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/cli/config.go internal/cli/config_test.go go.mod go.sum
git commit -m "Add config file load/save and token/API URL resolution"
```

---

### Task 4: HTTP transport (auth header injection, client construction)

**Files:**
- Create: `internal/cli/transport.go`
- Test: `internal/cli/transport_test.go`

**Interfaces:**
- Consumes: `cli.ResolveToken`, `cli.ResolveAPIURL` (Task 3); `client.NewClientWithResponses`, `client.WithHTTPClient`, `client.WithRequestEditorFn` (Task 2, generated).
- Produces: `cli.NewAPIClient(cfg Config) (*client.ClientWithResponses, error)` — consumed by every generated command (Task 12) and hand-written commands that call the API.

- [ ] **Step 1: Write the failing test**

```go
// internal/cli/transport_test.go
package cli_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/trustattic/trustattic-cli/internal/cli"
)

func TestNewAPIClient_SendsAuthTokenHeader(t *testing.T) {
	var gotHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("X-Auth-Token")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	t.Setenv("TRUSTATTIC_API_URL", srv.URL)
	t.Setenv("TRUSTATTIC_TOKEN", "svc-abc123")

	c, err := cli.NewAPIClient(cli.Config{})
	require.NoError(t, err)

	_, err = c.CommonHealthcheckWithResponse(context.Background())
	require.NoError(t, err)
	require.Equal(t, "svc-abc123", gotHeader)
}
```

`CommonHealthcheckWithResponse` is the client method Task 2 generates for
`GET /healthcheck` (tag `common`, per the operation table in the design
spec) — if `go doc ./internal/client` shows a different name for it, use
that name here instead; nothing else in this test depends on which endpoint
is called, only that some generated method reaches this server.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/... -run TestNewAPIClient -v`
Expected: FAIL (`NewAPIClient` undefined)

- [ ] **Step 3: Implement transport.go**

```go
// internal/cli/transport.go
package cli

import (
	"context"
	"net/http"
	"time"

	"github.com/trustattic/trustattic-cli/internal/client"
)

// NewAPIClient builds a typed API client pointed at the resolved base URL,
// injecting the resolved service-account token as X-Auth-Token on every
// request.
func NewAPIClient(cfg Config) (*client.ClientWithResponses, error) {
	token, err := ResolveToken(cfg)
	if err != nil {
		return nil, err
	}
	baseURL := ResolveAPIURL()

	httpClient := &http.Client{Timeout: 30 * time.Second}

	return client.NewClientWithResponses(
		baseURL,
		client.WithHTTPClient(httpClient),
		client.WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
			req.Header.Set("X-Auth-Token", token)
			return nil
		}),
	)
}
```

Note: the exact `client.With...` option names come from the file generated in Task 2 — if `go build` reports different names, run `go doc ./internal/client` to see the real generated option constructors and adjust this file.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/cli/... -run TestNewAPIClient -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cli/transport.go internal/cli/transport_test.go
git commit -m "Wire the typed API client to inject X-Auth-Token from resolved config"
```

---

### Task 5: Output formatting (table / key-value / JSON)

**Files:**
- Create: `internal/cli/output.go`
- Test: `internal/cli/output_test.go`

**Interfaces:**
- Produces: `cli.Mode` (`ModeAuto`, `ModeJSON`), `cli.Render(w io.Writer, mode Mode, isTTY bool, body []byte) error` — consumed by every generated command (Task 12) to print API responses.

- [ ] **Step 1: Add the table dependency**

```bash
go get github.com/charmbracelet/lipgloss@latest
go get github.com/charmbracelet/lipgloss/table@latest
```

- [ ] **Step 2: Write failing tests**

```go
// internal/cli/output_test.go
package cli_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/trustattic/trustattic-cli/internal/cli"
)

func TestRender_JSONMode_PrintsIndentedJSON(t *testing.T) {
	var buf bytes.Buffer
	err := cli.Render(&buf, cli.ModeJSON, true, []byte(`{"id":"1","name":"acme"}`))
	require.NoError(t, err)
	require.Contains(t, buf.String(), "\"name\": \"acme\"")
}

func TestRender_NonTTY_PrintsJSONEvenInAutoMode(t *testing.T) {
	var buf bytes.Buffer
	err := cli.Render(&buf, cli.ModeAuto, false, []byte(`{"id":"1"}`))
	require.NoError(t, err)
	require.Contains(t, buf.String(), "\"id\": \"1\"")
}

func TestRender_TTY_ArrayUnderData_RendersAsTable(t *testing.T) {
	var buf bytes.Buffer
	body := []byte(`{"data":[{"id":"1","name":"acme"},{"id":"2","name":"globex"}]}`)
	err := cli.Render(&buf, cli.ModeAuto, true, body)
	require.NoError(t, err)
	out := buf.String()
	require.Contains(t, out, "acme")
	require.Contains(t, out, "globex")
	require.Contains(t, out, "id")
	require.Contains(t, out, "name")
}

func TestRender_TTY_EmptyDataArray_PrintsNoResults(t *testing.T) {
	var buf bytes.Buffer
	err := cli.Render(&buf, cli.ModeAuto, true, []byte(`{"data":[]}`))
	require.NoError(t, err)
	require.Contains(t, buf.String(), "no results")
}

func TestRender_TTY_PlainObject_RendersAsKeyValue(t *testing.T) {
	var buf bytes.Buffer
	err := cli.Render(&buf, cli.ModeAuto, true, []byte(`{"id":"1","name":"acme"}`))
	require.NoError(t, err)
	out := buf.String()
	require.Contains(t, out, "id")
	require.Contains(t, out, "1")
	require.Contains(t, out, "name")
	require.Contains(t, out, "acme")
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/cli/... -run TestRender -v`
Expected: FAIL (`Render`/`Mode`/`ModeJSON`/`ModeAuto` undefined)

- [ ] **Step 4: Implement output.go**

```go
// internal/cli/output.go
package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

// Mode selects how a response body is rendered.
type Mode int

const (
	// ModeAuto renders styled tables/key-value views on a TTY, and falls
	// back to raw JSON otherwise.
	ModeAuto Mode = iota
	// ModeJSON always renders raw indented JSON.
	ModeJSON
)

// Render writes body (a raw JSON API response) to w, choosing table,
// key-value, or JSON rendering generically from the response's shape - never
// from which endpoint produced it.
func Render(w io.Writer, mode Mode, isTTY bool, body []byte) error {
	if mode == ModeJSON || !isTTY {
		var buf bytes.Buffer
		if err := json.Indent(&buf, body, "", "  "); err != nil {
			return fmt.Errorf("indent json: %w", err)
		}
		buf.WriteByte('\n')
		_, err := w.Write(buf.Bytes())
		return err
	}

	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		return fmt.Errorf("parse json: %w", err)
	}

	if obj, ok := v.(map[string]any); ok {
		if data, ok := obj["data"]; ok {
			if rows, ok := data.([]any); ok {
				return renderTable(w, rows)
			}
		}
		return renderKV(w, obj)
	}
	if rows, ok := v.([]any); ok {
		return renderTable(w, rows)
	}
	_, err := fmt.Fprintln(w, string(body))
	return err
}

func renderTable(w io.Writer, rows []any) error {
	if len(rows) == 0 {
		_, err := fmt.Fprintln(w, "(no results)")
		return err
	}

	seen := map[string]bool{}
	var cols []string
	for _, r := range rows {
		obj, ok := r.(map[string]any)
		if !ok {
			continue
		}
		for k := range obj {
			if !seen[k] {
				seen[k] = true
				cols = append(cols, k)
			}
		}
	}
	sort.Strings(cols)

	headerStyle := lipgloss.NewStyle().Bold(true)
	t := table.New().
		Headers(cols...).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			return lipgloss.NewStyle().PaddingLeft(1).PaddingRight(1)
		})

	for _, r := range rows {
		obj, _ := r.(map[string]any)
		row := make([]string, len(cols))
		for i, c := range cols {
			row[i] = fmt.Sprint(obj[c])
		}
		t.Row(row...)
	}
	_, err := fmt.Fprintln(w, t.Render())
	return err
}

func renderKV(w io.Writer, obj map[string]any) error {
	var keys []string
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	keyStyle := lipgloss.NewStyle().Bold(true)
	for _, k := range keys {
		if _, err := fmt.Fprintf(w, "%s: %v\n", keyStyle.Render(k), obj[k]); err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/cli/... -run TestRender -v`
Expected: PASS. If `lipgloss/table` isn't a separate importable package in the resolved version, check `go doc github.com/charmbracelet/lipgloss` — some versions ship the table helper under `github.com/charmbracelet/lipgloss/table` as a submodule with its own go.mod; run `go get` from that path directly if the first `go get` failed.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/output.go internal/cli/output_test.go go.mod go.sum
git commit -m "Add generic table/key-value/JSON response rendering"
```

---

### Task 6: Styled error rendering

**Files:**
- Create: `internal/cli/errors.go`
- Test: `internal/cli/errors_test.go`

**Interfaces:**
- Produces: `cli.APIError{StatusCode int, Body []byte}` (implements `error`), `cli.RenderError(w io.Writer, err error)` — consumed by generated commands (Task 12) when a response status is >= 400.

- [ ] **Step 1: Write failing tests**

```go
// internal/cli/errors_test.go
package cli_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/trustattic/trustattic-cli/internal/cli"
)

func TestAPIError_ErrorIncludesStatusAndBody(t *testing.T) {
	err := &cli.APIError{StatusCode: 404, Body: []byte(`{"message":"not found"}`)}
	require.Contains(t, err.Error(), "404")
	require.Contains(t, err.Error(), "not found")
}

func TestRenderError_WritesMessage(t *testing.T) {
	var buf bytes.Buffer
	cli.RenderError(&buf, errors.New("boom"))
	require.Contains(t, buf.String(), "boom")
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/cli/... -run 'TestAPIError|TestRenderError' -v`
Expected: FAIL

- [ ] **Step 3: Implement errors.go**

```go
// internal/cli/errors.go
package cli

import (
	"fmt"
	"io"

	"github.com/charmbracelet/lipgloss"
)

// APIError wraps a non-2xx API response so it can be rendered consistently.
type APIError struct {
	StatusCode int
	Body       []byte
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error %d: %s", e.StatusCode, string(e.Body))
}

// RenderError writes a styled error box for err to w.
func RenderError(w io.Writer, err error) {
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("9")).
		Padding(0, 1)
	fmt.Fprintln(w, box.Render("Error: "+err.Error()))
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/cli/... -run 'TestAPIError|TestRenderError' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cli/errors.go internal/cli/errors_test.go
git commit -m "Add styled API error type and renderer"
```

---

### Task 7: `login` / `logout` commands

**Files:**
- Create: `internal/commands/login.go`
- Test: `internal/commands/login_test.go`

**Interfaces:**
- Consumes: `cli.LoadConfig`, `cli.SaveConfig`, `cli.Config` (Task 3).
- Produces: `commands.NewLoginCommand() *cobra.Command`, `commands.NewLogoutCommand() *cobra.Command` — consumed by Task 9 (root wiring).

- [ ] **Step 1: Add the huh dependency**

```bash
go get github.com/charmbracelet/huh@latest
```

- [ ] **Step 2: Write failing tests**

```go
// internal/commands/login_test.go
package commands_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/trustattic/trustattic-cli/internal/cli"
	"github.com/trustattic/trustattic-cli/internal/commands"
)

func TestLoginCommand_WithTokenFlag_StoresToken(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	cmd := commands.NewLoginCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"--token", "svc-abc123"})
	require.NoError(t, cmd.Execute())

	cfg, err := cli.LoadConfig()
	require.NoError(t, err)
	require.Equal(t, "svc-abc123", cfg.Token)
}

func TestLoginCommand_WithoutToken_ErrorsInNonInteractiveContext(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	cmd := commands.NewLoginCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	require.Error(t, err)
}

func TestLogoutCommand_ClearsToken(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	require.NoError(t, cli.SaveConfig(cli.Config{Token: "svc-abc123"}))

	cmd := commands.NewLogoutCommand()
	cmd.SetOut(&bytes.Buffer{})
	require.NoError(t, cmd.Execute())

	cfg, err := cli.LoadConfig()
	require.NoError(t, err)
	require.Empty(t, cfg.Token)
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/commands/... -run 'TestLoginCommand|TestLogoutCommand' -v`
Expected: FAIL (package `commands` doesn't exist yet)

- [ ] **Step 4: Implement login.go**

The `huh` prompt only runs when `--token` is omitted **and** stdin looks
interactive; the test above passes no `--token` and runs under `go test`
(non-interactive stdin), so it must error rather than hang waiting on a
prompt.

```go
// internal/commands/login.go
package commands

import (
	"errors"
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"github.com/trustattic/trustattic-cli/internal/cli"
	"golang.org/x/term"
)

// NewLoginCommand builds `trustattic login`, which stores a service-account
// token in the config file. This is the only supported auth path: the CLI
// always authenticates as a service account, never a user via JWT.
func NewLoginCommand() *cobra.Command {
	var token string
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Store a TrustAttic service-account token",
		RunE: func(cmd *cobra.Command, args []string) error {
			if token == "" {
				if !term.IsTerminal(int(os.Stdin.Fd())) {
					return errors.New("a token is required: pass --token, or run interactively")
				}
				form := huh.NewForm(huh.NewGroup(
					huh.NewInput().
						Title("Service-account token").
						Password(true).
						Value(&token),
				))
				if err := form.Run(); err != nil {
					return err
				}
			}
			if token == "" {
				return errors.New("a token is required")
			}

			cfg, err := cli.LoadConfig()
			if err != nil {
				return err
			}
			cfg.Token = token
			if err := cli.SaveConfig(cfg); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Logged in.")
			return nil
		},
	}
	cmd.Flags().StringVar(&token, "token", "", "service-account token")
	return cmd
}

// NewLogoutCommand builds `trustattic logout`, which removes the stored
// token.
func NewLogoutCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove the stored service-account token",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := cli.LoadConfig()
			if err != nil {
				return err
			}
			cfg.Token = ""
			if err := cli.SaveConfig(cfg); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Logged out.")
			return nil
		},
	}
}
```

```bash
go get golang.org/x/term@latest
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/commands/... -run 'TestLoginCommand|TestLogoutCommand' -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/commands/login.go internal/commands/login_test.go go.mod go.sum
git commit -m "Add login/logout commands storing a service-account token"
```

---

### Task 8: `use` command (current project)

**Files:**
- Create: `internal/commands/use.go`
- Test: `internal/commands/use_test.go`

**Interfaces:**
- Consumes: `cli.LoadConfig`, `cli.SaveConfig`, `cli.Config` (Task 3).
- Produces: `commands.NewUseCommand() *cobra.Command` — consumed by Task 9 (root wiring) and by Task 14's shared `--project` fallback `PreRunE`, which reads `cli.LoadConfig().CurrentProject`.

- [ ] **Step 1: Write failing tests**

```go
// internal/commands/use_test.go
package commands_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/trustattic/trustattic-cli/internal/cli"
	"github.com/trustattic/trustattic-cli/internal/commands"
)

func TestUseCommand_WithArg_SetsCurrentProject(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	cmd := commands.NewUseCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"acme"})
	require.NoError(t, cmd.Execute())

	cfg, err := cli.LoadConfig()
	require.NoError(t, err)
	require.Equal(t, "acme", cfg.CurrentProject)
}

func TestUseCommand_NoArgs_PrintsCurrentProject(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	require.NoError(t, cli.SaveConfig(cli.Config{CurrentProject: "acme"}))

	var out bytes.Buffer
	cmd := commands.NewUseCommand()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{})
	require.NoError(t, cmd.Execute())
	require.Contains(t, out.String(), "acme")
}

func TestUseCommand_Clear_UnsetsCurrentProject(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	require.NoError(t, cli.SaveConfig(cli.Config{CurrentProject: "acme"}))

	cmd := commands.NewUseCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"--clear"})
	require.NoError(t, cmd.Execute())

	cfg, err := cli.LoadConfig()
	require.NoError(t, err)
	require.Empty(t, cfg.CurrentProject)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/commands/... -run TestUseCommand -v`
Expected: FAIL (`NewUseCommand` undefined)

- [ ] **Step 3: Implement use.go**

```go
// internal/commands/use.go
package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/trustattic/trustattic-cli/internal/cli"
)

// NewUseCommand builds `trustattic use`, which gets or sets the current
// project stored in the config file. Generated commands whose --project flag
// is bound to the project_slug path parameter fall back to this value when
// --project isn't passed explicitly (see internal/commands/generated).
func NewUseCommand() *cobra.Command {
	var clear bool
	cmd := &cobra.Command{
		Use:   "use [project]",
		Short: "Get or set the current project",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := cli.LoadConfig()
			if err != nil {
				return err
			}
			switch {
			case clear:
				cfg.CurrentProject = ""
				if err := cli.SaveConfig(cfg); err != nil {
					return err
				}
				fmt.Fprintln(cmd.OutOrStdout(), "Current project cleared.")
			case len(args) == 1:
				cfg.CurrentProject = args[0]
				if err := cli.SaveConfig(cfg); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Current project set to %q.\n", args[0])
			default:
				if cfg.CurrentProject == "" {
					fmt.Fprintln(cmd.OutOrStdout(), "No current project set.")
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), cfg.CurrentProject)
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&clear, "clear", false, "unset the current project")
	return cmd
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/commands/... -run TestUseCommand -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/commands/use.go internal/commands/use_test.go
git commit -m "Add 'trustattic use' for a stored current project"
```

---

### Task 9: Wire hand-written commands into the root command

**Files:**
- Modify: `internal/cli/root.go`
- Test: `internal/cli/root_test.go`

**Interfaces:**
- Consumes: `commands.NewLoginCommand`, `commands.NewLogoutCommand` (Task 7), `commands.NewUseCommand` (Task 8).
- Produces: `NewRootCommand()` now returns a command whose children include `login`, `logout`, `use` — the shape that Task 14 later adds the generated resource commands onto.

- [ ] **Step 1: Write the failing test**

```go
// internal/cli/root_test.go — add to the existing file
func TestNewRootCommand_RegistersAuthAndUseCommands(t *testing.T) {
	root := cli.NewRootCommand()
	names := map[string]bool{}
	for _, c := range root.Commands() {
		names[c.Name()] = true
	}
	require.True(t, names["login"])
	require.True(t, names["logout"])
	require.True(t, names["use"])
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/... -run TestNewRootCommand_RegistersAuthAndUseCommands -v`
Expected: FAIL (root has no subcommands yet)

- [ ] **Step 3: Wire the commands in**

`internal/cli` cannot import `internal/commands` directly if `internal/commands`
ever needs to import `internal/cli` (it does, for `Config`/`LoadConfig`) —
that would be an import cycle. Move command registration to a small
constructor in `internal/commands` instead, and have `main.go` compose the
two:

```go
// internal/commands/root.go
package commands

import "github.com/spf13/cobra"

// Register attaches every hand-written command onto root.
func Register(root *cobra.Command) {
	root.AddCommand(NewLoginCommand())
	root.AddCommand(NewLogoutCommand())
	root.AddCommand(NewUseCommand())
}
```

Move the new test to where it can call both packages without a cycle:

```go
// internal/commands/root_test.go
package commands_test

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"github.com/trustattic/trustattic-cli/internal/commands"
)

func TestRegister_AddsAuthAndUseCommands(t *testing.T) {
	root := &cobra.Command{Use: "trustattic"}
	commands.Register(root)

	names := map[string]bool{}
	for _, c := range root.Commands() {
		names[c.Name()] = true
	}
	require.True(t, names["login"])
	require.True(t, names["logout"])
	require.True(t, names["use"])
}
```

Delete the `TestNewRootCommand_RegistersAuthAndUseCommands` test added in Step
1 from `internal/cli/root_test.go` — `internal/cli` stays command-agnostic.

Update `main.go` to compose root + registration:

```go
// cmd/trustattic/main.go
package main

import (
	"os"

	"github.com/charmbracelet/fang"
	"github.com/trustattic/trustattic-cli/internal/cli"
	"github.com/trustattic/trustattic-cli/internal/commands"
)

func main() {
	root := cli.NewRootCommand()
	commands.Register(root)
	if err := fang.Execute(context.Background(), root); err != nil {
		os.Exit(1)
	}
}
```

Since `Execute()` in `internal/cli/root.go` no longer has anywhere to attach
commands, remove it (only `NewRootCommand` remains there) and add the
`context` import to `main.go`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/commands/... -run TestRegister_AddsAuthAndUseCommands -v`
Expected: PASS

- [ ] **Step 5: Build and smoke-test**

Run: `go build -o /tmp/trustattic ./cmd/trustattic && /tmp/trustattic --help`
Expected: help output lists `login`, `logout`, `use` among the available commands.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/root.go internal/cli/root_test.go internal/commands/root.go internal/commands/root_test.go cmd/trustattic/main.go
git commit -m "Wire login/logout/use into the root command via commands.Register"
```

---

### Task 10: Generator — parse the spec into operations

**Files:**
- Create: `internal/generator/spec.go`
- Test: `internal/generator/spec_test.go`
- Test fixture: `internal/generator/testdata/fixture.yaml`

**Interfaces:**
- Produces: `generator.Operation{Tag, OperationID, Method, Path string, Params []Param, HasBody bool, BodyProps []BodyProp}`, `generator.Param{Name string}`, `generator.BodyProp{Name string, Required bool, Type string}`, `generator.LoadOperations(path string) ([]Operation, error)` — consumed by Task 11 (naming) and Task 12 (flags).

- [ ] **Step 1: Add the kin-openapi dependency**

```bash
go get github.com/getkin/kin-openapi@v0.132.0
```

- [ ] **Step 2: Write a small fixture spec**

```yaml
# internal/generator/testdata/fixture.yaml
openapi: 3.0.3
info:
  title: Fixture API
  version: v1
paths:
  /widget:
    get:
      operationId: WidgetGet
      tags: [widget]
      responses:
        "200":
          description: ok
    post:
      operationId: WidgetPost
      tags: [widget]
      requestBody:
        content:
          application/json:
            schema:
              type: object
              required: [name]
              properties:
                name:
                  type: string
                active:
                  type: boolean
      responses:
        "201":
          description: created
  /widget/{widget_id}:
    delete:
      operationId: WidgetDelete
      tags: [widget]
      parameters:
        - name: widget_id
          in: path
          required: true
          schema:
            type: string
      responses:
        "204":
          description: no content
```

- [ ] **Step 3: Write failing tests**

```go
// internal/generator/spec_test.go
package generator_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/trustattic/trustattic-cli/internal/generator"
)

func TestLoadOperations_FixtureSpec_ReturnsAllThreeOperations(t *testing.T) {
	ops, err := generator.LoadOperations("testdata/fixture.yaml")
	require.NoError(t, err)
	require.Len(t, ops, 3)
}

func TestLoadOperations_CapturesTagMethodAndPath(t *testing.T) {
	ops, err := generator.LoadOperations("testdata/fixture.yaml")
	require.NoError(t, err)

	byID := map[string]generator.Operation{}
	for _, op := range ops {
		byID[op.OperationID] = op
	}

	get := byID["WidgetGet"]
	require.Equal(t, "widget", get.Tag)
	require.Equal(t, "GET", get.Method)
	require.Equal(t, "/widget", get.Path)
	require.False(t, get.HasBody)
}

func TestLoadOperations_CapturesPathParams(t *testing.T) {
	ops, err := generator.LoadOperations("testdata/fixture.yaml")
	require.NoError(t, err)

	byID := map[string]generator.Operation{}
	for _, op := range ops {
		byID[op.OperationID] = op
	}

	del := byID["WidgetDelete"]
	require.Len(t, del.Params, 1)
	require.Equal(t, "widget_id", del.Params[0].Name)
}

func TestLoadOperations_CapturesRequestBodyProperties(t *testing.T) {
	ops, err := generator.LoadOperations("testdata/fixture.yaml")
	require.NoError(t, err)

	byID := map[string]generator.Operation{}
	for _, op := range ops {
		byID[op.OperationID] = op
	}

	post := byID["WidgetPost"]
	require.True(t, post.HasBody)
	require.Len(t, post.BodyProps, 2)

	byName := map[string]generator.BodyProp{}
	for _, p := range post.BodyProps {
		byName[p.Name] = p
	}
	require.Equal(t, "string", byName["name"].Type)
	require.True(t, byName["name"].Required)
	require.Equal(t, "boolean", byName["active"].Type)
	require.False(t, byName["active"].Required)
}

func TestLoadOperations_RealPlatformSpec_Has32Operations(t *testing.T) {
	ops, err := generator.LoadOperations("../../spec/api.yaml")
	require.NoError(t, err)
	require.Len(t, ops, 32)
}
```

- [ ] **Step 4: Run tests to verify they fail**

Run: `go test ./internal/generator/... -v`
Expected: FAIL (package `generator` doesn't exist yet)

- [ ] **Step 5: Implement spec.go**

```go
// internal/generator/spec.go
package generator

import (
	"fmt"
	"sort"

	"github.com/getkin/kin-openapi/openapi3"
)

// Param is a path parameter used by an operation.
type Param struct {
	Name string
}

// BodyProp is one top-level property of an operation's JSON request body.
type BodyProp struct {
	Name     string
	Required bool
	Type     string // "string", "boolean", "integer", "number", ...
}

// Operation is a flattened, generator-friendly view of one OpenAPI operation.
type Operation struct {
	Tag         string
	OperationID string
	Method      string // upper-case, e.g. "GET"
	Path        string
	Params      []Param // path parameters, in path order
	HasBody     bool
	BodyProps   []BodyProp
}

// LoadOperations parses the OpenAPI document at path and returns every
// operation with a non-empty operationId, in a stable, deterministic order
// (by path, then method).
func LoadOperations(path string) ([]Operation, error) {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile(path)
	if err != nil {
		return nil, fmt.Errorf("load spec: %w", err)
	}

	var ops []Operation
	for p, item := range doc.Paths.Map() {
		for method, op := range item.Operations() {
			if op.OperationID == "" {
				continue
			}
			o := Operation{
				Method: method,
				Path:   p,
			}
			if len(op.Tags) > 0 {
				o.Tag = op.Tags[0]
			}
			o.OperationID = op.OperationID

			for _, ref := range op.Parameters {
				if ref.Value != nil && ref.Value.In == "path" {
					o.Params = append(o.Params, Param{Name: ref.Value.Name})
				}
			}

			if op.RequestBody != nil && op.RequestBody.Value != nil {
				mt := op.RequestBody.Value.Content.Get("application/json")
				if mt != nil && mt.Schema != nil && mt.Schema.Value != nil {
					o.HasBody = true
					required := map[string]bool{}
					for _, name := range mt.Schema.Value.Required {
						required[name] = true
					}
					for name, propRef := range mt.Schema.Value.Properties {
						bp := BodyProp{Name: name, Required: required[name]}
						if propRef.Value != nil && propRef.Value.Type != nil && len(*propRef.Value.Type) > 0 {
							bp.Type = (*propRef.Value.Type)[0]
						}
						o.BodyProps = append(o.BodyProps, bp)
					}
					sort.Slice(o.BodyProps, func(i, j int) bool {
						return o.BodyProps[i].Name < o.BodyProps[j].Name
					})
				}
			}

			ops = append(ops, o)
		}
	}

	sort.Slice(ops, func(i, j int) bool {
		if ops[i].Path != ops[j].Path {
			return ops[i].Path < ops[j].Path
		}
		return ops[i].Method < ops[j].Method
	})
	return ops, nil
}
```

`kin-openapi` v0.132.0's `Schema.Type` field is `*openapi3.Types` (a pointer
to a `[]string`); the dereference-and-index above (`(*propRef.Value.Type)[0]`)
matches that shape. If `go build` reports a different type for `.Type` (kin-
openapi has changed this across versions before), run
`go doc github.com/getkin/kin-openapi/openapi3 Schema` and adjust the one line
that reads it — nothing else in this task depends on the exact shape.

Also note `doc.Validate` is deliberately *not* called here: the vendored spec
is `platform`'s own merged spec, already validated by its own build; skipping
it keeps this loader fast and avoids a second, redundant validation pass.

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/generator/... -v`
Expected: PASS. `TestLoadOperations_RealPlatformSpec_Has32Operations` is the
one that exercises the vendored spec end-to-end — if the count differs from
32, recount with `grep -c operationId: spec/api.yaml` and fix the test's
expected count (the spec may have changed since this plan was written), not
the loader.

- [ ] **Step 7: Commit**

```bash
git add internal/generator/spec.go internal/generator/spec_test.go internal/generator/testdata go.mod go.sum
git commit -m "Add generator spec loader (OpenAPI -> []Operation)"
```

---

### Task 11: Generator — naming algorithm

**Files:**
- Create: `internal/generator/naming.go`
- Test: `internal/generator/naming_test.go`

**Interfaces:**
- Consumes: `generator.Operation`, `generator.Param` (Task 10).
- Produces: `generator.CommandSpec{Tag, Group []string, Verb string, Positional *Param, Flags []Param, Operation Operation}`, `generator.BuildCommandSpecs(ops []Operation) []CommandSpec` — consumed by Task 12 (flags/overrides) and Task 13 (emit).

This is the algorithm documented in the spec's "Command generation algorithm"
section — verify every test below against that section if anything here
seems to disagree with it.

- [ ] **Step 1: Write failing tests reproducing the spec's worked examples**

```go
// internal/generator/naming_test.go
package generator_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/trustattic/trustattic-cli/internal/generator"
)

func op(tag, id, method, path string, params ...string) generator.Operation {
	o := generator.Operation{Tag: tag, OperationID: id, Method: method, Path: path}
	for _, p := range params {
		o.Params = append(o.Params, generator.Param{Name: p})
	}
	return o
}

func specByID(t *testing.T, specs []generator.CommandSpec, id string) generator.CommandSpec {
	t.Helper()
	for _, s := range specs {
		if s.Operation.OperationID == id {
			return s
		}
	}
	t.Fatalf("no CommandSpec for operationId %q", id)
	return generator.CommandSpec{}
}

func TestBuildCommandSpecs_AccountCRUDGroup_SharesEmptyRemainder(t *testing.T) {
	ops := []generator.Operation{
		op("account", "AccountGet", "GET", "/account"),
		op("account", "AccountPost", "POST", "/account"),
		op("account", "AccountPut", "PUT", "/account/{account_id}", "account_id"),
	}
	specs := generator.BuildCommandSpecs(ops)

	list := specByID(t, specs, "AccountGet")
	require.Equal(t, "account", list.Tag)
	require.Empty(t, list.Group)
	require.Equal(t, "list", list.Verb)
	require.Nil(t, list.Positional)

	create := specByID(t, specs, "AccountPost")
	require.Equal(t, "create", create.Verb)

	update := specByID(t, specs, "AccountPut")
	require.Equal(t, "update", update.Verb)
	require.NotNil(t, update.Positional)
	require.Equal(t, "account_id", update.Positional.Name)
}

func TestBuildCommandSpecs_SingleOpWithVerbLikeRemainder_BecomesLeaf(t *testing.T) {
	ops := []generator.Operation{
		op("account", "AccountPutObtain", "PUT", "/account"),
	}
	specs := generator.BuildCommandSpecs(ops)

	obtain := specByID(t, specs, "AccountPutObtain")
	require.Empty(t, obtain.Group)
	require.Equal(t, "obtain", obtain.Verb)
}

func TestBuildCommandSpecs_ParamTokenInOperationID_IsStripped(t *testing.T) {
	// ProjectSlugGet's "Slug" word is a naming artifact of the project_slug
	// path parameter, not a real subgroup - it must land in the same group
	// as ProjectGet/ProjectPost/ProjectPut, not a spurious "project slug" one.
	ops := []generator.Operation{
		op("project", "ProjectGet", "GET", "/project"),
		op("project", "ProjectPost", "POST", "/project"),
		op("project", "ProjectSlugGet", "GET", "/project/{project_slug}", "project_slug"),
		op("project", "ProjectPut", "PUT", "/project/{project_slug}", "project_slug"),
	}
	specs := generator.BuildCommandSpecs(ops)

	get := specByID(t, specs, "ProjectSlugGet")
	require.Empty(t, get.Group)
	require.Equal(t, "get", get.Verb)
	require.NotNil(t, get.Positional)
	require.Equal(t, "project_slug", get.Positional.Name)
}

func TestBuildCommandSpecs_TwoOpsSharingAnActionWord_NestAsAGroup(t *testing.T) {
	ops := []generator.Operation{
		op("backup", "BackupRestorePut", "POST", "/project/{project_slug}/backup/{backup_id}/restore", "project_slug", "backup_id"),
		op("backup", "BackupRestoreGet", "GET", "/project/{project_slug}/backup/{backup_id}/restore/{restore_id}", "project_slug", "backup_id", "restore_id"),
	}
	specs := generator.BuildCommandSpecs(ops)

	// BackupRestorePut's URL ends in the static "restore" segment, not a
	// path param, so it gets no positional arg - both path params are
	// flags. Only BackupRestoreGet's URL ends in {restore_id}.
	create := specByID(t, specs, "BackupRestorePut")
	require.Equal(t, []string{"restore"}, create.Group)
	require.Equal(t, "create", create.Verb)
	require.Nil(t, create.Positional)
	require.Len(t, create.Flags, 2)
	require.Equal(t, "project_slug", create.Flags[0].Name)
	require.Equal(t, "backup_id", create.Flags[1].Name)

	get := specByID(t, specs, "BackupRestoreGet")
	require.Equal(t, []string{"restore"}, get.Group)
	require.Equal(t, "get", get.Verb)
	require.NotNil(t, get.Positional)
	require.Equal(t, "restore_id", get.Positional.Name)
	require.Len(t, get.Flags, 2)
	require.Equal(t, "project_slug", get.Flags[0].Name)
	require.Equal(t, "backup_id", get.Flags[1].Name)
}

func TestBuildCommandSpecs_SingleOpEmptyRemainder_UsesMechanicalVerb(t *testing.T) {
	ops := []generator.Operation{
		op("resource", "ResourceGet", "GET", "/project/{project_slug}/resource", "project_slug"),
	}
	specs := generator.BuildCommandSpecs(ops)

	// The URL ends in the static "resource" segment, not {project_slug}, so
	// there's no positional arg - project_slug is a flag, like every other
	// plain "list" command.
	list := specByID(t, specs, "ResourceGet")
	require.Empty(t, list.Group)
	require.Equal(t, "list", list.Verb)
	require.Nil(t, list.Positional)
	require.Len(t, list.Flags, 1)
	require.Equal(t, "project_slug", list.Flags[0].Name)
}

func TestBuildCommandSpecs_ActionOnAStaticTailURL_HasNoPositional(t *testing.T) {
	ops := []generator.Operation{
		op("connection", "ConnectionCheck", "GET", "/project/{project_slug}/connection/{connection_id}/check", "project_slug", "connection_id"),
	}
	specs := generator.BuildCommandSpecs(ops)

	// The URL ends in the static "check" segment, not {connection_id} - by
	// the URL-shape rule that means no positional arg here, even though
	// connection_id is this command's obvious "target". Both path params
	// become flags, in path order.
	check := specByID(t, specs, "ConnectionCheck")
	require.Empty(t, check.Group)
	require.Equal(t, "check", check.Verb)
	require.Nil(t, check.Positional)
	require.Len(t, check.Flags, 2)
	require.Equal(t, "project_slug", check.Flags[0].Name)
	require.Equal(t, "connection_id", check.Flags[1].Name)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/generator/... -run TestBuildCommandSpecs -v`
Expected: FAIL (`CommandSpec`/`BuildCommandSpecs` undefined)

- [ ] **Step 3: Implement naming.go**

```go
// internal/generator/naming.go
package generator

import (
	"strings"
	"unicode"
)

// CommandSpec is the fully-resolved shape of one generated command: which
// top-level tag it hangs off, an optional nested group, its leaf verb, and
// how its path parameters split into a trailing positional argument plus
// leading flags.
type CommandSpec struct {
	Tag        string
	Group      []string // kebab-case words, nested subcommand path; empty for a direct leaf
	Verb       string   // kebab-case leaf command name
	Positional *Param   // the last path parameter, if any
	Flags      []Param  // every other path parameter, in path order
	Operation  Operation
}

var httpMethodWords = map[string]bool{
	"Get": true, "Post": true, "Put": true, "Delete": true, "Patch": true,
}

// splitPascal splits a PascalCase identifier into its words, e.g.
// "ScheduleGetHistory" -> ["Schedule", "Get", "History"].
func splitPascal(s string) []string {
	var words []string
	var cur []rune
	runes := []rune(s)
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) {
			words = append(words, string(cur))
			cur = nil
		}
		cur = append(cur, r)
	}
	if len(cur) > 0 {
		words = append(words, string(cur))
	}
	return words
}

// paramTokens splits a snake_case path parameter name into title-cased
// tokens, e.g. "project_slug" -> ["Project", "Slug"].
func paramTokens(name string) []string {
	parts := strings.Split(name, "_")
	tokens := make([]string, len(parts))
	for i, p := range parts {
		if p == "" {
			continue
		}
		tokens[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return tokens
}

// remainderWords computes the operationId words left over after stripping
// the leading tag word, any HTTP-method words, and any word that is just a
// path parameter name leaking into the operationId (see ProjectSlugGet in
// the design spec).
func remainderWords(op Operation) []string {
	paramTok := map[string]bool{}
	for _, p := range op.Params {
		for _, t := range paramTokens(p.Name) {
			paramTok[strings.ToLower(t)] = true
		}
	}

	words := splitPascal(op.OperationID)
	var out []string
	for i, w := range words {
		if i == 0 && strings.EqualFold(w, op.Tag) {
			continue
		}
		if httpMethodWords[w] {
			continue
		}
		if paramTok[strings.ToLower(w)] {
			continue
		}
		out = append(out, w)
	}
	return out
}

func kebab(words []string) []string {
	out := make([]string, len(words))
	for i, w := range words {
		out[i] = strings.ToLower(w)
	}
	return out
}

// endsInPathParam reports whether op's path's final segment is the last
// entry of op.Params (a {param} segment), as opposed to a static segment.
func endsInPathParam(op Operation) bool {
	if len(op.Params) == 0 {
		return false
	}
	last := op.Params[len(op.Params)-1]
	return strings.HasSuffix(op.Path, "{"+last.Name+"}")
}

// mechanicalVerb derives a verb purely from the HTTP method and whether the
// path ends in a path parameter, with no knowledge of what the resource is.
func mechanicalVerb(op Operation) string {
	switch op.Method {
	case "GET":
		if endsInPathParam(op) {
			return "get"
		}
		return "list"
	case "POST":
		return "create"
	case "PUT", "PATCH":
		if endsInPathParam(op) {
			return "update"
		}
		return "update"
	case "DELETE":
		return "delete"
	default:
		return strings.ToLower(op.Method)
	}
}

// BuildCommandSpecs converts every operation into a CommandSpec, in the same
// order as ops.
func BuildCommandSpecs(ops []Operation) []CommandSpec {
	type groupKey struct {
		tag       string
		remainder string
	}
	groupOf := make([]groupKey, len(ops))
	sizes := map[groupKey]int{}

	for i, op := range ops {
		k := groupKey{tag: op.Tag, remainder: strings.Join(remainderWords(op), "\x00")}
		groupOf[i] = k
		sizes[k]++
	}

	specs := make([]CommandSpec, len(ops))
	for i, op := range ops {
		remainder := remainderWords(op)
		size := sizes[groupOf[i]]

		spec := CommandSpec{Tag: op.Tag, Operation: op}
		switch {
		case size == 1 && len(remainder) > 0:
			spec.Verb = strings.Join(kebab(remainder), "-")
		case size == 1:
			spec.Verb = mechanicalVerb(op)
		default:
			spec.Group = kebab(remainder)
			spec.Verb = mechanicalVerb(op)
		}

		// A path parameter is positional only when the operation's URL
		// literally ends in it (classic get/update/delete-by-id shape).
		// Every other path parameter - including one on a URL whose final
		// segment is a static action word like "restore" or "check" - is a
		// flag. This is a strict URL-shape check, not a judgment call about
		// whether the operation "feels like" it targets that parameter.
		if n := len(op.Params); n > 0 {
			if endsInPathParam(op) {
				p := op.Params[n-1]
				spec.Positional = &p
				spec.Flags = append(spec.Flags, op.Params[:n-1]...)
			} else {
				spec.Flags = append(spec.Flags, op.Params...)
			}
		}

		specs[i] = spec
	}
	return specs
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/generator/... -run TestBuildCommandSpecs -v`
Expected: PASS

- [ ] **Step 5: Add a full-spec regression test**

```go
// internal/generator/naming_test.go — append
func TestBuildCommandSpecs_RealSpec_MatchesDesignDocCommandTree(t *testing.T) {
	ops, err := generator.LoadOperations("../../spec/api.yaml")
	require.NoError(t, err)
	specs := generator.BuildCommandSpecs(ops)

	want := map[string]string{ // operationId -> "tag/group.../verb"
		"AccountGet":            "account/list",
		"AccountPost":           "account/create",
		"AccountPut":            "account/update",
		"AccountPutObtain":      "account/obtain",
		"AccountTokensGet":      "account/tokens/list",
		"AccountTokensPost":     "account/tokens/create",
		"AccountTokensDelete":   "account/tokens/delete",
		"ProjectGet":            "project/list",
		"ProjectPost":           "project/create",
		"ProjectSlugGet":        "project/get",
		"ProjectPut":            "project/update",
		"ProjectAccountsGet":    "project/accounts",
		"ProjectInvitePut":      "project/invite",
		"ProjectPermissionsGet": "project/permissions",
		"BackupGet":             "backup/list",
		"BackupRestorePut":      "backup/restore/create",
		"BackupRestoreGet":      "backup/restore/get",
		"ConnectionGet":         "connection/list",
		"ConnectionPost":        "connection/create",
		"ConnectionDelete":      "connection/delete",
		"ConnectionPut":         "connection/update",
		"ConnectionCheck":       "connection/check",
		"ResourceGet":           "resource/list",
		"ScheduleGet":           "schedule/list",
		"SchedulePost":          "schedule/create",
		"ScheduleDelete":        "schedule/delete",
		"SchedulePut":           "schedule/update",
		"ScheduleGetHistory":    "schedule/history",
		"ScheduleRunPost":       "schedule/run",
		"CommonExternalTypes":   "/external-types",
		"CommonHealthcheck":     "/healthcheck",
		"CommonPermissions":     "/permissions",
	}
	require.Len(t, specs, len(want), "update this table if spec/api.yaml gained or lost operations")

	for _, s := range specs {
		parts := append([]string{s.Tag}, s.Group...)
		parts = append(parts, s.Verb)
		got := strings.Join(parts, "/")
		require.Equal(t, want[s.Operation.OperationID], got, "operationId %s", s.Operation.OperationID)
	}
}
```

Add `"strings"` to the test file's imports. Note the three `common`-tag rows
use a leading `/` (empty tag) — Task 13's emit step is what actually promotes
the `common` tag to ungrouped top-level commands; this test only checks the
naming algorithm's raw tag/group/verb output.

- [ ] **Step 6: Run the full-spec test**

Run: `go test ./internal/generator/... -run TestBuildCommandSpecs_RealSpec -v`
Expected: PASS. If it fails, the mismatch is either a real bug in the
algorithm or a sign the vendored spec changed since this plan was written —
check which by comparing the failing operationId's actual path/method/params
against the spec file.

- [ ] **Step 7: Commit**

```bash
git add internal/generator/naming.go internal/generator/naming_test.go
git commit -m "Add the OpenAPI -> command-tree naming algorithm"
```

---

### Task 12: Generator — flag/positional derivation and body-property flags

**Files:**
- Create: `internal/generator/flags.go`
- Test: `internal/generator/flags_test.go`

**Interfaces:**
- Consumes: `generator.CommandSpec`, `generator.Param`, `generator.BodyProp` (Tasks 10-11).
- Produces: `generator.FlagName(param string) string`, `generator.GoIdent(name string) string`, `generator.BodyFlagKind`, `generator.FlagKindForType(openAPIType string) BodyFlagKind`, `generator.FlagDef{Name, GoIdent string, Kind BodyFlagKind, Required, Optional bool}`, `generator.GeneratedCommand{Spec CommandSpec, PositionalFlag *FlagDef, Flags, BodyFlags []FlagDef}`, `generator.BuildGeneratedCommands(specs []CommandSpec) []GeneratedCommand` — consumed by Task 13 (emit).

- [ ] **Step 1: Write failing tests**

```go
// internal/generator/flags_test.go
package generator_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/trustattic/trustattic-cli/internal/generator"
)

func TestFlagName_DropsTrailingSlugKeepsID(t *testing.T) {
	require.Equal(t, "project", generator.FlagName("project_slug"))
	require.Equal(t, "account-id", generator.FlagName("account_id"))
	require.Equal(t, "backup-id", generator.FlagName("backup_id"))
}

func TestGoIdent_CapitalizesIDSuffixCamelCase(t *testing.T) {
	require.Equal(t, "accountID", generator.GoIdent("account-id"))
	require.Equal(t, "project", generator.GoIdent("project"))
	require.Equal(t, "restoreID", generator.GoIdent("restore-id"))
}

func TestFlagKindForType_MapsKnownOpenAPITypes(t *testing.T) {
	require.Equal(t, generator.KindBool, generator.FlagKindForType("boolean"))
	require.Equal(t, generator.KindInt, generator.FlagKindForType("integer"))
	require.Equal(t, generator.KindString, generator.FlagKindForType("string"))
	require.Equal(t, generator.KindString, generator.FlagKindForType(""))
}

func TestBuildGeneratedCommands_ProjectSlugFlagIsOptional(t *testing.T) {
	specs := []generator.CommandSpec{
		{
			Tag:  "backup",
			Verb: "list",
			Flags: []generator.Param{
				{Name: "project_slug"},
			},
		},
	}
	got := generator.BuildGeneratedCommands(specs)
	require.Len(t, got[0].Flags, 1)
	require.Equal(t, "project", got[0].Flags[0].Name)
	require.True(t, got[0].Flags[0].Optional)
	require.False(t, got[0].Flags[0].Required)
}

func TestBuildGeneratedCommands_OtherPathParamFlagsAreRequired(t *testing.T) {
	specs := []generator.CommandSpec{
		{
			Tag:  "account",
			Verb: "tokens-list",
			Flags: []generator.Param{
				{Name: "account_id"},
			},
		},
	}
	got := generator.BuildGeneratedCommands(specs)
	require.Len(t, got[0].Flags, 1)
	require.Equal(t, "account-id", got[0].Flags[0].Name)
	require.Equal(t, "accountID", got[0].Flags[0].GoIdent)
	require.True(t, got[0].Flags[0].Required)
	require.False(t, got[0].Flags[0].Optional)
}

func TestBuildGeneratedCommands_PositionalComesFromSpec(t *testing.T) {
	positional := generator.Param{Name: "token_id"}
	specs := []generator.CommandSpec{
		{Tag: "account", Verb: "delete", Positional: &positional},
	}
	got := generator.BuildGeneratedCommands(specs)
	require.NotNil(t, got[0].PositionalFlag)
	require.Equal(t, "token-id", got[0].PositionalFlag.Name)
	require.Equal(t, "tokenID", got[0].PositionalFlag.GoIdent)
}

func TestBuildGeneratedCommands_BodyPropsBecomeTypedFlags(t *testing.T) {
	specs := []generator.CommandSpec{
		{
			Tag:  "project",
			Verb: "create",
			Operation: generator.Operation{
				HasBody: true,
				BodyProps: []generator.BodyProp{
					{Name: "name", Type: "string", Required: true},
					{Name: "active", Type: "boolean", Required: false},
				},
			},
		},
	}
	got := generator.BuildGeneratedCommands(specs)
	require.Len(t, got[0].BodyFlags, 2)

	byName := map[string]generator.FlagDef{}
	for _, f := range got[0].BodyFlags {
		byName[f.Name] = f
	}
	require.Equal(t, generator.KindString, byName["name"].Kind)
	require.True(t, byName["name"].Required)
	require.Equal(t, generator.KindBool, byName["active"].Kind)
	require.False(t, byName["active"].Required)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/generator/... -run 'TestFlagName|TestGoIdent|TestFlagKindForType|TestBuildGeneratedCommands' -v`
Expected: FAIL (symbols undefined)

- [ ] **Step 3: Implement flags.go**

```go
// internal/generator/flags.go
package generator

import "strings"

// FlagName derives a CLI flag name from a snake_case path parameter or body
// property name: underscores become hyphens, and a trailing "_slug" is
// dropped (project_slug -> "project", not "project-slug") since the
// resource name alone is unambiguous. "_id" is kept as "-id" since bare
// resource names would be ambiguous there (e.g. "account" could mean many
// things; "account-id" cannot).
func FlagName(param string) string {
	name := strings.ReplaceAll(param, "_", "-")
	return strings.TrimSuffix(name, "-slug")
}

// GoIdent turns a kebab-case flag name into a camelCase Go identifier,
// special-casing a trailing "id" segment to "ID" (Go style), e.g.
// "account-id" -> "accountID".
func GoIdent(name string) string {
	parts := strings.Split(name, "-")
	var b strings.Builder
	for i, p := range parts {
		if p == "" {
			continue
		}
		if p == "id" {
			b.WriteString("ID")
			continue
		}
		if i == 0 {
			b.WriteString(strings.ToLower(p[:1]) + p[1:])
		} else {
			b.WriteString(strings.ToUpper(p[:1]) + p[1:])
		}
	}
	return b.String()
}

// BodyFlagKind is the Cobra flag kind used for a request body property.
type BodyFlagKind string

const (
	KindString BodyFlagKind = "string"
	KindBool   BodyFlagKind = "bool"
	KindInt    BodyFlagKind = "int"
)

// FlagKindForType maps an OpenAPI schema type to a Cobra flag kind. Every
// request body in the current spec is flat (see the design spec's "no
// nested-object escape hatch" decision), so object/array/unknown types fall
// back to KindString - the flag simply carries the raw scalar as text.
func FlagKindForType(openAPIType string) BodyFlagKind {
	switch openAPIType {
	case "boolean":
		return KindBool
	case "integer":
		return KindInt
	default:
		return KindString
	}
}

// IsProjectFlag reports whether p is the project_slug path parameter - the
// one path parameter that gets an optional/current-project fallback instead
// of being required. See the design spec's "Current project" section.
func IsProjectFlag(p Param) bool {
	return p.Name == "project_slug"
}

// FlagDef is one Cobra flag or positional argument, template-ready.
type FlagDef struct {
	Name     string // CLI flag name, kebab-case
	GoIdent  string // Go variable name
	Kind     BodyFlagKind
	Required bool
	Optional bool // true only for the project flag
}

// GeneratedCommand is a fully-resolved, template-ready description of one
// generated Cobra command.
type GeneratedCommand struct {
	Spec           CommandSpec
	PositionalFlag *FlagDef
	Flags          []FlagDef // path-param flags, excluding the positional one
	BodyFlags      []FlagDef // request body property flags
}

// BuildGeneratedCommands resolves every CommandSpec's path parameters and
// body properties into template-ready flag definitions.
func BuildGeneratedCommands(specs []CommandSpec) []GeneratedCommand {
	out := make([]GeneratedCommand, len(specs))
	for i, s := range specs {
		gc := GeneratedCommand{Spec: s}

		if s.Positional != nil {
			name := FlagName(s.Positional.Name)
			gc.PositionalFlag = &FlagDef{Name: name, GoIdent: GoIdent(name), Kind: KindString}
		}

		for _, p := range s.Flags {
			name := FlagName(p.Name)
			gc.Flags = append(gc.Flags, FlagDef{
				Name:     name,
				GoIdent:  GoIdent(name),
				Kind:     KindString,
				Required: !IsProjectFlag(p),
				Optional: IsProjectFlag(p),
			})
		}

		for _, bp := range s.Operation.BodyProps {
			name := strings.ReplaceAll(bp.Name, "_", "-")
			gc.BodyFlags = append(gc.BodyFlags, FlagDef{
				Name:     name,
				GoIdent:  GoIdent(name),
				Kind:     FlagKindForType(bp.Type),
				Required: bp.Required,
			})
		}

		out[i] = gc
	}
	return out
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/generator/... -run 'TestFlagName|TestGoIdent|TestFlagKindForType|TestBuildGeneratedCommands' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/generator/flags.go internal/generator/flags_test.go
git commit -m "Derive Cobra flag/positional definitions from path params and request bodies"
```

---

### Task 13: Generator — `operationId` name overrides

**Files:**
- Create: `internal/generator/overrides.go`
- Create: `internal/generator/overrides.yaml`
- Test: `internal/generator/overrides_test.go`

**Interfaces:**
- Consumes: `generator.CommandSpec` (Task 11).
- Produces: `generator.Override{Group []string, Verb string}`, `generator.LoadOverrides(path string) (map[string]Override, error)`, `generator.ApplyOverrides(specs []CommandSpec, overrides map[string]Override) []CommandSpec` — consumed by Task 14 (generator entrypoint), applied after `BuildCommandSpecs` and before `BuildGeneratedCommands`.

The real spec's naming algorithm output (verified in Task 11) needs no
overrides today - `overrides.yaml` starts empty. This mechanism exists so a
future awkward name can be fixed with one YAML line instead of a code change,
per the design spec.

- [ ] **Step 1: Write failing tests**

```go
// internal/generator/overrides_test.go
package generator_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/trustattic/trustattic-cli/internal/generator"
)

func TestLoadOverrides_ParsesGroupAndVerb(t *testing.T) {
	path := filepath.Join(t.TempDir(), "overrides.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
AccountPutObtain:
  verb: obtain
BackupRestoreGet:
  group: [restore]
  verb: status
`), 0o644))

	overrides, err := generator.LoadOverrides(path)
	require.NoError(t, err)
	require.Equal(t, "obtain", overrides["AccountPutObtain"].Verb)
	require.Equal(t, []string{"restore"}, overrides["BackupRestoreGet"].Group)
	require.Equal(t, "status", overrides["BackupRestoreGet"].Verb)
}

func TestLoadOverrides_MissingFile_ReturnsEmptyMapNoError(t *testing.T) {
	overrides, err := generator.LoadOverrides(filepath.Join(t.TempDir(), "missing.yaml"))
	require.NoError(t, err)
	require.Empty(t, overrides)
}

func TestApplyOverrides_ReplacesVerbAndGroupByOperationID(t *testing.T) {
	specs := []generator.CommandSpec{
		{Operation: generator.Operation{OperationID: "BackupRestoreGet"}, Verb: "get"},
		{Operation: generator.Operation{OperationID: "BackupRestoreCreate"}, Verb: "create"},
	}
	overrides := map[string]generator.Override{
		"BackupRestoreGet": {Verb: "status"},
	}

	got := generator.ApplyOverrides(specs, overrides)
	require.Equal(t, "status", got[0].Verb)
	require.Equal(t, "create", got[1].Verb, "operation without an override is untouched")
}

func TestApplyOverrides_EmptyMap_LeavesTheRealSpecUnchanged(t *testing.T) {
	ops, err := generator.LoadOperations("../../spec/api.yaml")
	require.NoError(t, err)
	specs := generator.BuildCommandSpecs(ops)
	overrides, err := generator.LoadOverrides("overrides.yaml")
	require.NoError(t, err)

	got := generator.ApplyOverrides(specs, overrides)
	require.Equal(t, specs, got)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/generator/... -run 'TestLoadOverrides|TestApplyOverrides' -v`
Expected: FAIL (symbols undefined)

- [ ] **Step 3: Implement overrides.go**

```go
// internal/generator/overrides.go
package generator

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Override replaces the mechanically-derived Group and/or Verb for one
// operationId. A zero-value field (nil Group, empty Verb) leaves that part
// of the mechanical result untouched.
type Override struct {
	Group []string `yaml:"group,omitempty"`
	Verb  string   `yaml:"verb,omitempty"`
}

// LoadOverrides reads operationId -> Override entries from path. A missing
// file is not an error; it returns an empty map.
func LoadOverrides(path string) (map[string]Override, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]Override{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read overrides: %w", err)
	}
	var overrides map[string]Override
	if err := yaml.Unmarshal(data, &overrides); err != nil {
		return nil, fmt.Errorf("parse overrides: %w", err)
	}
	if overrides == nil {
		overrides = map[string]Override{}
	}
	return overrides, nil
}

// ApplyOverrides returns specs with each entry's Group/Verb replaced by its
// matching override, if any (matched by operationId).
func ApplyOverrides(specs []CommandSpec, overrides map[string]Override) []CommandSpec {
	out := make([]CommandSpec, len(specs))
	for i, s := range specs {
		o, ok := overrides[s.Operation.OperationID]
		if !ok {
			out[i] = s
			continue
		}
		if o.Group != nil {
			s.Group = o.Group
		}
		if o.Verb != "" {
			s.Verb = o.Verb
		}
		out[i] = s
	}
	return out
}
```

- [ ] **Step 4: Add the (empty) overrides file**

```yaml
# internal/generator/overrides.yaml
# operationId -> { group: [...], verb: "..." } overrides for the mechanical
# naming algorithm in naming.go. Empty today - see naming_test.go's
# TestBuildCommandSpecs_RealSpec_MatchesDesignDocCommandTree, which passes
# against the real spec with no overrides needed. Add an entry here only
# when the mechanical result reads badly; never add per-resource behavior.
{}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/generator/... -run 'TestLoadOverrides|TestApplyOverrides' -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/generator/overrides.go internal/generator/overrides_test.go internal/generator/overrides.yaml
git commit -m "Add a transparent operationId->name override table for the generator"
```

---

### Task 14: Generator — emit Go source for the command tree

**Files:**
- Create: `internal/generator/emit.go`
- Test: `internal/generator/emit_test.go`

**Interfaces:**
- Consumes: `generator.GeneratedCommand` (Task 12).
- Produces: `generator.EmitTagFile(w io.Writer, cmds []GeneratedCommand) error`, `generator.EmitRegisterFile(w io.Writer, cmds []GeneratedCommand) error`, `generator.GroupByTag(cmds []GeneratedCommand) map[string][]GeneratedCommand` — consumed by Task 15 (generator entrypoint).

This task builds emitted Go source with plain string-building (not nested
`text/template` conditionals, which get unreadable fast for code this
branchy) — a small outer `text/template` only wraps the package/import
skeleton. This task's test only checks the emitted code is syntactically
valid Go for a *fixture* spec, since the fixture's `client.Widget...`
methods don't exist in the real generated client; Task 15 is what proves the
real spec's output actually compiles and links against `internal/client`.

- [ ] **Step 1: Write failing tests**

```go
// internal/generator/emit_test.go
package generator_test

import (
	"bytes"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/trustattic/trustattic-cli/internal/generator"
)

func fixtureCommands(t *testing.T) []generator.GeneratedCommand {
	t.Helper()
	ops, err := generator.LoadOperations("testdata/fixture.yaml")
	require.NoError(t, err)
	specs := generator.BuildCommandSpecs(ops)
	return generator.BuildGeneratedCommands(specs)
}

func TestEmitTagFile_FixtureSpec_ProducesValidGo(t *testing.T) {
	cmds := fixtureCommands(t)

	var buf bytes.Buffer
	require.NoError(t, generator.EmitTagFile(&buf, cmds))

	_, err := parser.ParseFile(token.NewFileSet(), "widget.gen.go", buf.Bytes(), parser.AllErrors)
	require.NoError(t, err, "emitted source:\n%s", buf.String())

	out := buf.String()
	require.Contains(t, out, "func NewWidgetGetCommand() *cobra.Command")
	require.Contains(t, out, "func NewWidgetPostCommand() *cobra.Command")
	require.Contains(t, out, "func NewWidgetDeleteCommand() *cobra.Command")
}

func TestEmitTagFile_PositionalArgUsesExactArgsOne(t *testing.T) {
	cmds := fixtureCommands(t)

	var buf bytes.Buffer
	require.NoError(t, generator.EmitTagFile(&buf, cmds))

	require.Contains(t, buf.String(), `Use:  "delete <widget_id>"`)
	require.Contains(t, buf.String(), "Args: cobra.ExactArgs(1)")
}

func TestEmitTagFile_RequiredBodyPropIsMarkedRequired(t *testing.T) {
	cmds := fixtureCommands(t)

	var buf bytes.Buffer
	require.NoError(t, generator.EmitTagFile(&buf, cmds))

	out := buf.String()
	require.Contains(t, out, `cmd.Flags().StringVar(&name, "name", "", "name")`)
	require.Contains(t, out, `cmd.MarkFlagRequired("name")`)
	require.NotContains(t, out, `MarkFlagRequired("active")`) // not required in the fixture
}

func TestEmitRegisterFile_FixtureSpec_ProducesValidGo(t *testing.T) {
	cmds := fixtureCommands(t)

	var buf bytes.Buffer
	require.NoError(t, generator.EmitRegisterFile(&buf, cmds))

	_, err := parser.ParseFile(token.NewFileSet(), "register.gen.go", buf.Bytes(), parser.AllErrors)
	require.NoError(t, err, "emitted source:\n%s", buf.String())

	out := buf.String()
	require.Contains(t, out, `widgetCmd := getOrAddCommand(root, "widget")`)
	require.Contains(t, out, "widgetCmd.AddCommand(NewWidgetGetCommand())")
	require.Contains(t, out, "widgetCmd.AddCommand(NewWidgetPostCommand())")
	require.Contains(t, out, "widgetCmd.AddCommand(NewWidgetDeleteCommand())")
	// the parent line must appear exactly once even though three commands share it
	require.Equal(t, 1, strings.Count(out, `widgetCmd := getOrAddCommand`))
}

func TestGroupByTag_GroupsFixtureCommandsUnderWidget(t *testing.T) {
	cmds := fixtureCommands(t)
	groups := generator.GroupByTag(cmds)
	require.Len(t, groups["widget"], 3)
}
```

Add `"strings"` to this test file's imports.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/generator/... -run 'TestEmitTagFile|TestEmitRegisterFile|TestGroupByTag' -v`
Expected: FAIL (symbols undefined)

- [ ] **Step 3: Implement emit.go**

```go
// internal/generator/emit.go
package generator

import (
	"fmt"
	"io"
	"strings"
	"text/template"
)

const fileSkeleton = `// Code generated by internal/generator. DO NOT EDIT.
package generated

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/trustattic/trustattic-cli/internal/cli"
	"github.com/trustattic/trustattic-cli/internal/client"
)
{{.Body}}`

var fileTmpl = template.Must(template.New("file").Parse(fileSkeleton))

// GroupByTag buckets cmds by their Spec.Tag, preserving each bucket's
// relative order.
func GroupByTag(cmds []GeneratedCommand) map[string][]GeneratedCommand {
	groups := map[string][]GeneratedCommand{}
	for _, c := range cmds {
		groups[c.Spec.Tag] = append(groups[c.Spec.Tag], c)
	}
	return groups
}

// EmitTagFile renders the Go source declaring one New<OperationID>Command
// function per entry in cmds to w. Every entry is expected to share the
// same Spec.Tag (call GroupByTag first and emit one file per group).
func EmitTagFile(w io.Writer, cmds []GeneratedCommand) error {
	var body strings.Builder
	for _, c := range cmds {
		body.WriteString("\n")
		body.WriteString(renderCommandFunc(c))
	}
	return fileTmpl.Execute(w, struct{ Body string }{Body: body.String()})
}

func renderCommandFunc(c GeneratedCommand) string {
	opID := c.Spec.Operation.OperationID
	var b strings.Builder

	fmt.Fprintf(&b, "func New%sCommand() *cobra.Command {\n", opID)
	for _, f := range c.Flags {
		fmt.Fprintf(&b, "\tvar %s string\n", f.GoIdent)
	}
	for _, f := range c.BodyFlags {
		fmt.Fprintf(&b, "\tvar %s %s\n", f.GoIdent, goType(f.Kind))
	}

	use := c.Spec.Verb
	args := "cobra.NoArgs"
	if c.PositionalFlag != nil {
		use = fmt.Sprintf("%s <%s>", c.Spec.Verb, c.Spec.Positional.Name)
		args = "cobra.ExactArgs(1)"
	}
	fmt.Fprintf(&b, "\tcmd := &cobra.Command{\n\t\tUse:  %q,\n\t\tArgs: %s,\n\t\tRunE: func(cmd *cobra.Command, args []string) error {\n", use, args)

	b.WriteString("\t\t\tcfg, err := cli.LoadConfig()\n\t\t\tif err != nil {\n\t\t\t\treturn err\n\t\t\t}\n")
	for _, f := range c.Flags {
		if !f.Optional {
			continue
		}
		fmt.Fprintf(&b, "\t\t\tif %s == \"\" {\n\t\t\t\t%s = cfg.CurrentProject\n\t\t\t}\n", f.GoIdent, f.GoIdent)
		fmt.Fprintf(&b, "\t\t\tif %s == \"\" {\n\t\t\t\treturn fmt.Errorf(\"no project set: pass --%s or run `trustattic use <project>`\")\n\t\t\t}\n", f.GoIdent, f.Name)
	}
	b.WriteString("\t\t\tapiClient, err := cli.NewAPIClient(cfg)\n\t\t\tif err != nil {\n\t\t\t\treturn err\n\t\t\t}\n")

	// The real generated client method takes path parameters as positional
	// Go arguments in URL order. c.Flags is already every path param
	// *except* the positional one, in URL order, and (per naming.go) the
	// positional one - when present - is always the URL's last path param.
	// So "flags first, positional last" here reproduces URL order exactly;
	// do not reorder this.
	callArgs := []string{"context.Background()"}
	for _, f := range c.Flags {
		callArgs = append(callArgs, f.GoIdent)
	}
	if c.PositionalFlag != nil {
		callArgs = append(callArgs, "args[0]")
	}
	callArgs = append(callArgs, fmt.Sprintf("&client.%sParams{}", opID))
	if c.Spec.Operation.HasBody {
		var props strings.Builder
		for _, f := range c.BodyFlags {
			fmt.Fprintf(&props, "\t\t\t\t%s: &%s,\n", pascalCase(f.GoIdent), f.GoIdent)
		}
		callArgs = append(callArgs, fmt.Sprintf("client.%sJSONRequestBody{\n%s\t\t\t}", opID, props.String()))
	}
	fmt.Fprintf(&b, "\t\t\tresp, err := apiClient.%sWithResponse(\n\t\t\t\t%s,\n\t\t\t)\n", opID, strings.Join(callArgs, ",\n\t\t\t\t"))

	b.WriteString("\t\t\tif err != nil {\n\t\t\t\treturn err\n\t\t\t}\n")
	b.WriteString("\t\t\tif resp.StatusCode() >= 400 {\n\t\t\t\tcli.RenderError(cmd.ErrOrStderr(), &cli.APIError{StatusCode: resp.StatusCode(), Body: resp.Body})\n\t\t\t\treturn fmt.Errorf(\"request failed with status %d\", resp.StatusCode())\n\t\t\t}\n")
	b.WriteString("\t\t\tmode := cli.ModeAuto\n\t\t\tif out, _ := cmd.Flags().GetString(\"output\"); out == \"json\" {\n\t\t\t\tmode = cli.ModeJSON\n\t\t\t}\n")
	b.WriteString("\t\t\tisTTY := term.IsTerminal(int(os.Stdout.Fd()))\n\t\t\treturn cli.Render(cmd.OutOrStdout(), mode, isTTY, resp.Body)\n")
	b.WriteString("\t\t},\n\t}\n")

	for _, f := range c.Flags {
		help := f.Name
		if f.Optional {
			help = fmt.Sprintf("%s (falls back to the current project set via `trustattic use`)", f.Name)
		}
		fmt.Fprintf(&b, "\tcmd.Flags().StringVar(&%s, %q, \"\", %q)\n", f.GoIdent, f.Name, help)
		if f.Required {
			fmt.Fprintf(&b, "\t_ = cmd.MarkFlagRequired(%q)\n", f.Name)
		}
	}
	for _, f := range c.BodyFlags {
		setter, zero := "StringVar", `""`
		switch f.Kind {
		case KindBool:
			setter, zero = "BoolVar", "false"
		case KindInt:
			setter, zero = "IntVar", "0"
		}
		fmt.Fprintf(&b, "\tcmd.Flags().%s(&%s, %q, %s, %q)\n", setter, f.GoIdent, f.Name, zero, f.Name)
		if f.Required {
			fmt.Fprintf(&b, "\t_ = cmd.MarkFlagRequired(%q)\n", f.Name)
		}
	}
	b.WriteString("\treturn cmd\n}\n")
	return b.String()
}

func goType(k BodyFlagKind) string {
	switch k {
	case KindBool:
		return "bool"
	case KindInt:
		return "int"
	default:
		return "string"
	}
}

// pascalCase capitalizes a GoIdent-style camelCase name into the PascalCase
// oapi-codegen uses for exported JSON request-body struct fields (e.g.
// "name" -> "Name"). This covers every plain-word body property in the
// current spec. oapi-codegen applies its own initialism rules for words
// like "url"/"id" (e.g. a property named "logo_url" becomes struct field
// "LogoURL", not "LogoUrl") - if `go build` (Task 15) reports an unknown
// field for a specific property, check its real name with
// `go doc ./internal/client <OperationID>JSONRequestBody` and special-case
// it in this function.
func pascalCase(camel string) string {
	if camel == "" {
		return camel
	}
	return strings.ToUpper(camel[:1]) + camel[1:]
}

// EmitRegisterFile renders register.gen.go, which wires every command in
// cmds onto its tag/group parent (special-cased: the "common" tag's
// commands attach directly to root, ungrouped, per the design spec).
func EmitRegisterFile(w io.Writer, cmds []GeneratedCommand) error {
	var b strings.Builder
	b.WriteString("// Code generated by internal/generator. DO NOT EDIT.\n")
	b.WriteString("package generated\n\n")
	b.WriteString(`import "github.com/spf13/cobra"` + "\n\n")
	b.WriteString("func getOrAddCommand(parent *cobra.Command, use string) *cobra.Command {\n")
	b.WriteString("\tfor _, c := range parent.Commands() {\n\t\tif c.Name() == use {\n\t\t\treturn c\n\t\t}\n\t}\n")
	b.WriteString("\tc := &cobra.Command{Use: use}\n\tparent.AddCommand(c)\n\treturn c\n}\n\n")
	b.WriteString("// Register wires every generated command onto root.\n")
	b.WriteString("func Register(root *cobra.Command) {\n")

	declared := map[string]bool{}
	for _, c := range cmds {
		opID := c.Spec.Operation.OperationID
		if c.Spec.Tag == "common" {
			fmt.Fprintf(&b, "\troot.AddCommand(New%sCommand())\n", opID)
			continue
		}

		parentVar := "root"
		pathKey := c.Spec.Tag
		tagVar := c.Spec.Tag + "Cmd"
		if !declared[pathKey] {
			fmt.Fprintf(&b, "\t%s := getOrAddCommand(%s, %q)\n", tagVar, parentVar, c.Spec.Tag)
			declared[pathKey] = true
		}
		parentVar = tagVar

		for _, g := range c.Spec.Group {
			pathKey += "/" + g
			groupVar := c.Spec.Tag + pascalCase(g) + "Cmd"
			if !declared[pathKey] {
				fmt.Fprintf(&b, "\t%s := getOrAddCommand(%s, %q)\n", groupVar, parentVar, g)
				declared[pathKey] = true
			}
			parentVar = groupVar
		}

		fmt.Fprintf(&b, "\t%s.AddCommand(New%sCommand())\n", parentVar, opID)
	}

	b.WriteString("}\n")
	_, err := w.Write([]byte(b.String()))
	return err
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/generator/... -run 'TestEmitTagFile|TestEmitRegisterFile|TestGroupByTag' -v`
Expected: PASS. If `parser.ParseFile` reports a syntax error, the failure
message includes the full emitted source (via `require.NoError`'s message
args) — fix the specific `Fprintf`/`WriteString` call producing the bad
line, most commonly a missing/extra comma or brace.

- [ ] **Step 5: Commit**

```bash
git add internal/generator/emit.go internal/generator/emit_test.go
git commit -m "Emit Go source for the generated Cobra command tree"
```

---

### Task 15: Generator entrypoint — generate the real command tree and wire it in

**Files:**
- Create: `cmd/gen/main.go`
- Create: `internal/commands/generated/*.gen.go` (generated, committed)
- Modify: `cmd/trustattic/main.go`
- Modify: `Makefile`
- Test: `internal/commands/generated/register_test.go`

**Interfaces:**
- Consumes: `generator.LoadOperations`, `BuildCommandSpecs`, `LoadOverrides`, `ApplyOverrides`, `BuildGeneratedCommands`, `GroupByTag`, `EmitTagFile`, `EmitRegisterFile` (Tasks 10-14).
- Produces: package `internal/commands/generated` with `generated.Register(root *cobra.Command)`, consumed by `cmd/trustattic/main.go`.

Note the entrypoint lives at `cmd/gen`, not inside `internal/generator` itself
— a directory can only hold one package, and `internal/generator` is already
`package generator` (a library), so its CLI wrapper needs its own directory,
same as `cmd/trustattic`.

- [ ] **Step 1: Write cmd/gen/main.go**

```go
// cmd/gen/main.go
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"io"
	"os"
	"path/filepath"

	"github.com/trustattic/trustattic-cli/internal/generator"
)

func main() {
	specPath := flag.String("spec", "spec/api.yaml", "path to the OpenAPI spec")
	outDir := flag.String("out", "internal/commands/generated", "output directory")
	overridesPath := flag.String("overrides", "internal/generator/overrides.yaml", "path to the name-override table")
	flag.Parse()

	if err := run(*specPath, *outDir, *overridesPath); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(specPath, outDir, overridesPath string) error {
	ops, err := generator.LoadOperations(specPath)
	if err != nil {
		return fmt.Errorf("load operations: %w", err)
	}
	specs := generator.BuildCommandSpecs(ops)

	overrides, err := generator.LoadOverrides(overridesPath)
	if err != nil {
		return fmt.Errorf("load overrides: %w", err)
	}
	specs = generator.ApplyOverrides(specs, overrides)

	cmds := generator.BuildGeneratedCommands(specs)
	groups := generator.GroupByTag(cmds)

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	for tag, tagCmds := range groups {
		path := filepath.Join(outDir, tag+".gen.go")
		if err := writeFormatted(path, func(w io.Writer) error {
			return generator.EmitTagFile(w, tagCmds)
		}); err != nil {
			return fmt.Errorf("emit %s: %w", path, err)
		}
	}

	registerPath := filepath.Join(outDir, "register.gen.go")
	if err := writeFormatted(registerPath, func(w io.Writer) error {
		return generator.EmitRegisterFile(w, cmds)
	}); err != nil {
		return fmt.Errorf("emit %s: %w", registerPath, err)
	}

	return nil
}

func writeFormatted(path string, render func(w io.Writer) error) error {
	var buf bytes.Buffer
	if err := render(&buf); err != nil {
		return err
	}
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return fmt.Errorf("gofmt: %w\n--- unformatted source ---\n%s", err, buf.String())
	}
	return os.WriteFile(path, formatted, 0o644)
}
```

- [ ] **Step 2: Generate the real command tree**

```bash
go run ./cmd/gen
```

Expected: `internal/commands/generated/account.gen.go`, `project.gen.go`,
`backup.gen.go`, `connection.gen.go`, `resource.gen.go`, `schedule.gen.go`,
`common.gen.go`, and `register.gen.go` are created, each `gofmt`-clean.

- [ ] **Step 3: Wire the generated tree into main.go**

```go
// cmd/trustattic/main.go
package main

import (
	"context"
	"os"

	"github.com/charmbracelet/fang"
	"github.com/trustattic/trustattic-cli/internal/cli"
	"github.com/trustattic/trustattic-cli/internal/commands"
	"github.com/trustattic/trustattic-cli/internal/commands/generated"
)

func main() {
	root := cli.NewRootCommand()
	commands.Register(root)
	generated.Register(root)
	if err := fang.Execute(context.Background(), root); err != nil {
		os.Exit(1)
	}
}
```

- [ ] **Step 4: Build the whole module**

```bash
go build ./...
```

Expected: this is very likely to fail on the first try with real
`internal/client` symbol mismatches — that's expected, not a sign the plan
is wrong, and is exactly why Task 2 generated a real client to compile
against rather than a mock. Work through errors in this order:

1. **Unknown field in a `<OperationID>JSONRequestBody` literal** — run
   `go doc ./internal/client <OperationID>JSONRequestBody` to see the real
   field name/type, then fix the specific mapping in `pascalCase` (Task 14,
   `internal/generator/emit.go`) if it's a naming mismatch (e.g. an
   initialism like `URL`), or drop the leading `&` in `bodyValueExpr`'s call
   site in `renderCommandFunc` if the real field is a plain value instead of
   a pointer. Re-run `go run ./cmd/gen` and `go build ./...` after each fix.
2. **Wrong number/type of arguments to `<OperationID>WithResponse`** — run
   `go doc ./internal/client <OperationID>WithResponse` and compare against
   `renderCommandFunc`'s `callArgs` construction (path params, then
   `&client.<OperationID>Params{}`, then the body).
3. **`resp.StatusCode()` or `resp.Body` not found** — run
   `go doc ./internal/client <OperationID>Resp` to confirm the response
   wrapper's real field/method names (set via `response-type-suffix: Resp`
   in Task 2); adjust `renderCommandFunc` if they differ.

Do not hand-edit any `*.gen.go` file — fix the generator in
`internal/generator/emit.go` and re-run `go run ./cmd/gen`, so the fix
survives the next regeneration.

- [ ] **Step 5: Add the register_test.go regression test**

```go
// internal/commands/generated/register_test.go
package generated_test

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"github.com/trustattic/trustattic-cli/internal/commands/generated"
)

func findCommand(t *testing.T, root *cobra.Command, path ...string) *cobra.Command {
	t.Helper()
	cur := root
	for _, name := range path {
		var next *cobra.Command
		for _, c := range cur.Commands() {
			if c.Name() == name {
				next = c
				break
			}
		}
		require.NotNilf(t, next, "no command %q under %q", name, cur.Name())
		cur = next
	}
	return cur
}

func TestRegister_BuildsTheDocumentedCommandTree(t *testing.T) {
	root := &cobra.Command{Use: "trustattic"}
	generated.Register(root)

	findCommand(t, root, "account", "list")
	findCommand(t, root, "account", "create")
	findCommand(t, root, "account", "update")
	findCommand(t, root, "account", "obtain")
	findCommand(t, root, "account", "tokens", "list")
	findCommand(t, root, "account", "tokens", "create")
	findCommand(t, root, "account", "tokens", "delete")

	findCommand(t, root, "project", "list")
	findCommand(t, root, "project", "create")
	findCommand(t, root, "project", "get")
	findCommand(t, root, "project", "update")
	findCommand(t, root, "project", "accounts")
	findCommand(t, root, "project", "invite")
	findCommand(t, root, "project", "permissions")

	findCommand(t, root, "backup", "list")
	findCommand(t, root, "backup", "restore", "create")
	findCommand(t, root, "backup", "restore", "get")

	findCommand(t, root, "connection", "list")
	findCommand(t, root, "connection", "create")
	findCommand(t, root, "connection", "update")
	findCommand(t, root, "connection", "delete")
	findCommand(t, root, "connection", "check")

	findCommand(t, root, "resource", "list")

	findCommand(t, root, "schedule", "list")
	findCommand(t, root, "schedule", "create")
	findCommand(t, root, "schedule", "update")
	findCommand(t, root, "schedule", "delete")
	findCommand(t, root, "schedule", "run")
	findCommand(t, root, "schedule", "history")

	findCommand(t, root, "healthcheck")
	findCommand(t, root, "permissions")
	findCommand(t, root, "external-types")
}
```

- [ ] **Step 6: Run all tests**

Run: `go test ./... -v`
Expected: PASS, including `TestRegister_BuildsTheDocumentedCommandTree`.

- [ ] **Step 7: Smoke-test the real binary**

```bash
go build -o /tmp/trustattic ./cmd/trustattic
/tmp/trustattic project --help
/tmp/trustattic backup restore --help
```

Expected: `project --help` lists `list`, `create`, `get`, `update`,
`accounts`, `invite`, `permissions`; `backup restore --help` lists `create`
and `get`.

- [ ] **Step 8: Add `generate` to the Makefile**

```makefile
# Makefile — replace the existing generate target
generate:
	cd tools && go generate -tags=tools ./...
	go run ./cmd/gen
```

- [ ] **Step 9: Commit**

```bash
git add cmd/gen cmd/trustattic/main.go internal/commands/generated Makefile
git commit -m "Generate the full command tree from the real spec and wire it into main"
```

---

### Task 16: Integration test infrastructure (containers + bootstrap JWT helper)

**Files:**
- Create: `testing/integration/helper/jwt.go`
- Test: `testing/integration/helper/jwt_test.go`
- Create: `testing/container/containers.go` (build-tagged `integration`)

**Interfaces:**
- Produces: `helper.JWT`, `helper.NewJWT() (*JWT, error)`, `(*JWT) PublicKeyPEM() (string, error)`, `(*JWT) Mint(sub string) (string, error)` — consumed by Task 17's bootstrap. `container.Stack{PlatformURL string}`, `container.Start(ctx context.Context, jwtPublicKeyPEM string) (*Stack, error)`, `(*Stack) Terminate(ctx context.Context)` — consumed by Task 17's `SetupSuite`/`TearDownSuite`.

Container-starting code is gated behind the `integration` build tag so
`go test ./...` (every earlier task, and CI's default run) never needs
Docker. `make test-integration` (Task 18) is what actually runs it.

- [ ] **Step 1: Add dependencies**

```bash
go get github.com/golang-jwt/jwt/v5@latest
go get github.com/testcontainers/testcontainers-go@latest
go get github.com/testcontainers/testcontainers-go/modules/postgres@latest
```

- [ ] **Step 2: Write the failing JWT helper test**

```go
// testing/integration/helper/jwt_test.go
package helper_test

import (
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"github.com/trustattic/trustattic-cli/testing/integration/helper"
)

func TestJWT_MintedTokenParsesWithThePublicKey(t *testing.T) {
	j, err := helper.NewJWT()
	require.NoError(t, err)

	pemStr, err := j.PublicKeyPEM()
	require.NoError(t, err)
	require.Contains(t, pemStr, "BEGIN PUBLIC KEY")

	token, err := j.Mint("test-user-1")
	require.NoError(t, err)
	require.NotEmpty(t, token)

	block, _ := pemDecode(t, pemStr)
	pub, err := parsePublicKey(block)
	require.NoError(t, err)

	parsed, err := jwt.Parse(token, func(*jwt.Token) (any, error) { return pub, nil })
	require.NoError(t, err)
	claims, ok := parsed.Claims.(jwt.MapClaims)
	require.True(t, ok)
	require.Equal(t, "test-user-1", claims["sub"])
}
```

```go
// testing/integration/helper/jwt_test_helpers.go — small test-only utilities
// kept separate from jwt_test.go for clarity
package helper_test

import (
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/stretchr/testify/require"
)

func pemDecode(t *testing.T, s string) (*pem.Block, []byte) {
	t.Helper()
	block, rest := pem.Decode([]byte(s))
	require.NotNil(t, block)
	return block, rest
}

func parsePublicKey(block *pem.Block) (any, error) {
	return x509.ParsePKIXPublicKey(block.Bytes)
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./testing/integration/helper/... -v`
Expected: FAIL (package `helper` doesn't exist yet)

- [ ] **Step 4: Implement jwt.go**

```go
// testing/integration/helper/jwt.go
package helper

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWT mints test-only JWTs signed by a fresh, per-instance RSA keypair. It
// exists solely to bootstrap a real user account, project, and
// service-account token before an integration test run (see suite.go) — the
// trustattic CLI itself never authenticates with a JWT, only with the
// service-account token that bootstrap produces.
type JWT struct {
	privateKey *rsa.PrivateKey
}

// NewJWT generates a fresh 2048-bit RSA keypair.
func NewJWT() (*JWT, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}
	return &JWT{privateKey: key}, nil
}

// PublicKeyPEM returns the PEM-encoded public key, for platform-api's
// TOKENIZER_0_PUBLICKEY env var.
func (j *JWT) PublicKeyPEM() (string, error) {
	der, err := x509.MarshalPKIXPublicKey(&j.privateKey.PublicKey)
	if err != nil {
		return "", fmt.Errorf("marshal public key: %w", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})), nil
}

// Mint signs a short-lived RS256 JWT for subject sub.
func (j *JWT) Mint(sub string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub": sub,
		"iss": "TEST",
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	signed, err := token.SignedString(j.privateKey)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signed, nil
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./testing/integration/helper/... -v`
Expected: PASS

- [ ] **Step 6: Implement the container stack**

```go
// testing/container/containers.go
//go:build integration

package container

import (
	"context"
	"fmt"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	platformImage = "ghcr.io/trustattic/platform:v2.0.0-RC3"
	cloudapiImage = "ghcr.io/trustattic/cloudapi:v0.2.1"

	// issuerSecretKey is the same test/dev PASETO key platform's own
	// docker-compose.yaml uses for ISSUER_SECRETKEY — it governs only
	// platform's own self-issued tokens (e.g. service-account tokens), not
	// what this test trusts as an incoming login credential (that's the
	// per-run RSA keypair from helper.JWT, passed in as jwtPublicKeyPEM).
	issuerSecretKey = "8f77272d4bb2d599747bce36e6175db27e042c482986f22ba7dc8640ee92731f3d57f5ddc11bc565553dc047e3c2ad54973fd292343ae08f2b507b8d40c201d9"
)

// Stack is a running postgres + cloudapi + platform-api topology, mirroring
// platform/docker-compose.yaml, for CLI integration tests. The CLI module
// does not import platform's Go code; it drives the published images as a
// black box over HTTP.
type Stack struct {
	net      *testcontainers.DockerNetwork
	pg       *postgres.PostgresContainer
	cloudapi testcontainers.Container
	platform testcontainers.Container

	// PlatformURL is the host-reachable base URL of platform-api's API
	// (e.g. "http://localhost:32871/api/v2").
	PlatformURL string
}

// Start brings up the stack, configuring platform-api to trust JWTs signed
// by the keypair whose PEM-encoded public key is jwtPublicKeyPEM.
func Start(ctx context.Context, jwtPublicKeyPEM string) (*Stack, error) {
	net, err := network.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("create network: %w", err)
	}

	pg, err := postgres.Run(ctx, "postgres:17-alpine",
		postgres.WithDatabase("platform"),
		postgres.WithUsername("platform"),
		postgres.WithPassword("platform"),
		postgres.BasicWaitStrategies(),
		network.WithNetwork([]string{"postgres"}, net),
	)
	if err != nil {
		return nil, fmt.Errorf("start postgres: %w", err)
	}

	cloudapiC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:          cloudapiImage,
			Networks:       []string{net.Name},
			NetworkAliases: map[string][]string{net.Name: {"cloudapi"}},
			ExposedPorts:   []string{"50051/tcp"},
			WaitingFor:     wait.ForListeningPort("50051/tcp"),
		},
		Started: true,
	})
	if err != nil {
		return nil, fmt.Errorf("start cloudapi: %w", err)
	}

	platformC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        platformImage,
			Cmd:          []string{"api"},
			Networks:     []string{net.Name},
			ExposedPorts: []string{"8080/tcp"},
			Env: map[string]string{
				"DB_SOURCE":             "postgres://platform:platform@postgres:5432/platform?sslmode=disable",
				"DB_MIGRATIONSDIR":      "/var/run/ko/migrations",
				"JOB_SERVICE":           "river",
				"JOB_DBSOURCE":          "postgres://platform:platform@postgres:5432/platform?sslmode=disable",
				"TOKENCIPHER_KEY":       "N1PCdw3M2B1TfJhoaY2mL736p2vCUc47",
				"CONNECTIONCIPHER_KEY":  "N1PCdw3M2B1TfJhoaY2mL736p2vCUc47",
				"TOKENIZER_0_KIND":      "jwt",
				"TOKENIZER_0_PUBLICKEY": jwtPublicKeyPEM,
				"ISSUER_KIND":           "paseto",
				"ISSUER_SECRETKEY":      issuerSecretKey,
				"CLOUDAPI_URL":          "cloudapi:50051",
				"LOGLEVEL":              "debug",
			},
			WaitingFor: wait.ForHTTP("/api/v2/healthcheck").
				WithPort("8080/tcp").
				WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		return nil, fmt.Errorf("start platform: %w", err)
	}

	host, err := platformC.Host(ctx)
	if err != nil {
		return nil, fmt.Errorf("platform host: %w", err)
	}
	port, err := platformC.MappedPort(ctx, "8080")
	if err != nil {
		return nil, fmt.Errorf("platform mapped port: %w", err)
	}

	return &Stack{
		net:         net,
		pg:          pg,
		cloudapi:    cloudapiC,
		platform:    platformC,
		PlatformURL: fmt.Sprintf("http://%s:%s/api/v2", host, port.Port()),
	}, nil
}

// Terminate stops every container and removes the network.
func (s *Stack) Terminate(ctx context.Context) {
	_ = s.platform.Terminate(ctx)
	_ = s.cloudapi.Terminate(ctx)
	_ = s.pg.Terminate(ctx)
	_ = s.net.Remove(ctx)
}
```

The exact field/function names above (`network.WithNetwork`,
`ContainerRequest.NetworkAliases`, `wait.ForHTTP(...).WithPort(...)`) are
from `testcontainers-go`'s general shape and may differ slightly by the
version `go get` resolves — if `go vet ./testing/... -tags=integration`
reports an unknown field or function, run
`go doc github.com/testcontainers/testcontainers-go ContainerRequest` (and
the `network`/`wait` subpackages) and adjust to match.

- [ ] **Step 7: Verify it at least builds under the integration tag**

Run: `go build -tags=integration ./testing/...`
Expected: PASS (no Docker needed yet — this only proves the code compiles;
Task 17 is what actually starts it).

- [ ] **Step 8: Commit**

```bash
git add testing/integration/helper testing/container go.mod go.sum
git commit -m "Add integration test container stack and a bootstrap JWT helper"
```

---

### Task 17: Integration test — bootstrap a service account, run the CLI end-to-end

**Files:**
- Create: `testing/integration/suite.go` (build-tagged `integration`)
- Create: `testing/integration/main_test.go` (build-tagged `integration`)
- Create: `testing/integration/project_test.go` (build-tagged `integration`)

**Interfaces:**
- Consumes: `container.Start`, `container.Stack` (Task 16); `helper.NewJWT`, `(*JWT) PublicKeyPEM`, `(*JWT) Mint` (Task 16); `client.NewClientWithResponses` and its typed methods (Task 2).
- Produces: `integration.BaseTestSuite{Token, Project string}` with `RunCLI(args ...string) (stdout string, err error)`, and `integration.BinaryPath` (set by `TestMain`) — this is the plan's final deliverable: a real end-to-end proof the generated CLI works against a real platform instance.

- [ ] **Step 1: Write suite.go (container bring-up + bootstrap)**

```go
// testing/integration/suite.go
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
	s.Token = *tokenResp.JSON201.Token
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
```

Two spots to verify against the real generated client once this compiles:

1. `AccountTokensPostJSONRequestBody{Name: ...}` and reading the minted
   secret back as `tokenResp.JSON201.Token` — run
   `go doc ./internal/client AccountTokensPostJSONRequestBody` and
   `go doc ./internal/client AccountTokensPostResp` to confirm the real
   request/response field names (the response field holding the actual
   secret value may be named differently, e.g. `Identifier` or `Secret`) and
   fix both to match.
2. `AccountPostJSONRequestBody{Scope: &client.PermissionsScopes{...}}` and
   `client.Permissions{"project/manage"}` — confirm these generated type
   names via `go doc ./internal/client AccountPostJSONRequestBody`.

- [ ] **Step 2: Write TestMain (builds the binary once per test run)**

```go
// testing/integration/main_test.go
//go:build integration

package integration_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/trustattic/trustattic-cli/testing/integration"
)

func TestMain(m *testing.M) {
	tmpDir, err := os.MkdirTemp("", "trustattic-cli-integration")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)

	bin := filepath.Join(tmpDir, "trustattic")
	build := exec.Command("go", "build", "-o", bin, "../../cmd/trustattic")
	if out, err := build.CombinedOutput(); err != nil {
		panic("build trustattic binary: " + err.Error() + "\n" + string(out))
	}
	integration.BinaryPath = bin

	os.Exit(m.Run())
}
```

- [ ] **Step 3: Write the list/create/list test**

```go
// testing/integration/project_test.go
//go:build integration

package integration_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/trustattic/trustattic-cli/testing/integration"
)

func TestProjectFlow(t *testing.T) {
	suite.Run(t, new(ProjectFlowSuite))
}

type ProjectFlowSuite struct {
	integration.BaseTestSuite
}

type projectListOutput struct {
	Data []struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	} `json:"data"`
}

func (s *ProjectFlowSuite) TestListCreateList() {
	out, err := s.RunCLI("project", "list", "--output", "json")
	s.Require().NoError(err, out)

	var before projectListOutput
	s.Require().NoError(json.Unmarshal([]byte(out), &before))

	out, err = s.RunCLI("project", "create", "--name", "cli-created-project", "--output", "json")
	s.Require().NoError(err, out)

	out, err = s.RunCLI("project", "list", "--output", "json")
	s.Require().NoError(err, out)

	var after projectListOutput
	s.Require().NoError(json.Unmarshal([]byte(out), &after))

	found := false
	for _, p := range after.Data {
		if p.Name == "cli-created-project" {
			found = true
		}
	}
	s.Require().True(found, "created project not found in second 'project list': %s", out)
	s.Require().Greater(len(after.Data), len(before.Data))
}
```

- [ ] **Step 4: Run the integration test (requires Docker)**

Run: `go test -tags=integration ./testing/... -v -timeout 5m`
Expected: PASS. If `SetupSuite` fails, check the container startup order
first (the failure message will show which container/HTTP call failed); if
`RunCLI` calls fail with a 401, the bootstrap's `TOKENIZER_0_PUBLICKEY` env
var or the minted JWT's claims are the first thing to check.

- [ ] **Step 5: Commit**

```bash
git add testing/integration/suite.go testing/integration/main_test.go testing/integration/project_test.go
git commit -m "Add the end-to-end project list/create/list integration test"
```

---

### Task 18: Finalize the Makefile and README

**Files:**
- Modify: `Makefile`
- Modify: `README.md`

**Interfaces:** none — this task only wires together commands already implemented in Tasks 2-17.

- [ ] **Step 1: Rewrite the Makefile with every target**

```makefile
.PHONY: generate update-spec build test test-integration

update-spec:
	cp ../platform/api.yaml spec/api.yaml

generate:
	cd tools && go generate -tags=tools ./...
	go run ./cmd/gen

build:
	go build -o dist/trustattic ./cmd/trustattic

test:
	go test ./...

test-integration:
	go test -tags=integration ./testing/... -v -timeout 5m
```

- [ ] **Step 2: Update README.md**

```markdown
# TrustAttic CLI

A command-line client for the TrustAttic Platform API, generated from its
OpenAPI spec. See
[`docs/superpowers/specs/2026-09-17-trustattic-cli-design.md`](docs/superpowers/specs/2026-09-17-trustattic-cli-design.md)
for the design.

## Install / build

```bash
make build          # -> dist/trustattic
```

## Usage

```bash
trustattic login --token <service-account-token>
trustattic use my-project        # set a default --project for every command below
trustattic project list
trustattic project create --name "New Project"
trustattic backup list
trustattic schedule run <schedule_id>
```

Pass `--output json` (or pipe to another program) for raw JSON instead of
styled tables.

`TRUSTATTIC_API_URL` overrides the API base URL (default
`https://api.trustattic.com`); `TRUSTATTIC_TOKEN` overrides the stored token,
for CI.

## Development

```bash
make update-spec    # copy ../platform/api.yaml -> spec/api.yaml
make generate        # regenerate internal/client and internal/commands/generated
make test            # unit tests, no Docker required
make test-integration # end-to-end test against real platform/cloudapi containers (needs Docker)
```

The command tree, flags, and request wiring are generated from `spec/api.yaml`
— see the design doc for the naming algorithm. Never hand-edit a `*.gen.go`
file; fix `internal/generator` and re-run `make generate`.
```

- [ ] **Step 3: Run the full unit test suite one more time**

Run: `make test`
Expected: PASS, everything from Tasks 1-15.

- [ ] **Step 4: Commit**

```bash
git add Makefile README.md
git commit -m "Finalize Makefile targets and document CLI usage in README"
```

---

## Plan Self-Review

**Spec coverage:**
- Two-stage generation (typed client + Cobra tree) — Tasks 2, 10-15.
- Naming algorithm, verified against all 32 real operations — Tasks 11, 15 (`register_test.go`).
- Path params -> flags/positional, `project_slug` optional exception — Tasks 11, 12, 14.
- Request body -> typed flags — Tasks 10, 12, 14.
- Override table — Task 13.
- Charm.land styling (Fang, table/kv rendering, styled errors, huh login prompt) — Tasks 1, 5, 6, 7.
- Output modes (TTY-detected, `--output json`) — Task 5, applied in every generated command (Task 14).
- Auth & config (service-account-only, config file, env overrides) — Tasks 3, 4, 7.
- Current project (`trustattic use`, optional `--project` fallback) — Tasks 8, 14.
- Repo layout — matches the File Structure section (adjusted below vs. the design spec's sketch: the generator's CLI entrypoint lives at `cmd/gen`, not inside `internal/generator`, since a directory can only hold one Go package — noted in Task 15).
- Integration testing (testcontainers topology, JWT-bootstrap-only, service-account CLI usage, list/create/list) — Tasks 16-17.

**Placeholder scan:** none found — every step has complete code or a
concrete shell command; the few "verify against `go doc`" notes name the
exact command and exact symbol to check, not an open-ended TODO.

**Type consistency check:** `cli.Config{Token, CurrentProject}` (Task 3) is
read/written identically in Tasks 4, 7, 8, and referenced by the generated
code's `cfg.CurrentProject` fallback (Task 14). `cli.Mode`/`ModeAuto`/
`ModeJSON` (Task 5) and `cli.Render`'s signature match its call site in Task
14's `renderCommandFunc`. `generator.Operation`/`Param`/`BodyProp` (Task 10)
flow unchanged into `CommandSpec` (Task 11), `GeneratedCommand`/`FlagDef`
(Task 12), `Override`/`ApplyOverrides` (Task 13), and `EmitTagFile`/
`EmitRegisterFile` (Task 14) — no field renamed or reshaped along the way.
`container.Stack.PlatformURL` (Task 16) is what `BaseTestSuite.SetupSuite`
(Task 17) both calls `client.NewClientWithResponses` on and later passes to
`RunCLI` as `TRUSTATTIC_API_URL`.

A third correction, made during the subagent-driven-development preflight
scan (after this plan was first approved, before Task 1 was dispatched):
the positional-vs-flag rule as originally written contradicted 7 rows of
the design spec's own worked command tree. Fixed by keeping the simpler,
fully mechanical rule (positional iff the URL literally ends in that path
param) as authoritative and correcting the 7 worked examples to match — see
the design spec's dated note under its command tree, and this plan's
Task 11 (`BuildCommandSpecs`, 3 tests) and Task 14 (`emit.go` call-arg
ordering, which had the same bug independently: it built path-parameter
call arguments positional-first instead of in URL order).

Two further corrections made during the plan's own self-review, both fixed inline in the File Structure
section above rather than left as discrepancies: it originally sketched a
separate `internal/generator/templates/command.go.tmpl` file, which Task 14
doesn't use (it builds emitted source with plain Go string-building —
`text/template` only wraps the package/import skeleton — since deeply nested
template conditionals for code this branchy are a maintenance risk); and it
originally placed the generator's entrypoint at `internal/generator/main.go`,
which can't work since that directory is already `package generator` — Task
15 places it at `cmd/gen/main.go` instead, alongside `cmd/trustattic`.


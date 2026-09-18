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

// TestSaveConfig_TightensPermissionsOnAnExistingLooserFile covers the fact
// that os.WriteFile's mode argument only applies when it *creates* the file:
// a config file that already exists as 0644 (however it got that way) kept
// those permissions across every later `trustattic login`, leaving the
// service-account token world-readable.
func TestSaveConfig_TightensPermissionsOnAnExistingLooserFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	configDir := filepath.Join(dir, "trustattic")
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	path := filepath.Join(configDir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("token: stale\n"), 0o644))
	require.NoError(t, os.Chmod(path, 0o644))
	require.NoError(t, os.Chmod(configDir, 0o755))

	require.NoError(t, cli.SaveConfig(cli.Config{Token: "svc-abc"}))

	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	dirInfo, err := os.Stat(configDir)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o700), dirInfo.Mode().Perm())
}

// TestDefaultAPIURL_IncludesTheSpecsBasePath guards the base-path prefix:
// oapi-codegen's client doesn't embed the spec's `servers:` block, so a base
// URL without /api/v2 would 404 against every real endpoint.
func TestDefaultAPIURL_IncludesTheSpecsBasePath(t *testing.T) {
	require.Equal(t, "https://api.trustattic.com/api/v2", cli.DefaultAPIURL)
}

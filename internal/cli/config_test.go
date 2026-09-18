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

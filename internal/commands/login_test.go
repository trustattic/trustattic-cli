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

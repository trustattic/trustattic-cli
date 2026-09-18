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

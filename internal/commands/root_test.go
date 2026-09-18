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

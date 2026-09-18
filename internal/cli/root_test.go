package cli_test

import (
	"io"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"github.com/trustattic/trustattic-cli/internal/cli"
)

func TestNewRootCommand_HasNameAndOutputFlag(t *testing.T) {
	root := cli.NewRootCommand()
	require.Equal(t, "trustattic", root.Use)

	flag := root.PersistentFlags().Lookup("output")
	require.NotNil(t, flag, "expected a persistent --output flag")
}

func TestNewRootCommand_OutputFlagHasShorthandO(t *testing.T) {
	root := cli.NewRootCommand()

	flag := root.PersistentFlags().Lookup("output")
	require.NotNil(t, flag)
	require.Equal(t, "o", flag.Shorthand, "the design spec documents both --output json and -o json")
	require.Same(t, flag, root.PersistentFlags().ShorthandLookup("o"))
}

func TestNewRootCommand_RejectsUnknownOutputValue(t *testing.T) {
	root := cli.NewRootCommand()
	root.SetArgs([]string{"--output", "yaml"})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.RunE = func(cmd *cobra.Command, args []string) error { return nil }

	err := root.Execute()
	require.Error(t, err, "an unsupported --output value must not silently fall back to styled output")
	require.Contains(t, err.Error(), `unknown --output value "yaml"`)
}

func TestNewRootCommand_AcceptsJSONAndEmptyOutputValues(t *testing.T) {
	for _, args := range [][]string{{}, {"--output", "json"}, {"-o", "json"}} {
		root := cli.NewRootCommand()
		root.SetArgs(args)
		root.SetOut(io.Discard)
		root.SetErr(io.Discard)
		root.RunE = func(cmd *cobra.Command, args []string) error { return nil }

		require.NoErrorf(t, root.Execute(), "args %v", args)
	}
}

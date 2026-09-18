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

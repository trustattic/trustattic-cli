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

package cli

import (
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
		// Cobra runs only the nearest PersistentPreRunE in the chain, and
		// no subcommand defines one, so this runs for every command and is
		// the single place --output is validated. Every command reads the
		// flag's value again for itself; this just refuses a value none of
		// them would understand, before any request is made.
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			out, err := cmd.Flags().GetString("output")
			if err != nil {
				return err
			}
			_, err = ParseMode(out)
			return err
		},
	}
	root.PersistentFlags().StringP("output", "o", "", `output mode: "json", or empty for interactive (auto-detected)`)
	return root
}

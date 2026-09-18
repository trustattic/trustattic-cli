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

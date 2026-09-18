package commands

import "github.com/spf13/cobra"

// Register attaches every hand-written command onto root.
func Register(root *cobra.Command) {
	root.AddCommand(NewLoginCommand())
	root.AddCommand(NewLogoutCommand())
	root.AddCommand(NewUseCommand())
}

package commands

import (
	"errors"
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"github.com/trustattic/trustattic-cli/internal/cli"
	"golang.org/x/term"
)

// NewLoginCommand builds `trustattic login`, which stores a service-account
// token in the config file. This is the only supported auth path: the CLI
// always authenticates as a service account, never a user via JWT.
func NewLoginCommand() *cobra.Command {
	var token string
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Store a TrustAttic service-account token",
		RunE: func(cmd *cobra.Command, args []string) error {
			if token == "" {
				if !term.IsTerminal(int(os.Stdin.Fd())) {
					return errors.New("a token is required: pass --token, or run interactively")
				}
				form := huh.NewForm(huh.NewGroup(
					huh.NewInput().
						Title("Service-account token").
						Password(true).
						Value(&token),
				))
				if err := form.Run(); err != nil {
					return err
				}
			}
			if token == "" {
				return errors.New("a token is required")
			}

			cfg, err := cli.LoadConfig()
			if err != nil {
				return err
			}
			cfg.Token = token
			if err := cli.SaveConfig(cfg); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Logged in.")
			return nil
		},
	}
	cmd.Flags().StringVar(&token, "token", "", "service-account token")
	return cmd
}

// NewLogoutCommand builds `trustattic logout`, which removes the stored
// token.
func NewLogoutCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove the stored service-account token",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := cli.LoadConfig()
			if err != nil {
				return err
			}
			cfg.Token = ""
			if err := cli.SaveConfig(cfg); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Logged out.")
			return nil
		},
	}
}

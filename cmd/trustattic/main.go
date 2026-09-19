package main

import (
	"context"
	"os"
	"syscall"

	"github.com/charmbracelet/fang"
	"github.com/trustattic/trustattic-cli/internal/cli"
	"github.com/trustattic/trustattic-cli/internal/commands"
	"github.com/trustattic/trustattic-cli/internal/commands/generated"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	root := cli.NewRootCommand()
	root.Version = version
	commands.Register(root)
	generated.Register(root)
	// WithNotifySignal is what makes fang install a signal.NotifyContext
	// around the context it passes to root.ExecuteContext (see fang's
	// Execute: it only does so when at least one signal is configured).
	// Without it, the cmd.Context() every generated command hands to its
	// client call is a plain context.Background(), and Ctrl-C kills the
	// process outright via Go's default signal disposition instead of
	// cancelling the in-flight request.
	//
	// WithVersion is required too: fang.Execute unconditionally overwrites
	// root.Version from build info (falling back to "unknown (built from
	// source)") unless a version is supplied via this option, so setting
	// root.Version above is not enough on its own.
	if err := fang.Execute(
		context.Background(),
		root,
		fang.WithVersion(version),
		fang.WithNotifySignal(os.Interrupt, syscall.SIGTERM),
	); err != nil {
		os.Exit(1)
	}
}

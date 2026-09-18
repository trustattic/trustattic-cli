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

func main() {
	root := cli.NewRootCommand()
	commands.Register(root)
	generated.Register(root)
	// WithNotifySignal is what makes fang install a signal.NotifyContext
	// around the context it passes to root.ExecuteContext (see fang's
	// Execute: it only does so when at least one signal is configured).
	// Without it, the cmd.Context() every generated command hands to its
	// client call is a plain context.Background(), and Ctrl-C kills the
	// process outright via Go's default signal disposition instead of
	// cancelling the in-flight request.
	if err := fang.Execute(
		context.Background(),
		root,
		fang.WithNotifySignal(os.Interrupt, syscall.SIGTERM),
	); err != nil {
		os.Exit(1)
	}
}

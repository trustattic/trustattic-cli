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
	// Setting root.Version here is effectively a no-op: fang.Execute
	// overwrites it (see the fang.WithVersion note below), which is what
	// actually supplies the version. Kept as harmless defensive wiring in case
	// the command is ever executed without going through fang.
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
	// WithVersion is what actually sets the reported version: fang.Execute
	// unconditionally overwrites root.Version from build info (falling back to
	// "unknown (built from source)") unless a version is supplied via this
	// option, so the root.Version assignment above does nothing on its own.
	if err := fang.Execute(
		context.Background(),
		root,
		fang.WithVersion(version),
		fang.WithNotifySignal(os.Interrupt, syscall.SIGTERM),
	); err != nil {
		os.Exit(1)
	}
}

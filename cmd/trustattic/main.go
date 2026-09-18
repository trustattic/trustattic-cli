package main

import (
	"context"
	"os"

	"github.com/charmbracelet/fang"
	"github.com/trustattic/trustattic-cli/internal/cli"
	"github.com/trustattic/trustattic-cli/internal/commands"
)

func main() {
	root := cli.NewRootCommand()
	commands.Register(root)
	if err := fang.Execute(context.Background(), root); err != nil {
		os.Exit(1)
	}
}

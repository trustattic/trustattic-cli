// internal/commands/generated/register_test.go
package generated_test

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
	"github.com/trustattic/trustattic-cli/internal/commands/generated"
)

func findCommand(t *testing.T, root *cobra.Command, path ...string) *cobra.Command {
	t.Helper()
	cur := root
	for _, name := range path {
		var next *cobra.Command
		for _, c := range cur.Commands() {
			if c.Name() == name {
				next = c
				break
			}
		}
		require.NotNilf(t, next, "no command %q under %q", name, cur.Name())
		cur = next
	}
	return cur
}

func TestRegister_BuildsTheDocumentedCommandTree(t *testing.T) {
	root := &cobra.Command{Use: "trustattic"}
	generated.Register(root)

	findCommand(t, root, "account", "list")
	findCommand(t, root, "account", "create")
	findCommand(t, root, "account", "update")
	findCommand(t, root, "account", "obtain")
	findCommand(t, root, "account", "tokens", "list")
	findCommand(t, root, "account", "tokens", "create")
	findCommand(t, root, "account", "tokens", "delete")

	findCommand(t, root, "project", "list")
	findCommand(t, root, "project", "create")
	findCommand(t, root, "project", "get")
	findCommand(t, root, "project", "update")
	findCommand(t, root, "project", "accounts")
	findCommand(t, root, "project", "invite")
	findCommand(t, root, "project", "permissions")

	findCommand(t, root, "backup", "list")
	findCommand(t, root, "backup", "restore", "create")
	findCommand(t, root, "backup", "restore", "get")

	findCommand(t, root, "connection", "list")
	findCommand(t, root, "connection", "create")
	findCommand(t, root, "connection", "update")
	findCommand(t, root, "connection", "delete")
	findCommand(t, root, "connection", "check")
	// ResourceGet is tagged "connection" in the real spec (no "resource" tag
	// exists) — confirmed in Task 11's report — so it lands here, not under
	// a standalone "resource" tag.
	findCommand(t, root, "connection", "resource")

	findCommand(t, root, "schedule", "list")
	findCommand(t, root, "schedule", "create")
	findCommand(t, root, "schedule", "update")
	findCommand(t, root, "schedule", "delete")
	findCommand(t, root, "schedule", "run")
	findCommand(t, root, "schedule", "history")

	findCommand(t, root, "healthcheck")
	findCommand(t, root, "permissions")
	findCommand(t, root, "external-types")
}

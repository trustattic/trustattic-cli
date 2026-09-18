package generator_test

import (
	"bytes"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/trustattic/trustattic-cli/internal/generator"
)

func fixtureCommands(t *testing.T) []generator.GeneratedCommand {
	t.Helper()
	ops, err := generator.LoadOperations("testdata/fixture.yaml")
	require.NoError(t, err)
	specs := generator.BuildCommandSpecs(ops)
	return generator.BuildGeneratedCommands(specs)
}

func TestEmitTagFile_FixtureSpec_ProducesValidGo(t *testing.T) {
	cmds := fixtureCommands(t)

	var buf bytes.Buffer
	require.NoError(t, generator.EmitTagFile(&buf, cmds))

	_, err := parser.ParseFile(token.NewFileSet(), "widget.gen.go", buf.Bytes(), parser.AllErrors)
	require.NoError(t, err, "emitted source:\n%s", buf.String())

	out := buf.String()
	require.Contains(t, out, "func NewWidgetGetCommand() *cobra.Command")
	require.Contains(t, out, "func NewWidgetPostCommand() *cobra.Command")
	require.Contains(t, out, "func NewWidgetDeleteCommand() *cobra.Command")
}

func TestEmitTagFile_PositionalArgUsesExactArgsOne(t *testing.T) {
	cmds := fixtureCommands(t)

	var buf bytes.Buffer
	require.NoError(t, generator.EmitTagFile(&buf, cmds))

	require.Contains(t, buf.String(), `Use:  "delete <widget_id>"`)
	require.Contains(t, buf.String(), "Args: cobra.ExactArgs(1)")
}

func TestEmitTagFile_RequiredBodyPropIsMarkedRequired(t *testing.T) {
	cmds := fixtureCommands(t)

	var buf bytes.Buffer
	require.NoError(t, generator.EmitTagFile(&buf, cmds))

	out := buf.String()
	require.Contains(t, out, `cmd.Flags().StringVar(&name, "name", "", "name")`)
	require.Contains(t, out, `cmd.MarkFlagRequired("name")`)
	require.NotContains(t, out, `MarkFlagRequired("active")`) // not required in the fixture
}

func TestEmitRegisterFile_FixtureSpec_ProducesValidGo(t *testing.T) {
	cmds := fixtureCommands(t)

	var buf bytes.Buffer
	require.NoError(t, generator.EmitRegisterFile(&buf, cmds))

	_, err := parser.ParseFile(token.NewFileSet(), "register.gen.go", buf.Bytes(), parser.AllErrors)
	require.NoError(t, err, "emitted source:\n%s", buf.String())

	out := buf.String()
	require.Contains(t, out, `widgetCmd := getOrAddCommand(root, "widget")`)
	require.Contains(t, out, "widgetCmd.AddCommand(NewWidgetGetCommand())")
	require.Contains(t, out, "widgetCmd.AddCommand(NewWidgetPostCommand())")
	require.Contains(t, out, "widgetCmd.AddCommand(NewWidgetDeleteCommand())")
	// the parent line must appear exactly once even though three commands share it
	require.Equal(t, 1, strings.Count(out, `widgetCmd := getOrAddCommand`))
}

func TestGroupByTag_GroupsFixtureCommandsUnderWidget(t *testing.T) {
	cmds := fixtureCommands(t)
	groups := generator.GroupByTag(cmds)
	require.Len(t, groups["widget"], 4)
}

// TestEmitTagFile_FlagPathParamOrderedBeforePositionalInCallSite is a
// regression test for the bug this task was written to fix: call arguments
// for path parameters must be emitted in URL order (leading flag-style
// params first, then the trailing positional last), not positional-first.
// WidgetPartGet's path is /widget/{widget_id}/part/{part_id}: widget_id is
// a leading flag (Go ident "widgetID") and part_id is the trailing
// positional (args[0]). A reversion to positional-first would still parse
// as valid Go and every other fixture-based test would still pass, so this
// asserts on the actual emitted call-site text, not just parseability.
func TestEmitTagFile_FlagPathParamOrderedBeforePositionalInCallSite(t *testing.T) {
	cmds := fixtureCommands(t)

	var buf bytes.Buffer
	require.NoError(t, generator.EmitTagFile(&buf, cmds))

	out := buf.String()
	require.Contains(t, out, "func NewWidgetPartGetCommand() *cobra.Command")
	require.Contains(t, out,
		"apiClient.WidgetPartGetWithResponse(\n"+
			"\t\t\t\tcontext.Background(),\n"+
			"\t\t\t\twidgetID,\n"+
			"\t\t\t\targs[0],\n"+
			"\t\t\t\t&client.WidgetPartGetParams{},\n"+
			"\t\t\t)",
	)
}

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

// formatsCommands loads testdata/formats.yaml - a fixture dedicated to the
// UUID/email/date-time conversion, required-vs-pointer, nil-body, and
// required-non-flat-property behavior added in Task 15, kept separate from
// testdata/fixture.yaml so it doesn't disturb that fixture's exact
// operation/group counts asserted elsewhere.
func formatsCommands(t *testing.T) []generator.GeneratedCommand {
	t.Helper()
	ops, err := generator.LoadOperations("testdata/formats.yaml")
	require.NoError(t, err)
	specs := generator.BuildCommandSpecs(ops)
	return generator.BuildGeneratedCommands(specs)
}

// extractFunc returns just the body of one "func New<name>Command() ..."
// declaration out of a full emitted tag file, so a test can assert what a
// specific command does (or doesn't do) without being confused by sibling
// commands in the same file.
func extractFunc(t *testing.T, src, name string) string {
	t.Helper()
	marker := "func New" + name + "Command() *cobra.Command {"
	start := strings.Index(src, marker)
	require.GreaterOrEqualf(t, start, 0, "no %s in emitted source:\n%s", marker, src)
	rest := src[start+len(marker):]
	end := strings.Index(rest, "\nfunc ")
	if end < 0 {
		end = len(rest)
	}
	return rest[:end]
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

func TestEmitTagFile_FormatsFixture_ProducesValidGo(t *testing.T) {
	cmds := formatsCommands(t)

	var buf bytes.Buffer
	require.NoError(t, generator.EmitTagFile(&buf, cmds))

	_, err := parser.ParseFile(token.NewFileSet(), "gadget.gen.go", buf.Bytes(), parser.AllErrors)
	require.NoError(t, err, "emitted source:\n%s", buf.String())
}

// TestEmitTagFile_RequiredEmailBodyProp_IsAPlainValueNotAPointer covers the
// required-vs-pointer branching in bodyPropExpr: oapi-codegen makes a
// *required* body property a plain value in the real generated struct, so
// the emitted literal must assign the converted value directly, not take
// its address.
func TestEmitTagFile_RequiredEmailBodyProp_IsAPlainValueNotAPointer(t *testing.T) {
	cmds := formatsCommands(t)

	var buf bytes.Buffer
	require.NoError(t, generator.EmitTagFile(&buf, cmds))

	fn := extractFunc(t, buf.String(), "GadgetPut")
	require.Contains(t, fn, "OwnerEmail: openapi_types.Email(ownerEmail),")
	// &ownerEmail legitimately appears in the StringVar flag-binding call
	// below; what must NOT appear is the body literal taking its address.
	require.NotContains(t, fn, "OwnerEmail: &ownerEmail")
}

// TestEmitTagFile_OptionalDateTimeBodyProp_GuardsEmptyBeforeParsing covers
// the empty-string guard for an optional date-time body property: an unset
// optional flag must produce a nil pointer, not a parse error from feeding
// "" to time.Parse.
func TestEmitTagFile_OptionalDateTimeBodyProp_GuardsEmptyBeforeParsing(t *testing.T) {
	cmds := formatsCommands(t)

	var buf bytes.Buffer
	require.NoError(t, generator.EmitTagFile(&buf, cmds))

	fn := extractFunc(t, buf.String(), "GadgetPut")
	require.Contains(t, fn, `var reminderAtParsed *gotime.Time`)
	require.Contains(t, fn, `if reminderAt != "" {`)
	require.Contains(t, fn, `parsed, err := gotime.Parse(gotime.RFC3339, reminderAt)`)
	require.Contains(t, fn, `reminderAtParsed = &parsed`)
	require.Contains(t, fn, "ReminderAt: reminderAtParsed,")
}

// TestEmitTagFile_UUIDPositionalPathParam_ParsesBeforeCall covers
// convertPathParam's UUID branch for the trailing positional argument
// (args[0]), including the "gotime" package staying out of the way (this
// uses the "uuid" package, not "time").
func TestEmitTagFile_UUIDPositionalPathParam_ParsesBeforeCall(t *testing.T) {
	cmds := formatsCommands(t)

	var buf bytes.Buffer
	require.NoError(t, generator.EmitTagFile(&buf, cmds))

	fn := extractFunc(t, buf.String(), "GadgetPut")
	require.Contains(t, fn, "gadgetIDParsed, err := uuid.Parse(args[0])")
	require.Contains(t, fn, `return fmt.Errorf("invalid <gadget_id>: %w", err)`)
	require.Contains(t, fn, "gadgetIDParsed,\n") // used as the call argument
}

// TestEmitTagFile_UUIDFlagPathParam_ParsesBeforeCall covers
// convertPathParam's UUID branch for a non-positional path-parameter flag
// (GadgetResetPost's URL ends in the static segment "reset", so gadget_id
// is a --gadget-id flag, not the positional argument).
func TestEmitTagFile_UUIDFlagPathParam_ParsesBeforeCall(t *testing.T) {
	cmds := formatsCommands(t)

	var buf bytes.Buffer
	require.NoError(t, generator.EmitTagFile(&buf, cmds))

	fn := extractFunc(t, buf.String(), "GadgetResetPost")
	require.Contains(t, fn, "gadgetIDParsed, err := uuid.Parse(gadgetID)")
	require.Contains(t, fn, `return fmt.Errorf("invalid --gadget-id: %w", err)`)
}

// TestEmitTagFile_EmptySchemaBody_PassesNilNotAnEmptyStructLiteral covers
// the nil-body fallback: GadgetResetPost's request body is `schema: {}`,
// which oapi-codegen types as interface{} - an empty
// client.GadgetResetPostJSONRequestBody{} composite literal wouldn't
// compile against that, so a body with zero surviving flat properties must
// be passed as a literal nil instead.
func TestEmitTagFile_EmptySchemaBody_PassesNilNotAnEmptyStructLiteral(t *testing.T) {
	cmds := formatsCommands(t)

	var buf bytes.Buffer
	require.NoError(t, generator.EmitTagFile(&buf, cmds))

	fn := extractFunc(t, buf.String(), "GadgetResetPost")
	require.Contains(t, fn,
		"apiClient.GadgetResetPostWithResponse(\n"+
			"\t\t\t\tcontext.Background(),\n"+
			"\t\t\t\tgadgetIDParsed,\n"+
			"\t\t\t\t&client.GadgetResetPostParams{},\n"+
			"\t\t\t\tnil,\n"+
			"\t\t\t)",
	)
	require.NotContains(t, fn, "client.GadgetResetPostJSONRequestBody{")
}

// TestEmitTagFile_RequiredNonFlatBodyProp_FailsFastWithoutTouchingConfigOrNetwork
// is the regression test for Finding 1 of the Task 15 review: GadgetPost's
// body requires "tags" (an array, no flat CLI representation). Silently
// omitting it from the request would let the command run and send an
// incomplete request; instead RunE must return a clear, immediate error and
// never reach cli.LoadConfig/cli.NewAPIClient/apiClient at all. The
// surviving flat property ("nickname") must still get a real flag,
// declared and registered exactly as for any other command - only RunE
// changes.
func TestEmitTagFile_RequiredNonFlatBodyProp_FailsFastWithoutTouchingConfigOrNetwork(t *testing.T) {
	cmds := formatsCommands(t)

	var buf bytes.Buffer
	require.NoError(t, generator.EmitTagFile(&buf, cmds))

	fn := extractFunc(t, buf.String(), "GadgetPost")
	require.Contains(t, fn,
		`return fmt.Errorf("this command is not yet supported: request body field(s) \"tags\" (type \"array\") cannot be set via CLI flags")`,
	)
	require.NotContains(t, fn, "cli.LoadConfig")
	require.NotContains(t, fn, "cli.NewAPIClient")
	require.NotContains(t, fn, "apiClient")
	require.NotContains(t, fn, "client.GadgetPostJSONRequestBody")

	// The surviving flat property is still a real, registered flag.
	require.Contains(t, fn, `cmd.Flags().StringVar(&nickname, "nickname", "", "nickname")`)
}

// TestEmitTagFile_UnsupportedStubCommand_DoesNotForceConversionImports
// covers the EmitTagFile-level fix alongside Finding 1: a command whose
// RunE is the "not yet supported" stub never emits a uuid.Parse/gotime.Parse
// call or a client.-qualified expression, so its Flags/BodyFlags ValueKinds
// and HasParams/HasBody must not force those imports into the file on
// their own. (Other commands in testdata/formats.yaml.gen.go do need
// "uuid" and "internal/client" - this asserts the whole file's import list,
// not just this one function, so it only holds because GadgetPost is the
// only command whose *own* shape would otherwise justify them and it's
// excluded correctly.)
func TestEmitTagFile_UnsupportedStubCommand_StillCompilesWithSharedFileImports(t *testing.T) {
	// GadgetPost alone (no sibling commands needing uuid/client) must not
	// pull in imports it never references.
	ops, err := generator.LoadOperations("testdata/formats.yaml")
	require.NoError(t, err)
	var postOnly []generator.Operation
	for _, op := range ops {
		if op.OperationID == "GadgetPost" {
			postOnly = append(postOnly, op)
		}
	}
	require.Len(t, postOnly, 1)
	specs := generator.BuildCommandSpecs(postOnly)
	cmds := generator.BuildGeneratedCommands(specs)

	var buf bytes.Buffer
	require.NoError(t, generator.EmitTagFile(&buf, cmds))

	_, err = parser.ParseFile(token.NewFileSet(), "gadget_post_only.gen.go", buf.Bytes(), parser.AllErrors)
	require.NoError(t, err, "emitted source:\n%s", buf.String())

	out := buf.String()
	require.NotContains(t, out, `"github.com/google/uuid"`)
	require.NotContains(t, out, `"github.com/trustattic/trustattic-cli/internal/client"`)
	require.NotContains(t, out, `gotime "time"`)
	require.NotContains(t, out, `openapi_types "github.com/oapi-codegen/runtime/types"`)
}

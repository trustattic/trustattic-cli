// internal/generator/flags_test.go
package generator_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/trustattic/trustattic-cli/internal/generator"
)

func TestFlagName_DropsTrailingSlugKeepsID(t *testing.T) {
	require.Equal(t, "project", generator.FlagName("project_slug"))
	require.Equal(t, "account-id", generator.FlagName("account_id"))
	require.Equal(t, "backup-id", generator.FlagName("backup_id"))
}

func TestGoIdent_CapitalizesIDSuffixCamelCase(t *testing.T) {
	require.Equal(t, "accountID", generator.GoIdent("account-id"))
	require.Equal(t, "project", generator.GoIdent("project"))
	require.Equal(t, "restoreID", generator.GoIdent("restore-id"))
}

func TestFlagKindForType_MapsKnownOpenAPITypes(t *testing.T) {
	require.Equal(t, generator.KindBool, generator.FlagKindForType("boolean"))
	require.Equal(t, generator.KindInt, generator.FlagKindForType("integer"))
	require.Equal(t, generator.KindString, generator.FlagKindForType("string"))
	require.Equal(t, generator.KindString, generator.FlagKindForType(""))
}

func TestBuildGeneratedCommands_ProjectSlugFlagIsOptional(t *testing.T) {
	specs := []generator.CommandSpec{
		{
			Tag:  "backup",
			Verb: "list",
			Flags: []generator.Param{
				{Name: "project_slug"},
			},
		},
	}
	got := generator.BuildGeneratedCommands(specs)
	require.Len(t, got[0].Flags, 1)
	require.Equal(t, "project", got[0].Flags[0].Name)
	require.True(t, got[0].Flags[0].Optional)
	require.False(t, got[0].Flags[0].Required)
}

func TestBuildGeneratedCommands_OtherPathParamFlagsAreRequired(t *testing.T) {
	specs := []generator.CommandSpec{
		{
			Tag:  "account",
			Verb: "tokens-list",
			Flags: []generator.Param{
				{Name: "account_id"},
			},
		},
	}
	got := generator.BuildGeneratedCommands(specs)
	require.Len(t, got[0].Flags, 1)
	require.Equal(t, "account-id", got[0].Flags[0].Name)
	require.Equal(t, "accountID", got[0].Flags[0].GoIdent)
	require.True(t, got[0].Flags[0].Required)
	require.False(t, got[0].Flags[0].Optional)
}

func TestBuildGeneratedCommands_PositionalComesFromSpec(t *testing.T) {
	positional := generator.Param{Name: "token_id"}
	specs := []generator.CommandSpec{
		{Tag: "account", Verb: "delete", Positional: &positional},
	}
	got := generator.BuildGeneratedCommands(specs)
	require.NotNil(t, got[0].PositionalFlag)
	require.Equal(t, "token-id", got[0].PositionalFlag.Name)
	require.Equal(t, "tokenID", got[0].PositionalFlag.GoIdent)
}

func TestBuildGeneratedCommands_FormatDrivenValueKinds(t *testing.T) {
	positional := generator.Param{Name: "gadget_id", Format: "uuid"}
	specs := []generator.CommandSpec{
		{
			Tag:        "gadget",
			Verb:       "update",
			Positional: &positional,
			Flags: []generator.Param{
				{Name: "owner_id", Format: "uuid"},
				{Name: "project_slug"}, // no format - plain, and the one optional path flag
			},
			Operation: generator.Operation{
				HasBody: true,
				BodyProps: []generator.BodyProp{
					{Name: "owner_email", Type: "string", Format: "email", Required: true},
					{Name: "reminder_at", Type: "string", Format: "date-time", Required: false},
					{Name: "nickname", Type: "string", Required: false},
				},
			},
		},
	}
	got := generator.BuildGeneratedCommands(specs)
	require.Equal(t, generator.ValueUUID, got[0].PositionalFlag.Value)

	byName := map[string]generator.FlagDef{}
	for _, f := range got[0].Flags {
		byName[f.Name] = f
	}
	require.Equal(t, generator.ValueUUID, byName["owner-id"].Value)
	require.Equal(t, generator.ValuePlain, byName["project"].Value)

	byBodyName := map[string]generator.FlagDef{}
	for _, f := range got[0].BodyFlags {
		byBodyName[f.Name] = f
	}
	require.Equal(t, generator.ValueEmail, byBodyName["owner-email"].Value)
	require.Equal(t, generator.ValueDateTime, byBodyName["reminder-at"].Value)
	require.Equal(t, generator.ValuePlain, byBodyName["nickname"].Value)
}

func TestBuildGeneratedCommands_RequiredNonFlatBodyProp_RecordedAsUnsupportedAndExcludedFromFlags(t *testing.T) {
	specs := []generator.CommandSpec{
		{
			Tag:  "gadget",
			Verb: "create",
			Operation: generator.Operation{
				HasBody: true,
				BodyProps: []generator.BodyProp{
					{Name: "tags", Type: "array", Required: true},
					{Name: "nickname", Type: "string", Required: false},
				},
			},
		},
	}
	got := generator.BuildGeneratedCommands(specs)

	// The required array-typed property has no flat CLI representation, so
	// it must not become a flag ...
	for _, f := range got[0].BodyFlags {
		require.NotEqual(t, "tags", f.Name)
	}
	require.Len(t, got[0].BodyFlags, 1)
	require.Equal(t, "nickname", got[0].BodyFlags[0].Name)

	// ... but unlike a merely-optional non-flat property, it must be
	// recorded so the command can refuse to run instead of silently
	// sending an incomplete request.
	require.Len(t, got[0].UnsupportedRequiredBodyProps, 1)
	require.Equal(t, "tags", got[0].UnsupportedRequiredBodyProps[0].Name)
	require.Equal(t, "array", got[0].UnsupportedRequiredBodyProps[0].Type)
}

func TestBuildGeneratedCommands_OptionalNonFlatBodyProp_ExcludedButNotUnsupported(t *testing.T) {
	specs := []generator.CommandSpec{
		{
			Tag:  "gadget",
			Verb: "create",
			Operation: generator.Operation{
				HasBody: true,
				BodyProps: []generator.BodyProp{
					{Name: "settings", Type: "object", Required: false},
				},
			},
		},
	}
	got := generator.BuildGeneratedCommands(specs)

	require.Empty(t, got[0].BodyFlags)
	// An optional non-flat property isn't a reason to refuse to run - only
	// a *required* one is ...
	require.Empty(t, got[0].UnsupportedRequiredBodyProps)
	// ... but dropping it must not be silent either: it's recorded so
	// emit.go can name it in the command's Long help.
	require.Len(t, got[0].DroppedOptionalBodyProps, 1)
	require.Equal(t, "settings", got[0].DroppedOptionalBodyProps[0].Name)
	require.Equal(t, "object", got[0].DroppedOptionalBodyProps[0].Type)
}

func TestBuildGeneratedCommands_FlagHelpComesFromSpecDescription(t *testing.T) {
	positional := generator.Param{Name: "part_id", Description: "ID of the part"}
	specs := []generator.CommandSpec{
		{
			Tag:        "doodad",
			Verb:       "part",
			Positional: &positional,
			Flags: []generator.Param{
				{Name: "doodad_id", Description: "ID of the doodad"},
				{Name: "project_slug", Description: "Slug of the project"},
			},
			Operation: generator.Operation{
				HasBody: true,
				BodyProps: []generator.BodyProp{
					{Name: "label", Type: "string", Description: "Human-readable doodad label", Required: true},
				},
			},
		},
	}
	got := generator.BuildGeneratedCommands(specs)

	require.Equal(t, "ID of the part", got[0].PositionalFlag.Help)

	byName := map[string]generator.FlagDef{}
	for _, f := range got[0].Flags {
		byName[f.Name] = f
	}
	require.Equal(t, "ID of the doodad", byName["doodad-id"].Help)
	require.Equal(t, "Slug of the project", byName["project"].Help)

	require.Len(t, got[0].BodyFlags, 1)
	require.Equal(t, "Human-readable doodad label", got[0].BodyFlags[0].Help)
}

func TestBuildGeneratedCommands_FlagHelpFallsBackToFlagNameWhenSpecHasNoDescription(t *testing.T) {
	specs := []generator.CommandSpec{
		{
			Tag:   "doodad",
			Verb:  "create",
			Flags: []generator.Param{{Name: "doodad_id"}},
			Operation: generator.Operation{
				HasBody:   true,
				BodyProps: []generator.BodyProp{{Name: "nickname", Type: "string"}},
			},
		},
	}
	got := generator.BuildGeneratedCommands(specs)

	require.Equal(t, "doodad-id", got[0].Flags[0].Help)
	require.Equal(t, "nickname", got[0].BodyFlags[0].Help)
}

func TestBuildGeneratedCommands_BodyPropsBecomeTypedFlags(t *testing.T) {
	specs := []generator.CommandSpec{
		{
			Tag:  "project",
			Verb: "create",
			Operation: generator.Operation{
				HasBody: true,
				BodyProps: []generator.BodyProp{
					{Name: "name", Type: "string", Required: true},
					{Name: "active", Type: "boolean", Required: false},
				},
			},
		},
	}
	got := generator.BuildGeneratedCommands(specs)
	require.Len(t, got[0].BodyFlags, 2)

	byName := map[string]generator.FlagDef{}
	for _, f := range got[0].BodyFlags {
		byName[f.Name] = f
	}
	require.Equal(t, generator.KindString, byName["name"].Kind)
	require.True(t, byName["name"].Required)
	require.Equal(t, generator.KindBool, byName["active"].Kind)
	require.False(t, byName["active"].Required)
}

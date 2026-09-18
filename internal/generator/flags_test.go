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

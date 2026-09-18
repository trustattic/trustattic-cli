package generator_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/trustattic/trustattic-cli/internal/generator"
)

func TestLoadOverrides_ParsesGroupAndVerb(t *testing.T) {
	path := filepath.Join(t.TempDir(), "overrides.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
AccountPutObtain:
  verb: obtain
BackupRestoreGet:
  group: [restore]
  verb: status
`), 0o644))

	overrides, err := generator.LoadOverrides(path)
	require.NoError(t, err)
	require.Equal(t, "obtain", overrides["AccountPutObtain"].Verb)
	require.Equal(t, []string{"restore"}, overrides["BackupRestoreGet"].Group)
	require.Equal(t, "status", overrides["BackupRestoreGet"].Verb)
}

func TestLoadOverrides_MissingFile_ReturnsEmptyMapNoError(t *testing.T) {
	overrides, err := generator.LoadOverrides(filepath.Join(t.TempDir(), "missing.yaml"))
	require.NoError(t, err)
	require.Empty(t, overrides)
}

func TestApplyOverrides_ReplacesVerbAndGroupByOperationID(t *testing.T) {
	specs := []generator.CommandSpec{
		{Operation: generator.Operation{OperationID: "BackupRestoreGet"}, Verb: "get"},
		{Operation: generator.Operation{OperationID: "BackupRestoreCreate"}, Verb: "create"},
	}
	overrides := map[string]generator.Override{
		"BackupRestoreGet": {Verb: "status"},
	}

	got := generator.ApplyOverrides(specs, overrides)
	require.Equal(t, "status", got[0].Verb)
	require.Equal(t, "create", got[1].Verb, "operation without an override is untouched")
}

func TestApplyOverrides_EmptyMap_LeavesTheRealSpecUnchanged(t *testing.T) {
	ops, err := generator.LoadOperations("../../spec/api.yaml")
	require.NoError(t, err)
	specs := generator.BuildCommandSpecs(ops)
	overrides, err := generator.LoadOverrides("overrides.yaml")
	require.NoError(t, err)

	got := generator.ApplyOverrides(specs, overrides)
	require.Equal(t, specs, got)
}

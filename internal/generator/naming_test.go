package generator_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/trustattic/trustattic-cli/internal/generator"
)

func op(tag, id, method, path string, params ...string) generator.Operation {
	o := generator.Operation{Tag: tag, OperationID: id, Method: method, Path: path}
	for _, p := range params {
		o.Params = append(o.Params, generator.Param{Name: p})
	}
	return o
}

func specByID(t *testing.T, specs []generator.CommandSpec, id string) generator.CommandSpec {
	t.Helper()
	for _, s := range specs {
		if s.Operation.OperationID == id {
			return s
		}
	}
	t.Fatalf("no CommandSpec for operationId %q", id)
	return generator.CommandSpec{}
}

func TestBuildCommandSpecs_AccountCRUDGroup_SharesEmptyRemainder(t *testing.T) {
	ops := []generator.Operation{
		op("account", "AccountGet", "GET", "/account"),
		op("account", "AccountPost", "POST", "/account"),
		op("account", "AccountPut", "PUT", "/account/{account_id}", "account_id"),
	}
	specs := generator.BuildCommandSpecs(ops)

	list := specByID(t, specs, "AccountGet")
	require.Equal(t, "account", list.Tag)
	require.Empty(t, list.Group)
	require.Equal(t, "list", list.Verb)
	require.Nil(t, list.Positional)

	create := specByID(t, specs, "AccountPost")
	require.Equal(t, "create", create.Verb)

	update := specByID(t, specs, "AccountPut")
	require.Equal(t, "update", update.Verb)
	require.NotNil(t, update.Positional)
	require.Equal(t, "account_id", update.Positional.Name)
}

func TestBuildCommandSpecs_SingleOpWithVerbLikeRemainder_BecomesLeaf(t *testing.T) {
	ops := []generator.Operation{
		op("account", "AccountPutObtain", "PUT", "/account"),
	}
	specs := generator.BuildCommandSpecs(ops)

	obtain := specByID(t, specs, "AccountPutObtain")
	require.Empty(t, obtain.Group)
	require.Equal(t, "obtain", obtain.Verb)
}

func TestBuildCommandSpecs_ParamTokenInOperationID_IsStripped(t *testing.T) {
	// ProjectSlugGet's "Slug" word is a naming artifact of the project_slug
	// path parameter, not a real subgroup - it must land in the same group
	// as ProjectGet/ProjectPost/ProjectPut, not a spurious "project slug" one.
	ops := []generator.Operation{
		op("project", "ProjectGet", "GET", "/project"),
		op("project", "ProjectPost", "POST", "/project"),
		op("project", "ProjectSlugGet", "GET", "/project/{project_slug}", "project_slug"),
		op("project", "ProjectPut", "PUT", "/project/{project_slug}", "project_slug"),
	}
	specs := generator.BuildCommandSpecs(ops)

	get := specByID(t, specs, "ProjectSlugGet")
	require.Empty(t, get.Group)
	require.Equal(t, "get", get.Verb)
	require.NotNil(t, get.Positional)
	require.Equal(t, "project_slug", get.Positional.Name)
}

func TestBuildCommandSpecs_TwoOpsSharingAnActionWord_NestAsAGroup(t *testing.T) {
	ops := []generator.Operation{
		op("backup", "BackupRestorePut", "POST", "/project/{project_slug}/backup/{backup_id}/restore", "project_slug", "backup_id"),
		op("backup", "BackupRestoreGet", "GET", "/project/{project_slug}/backup/{backup_id}/restore/{restore_id}", "project_slug", "backup_id", "restore_id"),
	}
	specs := generator.BuildCommandSpecs(ops)

	// BackupRestorePut's URL ends in the static "restore" segment, not a
	// path param, so it gets no positional arg - both path params are
	// flags. Only BackupRestoreGet's URL ends in {restore_id}.
	create := specByID(t, specs, "BackupRestorePut")
	require.Equal(t, []string{"restore"}, create.Group)
	require.Equal(t, "create", create.Verb)
	require.Nil(t, create.Positional)
	require.Len(t, create.Flags, 2)
	require.Equal(t, "project_slug", create.Flags[0].Name)
	require.Equal(t, "backup_id", create.Flags[1].Name)

	get := specByID(t, specs, "BackupRestoreGet")
	require.Equal(t, []string{"restore"}, get.Group)
	require.Equal(t, "get", get.Verb)
	require.NotNil(t, get.Positional)
	require.Equal(t, "restore_id", get.Positional.Name)
	require.Len(t, get.Flags, 2)
	require.Equal(t, "project_slug", get.Flags[0].Name)
	require.Equal(t, "backup_id", get.Flags[1].Name)
}

func TestBuildCommandSpecs_SingleOpEmptyRemainder_UsesMechanicalVerb(t *testing.T) {
	ops := []generator.Operation{
		op("resource", "ResourceGet", "GET", "/project/{project_slug}/resource", "project_slug"),
	}
	specs := generator.BuildCommandSpecs(ops)

	// The URL ends in the static "resource" segment, not {project_slug}, so
	// there's no positional arg - project_slug is a flag, like every other
	// plain "list" command.
	list := specByID(t, specs, "ResourceGet")
	require.Empty(t, list.Group)
	require.Equal(t, "list", list.Verb)
	require.Nil(t, list.Positional)
	require.Len(t, list.Flags, 1)
	require.Equal(t, "project_slug", list.Flags[0].Name)
}

func TestBuildCommandSpecs_ActionOnAStaticTailURL_HasNoPositional(t *testing.T) {
	ops := []generator.Operation{
		op("connection", "ConnectionCheck", "GET", "/project/{project_slug}/connection/{connection_id}/check", "project_slug", "connection_id"),
	}
	specs := generator.BuildCommandSpecs(ops)

	// The URL ends in the static "check" segment, not {connection_id} - by
	// the URL-shape rule that means no positional arg here, even though
	// connection_id is this command's obvious "target". Both path params
	// become flags, in path order.
	check := specByID(t, specs, "ConnectionCheck")
	require.Empty(t, check.Group)
	require.Equal(t, "check", check.Verb)
	require.Nil(t, check.Positional)
	require.Len(t, check.Flags, 2)
	require.Equal(t, "project_slug", check.Flags[0].Name)
	require.Equal(t, "connection_id", check.Flags[1].Name)
}

func TestBuildCommandSpecs_RealSpec_MatchesDesignDocCommandTree(t *testing.T) {
	ops, err := generator.LoadOperations("../../spec/api.yaml")
	require.NoError(t, err)
	specs := generator.BuildCommandSpecs(ops)

	want := map[string]string{ // operationId -> "tag/group.../verb"
		"AccountGet":            "account/list",
		"AccountPost":           "account/create",
		"AccountPut":            "account/update",
		"AccountPutObtain":      "account/obtain",
		"AccountTokensGet":      "account/tokens/list",
		"AccountTokensPost":     "account/tokens/create",
		"AccountTokensDelete":   "account/tokens/delete",
		"ProjectGet":            "project/list",
		"ProjectPost":           "project/create",
		"ProjectSlugGet":        "project/get",
		"ProjectPut":            "project/update",
		"ProjectAccountsGet":    "project/accounts",
		"ProjectInvitePut":      "project/invite",
		"ProjectPermissionsGet": "project/permissions",
		"BackupGet":             "backup/list",
		"BackupRestorePut":      "backup/restore/create",
		"BackupRestoreGet":      "backup/restore/get",
		"ConnectionGet":         "connection/list",
		"ConnectionPost":        "connection/create",
		"ConnectionDelete":      "connection/delete",
		"ConnectionPut":         "connection/update",
		"ConnectionCheck":       "connection/check",
		// Design doc assumed ResourceGet carried its own "resource" tag
		// ("trustattic resource list"). The vendored spec has no "resource"
		// tag at all - ResourceGet (GET /project/{project_slug}/resource)
		// is tagged "connection", so it groups there as a leaf named after
		// its own remainder word ("Resource") instead of the CRUD group's
		// mechanical "list" verb. Confirmed via `grep -n '- resource$'
		// spec/api.yaml` (no hits) vs `- connection` (6 hits, incl. this
		// operation's tags block).
		"ResourceGet":         "connection/resource",
		"ScheduleGet":         "schedule/list",
		"SchedulePost":        "schedule/create",
		"ScheduleDelete":      "schedule/delete",
		"SchedulePut":         "schedule/update",
		"ScheduleGetHistory":  "schedule/history",
		"ScheduleRunPost":     "schedule/run",
		"CommonExternalTypes": "/external-types",
		"CommonHealthcheck":   "/healthcheck",
		"CommonPermissions":   "/permissions",
	}
	require.Len(t, specs, len(want), "update this table if spec/api.yaml gained or lost operations")

	for _, s := range specs {
		parts := append([]string{s.Tag}, s.Group...)
		parts = append(parts, s.Verb)
		got := strings.Join(parts, "/")
		require.Equal(t, want[s.Operation.OperationID], got, "operationId %s", s.Operation.OperationID)
	}
}

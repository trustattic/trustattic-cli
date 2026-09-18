package generator_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/trustattic/trustattic-cli/internal/generator"
)

func TestLoadOperations_FixtureSpec_ReturnsAllFourOperations(t *testing.T) {
	ops, err := generator.LoadOperations("testdata/fixture.yaml")
	require.NoError(t, err)
	require.Len(t, ops, 4)
}

func TestLoadOperations_CapturesTagMethodAndPath(t *testing.T) {
	ops, err := generator.LoadOperations("testdata/fixture.yaml")
	require.NoError(t, err)

	byID := map[string]generator.Operation{}
	for _, op := range ops {
		byID[op.OperationID] = op
	}

	get := byID["WidgetGet"]
	require.Equal(t, "widget", get.Tag)
	require.Equal(t, "GET", get.Method)
	require.Equal(t, "/widget", get.Path)
	require.False(t, get.HasBody)
}

func TestLoadOperations_CapturesPathParams(t *testing.T) {
	ops, err := generator.LoadOperations("testdata/fixture.yaml")
	require.NoError(t, err)

	byID := map[string]generator.Operation{}
	for _, op := range ops {
		byID[op.OperationID] = op
	}

	del := byID["WidgetDelete"]
	require.Len(t, del.Params, 1)
	require.Equal(t, "widget_id", del.Params[0].Name)
}

func TestLoadOperations_CapturesRequestBodyProperties(t *testing.T) {
	ops, err := generator.LoadOperations("testdata/fixture.yaml")
	require.NoError(t, err)

	byID := map[string]generator.Operation{}
	for _, op := range ops {
		byID[op.OperationID] = op
	}

	post := byID["WidgetPost"]
	require.True(t, post.HasBody)
	require.Len(t, post.BodyProps, 2)

	byName := map[string]generator.BodyProp{}
	for _, p := range post.BodyProps {
		byName[p.Name] = p
	}
	require.Equal(t, "string", byName["name"].Type)
	require.True(t, byName["name"].Required)
	require.Equal(t, "boolean", byName["active"].Type)
	require.False(t, byName["active"].Required)
}

func TestLoadOperations_RealPlatformSpec_Has32Operations(t *testing.T) {
	ops, err := generator.LoadOperations("../../spec/api.yaml")
	require.NoError(t, err)
	require.Len(t, ops, 32)
}

// formatsByID loads testdata/formats.yaml (a dedicated fixture for
// Format/HasParams coverage - kept separate from testdata/fixture.yaml so
// these additions don't disturb that fixture's exact operation/group counts
// asserted elsewhere) and indexes it by OperationID.
func formatsByID(t *testing.T) map[string]generator.Operation {
	t.Helper()
	ops, err := generator.LoadOperations("testdata/formats.yaml")
	require.NoError(t, err)
	byID := map[string]generator.Operation{}
	for _, op := range ops {
		byID[op.OperationID] = op
	}
	return byID
}

func TestLoadOperations_CapturesPathParamFormat(t *testing.T) {
	byID := formatsByID(t)

	put := byID["GadgetPut"]
	require.Len(t, put.Params, 1)
	require.Equal(t, "gadget_id", put.Params[0].Name)
	require.Equal(t, "uuid", put.Params[0].Format)
}

func TestLoadOperations_CapturesBodyPropFormat(t *testing.T) {
	byID := formatsByID(t)

	put := byID["GadgetPut"]
	byName := map[string]generator.BodyProp{}
	for _, p := range put.BodyProps {
		byName[p.Name] = p
	}
	require.Equal(t, "email", byName["owner_email"].Format)
	require.True(t, byName["owner_email"].Required)
	require.Equal(t, "date-time", byName["reminder_at"].Format)
	require.False(t, byName["reminder_at"].Required)
}

func TestLoadOperations_CapturesArrayTypedRequiredBodyProp(t *testing.T) {
	byID := formatsByID(t)

	post := byID["GadgetPost"]
	byName := map[string]generator.BodyProp{}
	for _, p := range post.BodyProps {
		byName[p.Name] = p
	}
	require.Equal(t, "array", byName["tags"].Type)
	require.True(t, byName["tags"].Required)
}

func TestLoadOperations_EmptySchemaBody_HasBodyTrueNoProps(t *testing.T) {
	byID := formatsByID(t)

	reset := byID["GadgetResetPost"]
	require.True(t, reset.HasBody)
	require.Empty(t, reset.BodyProps)
}

func TestLoadOperations_HasParams_TrueWhenOperationDeclaresAny(t *testing.T) {
	byID := formatsByID(t)

	require.True(t, byID["GadgetPut"].HasParams, "GadgetPut declares a path parameter")
	require.False(t, byID["GadgetPost"].HasParams, "GadgetPost declares no parameters at all")
}

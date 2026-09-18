package generator_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/trustattic/trustattic-cli/internal/generator"
)

func TestLoadOperations_FixtureSpec_ReturnsAllThreeOperations(t *testing.T) {
	ops, err := generator.LoadOperations("testdata/fixture.yaml")
	require.NoError(t, err)
	require.Len(t, ops, 3)
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

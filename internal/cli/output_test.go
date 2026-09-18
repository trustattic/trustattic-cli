package cli_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/trustattic/trustattic-cli/internal/cli"
)

func TestRender_JSONMode_PrintsIndentedJSON(t *testing.T) {
	var buf bytes.Buffer
	err := cli.Render(&buf, cli.ModeJSON, true, []byte(`{"id":"1","name":"acme"}`))
	require.NoError(t, err)
	require.Contains(t, buf.String(), "\"name\": \"acme\"")
}

func TestRender_NonTTY_PrintsJSONEvenInAutoMode(t *testing.T) {
	var buf bytes.Buffer
	err := cli.Render(&buf, cli.ModeAuto, false, []byte(`{"id":"1"}`))
	require.NoError(t, err)
	require.Contains(t, buf.String(), "\"id\": \"1\"")
}

func TestRender_TTY_ArrayUnderData_RendersAsTable(t *testing.T) {
	var buf bytes.Buffer
	body := []byte(`{"data":[{"id":"1","name":"acme"},{"id":"2","name":"globex"}]}`)
	err := cli.Render(&buf, cli.ModeAuto, true, body)
	require.NoError(t, err)
	out := buf.String()
	require.Contains(t, out, "acme")
	require.Contains(t, out, "globex")
	require.Contains(t, out, "id")
	require.Contains(t, out, "name")
}

func TestRender_TTY_EmptyDataArray_PrintsNoResults(t *testing.T) {
	var buf bytes.Buffer
	err := cli.Render(&buf, cli.ModeAuto, true, []byte(`{"data":[]}`))
	require.NoError(t, err)
	require.Contains(t, buf.String(), "no results")
}

func TestRender_TTY_PlainObject_RendersAsKeyValue(t *testing.T) {
	var buf bytes.Buffer
	err := cli.Render(&buf, cli.ModeAuto, true, []byte(`{"id":"1","name":"acme"}`))
	require.NoError(t, err)
	out := buf.String()
	require.Contains(t, out, "id")
	require.Contains(t, out, "1")
	require.Contains(t, out, "name")
	require.Contains(t, out, "acme")
}

func TestRender_TTY_ObjectUnderData_RendersAsKeyValue(t *testing.T) {
	var buf bytes.Buffer
	err := cli.Render(&buf, cli.ModeAuto, true, []byte(`{"data":{"create":true,"delete":false}}`))
	require.NoError(t, err)
	out := buf.String()
	require.Contains(t, out, "create")
	require.Contains(t, out, "true")
	require.Contains(t, out, "delete")
	require.Contains(t, out, "false")
	require.NotContains(t, out, "map[")
}

func TestRender_TTY_ArrayOfScalarsUnderData_PrintsEachValue(t *testing.T) {
	var buf bytes.Buffer
	err := cli.Render(&buf, cli.ModeAuto, true, []byte(`{"data":["a","b","c"]}`))
	require.NoError(t, err)
	out := buf.String()
	require.Contains(t, out, "a")
	require.Contains(t, out, "b")
	require.Contains(t, out, "c")
}

func TestParseMode_AcceptsEmptyAndJSON(t *testing.T) {
	mode, err := cli.ParseMode("")
	require.NoError(t, err)
	require.Equal(t, cli.ModeAuto, mode)

	mode, err = cli.ParseMode("json")
	require.NoError(t, err)
	require.Equal(t, cli.ModeJSON, mode)
}

func TestParseMode_RejectsAnythingElse(t *testing.T) {
	_, err := cli.ParseMode("yaml")
	require.Error(t, err)
	require.Contains(t, err.Error(), `unknown --output value "yaml"`)
	require.Contains(t, err.Error(), `"json"`)
}

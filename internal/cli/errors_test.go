package cli_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/trustattic/trustattic-cli/internal/cli"
)

func TestAPIError_ErrorIncludesStatusAndBody(t *testing.T) {
	err := &cli.APIError{StatusCode: 404, Body: []byte(`{"message":"not found"}`)}
	require.Contains(t, err.Error(), "404")
	require.Contains(t, err.Error(), "not found")
}

func TestRenderError_WritesMessage(t *testing.T) {
	var buf bytes.Buffer
	cli.RenderError(&buf, errors.New("boom"))
	require.Contains(t, buf.String(), "boom")
}

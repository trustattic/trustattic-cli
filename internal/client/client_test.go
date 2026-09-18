package client_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/trustattic/trustattic-cli/internal/client"
)

func TestNewClientWithResponses_ConstructsAgainstAnyServer(t *testing.T) {
	c, err := client.NewClientWithResponses("http://example.invalid")
	require.NoError(t, err)
	require.NotNil(t, c)
}

package cli_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/trustattic/trustattic-cli/internal/cli"
)

func TestNewAPIClient_SendsAuthTokenHeader(t *testing.T) {
	var gotHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("X-Auth-Token")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	t.Setenv("TRUSTATTIC_API_URL", srv.URL)
	t.Setenv("TRUSTATTIC_TOKEN", "svc-abc123")

	c, err := cli.NewAPIClient(cli.Config{})
	require.NoError(t, err)

	_, err = c.CommonHealthcheckWithResponse(context.Background())
	require.NoError(t, err)
	require.Equal(t, "svc-abc123", gotHeader)
}

package cli

import (
	"context"
	"net/http"
	"time"

	"github.com/trustattic/trustattic-cli/internal/client"
)

// NewAPIClient builds a typed API client pointed at the resolved base URL,
// injecting the resolved service-account token as X-Auth-Token on every
// request.
func NewAPIClient(cfg Config) (*client.ClientWithResponses, error) {
	token, err := ResolveToken(cfg)
	if err != nil {
		return nil, err
	}
	baseURL := ResolveAPIURL()

	httpClient := &http.Client{Timeout: 30 * time.Second}

	return client.NewClientWithResponses(
		baseURL,
		client.WithHTTPClient(httpClient),
		client.WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
			req.Header.Set("X-Auth-Token", token)
			return nil
		}),
	)
}

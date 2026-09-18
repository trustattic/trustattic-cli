//go:build integration

package container

import (
	"context"
	"fmt"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	platformImage = "ghcr.io/trustattic/platform:v2.0.0-RC3"
	cloudapiImage = "ghcr.io/trustattic/cloudapi:v0.2.1"

	// issuerSecretKey is the same test/dev PASETO key platform's own
	// docker-compose.yaml uses for ISSUER_SECRETKEY — it governs only
	// platform's own self-issued tokens (e.g. service-account tokens), not
	// what this test trusts as an incoming login credential (that's the
	// per-run RSA keypair from helper.JWT, passed in as jwtPublicKeyPEM).
	issuerSecretKey = "8f77272d4bb2d599747bce36e6175db27e042c482986f22ba7dc8640ee92731f3d57f5ddc11bc565553dc047e3c2ad54973fd292343ae08f2b507b8d40c201d9"
)

// Stack is a running postgres + cloudapi + platform-api topology, mirroring
// platform/docker-compose.yaml, for CLI integration tests. The CLI module
// does not import platform's Go code; it drives the published images as a
// black box over HTTP.
type Stack struct {
	net      *testcontainers.DockerNetwork
	pg       *postgres.PostgresContainer
	cloudapi testcontainers.Container
	platform testcontainers.Container

	// PlatformURL is the host-reachable base URL of platform-api's API
	// (e.g. "http://localhost:32871/api/v2").
	PlatformURL string
}

// Start brings up the stack, configuring platform-api to trust JWTs signed
// by the keypair whose PEM-encoded public key is jwtPublicKeyPEM.
func Start(ctx context.Context, jwtPublicKeyPEM string) (*Stack, error) {
	net, err := network.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("create network: %w", err)
	}

	pg, err := postgres.Run(ctx, "postgres:17-alpine",
		postgres.WithDatabase("platform"),
		postgres.WithUsername("platform"),
		postgres.WithPassword("platform"),
		postgres.BasicWaitStrategies(),
		network.WithNetwork([]string{"postgres"}, net),
	)
	if err != nil {
		return nil, fmt.Errorf("start postgres: %w", err)
	}

	cloudapiC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:          cloudapiImage,
			Networks:       []string{net.Name},
			NetworkAliases: map[string][]string{net.Name: {"cloudapi"}},
			ExposedPorts:   []string{"50051/tcp"},
			WaitingFor:     wait.ForListeningPort("50051/tcp"),
		},
		Started: true,
	})
	if err != nil {
		return nil, fmt.Errorf("start cloudapi: %w", err)
	}

	platformC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        platformImage,
			Cmd:          []string{"api"},
			Networks:     []string{net.Name},
			ExposedPorts: []string{"8080/tcp"},
			Env: map[string]string{
				"DB_SOURCE":             "postgres://platform:platform@postgres:5432/platform?sslmode=disable",
				"DB_MIGRATIONSDIR":      "/var/run/ko/migrations",
				"JOB_SERVICE":           "river",
				"JOB_DBSOURCE":          "postgres://platform:platform@postgres:5432/platform?sslmode=disable",
				"TOKENCIPHER_KEY":       "N1PCdw3M2B1TfJhoaY2mL736p2vCUc47",
				"CONNECTIONCIPHER_KEY":  "N1PCdw3M2B1TfJhoaY2mL736p2vCUc47",
				"TOKENIZER_0_KIND":      "jwt",
				"TOKENIZER_0_PUBLICKEY": jwtPublicKeyPEM,
				"ISSUER_KIND":           "paseto",
				"ISSUER_SECRETKEY":      issuerSecretKey,
				"CLOUDAPI_URL":          "cloudapi:50051",
				"LOGLEVEL":              "debug",
			},
			WaitingFor: wait.ForHTTP("/api/v2/healthcheck").
				WithPort("8080/tcp").
				WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		return nil, fmt.Errorf("start platform: %w", err)
	}

	host, err := platformC.Host(ctx)
	if err != nil {
		return nil, fmt.Errorf("platform host: %w", err)
	}
	port, err := platformC.MappedPort(ctx, "8080")
	if err != nil {
		return nil, fmt.Errorf("platform mapped port: %w", err)
	}

	return &Stack{
		net:         net,
		pg:          pg,
		cloudapi:    cloudapiC,
		platform:    platformC,
		PlatformURL: fmt.Sprintf("http://%s:%s/api/v2", host, port.Port()),
	}, nil
}

// Terminate stops every container and removes the network.
func (s *Stack) Terminate(ctx context.Context) {
	_ = s.platform.Terminate(ctx)
	_ = s.cloudapi.Terminate(ctx)
	_ = s.pg.Terminate(ctx)
	_ = s.net.Remove(ctx)
}

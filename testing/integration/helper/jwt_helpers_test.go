// testing/integration/helper/jwt_helpers_test.go — small test-only utilities
// kept separate from jwt_test.go for clarity. Named jwt_helpers_test.go
// (not jwt_test_helpers.go, as the plan sketch suggested) because Go only
// treats a file as a test file when its name ends in "_test.go".
package helper_test

import (
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/stretchr/testify/require"
)

func pemDecode(t *testing.T, s string) (*pem.Block, []byte) {
	t.Helper()
	block, rest := pem.Decode([]byte(s))
	require.NotNil(t, block)
	return block, rest
}

func parsePublicKey(block *pem.Block) (any, error) {
	return x509.ParsePKIXPublicKey(block.Bytes)
}

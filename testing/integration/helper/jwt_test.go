package helper_test

import (
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"github.com/trustattic/trustattic-cli/testing/integration/helper"
)

func TestJWT_MintedTokenParsesWithThePublicKey(t *testing.T) {
	j, err := helper.NewJWT()
	require.NoError(t, err)

	pemStr, err := j.PublicKeyPEM()
	require.NoError(t, err)
	require.Contains(t, pemStr, "BEGIN PUBLIC KEY")

	token, err := j.Mint("test-user-1")
	require.NoError(t, err)
	require.NotEmpty(t, token)

	block, _ := pemDecode(t, pemStr)
	pub, err := parsePublicKey(block)
	require.NoError(t, err)

	parsed, err := jwt.Parse(token, func(*jwt.Token) (any, error) { return pub, nil })
	require.NoError(t, err)
	claims, ok := parsed.Claims.(jwt.MapClaims)
	require.True(t, ok)
	require.Equal(t, "test-user-1", claims["sub"])
}

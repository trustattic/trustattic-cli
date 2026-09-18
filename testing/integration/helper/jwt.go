package helper

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWT mints test-only JWTs signed by a fresh, per-instance RSA keypair. It
// exists solely to bootstrap a real user account, project, and
// service-account token before an integration test run (see suite.go) — the
// trustattic CLI itself never authenticates with a JWT, only with the
// service-account token that bootstrap produces.
type JWT struct {
	privateKey *rsa.PrivateKey
}

// NewJWT generates a fresh 2048-bit RSA keypair.
func NewJWT() (*JWT, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}
	return &JWT{privateKey: key}, nil
}

// PublicKeyPEM returns the PEM-encoded public key, for platform-api's
// TOKENIZER_0_PUBLICKEY env var.
func (j *JWT) PublicKeyPEM() (string, error) {
	der, err := x509.MarshalPKIXPublicKey(&j.privateKey.PublicKey)
	if err != nil {
		return "", fmt.Errorf("marshal public key: %w", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})), nil
}

// Mint signs a short-lived RS256 JWT for subject sub.
func (j *JWT) Mint(sub string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub": sub,
		"iss": "TEST",
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	signed, err := token.SignedString(j.privateKey)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signed, nil
}

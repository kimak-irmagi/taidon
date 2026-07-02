package authsession

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
)

const pkceEntropyBytes = 32

// PKCEPair contains the verifier and S256 challenge required by the Google
// loopback authorization-code flow described in
// docs/architecture/cli-auth-component-structure.md.
type PKCEPair struct {
	Verifier  string
	Challenge string
	Method    string
}

// GeneratePKCEPair creates a high-entropy PKCE verifier and its S256 challenge.
func GeneratePKCEPair(source io.Reader) (PKCEPair, error) {
	if source == nil {
		source = rand.Reader
	}
	verifierBytes := make([]byte, pkceEntropyBytes)
	if _, err := io.ReadFull(source, verifierBytes); err != nil {
		return PKCEPair{}, fmt.Errorf("generate PKCE verifier: %w", err)
	}
	verifier := base64.RawURLEncoding.EncodeToString(verifierBytes)
	sum := sha256.Sum256([]byte(verifier))
	return PKCEPair{
		Verifier:  verifier,
		Challenge: base64.RawURLEncoding.EncodeToString(sum[:]),
		Method:    "S256",
	}, nil
}

func generateOpaqueToken(source io.Reader) (string, error) {
	if source == nil {
		source = rand.Reader
	}
	data := make([]byte, pkceEntropyBytes)
	if _, err := io.ReadFull(source, data); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

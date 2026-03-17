package diff

import (
	"crypto/sha256"
	"encoding/hex"
)

// HashContent returns the SHA-256 hex hash of content.
func HashContent(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

package prepare

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
)

type stateHasher struct {
	hash hash.Hash
}

func newStateHasher() *stateHasher {
	return &stateHasher{hash: sha256.New()}
}

func (s *stateHasher) write(key, value string) {
	fmt.Fprintf(s.hash, "%s=%d:%s\n", key, len(value), value)
}

func (s *stateHasher) sum() string {
	return hex.EncodeToString(s.hash.Sum(nil))
}

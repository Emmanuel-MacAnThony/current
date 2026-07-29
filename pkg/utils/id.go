package utils

import (
	"crypto/rand"
	"encoding/hex"
)

// NewID returns a random 128-bit hex id. It has no knowledge of where the id is
// used, so callers that need uniqueness within a live set (e.g. client ids) must
// still guard the clash — see manager.Register, which rejects a duplicate so the
// caller can retry. Generation and uniqueness are deliberately separate concerns.
func NewID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

package id

import (
	"crypto/rand"
	"fmt"
)

// New generates a new unique ID with the given prefix.
// Example: New("proj") -> "proj_a1b2c3d4"
func New(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%x", prefix, b)
}

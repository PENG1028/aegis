package id

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// New generates a new unique ID with the given prefix.
// Example: New("proj") -> "proj_a1b2c3d4"
func New(prefix string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%s_%x", prefix, b)
}

// GenerateRandomHex returns a cryptographically random hex string of n bytes (2n hex chars).
// This is THE canonical random-hex generator for the entire project.
// Do NOT create another generateSecret/generateToken/generateHex function in any package.
// If you need a different byte count, add a parameter to this function — don't create a new one.
// Standard library: crypto/rand.Read + hex.EncodeToString
func GenerateRandomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand.Read failing on a modern OS is extraordinarily rare.
		// Return a detectable prefix so callers know something went wrong,
		// rather than propagating an error everywhere.
		return fmt.Sprintf("INSECURE-FALLBACK-%x", b)
	}
	return hex.EncodeToString(b)
}

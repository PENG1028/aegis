//go:build !linux

package transparent

import "net"

// getOriginalDst is a no-op on non-Linux platforms.
func getOriginalDst(conn net.Conn) string {
	return ""
}

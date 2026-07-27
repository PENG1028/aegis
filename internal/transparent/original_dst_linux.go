//go:build linux

package transparent

import (
	"encoding/binary"
	"fmt"
	"net"
	"syscall"
	"unsafe"
)

// getOriginalDst returns the pre-DNAT destination IP:port for a connection
// that was redirected by iptables. Uses getsockopt(SO_ORIGINAL_DST).
//
// When iptables DNAT rewrites the destination, the kernel remembers the
// original address. This function retrieves it so the proxy can make
// routing decisions based on where the connection was originally headed.
//
// Returns empty string if the lookup fails (not Linux, not a TCP connection,
// or the connection was not DNAT'd).
func getOriginalDst(conn net.Conn) string {
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return ""
	}

	rawConn, err := tcpConn.SyscallConn()
	if err != nil {
		return ""
	}

	var addr string
	rawConn.Control(func(fd uintptr) {
		// SO_ORIGINAL_DST = 80 (SOL_IP=0, level=0)
		// Retrieve the pre-DNAT sockaddr_in from the kernel.
		// Layout: struct sockaddr_in { family:2, port:2(BE), addr:4, zero:8 }
		var raw [16]byte
		var size uint32 = 16
		_, _, errno := syscall.Syscall6(syscall.SYS_GETSOCKOPT, fd,
			uintptr(syscall.SOL_IP), uintptr(80),
			uintptr(unsafe.Pointer(&raw[0])),
			uintptr(unsafe.Pointer(&size)), 0)
		if errno != 0 {
			return
		}
		if size >= 8 {
			port := binary.BigEndian.Uint16(raw[2:4])
			ip := net.IPv4(raw[4], raw[5], raw[6], raw[7])
			if !ip.IsUnspecified() && port != 0 {
				addr = fmt.Sprintf("%s:%d", ip.String(), port)
			}
		}
	})
	return addr
}

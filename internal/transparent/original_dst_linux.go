//go:build linux

package transparent

import (
	"fmt"
	"net"
	"syscall"
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
		// SO_ORIGINAL_DST = 80 on Linux
		// Call getsockopt to retrieve the original destination sockaddr
		raw, err := syscall.GetsockoptIPv6Mreq(int(fd), syscall.IPPROTO_IP, 80)
		if err != nil {
			return
		}
		// The returned value contains the original IPv4 address + port
		// Layout: family(2) + port(2, network order) + ip(4) + zero(8)
		if len(raw) >= 8 {
			port := int(raw[2])<<8 | int(raw[3])
			ip := net.IPv4(raw[4], raw[5], raw[6], raw[7])
			addr = fmt.Sprintf("%s:%d", ip.String(), port)
		}
	})
	return addr
}

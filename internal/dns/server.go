package dns

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

// ─── DNS wire format constants ───

const (
	dnsTypeA    = 1
	dnsTypeAAAA = 28
	dnsClassIN  = 1

	dnsFlagQR      = 0x8000
	dnsFlagAA      = 0x0400
	dnsFlagRD      = 0x0100
	dnsFlagRA      = 0x8000
	dnsRcodeNoErr  = 0
	dnsRcodeServFail = 2
	dnsRcodeNXDomain = 3
)

// Server is a local DNS server that intercepts queries for managed domains.
type Server struct {
	resolver *Resolver
	upstream string
	listen   string

	conn     *net.UDPConn
	closeCh  chan struct{}
	wg       sync.WaitGroup

	mu      sync.Mutex
	running bool

	// stats
	statsMu       sync.Mutex
	localHits     int64
	upstreamCalls int64
}

// NewServer creates a new DNS server (stdlib only, no external dependency).
func NewServer(resolver *Resolver, listen, upstream string) *Server {
	if listen == "" {
		listen = ":53"
	}
	if upstream == "" {
		upstream = "1.1.1.1:53"
	}
	return &Server{
		resolver: resolver,
		listen:   listen,
		upstream: upstream,
		closeCh:  make(chan struct{}),
	}
}

// Start begins listening for DNS queries in a background goroutine.
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("dns server already running on %s", s.listen)
	}

	addr, err := net.ResolveUDPAddr("udp", s.listen)
	if err != nil {
		return fmt.Errorf("dns resolve addr: %w", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("dns listen on %s: %w", s.listen, err)
	}

	s.conn = conn

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.serve()
	}()

	s.running = true
	log.Printf("[dns] server listening on %s (upstream: %s)", s.listen, s.upstream)
	return nil
}

// Stop gracefully stops the DNS server.
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	if s.conn != nil {
		s.conn.Close()
	}

	s.running = false
	log.Printf("[dns] server stopped")
	return nil
}

// IsRunning returns whether the DNS server is currently running.
func (s *Server) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// Stats returns DNS query counters.
func (s *Server) Stats() (local, upstream int64) {
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	return s.localHits, s.upstreamCalls
}

// serve is the main UDP read loop.
func (s *Server) serve() {
	buf := make([]byte, 1500)
	for {
		n, remoteAddr, err := s.conn.ReadFromUDP(buf)
		if err != nil {
			select {
			case <-s.closeCh:
				return
			default:
				log.Printf("[dns] read error: %v", err)
				return
			}
		}

		// Copy the data so the next read doesn't overwrite
		query := make([]byte, n)
		copy(query, buf[:n])

		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handlePacket(query, remoteAddr)
		}()
	}
}

// handlePacket processes one DNS query.
func (s *Server) handlePacket(query []byte, remoteAddr *net.UDPAddr) {
	if len(query) < 12 {
		return
	}

	// Parse question section
	domain, qtype, qclass, qlen := parseQuestion(query[12:])
	if domain == "" {
		return
	}

	// Only A or AAAA
	if qtype != dnsTypeA && qtype != dnsTypeAAAA {
		s.forwardPacket(query, remoteAddr)
		return
	}

	// Look up in managed domains
	entry := s.resolver.Lookup(domain)
	if entry == nil {
		s.forwardPacket(query, remoteAddr)
		return
	}

	// Managed → build response
	s.statsMu.Lock()
	s.localHits++
	s.statsMu.Unlock()

	resp := buildAResponse(query, domain, qtype, qclass, qlen, entry.TargetIP)
	if resp == nil {
		// Fall back to forward
		s.forwardPacket(query, remoteAddr)
		return
	}

	if s.conn != nil {
		s.conn.WriteToUDP(resp, remoteAddr)
	}
}

// forwardPacket sends the raw query to upstream DNS and relays the response.
func (s *Server) forwardPacket(query []byte, remoteAddr *net.UDPAddr) {
	s.statsMu.Lock()
	s.upstreamCalls++
	s.statsMu.Unlock()

	upstreamAddr, err := net.ResolveUDPAddr("udp", s.upstream)
	if err != nil {
		log.Printf("[dns] resolve upstream %s: %v", s.upstream, err)
		s.sendServFail(query, remoteAddr)
		return
	}

	upstreamConn, err := net.DialUDP("udp", nil, upstreamAddr)
	if err != nil {
		log.Printf("[dns] dial upstream %s: %v", s.upstream, err)
		s.sendServFail(query, remoteAddr)
		return
	}
	defer upstreamConn.Close()

	upstreamConn.SetDeadline(time.Now().Add(5 * time.Second))

	if _, err := upstreamConn.Write(query); err != nil {
		log.Printf("[dns] upstream write: %v", err)
		s.sendServFail(query, remoteAddr)
		return
	}

	resp := make([]byte, 1500)
	n, err := upstreamConn.Read(resp)
	if err != nil {
		log.Printf("[dns] upstream read: %v", err)
		s.sendServFail(query, remoteAddr)
		return
	}

	if s.conn != nil {
		s.conn.WriteToUDP(resp[:n], remoteAddr)
	}
}

// sendServFail sends a SERVFAIL response.
func (s *Server) sendServFail(query []byte, remoteAddr *net.UDPAddr) {
	if len(query) < 2 {
		return
	}
	resp := make([]byte, 12)
	binary.BigEndian.PutUint16(resp[0:2], binary.BigEndian.Uint16(query[0:2])) // copy ID
	binary.BigEndian.PutUint16(resp[2:4], dnsFlagQR|dnsFlagRA|dnsRcodeServFail) // flags + rcode
	// Copy QDCOUNT from query
	resp[4] = query[4]
	resp[5] = query[5]
	// ANCOUNT = 0
	resp[6] = 0
	resp[7] = 0

	if s.conn != nil {
		s.conn.WriteToUDP(resp, remoteAddr)
	}
}

// ─── DNS wire format helpers ───

// parseQuestion extracts domain, type, class from a DNS question section.
// Returns the number of bytes consumed by the question (for echo-back).
func parseQuestion(data []byte) (domain string, qtype, qclass int, qlen int) {
	if len(data) < 4 {
		return "", 0, 0, 0
	}

	// Parse QNAME (sequence of labels)
	var labels []byte
	pos := 0
	for pos < len(data) {
		length := int(data[pos])
		if length == 0 {
			pos++ // consume the terminating zero
			break
		}
		if length > 63 { // compression pointer or invalid
			return "", 0, 0, 0
		}
		pos++
		if pos+length > len(data) {
			return "", 0, 0, 0
		}
		if len(labels) > 0 {
			labels = append(labels, '.')
		}
		labels = append(labels, data[pos:pos+length]...)
		pos += length
	}

	if pos+4 > len(data) {
		return "", 0, 0, 0
	}

	qtype = int(binary.BigEndian.Uint16(data[pos:]))
	qclass = int(binary.BigEndian.Uint16(data[pos+2:]))
	qlen = pos + 4

	return string(labels), qtype, qclass, qlen
}

// buildAResponse constructs a DNS response with an A/AAAA record.
func buildAResponse(query []byte, domain string, qtype, qclass, qlen int, targetIP string) []byte {
	ip := net.ParseIP(targetIP)
	if ip == nil {
		return nil
	}

	var rdata []byte
	var rdlength uint16

	switch qtype {
	case dnsTypeA:
		ip4 := ip.To4()
		if ip4 == nil {
			return nil
		}
		rdata = ip4
		rdlength = 4
	case dnsTypeAAAA:
		ip16 := ip.To16()
		if ip16 == nil || ip.To4() != nil {
			return nil
		}
		rdata = ip16
		rdlength = 16
	default:
		return nil
	}

	// Build response packet
	// Header: copy ID, set flags QR+AA+RD+RA
	resp := make([]byte, 0, 512)

	// Header (12 bytes)
	resp = append(resp, query[0], query[1])                   // ID
	flags := uint16(dnsFlagQR | dnsFlagAA | dnsFlagRA)
	if len(query) >= 4 && query[2]&0x01 != 0 { // copy RD from query
		flags |= dnsFlagRD
	}
	resp = append(resp, byte(flags>>8), byte(flags&0xFF))    // flags
	// QDCOUNT = copy from query
	resp = append(resp, query[4], query[5])
	// ANCOUNT = 1
	resp = append(resp, 0, 1)
	// NSCOUNT = 0
	resp = append(resp, 0, 0)
	// ARCOUNT = 0
	resp = append(resp, 0, 0)

	// Question section: echo back the original question
	qstart := 12
	questionBytes := query[qstart : qstart+qlen]
	resp = append(resp, questionBytes...)

	// Answer section: NAME pointer (0xC00C = pointer to byte 12, the start of the question name)
	resp = append(resp, 0xC0, 0x0C)                 // NAME (compressed, points to question name)
	resp = append(resp, byte(qtype>>8), byte(qtype&0xFF)) // TYPE
	resp = append(resp, byte(qclass>>8), byte(qclass&0xFF)) // CLASS
	// TTL = 60 seconds
	resp = append(resp, 0, 0, 0, 60)
	// RDLENGTH
	resp = append(resp, byte(rdlength>>8), byte(rdlength&0xFF))
	// RDATA
	resp = append(resp, rdata...)

	return resp
}

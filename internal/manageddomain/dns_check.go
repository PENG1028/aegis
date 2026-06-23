package manageddomain

import (
	"fmt"
	"net"
	"strings"
)

// checkDNSTXT performs a basic DNS TXT record check.
// Checks if the given TXT record name contains the expected value.
func checkDNSTXT(name, expectedValue string) (bool, string) {
	// Strip trailing dot if present for cleaner lookup
	lookupName := strings.TrimSuffix(name, ".")

	txtRecords, err := net.LookupTXT(lookupName)
	if err != nil {
		// DNS lookup failed — this is expected if the record hasn't been set up yet
		return false, fmt.Sprintf("DNS TXT lookup failed for %s: %v", lookupName, err)
	}

	for _, record := range txtRecords {
		if record == expectedValue {
			return true, fmt.Sprintf("TXT record verified: %s = %s", lookupName, expectedValue)
		}
	}

	return false, fmt.Sprintf("TXT record not found for %s (expected: %s, got: %v)", lookupName, expectedValue, txtRecords)
}

// checkDNSRecord performs a generic DNS record check.
// Returns the first matching record value or an error.
func checkDNSRecord(domain string, recordType string) (string, error) {
	switch strings.ToUpper(recordType) {
	case "A":
		ips, err := net.LookupIP(domain)
		if err != nil {
			return "", fmt.Errorf("A record lookup failed: %w", err)
		}
		for _, ip := range ips {
			if ipv4 := ip.To4(); ipv4 != nil {
				return ipv4.String(), nil
			}
		}
		return "", fmt.Errorf("no A record found for %s", domain)
	case "AAAA":
		ips, err := net.LookupIP(domain)
		if err != nil {
			return "", fmt.Errorf("AAAA record lookup failed: %w", err)
		}
		for _, ip := range ips {
			if ip.To4() == nil {
				return ip.String(), nil
			}
		}
		return "", fmt.Errorf("no AAAA record found for %s", domain)
	case "CNAME":
		cname, err := net.LookupCNAME(domain)
		if err != nil {
			return "", fmt.Errorf("CNAME lookup failed: %w", err)
		}
		return strings.TrimSuffix(cname, "."), nil
	default:
		return "", fmt.Errorf("unsupported record type: %s", recordType)
	}
}

// CheckCNAME checks if the domain has a CNAME pointing to the expected target.
func CheckCNAME(domain, expectedTarget string) (bool, string) {
	cname, err := checkDNSRecord(domain, "CNAME")
	if err != nil {
		return false, err.Error()
	}

	expectedTarget = strings.TrimSuffix(expectedTarget, ".")
	cname = strings.TrimSuffix(cname, ".")

	if strings.EqualFold(cname, expectedTarget) {
		return true, fmt.Sprintf("CNAME verified: %s -> %s", domain, cname)
	}
	return false, fmt.Sprintf("CNAME mismatch: %s -> %s (expected: %s)", domain, cname, expectedTarget)
}

package manageddomain

import (
	"fmt"
	"net"
	"strings"
)

// checkDNSTXTWithRecords checks DNS TXT record and returns records found.
func checkDNSTXTWithRecords(name, expectedValue string) (bool, string, []string) {
	lookupName := strings.TrimSuffix(name, ".")

	txtRecords, err := net.LookupTXT(lookupName)
	if err != nil {
		return false, fmt.Sprintf("DNS TXT lookup failed: %v", err), nil
	}

	for _, record := range txtRecords {
		if record == expectedValue {
			return true, fmt.Sprintf("TXT verified: %s = %s", lookupName, expectedValue), txtRecords
		}
	}

	return false, fmt.Sprintf("TXT record mismatch for %s (expected: %s, got: %v)", lookupName, expectedValue, txtRecords), txtRecords
}

// checkDNSRecordCNAME returns the CNAME for a domain.
func checkDNSRecordCNAME(domain string) (string, error) {
	cname, err := net.LookupCNAME(domain)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(cname, "."), nil
}

// lookupIP returns IPs of the specified version for a domain.
func lookupIP(domain, version string) ([]string, error) {
	ips, err := net.LookupIP(domain)
	if err != nil {
		return nil, err
	}

	var result []string
	for _, ip := range ips {
		isV4 := ip.To4() != nil
		if (version == "ip4" && isV4) || (version == "ip6" && !isV4) {
			result = append(result, ip.String())
		}
	}
	return result, nil
}

// checkDNSTXT performs a basic DNS TXT record check (legacy).
func checkDNSTXT(name, expectedValue string) (bool, string) {
	ok, msg, _ := checkDNSTXTWithRecords(name, expectedValue)
	return ok, msg
}

// checkDNSRecord performs a generic DNS record check (legacy).
func checkDNSRecord(domain string, recordType string) (string, error) {
	switch strings.ToUpper(recordType) {
	case "A":
		ips, err := lookupIP(domain, "ip4")
		if err != nil {
			return "", fmt.Errorf("A record lookup failed: %w", err)
		}
		if len(ips) == 0 {
			return "", fmt.Errorf("no A record for %s", domain)
		}
		return ips[0], nil
	case "AAAA":
		ips, err := lookupIP(domain, "ip6")
		if err != nil {
			return "", fmt.Errorf("AAAA record lookup failed: %w", err)
		}
		if len(ips) == 0 {
			return "", fmt.Errorf("no AAAA record for %s", domain)
		}
		return ips[0], nil
	case "CNAME":
		return checkDNSRecordCNAME(domain)
	default:
		return "", fmt.Errorf("unsupported record type: %s", recordType)
	}
}

// CheckCNAME checks if the domain has a CNAME pointing to the expected target.
func CheckCNAME(domain, expectedTarget string) (bool, string) {
	cname, err := checkDNSRecordCNAME(domain)
	if err != nil {
		return false, err.Error()
	}

	cname = strings.TrimSuffix(cname, ".")
	expectedTarget = strings.TrimSuffix(expectedTarget, ".")

	if strings.EqualFold(cname, expectedTarget) {
		return true, fmt.Sprintf("CNAME verified: %s -> %s", domain, cname)
	}
	return false, fmt.Sprintf("CNAME mismatch: %s -> %s (expected: %s)", domain, cname, expectedTarget)
}

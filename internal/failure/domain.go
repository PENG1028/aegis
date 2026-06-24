package failure

// Domain represents a failure containment boundary.
type Domain struct {
	Type     string `json:"type"` // node | space | service | global
	ID      string `json:"id"`
	Name    string `json:"name"`
	Status  string `json:"status"` // healthy | degraded | failed
	Message string `json:"message"`
}

// IsolateFailure determines if a failure should be contained vs escalated.
// Returns true if the failure is contained within its domain.
func IsolateFailure(domainType string) bool {
	switch domainType {
	case "node":
		return true // node failure ≠ cluster failure
	case "service":
		return true // service failure ≠ node failure
	case "space":
		return true // space failure ≠ global failure
	case "global":
		return false // global failure = escalate
	default:
		return true // default to contained
	}
}

// ShouldCascade determines if a rollback should cascade beyond the failure domain.
func ShouldCascade(domainType string, isCritical bool) bool {
	if isCritical {
		return true // critical failures always cascade
	}
	if domainType == "global" {
		return true
	}
	return false // non-critical, non-global → contained
}

// DomainSummary returns a human-readable summary.
func DomainSummary(d Domain) string {
	if d.Status == "healthy" {
		return d.Type + ":" + d.Name + " OK"
	}
	return d.Type + ":" + d.Name + " " + d.Status + " — " + d.Message
}

package manageddomain

import (
	"testing"
)

func TestAllowedTransitions(t *testing.T) {
	tests := []struct {
		from    string
		to      string
		allowed bool
	}{
		{"pending_verification", "verified", true},
		{"pending_verification", "failed", true},
		{"pending_verification", "active", false},
		{"verified", "active", true},
		{"verified", "failed", false},
		{"active", "disabled", true},
		{"active", "verified", false},
		{"disabled", "active", true},
		{"disabled", "verified", false},
		{"failed", "pending_verification", true},
		{"failed", "active", false},
	}

	svc := &AppService{}
	for _, tt := range tests {
		md := &ManagedDomain{Status: tt.from, Domain: "test.example.com"}
		err := svc.transitionStatus(md, tt.to)
		if tt.allowed && err != nil {
			t.Errorf("transition %s -> %s should be allowed, got error: %v", tt.from, tt.to, err)
		}
		if !tt.allowed && err == nil {
			t.Errorf("transition %s -> %s should be denied, but no error", tt.from, tt.to)
		}
	}
}

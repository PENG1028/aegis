package exposure

import (
	"testing"
)

func TestGeneratesConfig(t *testing.T) {
	tests := []struct {
		typ      string
		expected bool
	}{
		{TypeHTTP, true},
		{TypeTCP, false},
		{TypeUDP, false},
		{TypeTunnel, false},
		{TypeInternal, false},
	}

	for _, tt := range tests {
		result := GeneratesConfig(tt.typ)
		if result != tt.expected {
			t.Errorf("GeneratesConfig(%q) = %v, want %v", tt.typ, result, tt.expected)
		}
	}
}

func TestExposureStatusValues(t *testing.T) {
	// Verify all expected status constants
	statuses := []string{StatusPending, StatusActive, StatusActiveRecorded, StatusDisabled, StatusFailed}
	for _, s := range statuses {
		if s == "" {
			t.Error("status constant is empty")
		}
	}
}

func TestExposureTypeValues(t *testing.T) {
	types := []string{TypeHTTP, TypeTCP, TypeUDP, TypeTunnel, TypeInternal}
	for _, typ := range types {
		if typ == "" {
			t.Error("type constant is empty")
		}
	}
}

func TestStatsStruct(t *testing.T) {
	stats := &Stats{
		Total:           10,
		ByType:          map[string]int{"http": 5, "tcp": 3, "udp": 2},
		ByStatus:        map[string]int{"active": 4, "pending": 3, "active_recorded": 3},
		HTTPActive:      4,
		NonHTTPRecorded: 3,
	}

	if stats.Total != 10 {
		t.Error("total mismatch")
	}
	if stats.ByType["http"] != 5 {
		t.Error("by_type mismatch")
	}
}

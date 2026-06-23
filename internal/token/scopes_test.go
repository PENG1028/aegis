package token

import (
	"testing"
)

func TestHasScope(t *testing.T) {
	tests := []struct {
		scopes   []string
		required string
		expected bool
	}{
		{[]string{ScopeAdminAll}, ScopeRouteRead, true},
		{[]string{ScopeAdminAll}, ScopeApplyRun, true},
		{[]string{ScopeRouteRead}, ScopeRouteRead, true},
		{[]string{ScopeRouteRead}, ScopeRouteWrite, false},
		{[]string{ScopeRouteRead, ScopeRouteWrite}, ScopeRouteWrite, true},
		{[]string{ScopeConfigRead}, ScopeApplyRun, false},
		{[]string{}, ScopeSystemRead, false},
	}

	for _, tt := range tests {
		result := HasScope(tt.scopes, tt.required)
		if result != tt.expected {
			t.Errorf("HasScope(%v, %s) = %v, want %v", tt.scopes, tt.required, result, tt.expected)
		}
	}
}

func TestFindMatchingScope(t *testing.T) {
	tests := []struct {
		method   string
		path     string
		expected string
		found    bool
	}{
		{"GET", "/api/routes", ScopeRouteRead, true},
		{"POST", "/api/routes", ScopeRouteWrite, true},
		{"POST", "/api/apply", ScopeApplyRun, true},
		{"POST", "/api/rollback", ScopeRollbackRun, true},
		{"GET", "/api/config/preview", ScopeConfigRead, true},
		{"GET", "/api/unknown", "", false},
	}

	for _, tt := range tests {
		scope, found := FindMatchingScope(tt.method, tt.path)
		if found != tt.found {
			t.Errorf("FindMatchingScope(%s, %s) found=%v, want %v", tt.method, tt.path, found, tt.found)
		}
		if found && scope != tt.expected {
			t.Errorf("FindMatchingScope(%s, %s) = %s, want %s", tt.method, tt.path, scope, tt.expected)
		}
	}
}

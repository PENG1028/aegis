package hostdep

import "testing"

func TestAptManagerName(t *testing.T) {
	if got := (aptManager{}).Name(); got != "apt" {
		t.Errorf("aptManager.Name() = %q, want apt", got)
	}
}

// Detect() is host-dependent (needs apt-get in PATH), so we only assert it
// returns either a valid manager or nil — never panics — and that a returned
// manager reports a non-empty name.
func TestDetectReturnsValidOrNil(t *testing.T) {
	pm := Detect()
	if pm != nil && pm.Name() == "" {
		t.Error("Detect() returned a manager with empty Name()")
	}
}

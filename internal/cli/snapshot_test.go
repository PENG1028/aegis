package cli

import (
	"testing"
)

// TestRestoreCommandRegistersFromFlag is a regression test: `aegis snapshot
// restore --from <file>` was 100% unusable because the --from flag was read
// (cmd.Flags().GetString) but never registered, so cobra rejected it with
// "unknown flag: --from".
func TestRestoreCommandRegistersFromFlag(t *testing.T) {
	cmd := newRestoreCommand(nil, nil, nil, nil)
	flag := cmd.Flags().Lookup("from")
	if flag == nil {
		t.Fatal("--from flag is not registered — 'snapshot restore --from x.json' always fails")
	}
	if flag.Usage == "" {
		t.Error("--from flag has no usage string")
	}
}

// TestRestoreCommandRejectsMissingFrom verifies the required-flag guard fires
// when --from is absent.
func TestRestoreCommandRejectsMissingFrom(t *testing.T) {
	cmd := newRestoreCommand(nil, nil, nil, nil)
	if err := cmd.RunE(cmd, nil); err == nil {
		t.Fatal("restore without --from must return an error")
	}
}

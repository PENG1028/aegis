package deploy

import (
	"strings"
	"testing"
)

func TestServiceInstallCommandTerminatesHeredocBeforeMove(t *testing.T) {
	cmd := serviceInstallCommand("aegis", "[Unit]\nDescription=Aegis\n")

	if strings.Contains(cmd, "AEGISUNIT &&") {
		t.Fatalf("heredoc terminator must be on its own line: %q", cmd)
	}
	if !strings.Contains(cmd, "\nAEGISUNIT\nsudo mv /tmp/aegis.service /etc/systemd/system/aegis.service") {
		t.Fatalf("install command does not move service after heredoc terminator:\n%s", cmd)
	}
}

package apply

import (
	"fmt"
	"strings"
)

// Diff generates a human-readable diff between the current and new config.
func Diff(currentPath string, newContent []byte) (string, error) {
	// For v0.1, show a simple side-by-side summary
	var sb strings.Builder

	sb.WriteString("--- current\n")
	sb.WriteString("+++ new\n")

	if currentPath != "" {
		sb.WriteString(fmt.Sprintf("@@ config %s @@\n", currentPath))
	}

	// Read current if available
	// For simplicity, just indicate the change type
	sb.WriteString("\nNew configuration to be applied:\n")
	sb.WriteString(string(newContent))

	return sb.String(), nil
}

package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	data, err := os.ReadFile("cmd/aegis/main.go")
	if err != nil {
		panic(err)
	}

	content := string(data)

	// Remove duplicate import "aegis/internal/distnode" (only keep first)
	lines := strings.Split(content, "\n")
	var cleanLines []string
	seen := map[string]int{}
	for _, line := range lines {
		key := strings.TrimSpace(line)
		if key == `"aegis/internal/distnode"` {
			if seen[key] > 0 {
				continue // skip duplicate
			}
			seen[key]++
		}
		if key == `"aegis/internal/httpapi/handlers"` {
			if seen[key] > 0 {
				continue // skip duplicate
			}
			seen[key]++
		}
		cleanLines = append(cleanLines, line)
	}

	// Fix the broken Fprintf lines - they have literal newlines inside Go string literals
	// Pattern: fmt.Fprintf(os.Stderr, "...broken string with \n inside...
	// Fix: join the string across lines and replace \n with \\n
	var fixedLines []string
	skipNext := false
	for i, line := range cleanLines {
		if skipNext {
			skipNext = false
			continue
		}
		trimmed := strings.TrimSpace(line)
		// If line contains the start of a broken Fprintf and doesn't close properly
		if strings.Contains(trimmed, `fmt.Fprintf(os.Stderr, "info: distnode enabled`) && !strings.Contains(trimmed, `\n"`) {
			// Next line has the rest
			if i+1 < len(cleanLines) {
				nextTrimmed := strings.TrimSpace(cleanLines[i+1])
				// Check if next line is just "\n", ... with closing
				if strings.HasPrefix(nextTrimmed, `\n`, dn.ID`) || strings.HasPrefix(nextTrimmed, `"`) {
					// Replace with proper single line
					fixedLines = append(fixedLines, `		fmt.Fprintf(os.Stderr, "info: distnode enabled - id=%s addr=%s peers=%d\n", dn.ID, distCfg.Addr, len(distCfg.Peers))`)
					skipNext = true
					continue
				}
			}
		}
		if strings.Contains(trimmed, `fmt.Fprintf(os.Stderr, "info: distnode disabled`) && !strings.Contains(trimmed, `\n"`) {
			if i+1 < len(cleanLines) {
				nextTrimmed := strings.TrimSpace(cleanLines[i+1])
				if strings.HasPrefix(nextTrimmed, `\n"`) || strings.HasPrefix(nextTrimmed, `"`) {
					fixedLines = append(fixedLines, `		fmt.Fprintf(os.Stderr, "info: distnode disabled\n")`)
					skipNext = true
					continue
				}
			}
		}
		fixedLines = append(fixedLines, line)
	}

	// Fix the extra closing paren
	for i, line := range fixedLines {
		if strings.TrimSpace(line) == `go dn.Start(context.Background())` {
			// Check if previous line has extra ))
			if i > 0 && strings.Count(fixedLines[i-1], ")") > 2 {
				// Already fixed in the Fprintf replacement above
			}
		}
	}

	result := strings.Join(fixedLines, "\n")
	os.WriteFile("cmd/aegis/main.go", []byte(result), 0644)
	fmt.Println("fixed main.go")
}

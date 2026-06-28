// Package importcfg provides parsers for existing proxy configurations
// so Aegis can import already-managed routes.
package importcfg

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ImportedRoute represents a single parsed route from a Caddyfile.
type ImportedRoute struct {
	Domain      string `json:"domain"`
	PathPrefix  string `json:"path_prefix,omitempty"`
	UpstreamURL string `json:"upstream_url"`
	TLSEnabled  bool   `json:"tls_enabled"`
	StripPrefix bool   `json:"strip_prefix,omitempty"`
	SourceFile  string `json:"source_file"`
	SourceLine  int    `json:"source_line"`
}

// CaddyfileImportResult holds the result of scanning a Caddyfile.
type CaddyfileImportResult struct {
	Routes []ImportedRoute `json:"routes"`
	Count  int             `json:"count"`
	Errors []string        `json:"errors,omitempty"`
}

// siteBlock represents a parsed Caddy site block.
type siteBlock struct {
	domain     string
	startLine  int
	lines      []string
	TLSEnabled bool
}

// ScanCaddyfile reads a Caddy v2 format file and extracts domain→backend mappings.
func ScanCaddyfile(path string) (*CaddyfileImportResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open caddyfile: %w", err)
	}
	defer f.Close()

	result := &CaddyfileImportResult{Routes: []ImportedRoute{}}
	scanner := bufio.NewScanner(f)
	lineNum := 0

	var current *siteBlock
	var inBlock bool
	var braceDepth int

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Skip empty lines and comments
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			if inBlock && current != nil {
				current.lines = append(current.lines, line)
			}
			continue
		}

		// Detect site block start: "domain.com {" or "domain.com {" on next line
		if !inBlock && (strings.HasSuffix(trimmed, "{") || strings.HasSuffix(trimmed, "{")) {
			domainPart := strings.TrimSuffix(trimmed, "{")
			domainPart = strings.TrimSpace(domainPart)
			// Strip optional port from domain: "domain.com:443" → "domain.com"
			if colonIdx := strings.LastIndex(domainPart, ":"); colonIdx > 0 {
				// Check if it's a port number
				if _, err := strconv.Atoi(domainPart[colonIdx+1:]); err == nil {
					domainPart = domainPart[:colonIdx]
				}
			}
			if domainPart != "" && !strings.Contains(domainPart, " ") && !strings.Contains(domainPart, "/") {
				current = &siteBlock{
					domain:    domainPart,
					startLine: lineNum,
					lines:     []string{line},
				}
				inBlock = true
				braceDepth = 1
			}
			continue
		}

		if inBlock && current != nil {
			current.lines = append(current.lines, line)

			if strings.Contains(trimmed, "{") {
				braceDepth += strings.Count(trimmed, "{")
			}
			if strings.Contains(trimmed, "}") {
				braceDepth -= strings.Count(trimmed, "}")
			}

			if braceDepth <= 0 {
				// End of block — parse it
				parseSiteBlock(current, result)
				current = nil
				inBlock = false
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan caddyfile: %w", err)
	}

	result.Count = len(result.Routes)
	return result, nil
}

// parseSiteBlock extracts routes from a single Caddy site block.
func parseSiteBlock(block *siteBlock, result *CaddyfileImportResult) {
	// Check if TLS is configured (tls directive or :443 port)
	block.TLSEnabled = false
	var handlePath string // current handle /path/ scope
	var handleStrip bool  // is current handle a handle_path (strip prefix)

	for _, line := range block.lines {
		trimmed := strings.TrimSpace(line)

		switch {
		case strings.HasPrefix(trimmed, "tls "):
			block.TLSEnabled = true

		case strings.HasPrefix(trimmed, "handle_path "):
			// handle_path /path/* { ... }
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				handlePath = strings.TrimSuffix(parts[1], "/*")
				handleStrip = true
			}

		case strings.HasPrefix(trimmed, "handle "):
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 && parts[1] != "{" {
				handlePath = strings.TrimSuffix(parts[1], "/*")
				handleStrip = false
			} else {
				handlePath = ""
				handleStrip = false
			}

		case strings.HasPrefix(trimmed, "reverse_proxy "):
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				upstream := parts[1]
				// Strip sub-directives in { }
				if idx := strings.Index(upstream, "{"); idx >= 0 {
					upstream = strings.TrimSpace(upstream[:idx])
				}
				if upstream != "" && !strings.HasPrefix(upstream, "{") {
					result.Routes = append(result.Routes, ImportedRoute{
						Domain:      block.domain,
						PathPrefix:  handlePath,
						UpstreamURL: upstream,
						TLSEnabled:  block.TLSEnabled,
						StripPrefix: handleStrip,
						SourceFile:  "",
						SourceLine:  block.startLine,
					})
				}
			}
		}
	}
}

// FindCaddyfiles locates Caddyfile(s) in standard locations.
func FindCaddyfiles() []string {
	var candidates []string
	paths := []string{
		"/etc/caddy/Caddyfile",
		"/etc/caddy/caddy.json",
		"/usr/local/etc/caddy/Caddyfile",
		filepath.Join(os.Getenv("HOME"), "Caddyfile"),
		"Caddyfile",
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			candidates = append(candidates, p)
		}
	}
	return candidates
}

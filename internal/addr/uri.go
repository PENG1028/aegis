// Package uri parses database and service connection URIs.
//
// Supported schemes:
//   - postgres, postgresql
//   - mysql, mariadb
//   - redis, rediss
//   - mongodb
//   - ws, wss (WebSocket)
//   - credential (Aegis internal alias)
package addr

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// ConnInfo holds the parsed components of a connection URI.
type ConnInfo struct {
	Scheme   string // original scheme: postgres, mysql, etc.
	User     string
	Password string
	Host     string
	Port     int
	Database string // path stripped of leading /
	RawQuery string // query string (key=value pairs)
	Original string // the original URI string
}

// defaultPorts maps schemes to their default TCP ports.
var defaultPorts = map[string]int{
	"postgres":   5432,
	"postgresql": 5432,
	"mysql":      3306,
	"mariadb":    3306,
	"redis":      6379,
	"rediss":     6380,
	"mongodb":    27017,
	"ws":         80,
	"wss":        443,
}

// Parse parses a connection URI string and returns structured connection info.
//
// Examples:
//
//	postgres://user:pass@host:5432/db
//	postgres://user:pass@host/db?sslmode=require
//	mysql://user:pass@host:3306/db
//	redis://:pass@host:6379/0
//	ws://host:8080/chat
func ParseConnString(raw string) (*ConnInfo, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty connection URI")
	}

	// Handle credential:// alias (internal Aegis reference)
	if strings.HasPrefix(raw, "credential://") {
		alias := strings.TrimPrefix(raw, "credential://")
		if alias == "" {
			return nil, fmt.Errorf("credential:// alias is empty")
		}
		return &ConnInfo{
			Scheme:   "credential",
			Database: alias, // alias stored in Database field
			Original: raw,
		}, nil
	}

	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse URI: %w", err)
	}

	if u.Scheme == "" {
		return nil, fmt.Errorf("URI scheme is required (e.g. postgres://...)")
	}

	info := &ConnInfo{
		Scheme:   strings.ToLower(u.Scheme),
		Original: raw,
	}

	// User info
	if u.User != nil {
		info.User = u.User.Username()
		info.Password, _ = u.User.Password()
	}

	// Host and port
	info.Host = u.Hostname()
	if u.Port() != "" {
		info.Port, _ = strconv.Atoi(u.Port())
	}

	// Default port
	if info.Port == 0 {
		if dp, ok := defaultPorts[info.Scheme]; ok {
			info.Port = dp
		}
	}

	// Database name (strip leading /)
	info.Database = strings.TrimPrefix(u.Path, "/")
	if info.Database == "" && info.Host != "" {
		// Some URIs omit the database: postgres://user:pass@host:5432
		info.Database = ""
	}

	// Query string
	info.RawQuery = u.RawQuery

	return info, nil
}

// MaskPassword returns the URI with the password replaced by ***.
func (c *ConnInfo) MaskPassword() string {
	if c.Scheme == "credential" {
		return c.Original // alias is not sensitive
	}
	if c.Password == "" {
		return c.Original
	}
	masked := strings.Replace(c.Original, c.Password, "***", 1)
	// Also handle URL-encoded passwords
	encoded := url.QueryEscape(c.Password)
	if encoded != c.Password {
		masked = strings.Replace(masked, encoded, "***", 1)
	}
	return masked
}

// TargetAddr returns "host:port" suitable for TCP dialing.
func (c *ConnInfo) TargetAddr() string {
	if c.Port == 0 {
		return c.Host
	}
	return net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
}

// IsCredential returns true if this is a credential:// alias.
func (c *ConnInfo) IsCredential() bool {
	return c.Scheme == "credential"
}

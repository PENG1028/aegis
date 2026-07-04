package gateway

// Config for the local HTTP gateway.
type Config struct {
	Enabled             bool   `yaml:"enabled" json:"enabled"`
	BindAddr            string `yaml:"bind_addr" json:"bind_addr"`
	Port                int    `yaml:"port" json:"port"`
	UnmanagedMode       string `yaml:"unmanaged_mode" json:"unmanaged_mode"`
	PreserveHost        bool   `yaml:"preserve_host" json:"preserve_host"`
	RequestTimeoutSec   int    `yaml:"request_timeout_seconds" json:"request_timeout_seconds"`
	NodeID              string `yaml:"node_id" json:"node_id,omitempty"`
}

// UnmanagedMode constants.
const (
	UnmanagedReject           = "reject"
	UnmanagedPassthroughDefer = "passthrough_deferred"
	UnmanagedProxyDefer       = "proxy_deferred"
)

// DefaultConfig returns a local gateway config with defaults.
func DefaultConfig() *Config {
	return &Config{
		Enabled:           true,
		BindAddr:          "127.0.0.1",
		Port:              18080,
		UnmanagedMode:     UnmanagedReject,
		PreserveHost:      true,
		RequestTimeoutSec: 30,
		NodeID:            "",
	}
}

// ListenAddr returns the address to listen on.
func (c *Config) ListenAddr() string {
	return c.BindAddr
}

// ListenPort returns the port to listen on.
func (c *Config) ListenPort() int {
	return c.Port
}

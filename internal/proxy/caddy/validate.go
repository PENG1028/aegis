package caddy

// Additional Caddy-specific validation logic.
// Core validation is done in adapter.go via caddy validate command.
// This file is reserved for Caddyfile syntax checks (e.g., lint-like checks
// before calling the external validate command).

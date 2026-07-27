//go:build !linux

package handlers

// fillSystemHealthPlatform is a no-op on non-Linux platforms.
func (h *Handlers) fillSystemHealthPlatform(resp *SystemHealthResponse) {}

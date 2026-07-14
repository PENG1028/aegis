//go:build !linux

package tool

// DetectIPTables returns not-available on non-Linux platforms.
func DetectIPTables() Status {
	return Status{
		Name:    "iptables",
		Label:   "透明代理 (iptables)",
		Category: "transparent_proxy",
		Message: "仅支持 Linux — 当前系统不含 iptables",
	}
}

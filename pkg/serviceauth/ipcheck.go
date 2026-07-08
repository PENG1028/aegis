package serviceauth

import (
	"net"
	"sync"
	"time"
)

// ─── IPChecker ────────────────────────────────────────────────────────────
// Guard 用这个来判断调用方 IP 是否被允许。
// 默认 = 仅集群内网。可通过 Client.SetIPChecker() 替换。

// MaxWhitelistDuration 是临时白名单允许的最大时长。
// 硬编码 24 小时，不可绕过。所有白名单条目到期自动失效。
const MaxWhitelistDuration = 24 * time.Hour

// IPChecker decides whether a remote IP is allowed to call this service.
type IPChecker interface {
	Allow(remoteIP string) bool
}

// ─── ClusterOnly ──────────────────────────────────────────────────────────

// ClusterOnlyChecker 只允许内网 IP（默认）。
type ClusterOnlyChecker struct{}

func (c *ClusterOnlyChecker) Allow(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	if parsed.IsLoopback() {
		return true
	}
	return isPrivateIP(parsed)
}

// AllowCluster returns an IPChecker that only allows private / loopback IPs.
func AllowCluster() IPChecker { return &ClusterOnlyChecker{} }

// ─── AllowAll ─────────────────────────────────────────────────────────────

// AllowAllChecker 允许所有 IP（等于不做 IP 检查）。
type AllowAllChecker struct{}

func (c *AllowAllChecker) Allow(ip string) bool { return true }

func AllowAll() IPChecker { return &AllowAllChecker{} }

// ─── WhitelistChecker ────────────────────────────────────────────────────

// WhitelistChecker 先检查集群内网，再检查临时白名单。
// 临时白名单条目最长 24h，到期自动失效。
// 白名单通过 SetWhitelist() 更新，数据来源是 Aegis sync。
type WhitelistChecker struct {
	base    IPChecker
	mu      sync.RWMutex
	entries map[string]time.Time // ip → expires_at
}

func NewWhitelistChecker(base IPChecker) *WhitelistChecker {
	return &WhitelistChecker{
		base:    base,
		entries: make(map[string]time.Time),
	}
}

func (w *WhitelistChecker) Allow(ip string) bool {
	// 先过基础检查（内网）
	if w.base.Allow(ip) {
		return true
	}
	// 再过白名单
	w.mu.RLock()
	defer w.mu.RUnlock()
	expiry, ok := w.entries[ip]
	if !ok {
		return false
	}
	return time.Now().Before(expiry)
}

// SetWhitelist 全量替换白名单。每条的时间必须 <= MaxWhitelistDuration。
func (w *WhitelistChecker) SetWhitelist(entries map[string]time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.entries = make(map[string]time.Time, len(entries))
	for ip, expiry := range entries {
		// 硬编码：最长 24h
		maxExpiry := time.Now().Add(MaxWhitelistDuration)
		if expiry.After(maxExpiry) {
			expiry = maxExpiry
		}
		w.entries[ip] = expiry
	}
}

func isPrivateIP(ip net.IP) bool {
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 10 {
			return true
		}
		if ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31 {
			return true
		}
		if ip4[0] == 192 && ip4[1] == 168 {
			return true
		}
		return false
	}
	return ip.IsPrivate()
}

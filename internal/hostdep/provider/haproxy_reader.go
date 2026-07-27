package provider

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
)

// ============================================================================
// HAProxyReader — 从 HAProxy 配置中逆向解析 SNI 路由和 TCP 转发
// ============================================================================

// HAProxyReader implements the Reader interface for HAProxy.
// It parses both haproxy.cfg (SNI passthrough) and haproxy_tcp.cfg (TCP forwarding).
type HAProxyReader struct {
	configPath    string // /etc/haproxy/haproxy.cfg
	tcpConfigPath string // /etc/haproxy/haproxy_tcp.cfg
}

// NewHAProxyReader creates a HAProxyReader.
func NewHAProxyReader(configPath, tcpConfigPath string) *HAProxyReader {
	if configPath == "" {
		configPath = "/etc/haproxy/haproxy.cfg"
	}
	if tcpConfigPath == "" {
		tcpConfigPath = "/etc/haproxy/haproxy_tcp.cfg"
	}
	return &HAProxyReader{
		configPath:    configPath,
		tcpConfigPath: tcpConfigPath,
	}
}

// ID returns "haproxy", matching HAProxyProvider.State().ID.
func (r *HAProxyReader) ID() string { return "haproxy" }

// ReadConfig parses HAProxy config files and returns a structured snapshot.
func (r *HAProxyReader) ReadConfig(ctx context.Context) (*ConfigSnapshot, error) {
	snap := &ConfigSnapshot{
		ProviderID: "haproxy",
	}

	// Parse main config (SNI passthrough)
	sniRoutes, unmanaged, err := r.parseMainConfig()
	if err != nil {
		// File not exists = never managed by Aegis, not an error
		if os.IsNotExist(err) {
			return snap, nil
		}
		return nil, fmt.Errorf("parse haproxy config: %w", err)
	}
	snap.Routes = append(snap.Routes, sniRoutes...)
	snap.Unmanaged = append(snap.Unmanaged, unmanaged...)

	// Parse TCP config (raw port forwarding)
	tcpRoutes, tcpUnmanaged, err := r.parseTCPConfig()
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("parse haproxy tcp config: %w", err)
	}
	snap.Routes = append(snap.Routes, tcpRoutes...)
	snap.Unmanaged = append(snap.Unmanaged, tcpUnmanaged...)

	return snap, nil
}

// parseMainConfig 解析 haproxy.cfg，提取 SNI 路由。
func (r *HAProxyReader) parseMainConfig() (routes []RouteSpec, unmanaged []UnmanagedBlock, err error) {
	f, err := os.Open(r.configPath)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNum := 0

	// 解析两遍：
	// pass 1: use_backend → {sni → backend_name}
	// pass 2: backend → {backend_name → target}
	type sniRule struct {
		sni     string
		backend string
		line    int
	}
	var sniRules []sniRule
	backendTargets := make(map[string]string) // backend_name → target
	var currentBackend string
	var unmanagedBlock strings.Builder
	inUnmanagedBlock := false

	flushUnmanaged := func(line int) {
		if unmanagedBlock.Len() > 0 {
			unmanaged = append(unmanaged, UnmanagedBlock{
				Content:  strings.TrimSpace(unmanagedBlock.String()),
				Location: fmt.Sprintf("%s:%d", r.configPath, line),
			})
			unmanagedBlock.Reset()
		}
	}

	for scanner.Scan() {
		line := scanner.Text()
		lineNum++
		trimmed := strings.TrimSpace(line)

		// 跳过注释和空行
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			if inUnmanagedBlock {
				unmanagedBlock.WriteString(line + "\n")
			}
			continue
		}

		// 检测 backend 块
		if strings.HasPrefix(trimmed, "backend ") {
			flushUnmanaged(lineNum)
			inUnmanagedBlock = false
			name := strings.TrimSpace(strings.TrimPrefix(trimmed, "backend"))
			currentBackend = strings.Fields(name)[0]
			continue
		}

		// 检测 frontend
		if strings.HasPrefix(trimmed, "frontend ") {
			flushUnmanaged(lineNum)
			inUnmanagedBlock = false
			continue
		}

		// 检测 global / defaults
		if trimmed == "global" || trimmed == "defaults" {
			flushUnmanaged(lineNum)
			inUnmanagedBlock = true
			unmanagedBlock.WriteString(line + "\n")
			continue
		}

		// 在 block 内解析指令
		if inUnmanagedBlock {
			unmanagedBlock.WriteString(line + "\n")
			continue
		}

		// use_backend be_xxx if { req.ssl_sni -i hostname }
		if strings.HasPrefix(trimmed, "use_backend ") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 6 {
				sni := parts[len(parts)-2] // 最后一个字段是 hostname
				backend := parts[1]         // 第二个字段是 backend name
				sniRules = append(sniRules, sniRule{
					sni:     sni,
					backend: backend,
					line:    lineNum,
				})
			}
			continue
		}

		// 在 backend 块内: server target <upstream> check
		if strings.HasPrefix(trimmed, "server ") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 3 && currentBackend != "" {
				target := parts[2]
				// 跳过 reject backend
				if currentBackend != "be_reject" {
					backendTargets[currentBackend] = target
				}
			}
			continue
		}

		// default_backend be_reject — skip
		if strings.HasPrefix(trimmed, "default_backend") {
			continue
		}
		// mode, tcp-request, bind, log, timeout — skip known directives
		if strings.HasPrefix(trimmed, "mode ") ||
			strings.HasPrefix(trimmed, "tcp-request ") ||
			strings.HasPrefix(trimmed, "bind ") ||
			strings.HasPrefix(trimmed, "log ") ||
			strings.HasPrefix(trimmed, "timeout ") {
			continue
		}

		// 无法识别的行 → 标记为 unmanaged
		unmanaged = append(unmanaged, UnmanagedBlock{
			Content:  trimmed,
			Location: fmt.Sprintf("%s:%d", r.configPath, lineNum),
		})
	}
	flushUnmanaged(lineNum)

	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}

	// 交叉引用: SNI rule → backend → target
	for _, rule := range sniRules {
		target, found := backendTargets[rule.backend]
		if !found || rule.backend == "be_reject" {
			continue
		}
		routes = append(routes, RouteSpec{
			Transport: "tcp",
			TLSMode:   "passthrough",
			Match: MatchSpec{
				SNI: rule.sni,
			},
			Upstream: UpstreamSpec{
				Type:   "tcp",
				Target: target,
			},
		})
	}

	return routes, unmanaged, nil
}

// parseTCPConfig 解析 haproxy_tcp.cfg，提取 TCP 端口转发路由。
func (r *HAProxyReader) parseTCPConfig() (routes []RouteSpec, unmanaged []UnmanagedBlock, err error) {
	f, err := os.Open(r.tcpConfigPath)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	lineNum := 0

	var backendTargets = make(map[string]string)
	var currentBackend string
	var unmanagedBlock strings.Builder
	inUnmanagedBlock := false
	var currentPort int

	flushUnmanaged := func(line int) {
		if unmanagedBlock.Len() > 0 {
			unmanaged = append(unmanaged, UnmanagedBlock{
				Content:  strings.TrimSpace(unmanagedBlock.String()),
				Location: fmt.Sprintf("%s:%d", r.tcpConfigPath, line),
			})
			unmanagedBlock.Reset()
		}
	}

	for scanner.Scan() {
		line := scanner.Text()
		lineNum++
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			if inUnmanagedBlock {
				unmanagedBlock.WriteString(line + "\n")
			}
			continue
		}

		if strings.HasPrefix(trimmed, "frontend ") {
			flushUnmanaged(lineNum)
			inUnmanagedBlock = false
			// 从 frontend 名中提取端口: fe_tcp_XXXX
			name := strings.TrimSpace(strings.TrimPrefix(trimmed, "frontend"))
			name = strings.Fields(name)[0]
			if port, ok := extractTCPPort(name); ok {
				currentPort = port
			}
			continue
		}

		if strings.HasPrefix(trimmed, "backend ") {
			flushUnmanaged(lineNum)
			inUnmanagedBlock = false
			name := strings.TrimSpace(strings.TrimPrefix(trimmed, "backend"))
			currentBackend = strings.Fields(name)[0]
			continue
		}

		if trimmed == "global" || trimmed == "defaults" {
			flushUnmanaged(lineNum)
			inUnmanagedBlock = true
			unmanagedBlock.WriteString(line + "\n")
			continue
		}

		if inUnmanagedBlock {
			unmanagedBlock.WriteString(line + "\n")
			continue
		}

		// bind 0.0.0.0:XXXX
		if strings.HasPrefix(trimmed, "bind ") {
			parts := strings.Fields(trimmed)
			for _, p := range parts {
				if idx := strings.LastIndex(p, ":"); idx >= 0 {
					fmt.Sscanf(p[idx+1:], "%d", &currentPort)
				}
			}
			continue
		}

		// server target <upstream>
		if strings.HasPrefix(trimmed, "server ") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 3 && currentBackend != "" {
				backendTargets[currentBackend] = parts[2]
			}
			continue
		}

		// 已知指令跳过
		if strings.HasPrefix(trimmed, "mode ") ||
			strings.HasPrefix(trimmed, "log ") ||
			strings.HasPrefix(trimmed, "timeout ") {
			continue
		}

		unmanaged = append(unmanaged, UnmanagedBlock{
			Content:  trimmed,
			Location: fmt.Sprintf("%s:%d", r.tcpConfigPath, lineNum),
		})
	}
	flushUnmanaged(lineNum)

	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}

	// TCP config 没有 use_backend 规则直接映射端口到 backend，
	// 但我们可以从 frontend 名推断。
	// 由于 TCP config 的 frontend/backend 不交叉引用（同一行），
	// 暂且通过绑定端口 + server 行推断路由。
	for be, target := range backendTargets {
		if currentPort > 0 {
			routes = append(routes, RouteSpec{
				Transport: "tcp",
				Match: MatchSpec{
					Port: currentPort,
				},
				Upstream: UpstreamSpec{
					Type:   "tcp",
					Target: target,
				},
			})
			continue
		}
		// 看 backend 名是否能提取端口
		if port, ok := extractTCPPort(be); ok {
			routes = append(routes, RouteSpec{
				Transport: "tcp",
				Match: MatchSpec{
					Port: port,
				},
				Upstream: UpstreamSpec{
					Type:   "tcp",
					Target: target,
				},
			})
		}
	}

	return routes, unmanaged, nil
}

// extractTCPPort 从 backend/frontend 名称中提取端口号。
// 格式: be_tcp_8080 → 8080, fe_tcp_8080 → 8080
func extractTCPPort(name string) (int, bool) {
	parts := strings.Split(name, "_")
	if len(parts) < 2 {
		return 0, false
	}
	last := parts[len(parts)-1]
	var port int
	if _, err := fmt.Sscanf(last, "%d", &port); err == nil && port > 0 {
		return port, true
	}
	return 0, false
}

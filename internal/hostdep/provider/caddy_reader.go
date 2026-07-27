package provider

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
)

// ============================================================================
// CaddyReader — 从 Caddyfile 逆向解析路由配置
// ============================================================================

// CaddyReader implements the Reader interface for Caddy v2+.
type CaddyReader struct {
	configPath string // 当前 Caddyfile 路径
}

// NewCaddyReader creates a CaddyReader.
func NewCaddyReader(configPath string) *CaddyReader {
	return &CaddyReader{configPath: configPath}
}

// ID returns "caddy", matching CaddyProvider.State().ID.
func (r *CaddyReader) ID() string { return "caddy" }

// ReadConfig parses the current Caddyfile and returns a structured snapshot.
func (r *CaddyReader) ReadConfig(ctx context.Context) (*ConfigSnapshot, error) {
	f, err := os.Open(r.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// 配置文件不存在，说明还没被 Aegis 管理过 — 不是错误
			return &ConfigSnapshot{
				ProviderID: "caddy",
				Routes:     nil,
				Unmanaged:  nil,
			}, nil
		}
		return nil, fmt.Errorf("open caddyfile: %w", err)
	}
	defer f.Close()

	snap := &ConfigSnapshot{
		ProviderID: "caddy",
	}

	scanner := bufio.NewScanner(f)
	var currentBlock strings.Builder
	inBlock := false
	braceDepth := 0
	lineNum := 0

	flushBlock := func(content string, line int) {
		content = strings.TrimSpace(content)
		if content == "" {
			return
		}
		// 尝试解析为 site block
		if routes, ok := r.parseSiteBlock(content); ok {
			snap.Routes = append(snap.Routes, routes...)
		} else if !isCaddyGlobalBlock(content) {
			// 无法解析的块 → 标记为 Unmanaged
			snap.Unmanaged = append(snap.Unmanaged, UnmanagedBlock{
				Content:  content,
				Location: fmt.Sprintf("%s:%d", r.configPath, line),
			})
		}
	}

	for scanner.Scan() {
		line := scanner.Text()
		lineNum++

		// 跳过纯注释行和空行（只记结构）
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			if !inBlock {
				continue
			}
			currentBlock.WriteString(line + "\n")
			continue
		}

		// 跟踪花括号深度
		braceDepth += strings.Count(line, "{")
		braceDepth -= strings.Count(line, "}")

		if !inBlock && braceDepth > 0 {
			// 进入一个新的顶级块
			inBlock = true
			currentBlock.Reset()
			currentBlock.WriteString(line + "\n")
		} else if inBlock {
			currentBlock.WriteString(line + "\n")
			if braceDepth == 0 {
				// 块结束
				flushBlock(currentBlock.String(), lineNum)
				inBlock = false
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read caddyfile: %w", err)
	}

	return snap, nil
}

// parseSiteBlock 尝试解析一个 Caddy site block，返回 RouteSpec 列表。
// site_addr { ... } 可能包含多个 handle 子块。
func (r *CaddyReader) parseSiteBlock(block string) ([]RouteSpec, bool) {
	lines := strings.Split(block, "\n")
	if len(lines) < 2 {
		return nil, false
	}

	// 提取 site address：第一行到第一个 {
	firstLine := strings.TrimSpace(lines[0])
	addr := strings.TrimSuffix(firstLine, "{")
	addr = strings.TrimSpace(addr)

	// 校验是不是有效的域名
	domain := extractDomain(addr)
	if domain == "" {
		return nil, false // 不是 site block，可能是全局配置
	}

	// 解析内部指令
	var routes []RouteSpec
	var tlsCert string
	var currentPath string

	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])

		switch {
		case line == "" || strings.HasPrefix(line, "#"):
			continue

		case strings.HasPrefix(line, "tls "):
			// tls cert.pem [key.pem]
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				tlsCert = parts[1]
			}

		case strings.HasPrefix(line, "reverse_proxy "):
			target := extractTarget(line)
			if target != "" {
				routes = append(routes, buildRoute(domain, currentPath, target, tlsCert))
			}

		case strings.HasPrefix(line, "handle ") || strings.HasPrefix(line, "handle_path "):
			// handle /path/* { ... } 或 handle { ... }
			path := extractHandlePath(line)
			currentPath = path

		case strings.HasPrefix(line, "respond "):
			// respond "message" 503 — 维护模式，跳过路由解析
			continue

		case line == "}":
			currentPath = "" // 退出子块
		}
	}

	if len(routes) == 0 {
		return nil, false
	}
	return routes, true
}

// extractDomain 从 site address 中提取域名。
// 格式: "example.com", "http://example.com", "*.example.com"
func extractDomain(addr string) string {
	addr = strings.TrimPrefix(addr, "http://")
	addr = strings.TrimPrefix(addr, "https://")
	// 移除 :port 后缀
	if idx := strings.Index(addr, ":"); idx >= 0 {
		addr = addr[:idx]
	}
	if addr == "" || strings.Contains(addr, " ") {
		return ""
	}
	return addr
}

// extractTarget 从 reverse_proxy 指令中提取目标地址。
// 格式: "reverse_proxy 127.0.0.1:3000"
//       "reverse_proxy 127.0.0.1:3000 { ... }"
func extractTarget(line string) string {
	rest := strings.TrimSpace(strings.TrimPrefix(line, "reverse_proxy"))
	if idx := strings.Index(rest, "{"); idx >= 0 {
		rest = strings.TrimSpace(rest[:idx])
	}
	if idx := strings.Index(rest, " "); idx >= 0 {
		rest = rest[:idx]
	}
	return rest
}

// extractHandlePath 从 handle / handle_path 指令中提取路径。
// 格式: "handle /api/* {" → "/api/*"
//       "handle {" → ""
func extractHandlePath(line string) string {
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "handle_path ") {
		line = strings.TrimPrefix(line, "handle_path ")
	} else {
		line = strings.TrimPrefix(line, "handle ")
	}
	line = strings.TrimSpace(line)
	line = strings.TrimSuffix(line, "{")
	return strings.TrimSpace(line)
}

// buildRoute 从解析出的字段构建 RouteSpec。
func buildRoute(domain, path, target, tlsCert string) RouteSpec {
	r := RouteSpec{
		Transport:   "tcp",
		AppProtocol: "http",
		Match: MatchSpec{
			Host: domain,
			Path: path,
		},
		Upstream: UpstreamSpec{
			Type:   "http",
			Target: target,
		},
	}
	if tlsCert != "" {
		r.TLSMode = "terminate"
	}
	return r
}

// isCaddyGlobalBlock 检查是否是 Caddy 全局配置块（{ ... }），不是 site block。
func isCaddyGlobalBlock(block string) bool {
	lines := strings.SplitN(block, "\n", 2)
	if len(lines) < 2 {
		return false
	}
	first := strings.TrimSpace(lines[0])
	// 全局块以 { 开头（没有 site address）
	return first == "{"
}

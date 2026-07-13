package provider

import "fmt"

// ============================================================================
// ModeSwitchPreview — 模式切换影响分析
// ============================================================================

// ModeSwitchPreview describes the impact of switching from one RuntimeMode to another.
type ModeSwitchPreview struct {
	CurrentMode     string                `json:"current_mode"`      // "edge_mux"
	TargetMode      string                `json:"target_mode"`       // "legacy"
	TotalRoutes     int                   `json:"total_routes"`
	RouteBreakdown  []CompositionSummary  `json:"route_breakdown"`   // per-composition stats
	AffectedRoutes  AffectedRouteCounts   `json:"affected_routes"`
	ProviderChanges []ProviderChange      `json:"provider_changes"`  // what happens to each provider
	Risks           []string              `json:"risks"`             // human-readable warnings
}

// CompositionSummary shows route counts and support status for one composition.
type CompositionSummary struct {
	Key           string `json:"key"`            // "https_route"
	Name          string `json:"name"`           // "HTTPS Route"
	RouteCount    int    `json:"route_count"`
	CurrentModeOK bool   `json:"current_mode_ok"`
	TargetModeOK  bool   `json:"target_mode_ok"`
	Reason        string `json:"reason,omitempty"` // why unsupported
}

// AffectedRouteCounts summarizes how routes survive the switch.
type AffectedRouteCounts struct {
	Kept        int `json:"kept"`         // works in both modes
	Unsupported int `json:"unsupported"`  // target mode can't serve these
}

// ProviderChange describes how a provider is affected by the switch.
type ProviderChange struct {
	ProviderID string `json:"provider_id"` // "caddy" | "haproxy"
	Action     string `json:"action"`      // "reconfig" | "stop" | "start" | "unchanged"
	Detail     string `json:"detail"`      // human-readable explanation
}

// AnalyseModeSwitch computes the impact of switching to targetMode.
// routes should be all active routes from the DB.
// currentMode is the currently active RuntimeMode.
func AnalyseModeSwitch(routes []RouteSpec, currentMode, targetMode RuntimeMode) *ModeSwitchPreview {
	preview := &ModeSwitchPreview{
		CurrentMode: currentMode.ID,
		TargetMode:  targetMode.ID,
		TotalRoutes: len(routes),
	}

	// Group routes by composition key
	type compGroup struct {
		key   string
		routes []RouteSpec
	}
	groups := make(map[string]*compGroup)
	var groupOrder []string
	for _, r := range routes {
		// Derive composition key from route spec
		key := deriveCompKey(r)
		if key == "" {
			continue
		}
		if _, ok := groups[key]; !ok {
			groups[key] = &compGroup{key: key}
			groupOrder = append(groupOrder, key)
		}
		groups[key].routes = append(groups[key].routes, r)
	}

	// Build breakdown per composition
	for _, key := range groupOrder {
		g := groups[key]
		def := LookupComp(CompKey(key))

		cs := CompositionSummary{
			Key:        key,
			RouteCount: len(g.routes),
		}
		if def != nil {
			cs.Name = def.Name
		}
		// Check support in current mode
		if def != nil {
			cs.CurrentModeOK = CompKeySupported(CompKey(key), currentMode)
			cs.TargetModeOK = CompKeySupported(CompKey(key), targetMode)
		}
		if !cs.TargetModeOK && def != nil {
			cs.Reason = unsupportedReason(key, targetMode)
		}

		preview.RouteBreakdown = append(preview.RouteBreakdown, cs)

		if cs.TargetModeOK {
			preview.AffectedRoutes.Kept += cs.RouteCount
		} else {
			preview.AffectedRoutes.Unsupported += cs.RouteCount
		}
	}

	// Provider changes
	currentIDs := currentMode.ProviderIDs()
	targetIDs := targetMode.ProviderIDs()

	// Providers in current but not in target → need to stop
	for _, id := range currentIDs {
		found := false
		for _, tid := range targetIDs {
			if tid == id {
				found = true
				break
			}
		}
		change := ProviderChange{ProviderID: id}
		if !found {
			change.Action = "stop"
			change.Detail = stopReason(id, currentMode, targetMode)
		} else {
			// Same provider, but might need reconfig (different ports)
			change.Action = "reconfig"
			change.Detail = reconfigReason(id, currentMode, targetMode)
		}
		preview.ProviderChanges = append(preview.ProviderChanges, change)
	}

	// Providers in target but not in current → need to start
	for _, id := range targetIDs {
		found := false
		for _, cid := range currentIDs {
			if cid == id {
				found = true
				break
			}
		}
		if !found {
			preview.ProviderChanges = append(preview.ProviderChanges, ProviderChange{
				ProviderID: id,
				Action:     "start",
				Detail:     fmt.Sprintf("新模式需要 %s，当前未运行", providerLabel(id)),
			})
		}
	}

	// Generate human-readable risks
	if preview.AffectedRoutes.Unsupported > 0 {
		preview.Risks = append(preview.Risks,
			fmt.Sprintf("%d 条路由在 %s 模式下无法服务，涉及 %d 个组合能力",
				preview.AffectedRoutes.Unsupported, targetMode.Label, unsupportedGroupCount(preview.RouteBreakdown)))
	}
	for _, pc := range preview.ProviderChanges {
		if pc.Action == "stop" {
			preview.Risks = append(preview.Risks,
				fmt.Sprintf("%s 将停止，已有连接将被断开", providerLabel(pc.ProviderID)))
		}
		if pc.Action == "reconfig" {
			preview.Risks = append(preview.Risks,
				fmt.Sprintf("%s 配置将重新加载，可能有秒级连接中断", providerLabel(pc.ProviderID)))
		}
	}

	return preview
}

// deriveCompKey extracts a CompKey from a RouteSpec.
func deriveCompKey(r RouteSpec) string {
	if r.TLSMode == "passthrough" {
		return string(CompTLSPassthrough)
	}
	if r.TLSMode == "terminate" {
		if r.AppProtocol == "http" {
			if r.Transport == "udp" {
				return string(CompHTTP3)
			}
			return string(CompHTTPSRoute)
		}
	}
	if r.AppProtocol == "http" && r.TLSMode == "none" {
		return string(CompHTTPRoute)
	}
	if r.Transport == "tcp" {
		return string(CompRawTCP)
	}
	if r.Transport == "udp" {
		return string(CompRawUDP)
	}
	return ""
}

func unsupportedReason(key string, targetMode RuntimeMode) string {
	switch CompKey(key) {
	case CompTLSPassthrough:
		if !targetMode.hasAtomBinding("sni") {
			return "目标模式没有 SNI 预读能力（需要 HAProxy）"
		}
		return "TLS 直通需要 SNI 预读，当前模式不支持"
	case CompHTTPSRoute, CompHTTPRoute:
		return ""
	case CompHTTP3:
		if !targetMode.hasAtomBinding("quic") {
			return "目标模式没有 QUIC/HTTP3 能力"
		}
		return ""
	}
	return ""
}

// stopReason returns a human-readable reason why a provider is being stopped
// during a mode switch. Derived from port comparison, not hardcoded names.
func stopReason(id string, current, target RuntimeMode) string {
	currentPort, _ := current.PortFor(id, "tcp")
	if currentPort > 0 {
		return fmt.Sprintf("新模式不需要端口 :%d 上的监听（%s 将停止）", currentPort, id)
	}
	return fmt.Sprintf("%s 在新模式下不再需要", id)
}

// reconfigReason returns a human-readable reason why a provider's config is
// being regenerated during a mode switch. Derived from port comparison.
func reconfigReason(id string, current, target RuntimeMode) string {
	cp, _ := current.PortFor(id, "tcp")
	tp, _ := target.PortFor(id, "tcp")
	if cp != tp {
		return fmt.Sprintf("监听端口从 :%d 变更为 :%d", cp, tp)
	}
	return "配置重新生成"
}

// providerLabel returns a display label for a provider ID.
// Uses the registry when available; falls back to the raw ID.
func providerLabel(id string) string {
	return id
}

func unsupportedGroupCount(breakdown []CompositionSummary) int {
	count := 0
	for _, b := range breakdown {
		if !b.TargetModeOK {
			count++
		}
	}
	return count
}


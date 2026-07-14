package provider

import "fmt"

// ============================================================================
// Drift — 期望状态 (Plan) vs 实际状态 (Reader) 差异检测
// ============================================================================

// DriftReport 描述 DB 配置与中间件实际配置之间的差异。
type DriftReport struct {
	ProviderID string `json:"provider_id"`

	// Consistent 表示期望与实际完全一致。
	Consistent bool `json:"consistent"`

	// ExpectedRoutes 是 DB 中的路由数（Plan 里的路由数）。
	ExpectedRoutes int `json:"expected_routes"`

	// ActualRoutes 是从中间件实际读取到的路由数。
	ActualRoutes int `json:"actual_routes"`

	// Missing 是在 Plan 中存在但实际配置中没有的路由。
	Missing []DriftedRoute `json:"missing,omitempty"`

	// Unexpected 是在实际配置中存在但在 Plan 中没有的路由（残留配置）。
	Unexpected []DriftedRoute `json:"unexpected,omitempty"`

	// Changed 是 Plan 和实际配置都存在但参数不同的路由。
	Changed []DriftedRouteDiff `json:"changed,omitempty"`

	// UnmanagedBlocks 是无法解析的配置块（手写或其他管理工具写入的）。
	UnmanagedBlocks []UnmanagedBlock `json:"unmanaged_blocks,omitempty"`
}

// DriftedRoute 描述一条不在期望状态中的路由。
type DriftedRoute struct {
	Domain string `json:"domain"`
	Path   string `json:"path,omitempty"`
	Target string `json:"target"`
	Source string `json:"source"` // "plan" | "config"
}

// DriftedRouteDiff 描述期望和实际不一致的路由（同 domain+path 但 target 不同）。
type DriftedRouteDiff struct {
	Domain      string `json:"domain"`
	Path        string `json:"path,omitempty"`
	ExpectedTarget string `json:"expected_target"`
	ActualTarget   string `json:"actual_target"`
}

// DetectDrift 比较期望状态 (plan) 和实际状态 (snapshot) 之间的差异。
func DetectDrift(plan *Plan, snapshot *ConfigSnapshot) *DriftReport {
	report := &DriftReport{
		ProviderID: snapshot.ProviderID,
	}

	if plan == nil {
		// 没有 Plan = 没有期望状态
		report.Consistent = len(snapshot.Routes) == 0
		report.Unexpected = routesToDrifted(snapshot.Routes, "config")
		report.UnmanagedBlocks = snapshot.Unmanaged
		return report
	}

	report.ExpectedRoutes = len(plan.Routes)
	report.ActualRoutes = len(snapshot.Routes)

	// 构建索引: domain+path → RouteSpec
	expected := make(map[string]RouteSpec)
	for _, r := range plan.Routes {
		key := routeKey(r)
		expected[key] = r
	}

	actual := make(map[string]RouteSpec)
	for _, r := range snapshot.Routes {
		key := routeKey(r)
		actual[key] = r
	}

	// 找 Missing: 在 expected 中但不在 actual 中
	for key, r := range expected {
		if _, found := actual[key]; !found {
			report.Missing = append(report.Missing, DriftedRoute{
				Domain: r.Match.Host,
				Path:   r.Match.Path,
				Target: r.Upstream.Target,
				Source: "plan",
			})
		}
	}

	// 找 Unexpected: 在 actual 中但不在 expected 中
	for key, r := range actual {
		if _, found := expected[key]; !found {
			report.Unexpected = append(report.Unexpected, DriftedRoute{
				Domain: r.Match.Host,
				Path:   r.Match.Path,
				Target: r.Upstream.Target,
				Source: "config",
			})
		}
	}

	// 找 Changed: 同 key 但 target 不同
	for key, er := range expected {
		ar, found := actual[key]
		if found && er.Upstream.Target != ar.Upstream.Target {
			report.Changed = append(report.Changed, DriftedRouteDiff{
				Domain:         er.Match.Host,
				Path:           er.Match.Path,
				ExpectedTarget: er.Upstream.Target,
				ActualTarget:   ar.Upstream.Target,
			})
		}
	}

	report.Consistent = len(report.Missing) == 0 &&
		len(report.Unexpected) == 0 &&
		len(report.Changed) == 0

	report.UnmanagedBlocks = snapshot.Unmanaged

	return report
}

// routeKey 生成 RouteSpec 的唯一键（domain + path），用于 diff 匹配。
func routeKey(r RouteSpec) string {
	return fmt.Sprintf("%s|%s", r.Match.Host, r.Match.Path)
}

func routesToDrifted(routes []RouteSpec, source string) []DriftedRoute {
	d := make([]DriftedRoute, 0, len(routes))
	for _, r := range routes {
		d = append(d, DriftedRoute{
			Domain: r.Match.Host,
			Path:   r.Match.Path,
			Target: r.Upstream.Target,
			Source: source,
		})
	}
	return d
}

package provider

import (
	"sort"
	"strings"
)

// SortRoutesForMatch returns routes in provider-agnostic match order.
// Higher priority routes win first; ties prefer more specific Host/Path rules
// before catch-all fallback routes.
func SortRoutesForMatch(routes []RouteSpec) []RouteSpec {
	out := append([]RouteSpec(nil), routes...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Priority != b.Priority {
			return a.Priority > b.Priority
		}
		if hostSpecificity(a.Match.Host) != hostSpecificity(b.Match.Host) {
			return hostSpecificity(a.Match.Host) > hostSpecificity(b.Match.Host)
		}
		if pathSpecificity(a.Match.Path) != pathSpecificity(b.Match.Path) {
			return pathSpecificity(a.Match.Path) > pathSpecificity(b.Match.Path)
		}
		if a.Match.Host != b.Match.Host {
			return a.Match.Host < b.Match.Host
		}
		return a.Match.Path < b.Match.Path
	})
	return out
}

func hostSpecificity(host string) int {
	switch strings.TrimSpace(host) {
	case "":
		return 0
	case "http://":
		return 1
	default:
		return 2
	}
}

func pathSpecificity(path string) int {
	path = strings.Trim(path, "/")
	if path == "" {
		return 0
	}
	return len(strings.Split(path, "/"))
}

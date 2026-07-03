package noderuntime

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ReconcileMode constants.
const (
	ReconcileModeDryRun = "dry_run"
	ReconcileModeApply  = "apply"
)

// Reconciler runs the node sync loop.
type Reconciler struct {
	config            *Config
	client            *Client
	cache             *CacheManager
	nodeID            string
	gwStatusProvider  GatewayStatusProvider
	secretProvider    *InMemorySecretProvider
	caddyApplier      CaddyfileApplier
}

// NewReconciler creates a new node reconciler.
func NewReconciler(config *Config, client *Client, cache *CacheManager) *Reconciler {
	return &Reconciler{
		config: config,
		client: client,
		cache:  cache,
		nodeID: config.NodeID,
	}
}

// SetGatewayStatusProvider sets the gateway status provider for heartbeat reporting.
func (r *Reconciler) SetGatewayStatusProvider(p GatewayStatusProvider) {
	r.gwStatusProvider = p
}

// SetSecretProvider sets the InMemorySecretProvider for GatewayLink token injection.
// During each sync cycle, tokens for all routes with gateway_link_id are fetched
// from the control plane and populated into this provider.
// Tokens exist only in memory - never written to disk cache.
func (r *Reconciler) SetSecretProvider(p *InMemorySecretProvider) {
	r.secretProvider = p
}

// SetCaddyfileApplier sets the Caddyfile applier for rendering and reloading proxy config.
func (r *Reconciler) SetCaddyfileApplier(a CaddyfileApplier) {
	r.caddyApplier = a
}

// SyncOnce performs a single sync cycle.
// Returns the updated actual state cache on success.
func (r *Reconciler) SyncOnce() (*ActualStateCache, error) {
	// Step 0: Collect all gateway statuses for heartbeat
	var gwInfos []*LocalGatewayInfo
	if r.gwStatusProvider != nil {
		gwInfos = r.gwStatusProvider.LocalGatewayStatuses()
	}

	// Step 1: Send heartbeat
	hbResp, err := r.client.SendHeartbeat("online", "v1.8C", r.nodeID, gwInfos)
	if err != nil {
		return r.failedState(0, "", fmt.Sprintf("heartbeat failed: %v", err))
	}

	// Step 2: Check if outdated
	if !hbResp.NodeIsOutdated {
		// No update needed; read existing cached state
		existing, _ := r.cache.ReadActualState()
		if existing != nil {
			return existing, nil
		}
		return r.successState(0, "", "no_update_needed", ""), nil
	}

	// Step 3: Pull desired state
	ds, err := r.client.PullDesiredState()
	if err != nil {
		return r.failedState(0, "", fmt.Sprintf("pull desired state failed: %v", err))
	}

	return r.processDesiredState(ds)
}

// processDesiredState validates and caches a pulled desired state.
func (r *Reconciler) processDesiredState(ds *DesiredStateResponse) (*ActualStateCache, error) {
	// Step 4: Build DesiredStateCache
	dsCache := &DesiredStateCache{
		NodeID:    ds.NodeID,
		Revision:  ds.Revision,
		StateHash: ds.StateHash,
		StateJSON: ds.StateJSON,
	}

	// Step 5: Validate desired state
	validation := ValidateDesiredStateForNode(r.nodeID, dsCache)
	if !validation.IsValid {
		return r.failedState(ds.Revision, ds.StateHash,
			fmt.Sprintf("desired state validation failed: %s", strings.Join(validation.Errors, "; ")))
	}

	// Step 6: Extract routing table
	rtCache, err := extractRoutingTableFromState(ds.StateJSON)
	if err != nil {
		return r.failedState(ds.Revision, ds.StateHash,
			fmt.Sprintf("extract routing table failed: %v", err))
	}
	rtCache.NodeID = r.nodeID
	rtCache.Revision = ds.Revision

	// Step 7: Validate routing table
	rtValidation := ValidateRoutingTable(r.nodeID, rtCache)
	if !rtValidation.IsValid {
		return r.failedState(ds.Revision, ds.StateHash,
			fmt.Sprintf("routing table validation failed: %s", strings.Join(rtValidation.Errors, "; ")))
	}

	// Step 8: Dry-run: write caches
	if err := r.cache.EnsureDir(); err != nil {
		return r.failedState(ds.Revision, ds.StateHash,
			fmt.Sprintf("cache dir error: %v", err))
	}

	if err := r.cache.WriteDesiredState(dsCache); err != nil {
		return r.failedState(ds.Revision, ds.StateHash,
			fmt.Sprintf("write desired state cache: %v", err))
	}

	if err := r.cache.WriteRoutingTable(rtCache); err != nil {
		return r.failedState(ds.Revision, ds.StateHash,
			fmt.Sprintf("write routing table cache: %v", err))
	}

	// Step 8b: Sync GatewayLink secrets for relay auth
	if r.secretProvider != nil && r.client != nil {
		secrets := SyncGatewayLinkSecrets(r.client, rtCache.Entries)
		for linkID, token := range secrets {
			r.secretProvider.AddSecret(linkID, token)
		}
	}

	// Step 8c: Apply routing table to Caddy (render → validate → backup → replace → reload)
	caddyErr := ""
	if r.caddyApplier != nil {
		if err := r.caddyApplier.Apply(rtCache.Entries); err != nil {
			caddyErr = err.Error()
		}
	}

	// Step 9: Build diagnostics status
	diagMode := r.config.ReconcileMode
	diagnostics := map[string]interface{}{
		"routing_table_entries": len(rtCache.Entries),
		"cache_written":         true,
		"reconcile_mode":        diagMode,
		"caddy_applied":         caddyErr == "",
	}
	if caddyErr != "" {
		diagnostics["caddy_error"] = caddyErr
	}
	diagJSON := jsonMarshalSimple(diagnostics)

	// Build gateway status string for actual state (aggregate from all providers)
	gwStatusStr := ""
	if r.gwStatusProvider != nil {
		infos := r.gwStatusProvider.LocalGatewayStatuses()
		if len(infos) > 0 {
			// Aggregate: "online" if all online, "degraded" if any degraded, else first non-ok status
			allOnline := true
			for _, info := range infos {
				if info.Status != "online" && info.Status != "ready" {
					allOnline = false
					break
				}
			}
			if allOnline {
				gwStatusStr = "online"
			} else {
				gwStatusStr = "degraded"
			}
		}
	}

	// Step 10: Report actual state
	overallStatus := "applied"
	if caddyErr != "" {
		overallStatus = "degraded"
	}
	actual := r.successState(ds.Revision, ds.StateHash, overallStatus, diagJSON)
	if caddyErr != "" {
		actual.LastError = "caddy apply failed: " + caddyErr
	}

	if err := r.client.ReportActualState(ActualStateRequest{
		AppliedRevision:   actual.AppliedRevision,
		StateHash:         actual.StateHash,
		Status:            actual.Status,
		LastError:         actual.LastError,
		GatewayStatus:     gwStatusStr,
		DiagnosticsStatus: diagJSON,
	}); err != nil {
		// Report failed but cache was written — return degraded
		actual.Status = "degraded"
		actual.LastError = fmt.Sprintf("state applied locally but report failed: %v", err)
	}

	// Update actual state cache
	r.cache.WriteActualState(actual)

	return actual, nil
}

// successState creates an Applied actual state.
func (r *Reconciler) successState(revision int, stateHash, status string, diagJSON string) *ActualStateCache {
	if diagJSON == "" {
		diagJSON = "{}"
	}
	return &ActualStateCache{
		AppliedRevision: revision,
		StateHash:       stateHash,
		Status:          status,
		ReportedAt:      time.Now().Format(time.RFC3339),
	}
}

// failedState creates a Failed actual state and reports it.
func (r *Reconciler) failedState(revision int, stateHash, errMsg string) (*ActualStateCache, error) {
	actual := &ActualStateCache{
		AppliedRevision: revision,
		StateHash:       stateHash,
		Status:          "failed",
		LastError:       errMsg,
		ReportedAt:      time.Now().Format(time.RFC3339),
	}

	// Report to control plane (best-effort)
	r.client.ReportActualState(ActualStateRequest{
		AppliedRevision: revision,
		StateHash:       stateHash,
		Status:          "failed",
		LastError:       errMsg,
	})

	// Cache the failed state
	r.cache.WriteActualState(actual)

	return actual, fmt.Errorf(errMsg)
}

// ============================================================================
// json helpers
// ============================================================================

func jsonMarshalSimple(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(data)
}

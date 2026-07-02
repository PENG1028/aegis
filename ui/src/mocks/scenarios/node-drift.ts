// ─── Scenario 4: Node Drift ───
// node-c desired config version 43, actual version 41, status drifted
// Shows sync drift between desired and actual state.

import { scenarioNormal } from './normal';
import type { ScenarioData } from './types';

const NOW = '2026-07-02T11:00:00Z';

function deepClone<T>(obj: T): T {
  return JSON.parse(JSON.stringify(obj));
}

export const scenarioNodeDrift: ScenarioData = (() => {
  const base = deepClone(scenarioNormal);

  // Modify node-c: desired vs actual mismatch
  const nodeC = base.nodes.find(n => n.node_id === 'node-c')!;
  nodeC.desired_revision = 43;
  nodeC.applied_revision = 41;
  nodeC.sync_status = 'outdated';
  nodeC.status = 'degraded';
  nodeC.last_heartbeat_at = '2026-07-02T08:00:00Z'; // Stale heartbeat

  // Update sync status for node-c
  const syncC = base.syncStatuses.find(s => s.node_id === 'node-c')!;
  syncC.desired_revision = 43;
  syncC.applied_revision = 41;
  syncC.desired_hash = 'ghi789_v43';
  syncC.actual_hash = 'ghi789_v41';
  syncC.status = 'outdated';
  syncC.last_error = 'config drift detected: desired v43, actual v41';
  syncC.provider_status = { status: 'unknown', message: 'Config outdated' };

  // Update docs endpoint health
  const epDocs = base.endpoints.find(e => e.endpoint_id === 'ep-policy')!;
  epDocs.health_status = 'unknown';
  epDocs.last_checked_at = null;

  // Update entry point for docs
  const docsEP = base.entryPoints.find(e => e.route_id === 'route-policy');
  if (docsEP) {
    docsEP.health = 'unknown';
    docsEP.endpoints = docsEP.endpoints.map(ep =>
      ep.endpoint_id === 'ep-policy'
        ? { ...ep, health: 'unknown' as const }
        : ep,
    );
  } else {
    base.entryPoints.push({
      route_id: 'route-policy',
      domain: 'policy.example.com', protocol: 'http',
      tls_mode: 'http_only',
      listener: { bind_addr: '0.0.0.0', port: 80, provider: 'caddy', purpose: 'http_entry', status: 'active', gateway_id: 'gw_public_a', node_id: 'node-a' },
      gateway_id: 'gw_public_a', gateway_name: '边缘网关',
      service_id: 'svc-policy', service_name: 'docs-service',
      endpoints: [
        { endpoint_id: 'ep-policy', node_id: 'node-c', node_name: 'Server C', protocol: 'http', target: '127.0.0.1:8080', health: 'unknown' },
      ],
      health: 'unknown', safety: 'unknown', release_state: 'drifted',
    });
  }

  // Add anomaly
  base.anomalies = [
    {
      id: 'anomaly-node-drift',
      severity: 'critical',
      title: '节点配置漂移',
      description: 'Server C (node-c) 期望配置版本 43，实际版本 41，状态 drifted',
      affectedObjects: [
        { type: 'node', id: 'node-c', name: 'Server C' },
        { type: 'route', id: 'route-policy', name: 'policy.example.com' },
        { type: 'service', id: 'svc-policy', name: 'docs-service' },
      ],
      workspace: 'runtime',
      timestamp: NOW,
    },
  ];

  // Update dashboard
  base.dashboard.outdated_nodes = 1;
  base.dashboard.routing_tables_synced = 2;
  base.dashboard.recent_errors = [
    {
      node_id: 'node-c',
      node_name: 'Server C',
      error: 'config drift: desired v43, actual v41',
      last_seen: NOW,
    },
  ];

  return base;
})();

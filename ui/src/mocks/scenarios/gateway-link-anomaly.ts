// ─── Scenario 5: Gateway Link Anomaly ───
// gw_public_a → gw_public_a failed, affected relay paths
// The gateway link between main and edge gateway is broken.

import { scenarioNormal } from './normal';
import type { ScenarioData } from './types';

const NOW = '2026-07-02T11:15:00Z';

function deepClone<T>(obj: T): T {
  return JSON.parse(JSON.stringify(obj));
}

export const scenarioGatewayLinkAnomaly: ScenarioData = (() => {
  const base = deepClone(scenarioNormal);

  // Break the gateway link
  const link = base.gatewayLinks.find(l => l.gateway_link_id === 'gl-a-b')!;
  link.status = 'failed';
  link.last_verified_at = null;

  // Mark gw_public_a as degraded
  const gwMain = base.gateways.find(g => g.gateway_id === 'gw_public_a')!;
  gwMain.status = 'error';
  gwMain.last_error = 'Gateway link to gw_private_b failed';

  // Update topology edge
  const edge = base.topologyEdges.find(e => e.from_node_id === 'node-a' && e.to_node_id === 'node-b')!;
  edge.status = 'degraded';
  edge.last_error = 'Gateway link gl-a-b verification failed';

  // Update node-b sync
  const nodeBSync = base.syncStatuses.find(s => s.node_id === 'node-b')!;
  nodeBSync.gateway_status = { status: 'error', message: 'Gateway link from node-a broken' };
  nodeBSync.status = 'outdated';

  // Update dashboard
  base.dashboard.gateways_online = 2;
  base.dashboard.missing_gateway_links = 1;

  // Add anomaly
  base.anomalies = [
    {
      id: 'anomaly-gwlink',
      severity: 'critical',
      title: '网关链路异常',
      description: 'gw_public_a → gw_public_a 链路验证失败，影响跨节点中继路径',
      affectedObjects: [
        { type: 'gateway', id: 'gw_public_a', name: 'A Public Gateway' },
        { type: 'gateway', id: 'gw_private_b', name: 'B Private Gateway' },
        { type: 'node', id: 'node-b', name: 'Server B' },
      ],
      workspace: 'fabric',
      timestamp: NOW,
    },
    {
      id: 'anomaly-relay',
      severity: 'warning',
      title: '中继路径受影响',
      description: '由于网关链路异常，从 node-a 到 node-b 的中继路径不可用',
      affectedObjects: [
        { type: 'route', id: 'route-api-b', name: 'api-b.example.com' },
        { type: 'endpoint', id: 'ep-relay', name: 'ep-relay' },
      ],
      workspace: 'fabric',
      timestamp: NOW,
    },
  ];

  base.dashboard.recent_errors = [
    {
      node_id: 'node-a',
      node_name: 'Server A',
      error: 'Gateway link gl-a-b verification failed: timeout',
      last_seen: NOW,
    },
  ];

  return base;
})();

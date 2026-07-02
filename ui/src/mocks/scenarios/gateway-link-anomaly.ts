// ─── Scenario 5: Gateway Link Anomaly ───
// gateway-main → gateway-edge failed, affected relay paths
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
  const link = base.gatewayLinks.find(l => l.gateway_link_id === 'link-main-private')!;
  link.status = 'failed';
  link.last_verified_at = null;

  // Mark gateway-main as degraded
  const gwMain = base.gateways.find(g => g.gateway_id === 'gateway-main')!;
  gwMain.status = 'error';
  gwMain.last_error = 'Gateway link to gateway-private failed';

  // Update topology edge
  const edge = base.topologyEdges.find(e => e.from_node_id === 'node-a' && e.to_node_id === 'node-b')!;
  edge.status = 'degraded';
  edge.last_error = 'Gateway link link-main-private verification failed';

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
      description: 'gateway-main → gateway-edge 链路验证失败，影响跨节点中继路径',
      affectedObjects: [
        { type: 'gateway', id: 'gateway-main', name: '主网关' },
        { type: 'gateway', id: 'gateway-private', name: '私网网关' },
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
        { type: 'route', id: 'route-api', name: 'api.proofnote.dev' },
        { type: 'endpoint', id: 'endpoint-b', name: 'endpoint-b' },
      ],
      workspace: 'fabric',
      timestamp: NOW,
    },
  ];

  base.dashboard.recent_errors = [
    {
      node_id: 'node-a',
      node_name: 'Server A',
      error: 'Gateway link link-main-private verification failed: timeout',
      last_seen: NOW,
    },
  ];

  return base;
})();

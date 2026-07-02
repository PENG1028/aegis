// ─── Scenario 3: Pending Release ───
// docs.proofnote.dev → service-docs → release pending
// A new route configuration has been created but not yet applied.

import { scenarioNormal } from './normal';
import type { ScenarioData } from './types';

const NOW = '2026-07-02T10:40:00Z';

function deepClone<T>(obj: T): T {
  return JSON.parse(JSON.stringify(obj));
}

export const scenarioPendingRelease: ScenarioData = (() => {
  const base = deepClone(scenarioNormal);

  // Mark docs route as pending (not yet applied)
  const docsRoute = base.routes.find(r => r.route_id === 'route-docs')!;
  docsRoute.status = 'active'; // Route exists but config not applied

  // Add a new pending entry point
  base.entryPoints.push({
    route_id: 'route-docs',
    domain: 'docs.proofnote.dev',
    protocol: 'http',
    tls_mode: 'http_only',
    listener: { bind_addr: '0.0.0.0', port: 80, provider: 'caddy', purpose: 'http_entry', status: 'active', gateway_id: 'gateway-edge', node_id: 'node-a' },
    gateway_id: 'gateway-edge', gateway_name: '边缘网关',
    service_id: 'service-docs', service_name: 'docs-service',
    endpoints: [
      { endpoint_id: 'endpoint-docs-a', node_id: 'node-c', node_name: 'Server C', protocol: 'http', target: '127.0.0.1:8080', health: 'healthy' },
    ],
    health: 'healthy', safety: 'safe', release_state: 'pending',
  });

  // Add anomaly for pending release
  base.anomalies = [
    {
      id: 'anomaly-pending-release',
      severity: 'warning',
      title: '配置待发布',
      description: 'docs.proofnote.dev 的配置变更已创建但尚未推送到节点',
      affectedObjects: [
        { type: 'route', id: 'route-docs', name: 'docs.proofnote.dev' },
        { type: 'service', id: 'service-docs', name: 'docs-service' },
      ],
      workspace: 'release',
      timestamp: NOW,
    },
  ];

  // Update dashboard
  base.dashboard.pending_capabilities = ['route-docs pending apply'];

  return base;
})();

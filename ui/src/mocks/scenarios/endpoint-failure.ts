// ─── Scenario 2: Endpoint Failure ───
// auth.proofnote.dev → gateway-main → service-auth → endpoint-auth-b unreachable
// Shows degraded chain health due to one unhealthy endpoint.

import { scenarioNormal } from './normal';
import type { ScenarioData } from './types';

const NOW = '2026-07-02T10:35:00Z';

// Deep clone the normal scenario and apply endpoint-failure modifications
function deepClone<T>(obj: T): T {
  return JSON.parse(JSON.stringify(obj));
}

export const scenarioEndpointFailure: ScenarioData = (() => {
  const base = deepClone(scenarioNormal);

  // Modify auth service health
  const authService = base.services.find(s => s.service_id === 'service-auth')!;
  authService.health_status = 'unhealthy';
  authService.latency_ms = 2500;

  // Mark endpoint-auth-b as unhealthy
  const epAuthB = base.endpoints.find(e => e.endpoint_id === 'endpoint-auth-b')!;
  epAuthB.health_status = 'unhealthy';
  epAuthB.latency_ms = null;
  epAuthB.last_checked_at = null;

  // Update entry point health for auth
  const authEP = base.entryPoints.find(e => e.route_id === 'route-auth')!;
  authEP.health = 'degraded';
  authEP.endpoints = authEP.endpoints.map(ep =>
    ep.endpoint_id === 'endpoint-auth-b'
      ? { ...ep, health: 'unhealthy' as const }
      : ep,
  );

  // Update sync status for node-b to reflect issue
  const nodeBSync = base.syncStatuses.find(s => s.node_id === 'node-b')!;
  nodeBSync.status = 'outdated';
  nodeBSync.last_error = 'endpoint-auth-b health check failed: connection refused';

  // Add anomalies
  base.anomalies = [
    {
      id: 'anomaly-ep-auth-b',
      severity: 'critical',
      title: '端点不可达',
      description: '端点 endpoint-auth-b (auth-service @ Server B) 健康检查失败',
      affectedObjects: [
        { type: 'endpoint', id: 'endpoint-auth-b', name: 'endpoint-auth-b' },
        { type: 'service', id: 'service-auth', name: 'auth-service' },
        { type: 'route', id: 'route-auth', name: 'auth.proofnote.dev' },
        { type: 'node', id: 'node-b', name: 'Server B' },
      ],
      workspace: 'exposure',
      timestamp: NOW,
    },
  ];

  // Update dashboard
  base.dashboard.routes_unavailable = 1;
  base.dashboard.recent_errors = [
    {
      node_id: 'node-b',
      node_name: 'Server B',
      error: 'endpoint-auth-b health check failed: connection refused',
      last_seen: NOW,
    },
  ];

  return base;
})();

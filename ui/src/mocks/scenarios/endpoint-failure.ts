// ─── Scenario 2: Endpoint Failure ───
// relay.example.com → gw_public_a → svc-relay → ep-relay unreachable
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
  const authService = base.services.find(s => s.service_id === 'svc-relay')!;
  authService.health_status = 'unhealthy';
  authService.latency_ms = 2500;

  // Mark ep-relay as unhealthy
  const epAuthB = base.endpoints.find(e => e.endpoint_id === 'ep-relay')!;
  epAuthB.health_status = 'unhealthy';
  epAuthB.latency_ms = null;
  epAuthB.last_checked_at = null;

  // Update entry point health for auth
  const authEP = base.entryPoints.find(e => e.route_id === 'route-relay')!;
  authEP.health = 'degraded';
  authEP.endpoints = authEP.endpoints.map(ep =>
    ep.endpoint_id === 'ep-relay'
      ? { ...ep, health: 'unhealthy' as const }
      : ep,
  );

  // Update sync status for node-b to reflect issue
  const nodeBSync = base.syncStatuses.find(s => s.node_id === 'node-b')!;
  nodeBSync.status = 'outdated';
  nodeBSync.last_error = 'ep-relay health check failed: connection refused';

  // Add anomalies
  base.anomalies = [
    {
      id: 'anomaly-ep-auth-b',
      severity: 'critical',
      title: '端点不可达',
      description: '端点 ep-relay (auth-service @ Server B) 健康检查失败',
      affectedObjects: [
        { type: 'endpoint', id: 'ep-relay', name: 'ep-relay' },
        { type: 'service', id: 'svc-relay', name: 'auth-service' },
        { type: 'route', id: 'route-relay', name: 'relay.example.com' },
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
      error: 'ep-relay health check failed: connection refused',
      last_seen: NOW,
    },
  ];

  return base;
})();

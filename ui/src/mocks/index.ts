// ─── Mock Scenario Infrastructure ───
// Scenario selector + factory for relational mock data.
//
// Scenarios are loaded at import time but wrapped in safety helpers.
// Only the active scenario's data is exposed via getScenario().

import { scenarioNormal } from './scenarios/normal';
import { scenarioEndpointFailure } from './scenarios/endpoint-failure';
import { scenarioPendingRelease } from './scenarios/pending-release';
import { scenarioNodeDrift } from './scenarios/node-drift';
import { scenarioGatewayLinkAnomaly } from './scenarios/gateway-link-anomaly';
import type { ScenarioId, ScenarioData } from './scenarios/types';

export type { ScenarioId, ScenarioData } from './scenarios/types';

const SCENARIOS: Record<ScenarioId, ScenarioData> = {
  normal: scenarioNormal,
  'endpoint-failure': scenarioEndpointFailure,
  'pending-release': scenarioPendingRelease,
  'node-drift': scenarioNodeDrift,
  'gateway-link-anomaly': scenarioGatewayLinkAnomaly,
};

let activeScenarioId: ScenarioId = 'normal';

export function setScenario(id: ScenarioId) {
  activeScenarioId = id;
  window.dispatchEvent(new CustomEvent('scenario-change', { detail: id }));
}

export function getScenario(): ScenarioData {
  return SCENARIOS[activeScenarioId];
}

export function getActiveScenarioId(): ScenarioId {
  return activeScenarioId;
}

export function getScenarioList() {
  return [
    { id: 'normal' as const, name: '正常链路', description: '全链路正常' },
    { id: 'endpoint-failure' as const, name: '端点故障', description: '端点不可达' },
    { id: 'pending-release' as const, name: '待发布', description: '配置待发布' },
    { id: 'node-drift' as const, name: '节点漂移', description: '配置漂移' },
    { id: 'gateway-link-anomaly' as const, name: '网关链路异常', description: '链路故障' },
  ];
}

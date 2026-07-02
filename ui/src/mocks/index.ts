// ─── Mock Scenario Infrastructure ───
// Scenario selector + factory for relational mock data.

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
  // Dispatch event so all query caches know to refetch
  window.dispatchEvent(new CustomEvent('scenario-change', { detail: id }));
}

export function getScenario(): ScenarioData {
  return SCENARIOS[activeScenarioId];
}

export function getActiveScenarioId(): ScenarioId {
  return activeScenarioId;
}

export function getScenarioList() {
  return Object.entries(SCENARIOS).map(([id, data]) => ({
    id: id as ScenarioId,
    name: data.meta.name,
    description: data.meta.description,
  }));
}

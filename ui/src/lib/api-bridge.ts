/**
 * API Bridge — all pages import from here.
 * v1.8L-10: mock mode removed. All calls go to the real Go backend.
 */
export {
  fetchDashboard,
  fetchNodes,
  fetchNodeDetail,
  fetchTopologyMatrix,
  fetchTopologyPath,
  fetchServices,
  fetchServiceDetail,
  fetchRoutes,
  fetchRouteDetail,
  fetchEndpoints,
  fetchEndpointDetail,
  fetchPolicies,
  fetchRoutingTable,
  previewRouting,
  validateRouting,
  fetchAcceptance,
  fetchSettings,
  updateSettings,
  auth,
  system,
  safetyApi,
  traceApi,
  relayApi,
  gatewayLinkApi,
  nodeApi,
  providerApi,
  infraApi,
  exposureApi,
  adminApi,
  dnsApi,
  transparentApi,
  clusterHealthApi,
  portCheckApi,
  systemHealthApi,
  healthCheckApi,
  credentialApi,
  runtimeModeApi,
  compositionApi,
  routeApi,
  distnodeApi,
  certApi,
  acmeApi,
  flowbridgeApi,
} from './real-api-client';
export type { InfraItem } from './real-api-client';
export type { CertificateItem } from './real-api-client';
export type { FlowBridgeInstance } from './real-api-client';
export type { RuntimeModeDef, RuntimeModeRole, RuntimeModeBinding, RuntimeAtom, AtomSlot, ProviderAtoms, Composition, CompDef } from './real-api-client';

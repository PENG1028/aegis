// ─── Chain Factory ───
// Builds full ObjectChain from any object ID using scenario data.
// This is the core engine for PathRibbon, RelationshipMap, and Trace.

import type { Route, Gateway, Service, Endpoint, Node } from '@/types';
import type { ObjectChain, ListenerRef, ProviderRef, ChainHealth } from '@/types/workspace';
import { getScenario } from '../index';

function findRoute(id: string): Route | null {
  return getScenario().routes.find(r => r.route_id === id) || null;
}
function findGateway(id: string): Gateway | null {
  return getScenario().gateways.find(g => g.gateway_id === id) || null;
}
function findService(id: string): Service | null {
  return getScenario().services.find(s => s.service_id === id) || null;
}
function findEndpoints(serviceId: string): Endpoint[] {
  return getScenario().endpoints.filter(e => e.service_id === serviceId);
}
function findNodes(ids: string[]): Node[] {
  return getScenario().nodes.filter(n => ids.includes(n.node_id));
}
function findListeners(gatewayId: string): ListenerRef | null {
  const listener = getScenario().listeners.find(l => l.gateway_id === gatewayId);
  return listener || null;
}

function computeChainHealth(endpoints: Endpoint[]): ChainHealth {
  if (endpoints.length === 0) return 'broken';
  const unhealthy = endpoints.filter(e => e.health_status === 'unhealthy');
  if (unhealthy.length === endpoints.length) return 'broken';
  if (unhealthy.length > 0) return 'degraded';
  return 'healthy';
}

/**
 * Build full ObjectChain starting from a route.
 * Route → Gateway (via gateway_id in route or policy) → Service → Endpoints → Nodes
 */
export function chainFromRoute(routeId: string): ObjectChain {
  const route = findRoute(routeId);
  if (!route) {
    return { entryPoint: null, listener: null, gateway: null, service: null, endpoints: [], nodes: [], provider: null, status: 'broken' };
  }

  const service = findService(route.service_id);
  const endpoints = service ? findEndpoints(service.service_id) : [];
  const allNodeIds = endpoints.map(e => e.node_id);
  const nodes = findNodes(allNodeIds);

  // Try to find gateway from route's gateway_policy or from service's gateway_policy
  let gateway: Gateway | null = null;
  let listener: ListenerRef | null = null;
  const policy = route.gateway_policy;
  if (policy?.primary_gateway_id) {
    gateway = findGateway(policy.primary_gateway_id);
    if (gateway) listener = findListeners(gateway.gateway_id);
  }
  if (!gateway) {
    // Fallback: find first active gateway on any node with this route's endpoints
    const allGateways = getScenario().gateways;
    gateway = allGateways.find(g => g.status === 'active' && allNodeIds.includes(g.node_id)) || allGateways[0] || null;
    if (gateway) listener = findListeners(gateway.gateway_id);
  }

  const provider: ProviderRef | null = gateway ? {
    provider_id: gateway.provider,
    name: gateway.provider === 'caddy' ? 'Caddy' : gateway.provider === 'haproxy' ? 'HAProxy' : 'Aegis',
    kind: gateway.provider,
    status: gateway.status,
    node_id: gateway.node_id,
  } : null;

  return {
    entryPoint: route,
    listener,
    gateway,
    service,
    endpoints,
    nodes,
    provider,
    status: computeChainHealth(endpoints),
  };
}

/**
 * Build ObjectChain from a service.
 */
export function chainFromService(serviceId: string): ObjectChain {
  const service = findService(serviceId);
  if (!service) {
    return { entryPoint: null, listener: null, gateway: null, service: null, endpoints: [], nodes: [], provider: null, status: 'broken' };
  }

  const endpoints = findEndpoints(serviceId);
  const allNodeIds = endpoints.map(e => e.node_id);
  const nodes = findNodes(allNodeIds);

  // Find routes that point to this service
  const routes = getScenario().routes.filter(r => r.service_id === serviceId);
  const entryPoint = routes.length > 0 ? routes[0] : null;

  let gateway: Gateway | null = null;
  let listener: ListenerRef | null = null;
  if (entryPoint?.gateway_policy?.primary_gateway_id) {
    gateway = findGateway(entryPoint.gateway_policy.primary_gateway_id);
  }
  if (!gateway) {
    const allGateways = getScenario().gateways;
    gateway = allGateways.find(g => g.status === 'active') || allGateways[0] || null;
  }
  if (gateway) listener = findListeners(gateway.gateway_id);

  const provider: ProviderRef | null = gateway ? {
    provider_id: gateway.provider,
    name: gateway.provider === 'caddy' ? 'Caddy' : 'HAProxy',
    kind: gateway.provider,
    status: gateway.status,
    node_id: gateway.node_id,
  } : null;

  return {
    entryPoint,
    listener,
    gateway,
    service,
    endpoints,
    nodes,
    provider,
    status: computeChainHealth(endpoints),
  };
}

/**
 * Build ObjectChain from a gateway.
 */
export function chainFromGateway(gatewayId: string): ObjectChain {
  const gateway = findGateway(gatewayId);
  if (!gateway) {
    return { entryPoint: null, listener: null, gateway: null, service: null, endpoints: [], nodes: [], provider: null, status: 'broken' };
  }

  const listener = findListeners(gatewayId);
  const node = getScenario().nodes.find(n => n.node_id === gateway.node_id) || null;

  // Find services/routes that use this gateway
  const policies = getScenario().policies.filter(p => p.primary_gateway_id === gatewayId);
  const policyServiceIds = policies.map(p => p.target_id);
  const routes = getScenario().routes.filter(r => policyServiceIds.includes(r.service_id));
  const entryPoint = routes.length > 0 ? routes[0] : null;
  const service = entryPoint ? findService(entryPoint.service_id) : null;
  const endpoints = service ? findEndpoints(service.service_id) : [];

  const provider: ProviderRef = {
    provider_id: gateway.provider,
    name: gateway.provider === 'caddy' ? 'Caddy' : 'HAProxy',
    kind: gateway.provider,
    status: gateway.status,
    node_id: gateway.node_id,
  };

  return {
    entryPoint,
    listener,
    gateway,
    service,
    endpoints,
    nodes: node ? [node] : [],
    provider,
    status: computeChainHealth(endpoints),
  };
}

/**
 * Build ObjectChain from a node.
 */
export function chainFromNode(nodeId: string): ObjectChain {
  const node = getScenario().nodes.find(n => n.node_id === nodeId);
  if (!node) {
    return { entryPoint: null, listener: null, gateway: null, service: null, endpoints: [], nodes: [], provider: null, status: 'broken' };
  }

  // Find gateways on this node
  const gateways = getScenario().gateways.filter(g => g.node_id === nodeId);
  const gateway = gateways.length > 0 ? gateways[0] : null;
  const listener = gateway ? findListeners(gateway.gateway_id) : null;

  // Find endpoints on this node, then services, then routes
  const endpoints = getScenario().endpoints.filter(e => e.node_id === nodeId);
  const serviceIds = [...new Set(endpoints.map(e => e.service_id))];
  const services = getScenario().services.filter(s => serviceIds.includes(s.service_id));
  const service = services.length > 0 ? services[0] : null;

  const routes = service
    ? getScenario().routes.filter(r => r.service_id === service.service_id)
    : [];
  const entryPoint = routes.length > 0 ? routes[0] : null;

  const provider: ProviderRef | null = gateway ? {
    provider_id: gateway.provider,
    name: gateway.provider === 'caddy' ? 'Caddy' : 'HAProxy',
    kind: gateway.provider,
    status: gateway.status,
    node_id: gateway.node_id,
  } : null;

  return {
    entryPoint,
    listener,
    gateway,
    service,
    endpoints,
    nodes: [node],
    provider,
    status: computeChainHealth(endpoints),
  };
}

/**
 * Build ObjectChain from an endpoint.
 */
export function chainFromEndpoint(endpointId: string): ObjectChain {
  const endpoint = getScenario().endpoints.find(e => e.endpoint_id === endpointId);
  if (!endpoint) {
    return { entryPoint: null, listener: null, gateway: null, service: null, endpoints: [], nodes: [], provider: null, status: 'broken' };
  }
  // Delegate to service chain, which already includes this endpoint
  return chainFromService(endpoint.service_id);
}

/**
 * Universal chain resolver.
 */
export function resolveChain(type: string, id: string): ObjectChain {
  switch (type) {
    case 'route':
    case 'entry':
      return chainFromRoute(id);
    case 'service':
      return chainFromService(id);
    case 'gateway':
      return chainFromGateway(id);
    case 'node':
      return chainFromNode(id);
    case 'endpoint':
      return chainFromEndpoint(id);
    default:
      return { entryPoint: null, listener: null, gateway: null, service: null, endpoints: [], nodes: [], provider: null, status: 'broken' };
  }
}

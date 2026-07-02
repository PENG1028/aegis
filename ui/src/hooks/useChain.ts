// ─── useChain Hook ───
// Fetches the full ObjectChain for any domain object.

import { useQuery } from '@tanstack/react-query';
import type { ObjectChain } from '@/types/workspace';
import { API_CONFIG } from '@/lib/api-config';
import { resolveChain } from '@/mocks/generators/chain-factory';
import { fetchRouteDetail, fetchServiceDetail, fetchGatewayDetail, fetchNodeDetail, fetchEndpoints } from '@/lib/api-bridge';

async function fetchChain(type: string, id: string): Promise<ObjectChain> {
  if (API_CONFIG.useMock) {
    // In mock mode, resolve from scenario data
    await new Promise(r => setTimeout(r, 200));
    return resolveChain(type, id);
  }
  // In real mode, fetch all required data and assemble
  // This is a simplified version — real implementation would fetch incrementally
  const chain: ObjectChain = {
    entryPoint: null,
    listener: null,
    gateway: null,
    service: null,
    endpoints: [],
    nodes: [],
    provider: null,
    status: 'healthy' as const,
  };

  try {
    if (type === 'route' || type === 'entry') {
      const route = await fetchRouteDetail(id);
      chain.entryPoint = route;
      if (route?.service_id) {
        const svc = await fetchServiceDetail(route.service_id);
        chain.service = svc;
        chain.endpoints = svc?.endpoints || [];
      }
    } else if (type === 'service') {
      const svc = await fetchServiceDetail(id);
      chain.service = svc;
      chain.endpoints = svc?.endpoints || [];
    } else if (type === 'gateway') {
      const gw = await fetchGatewayDetail(id);
      chain.gateway = gw;
      // Gateway links to routes via gateway_policy, not direct service_id.
      // Full chain resolution needs routing table lookup.
    } else if (type === 'node') {
      const node = await fetchNodeDetail(id);
      if (node) {
        chain.nodes = [node as any];
      }
      // Fetch endpoints on this node
      try {
        const eps = await fetchEndpoints();
        chain.endpoints = (Array.isArray(eps) ? eps : []).filter((e: any) => e.node_id === id);
      } catch { /* endpoints optional */ }
    }
    // Further real-mode chain assembly would go here
  } catch (err) {
    chain.status = 'broken';
    chain.error = (err as Error).message || String(err);
  }

  return chain;
}

export function useChain(type: string, id: string | undefined) {
  return useQuery({
    queryKey: ['chain', type, id],
    queryFn: () => fetchChain(type, id!),
    enabled: !!id && !!type,
    staleTime: 30_000,
  });
}

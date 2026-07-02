// ─── useImpact Hook ───
// Computes impact scope before write operations.

import { useQuery } from '@tanstack/react-query';
import type { ImpactScope } from '@/types/impact';
import { API_CONFIG } from '@/lib/api-config';

async function computeImpact(
  operation: string,
  targetType: string,
  targetId: string,
  targetName: string,
): Promise<ImpactScope> {
  if (API_CONFIG.useMock) {
    await new Promise(r => setTimeout(r, 300));
    // In mock mode, return a simulated impact based on operation type
    const mockAffected = {
      type: targetType,
      id: targetId,
      name: targetName,
      status: 'healthy',
      impact: 'direct' as const,
      description: `此操作将直接影响 ${targetName}`,
    };

    return {
      target: { type: targetType, id: targetId, name: targetName },
      operation,
      affectedEntries: targetType === 'route' || targetType === 'entry' ? [mockAffected] : [],
      affectedServices: targetType === 'service' ? [mockAffected] : [],
      affectedGateways: targetType === 'gateway' ? [mockAffected] : [],
      affectedNodes: targetType === 'node' ? [mockAffected] : [],
      totalAffected: 1,
      hasDownstreamFailures: false,
    };
  }

  // Real mode: would call backend impact analysis API
  return {
    target: { type: targetType, id: targetId, name: targetName },
    operation,
    affectedEntries: [],
    affectedServices: [],
    affectedGateways: [],
    affectedNodes: [],
    totalAffected: 0,
    hasDownstreamFailures: false,
  };
}

export function useImpact(
  operation: string,
  targetType: string,
  targetId: string | undefined,
  targetName: string,
) {
  return useQuery({
    queryKey: ['impact', operation, targetType, targetId],
    queryFn: () => computeImpact(operation, targetType, targetId!, targetName),
    enabled: false, // Only fetch on demand (when user clicks)
    staleTime: 60_000,
  });
}

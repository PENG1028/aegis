// ─── useImpact Hook ───
// Computes impact scope before write operations.

import { useQuery } from '@tanstack/react-query';
import type { ImpactScope } from '@/types/impact';

async function computeImpact(
  operation: string,
  targetType: string,
  targetId: string,
  targetName: string,
): Promise<ImpactScope> {

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

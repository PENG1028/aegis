// ─── Impact Calculator ───
// Computes impact scope for write operations.

import type { ImpactScope, AffectedObject } from '@/types/impact';

export function createEmptyImpact(
  targetType: string,
  targetId: string,
  targetName: string,
  operation: string,
): ImpactScope {
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

export function addAffected(
  scope: ImpactScope,
  category: 'affectedEntries' | 'affectedServices' | 'affectedGateways' | 'affectedNodes',
  obj: AffectedObject,
): ImpactScope {
  const updated = [...scope[category], obj];
  return {
    ...scope,
    [category]: updated,
    totalAffected: scope.totalAffected + 1,
    hasDownstreamFailures: scope.hasDownstreamFailures || obj.status === 'failed' || obj.status === 'unhealthy',
  };
}

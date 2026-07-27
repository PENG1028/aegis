// ─── Impact Analysis Types ───

export interface ImpactScope {
  target: { type: string; id: string; name: string };
  operation: string;
  affectedEntries: AffectedObject[];
  affectedServices: AffectedObject[];
  affectedGateways: AffectedObject[];
  affectedNodes: AffectedObject[];
  totalAffected: number;
  hasDownstreamFailures: boolean;
}

export interface AffectedObject {
  type: string;
  id: string;
  name: string;
  status: string;
  impact: 'direct' | 'indirect';
  description: string;
}

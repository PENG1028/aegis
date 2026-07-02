// ─── Release / Configuration Diff Types ───

export interface ReleaseDiff {
  versionFrom: string;
  versionTo: string;
  summary: string;
  changes: DiffChange[];
  rawDiff: string;
}

export interface DiffChange {
  type: 'added' | 'removed' | 'modified';
  domain?: string;
  service?: string;
  endpoint?: string;
  gateway?: string;
  description: string;
  before?: string;
  after?: string;
}

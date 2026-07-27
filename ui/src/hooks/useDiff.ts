// ─── useDiff Hook ───
// Fetches and caches release configuration diffs.

import { useQuery } from '@tanstack/react-query';
import type { ReleaseDiff } from '@/types/diff';
import { adminApi } from '@/lib/api-bridge';

async function fetchDiff(fromVersion?: string, toVersion?: string): Promise<ReleaseDiff> {
  // Real mode
  const result = await adminApi.configDiff();
  return result as unknown as ReleaseDiff;
}

export function useDiff(fromVersion?: string, toVersion?: string) {
  return useQuery({
    queryKey: ['diff', fromVersion, toVersion],
    queryFn: () => fetchDiff(fromVersion, toVersion),
    staleTime: 60_000,
  });
}

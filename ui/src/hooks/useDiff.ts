// ─── useDiff Hook ───
// Fetches and caches release configuration diffs.

import { useQuery } from '@tanstack/react-query';
import type { ReleaseDiff } from '@/types/diff';
import { API_CONFIG } from '@/lib/api-config';
import { adminApi } from '@/lib/api-bridge';

async function fetchDiff(fromVersion?: string, toVersion?: string): Promise<ReleaseDiff> {
  if (API_CONFIG.useMock) {
    await new Promise(r => setTimeout(r, 400));
    return {
      versionFrom: fromVersion || 'v41',
      versionTo: toVersion || 'v43',
      summary: '新增路由 docs.proofnote.dev，修改 api.proofnote.dev 端点配置',
      changes: [
        {
          type: 'added',
          domain: 'docs.proofnote.dev',
          service: 'service-docs',
          description: '新增文档服务路由',
          before: undefined,
          after: 'docs.proofnote.dev → gateway-main → service-docs → endpoint-docs-a (node-c:8080)',
        },
        {
          type: 'modified',
          domain: 'api.proofnote.dev',
          endpoint: 'endpoint-a',
          description: '端点 endpoint-a 健康检查间隔从 30s 改为 10s',
          before: 'check_interval: 30s',
          after: 'check_interval: 10s',
        },
      ],
      rawDiff: `  api.proofnote.dev {\n    reverse_proxy node-a:3000 node-b:3001\n+   health_check_interval 10s\n  }\n+ docs.proofnote.dev {\n+   reverse_proxy node-c:8080\n+ }`,
    };
  }
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

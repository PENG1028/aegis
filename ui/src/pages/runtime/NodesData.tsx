// ─── NodesData — one API's data across all cluster nodes, batched ───
//
// Reuses the backend AdminDistNodeAggregate primitive (GET
// /api/admin/v1/distnode/aggregate?path=...), which fans out to every alive
// peer via Aegis.ProxyRequest and returns each node's result. This is the
// "show 5 nodes' data on one panel" view — one card per node.
import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { distnodeApi } from '@/lib/api-bridge';
import { PageHeader, Card, EmptyState, LoadingState, StatusBadge, Btn } from '@/components/shared';
import { cn } from '@/lib/utils';

// Curated set of common aggregatable read APIs. Any GET path works — these are
// shortcuts so the user doesn't have to type. Add more as needed.
const PRESETS: { label: string; path: string }[] = [
  { label: '路由', path: '/api/admin/v1/routes' },
  { label: '服务', path: '/api/admin/v1/services' },
  { label: '系统状态', path: '/api/system/status' },
  { label: '运行时模式', path: '/api/system/runtime-mode' },
  { label: 'Provider', path: '/api/admin/v1/providers' },
  { label: '证书', path: '/api/admin/v1/certificates' },
];

export default function NodesData() {
  const [path, setPath] = useState(PRESETS[0].path);
  const [pending, setPending] = useState(path);

  const { data, isLoading, error, refetch, isFetching } = useQuery({
    queryKey: ['aggregate', path],
    queryFn: () => distnodeApi.aggregate(path),
    refetchInterval: 30_000,
  });

  const results = data?.results || [];

  return (
    <div className="p-6 space-y-6">
      <PageHeader
        title="多节点数据"
        subtitle={`同时查看所有节点的同一个 API · ${results.length} 个节点响应`}
        actions={<Btn onClick={() => refetch()} className="text-xs" disabled={isFetching}>{isFetching ? '刷新中…' : '刷新'}</Btn>}
      />

      {/* ── Path selector ── */}
      <Card>
        <div className="flex flex-wrap items-center gap-2 mb-3">
          {PRESETS.map(p => (
            <button
              key={p.path}
              onClick={() => { setPath(p.path); setPending(p.path); }}
              className={cn(
                'px-2.5 py-1 rounded text-[11px] border transition-colors',
                path === p.path
                  ? 'bg-a-accent/15 text-a-accent border-a-accent/30'
                  : 'bg-a-bg text-a-muted border-a-border hover:text-a-fg',
              )}>
              {p.label}
            </button>
          ))}
        </div>
        <form
          onSubmit={(e) => { e.preventDefault(); setPath(pending.trim()); }}
          className="flex items-center gap-2">
          <input
            value={pending}
            onChange={e => setPending(e.target.value)}
            placeholder="/api/admin/v1/..."
            className="flex-1 bg-a-bg border border-a-border rounded px-3 py-1.5 text-xs font-mono text-a-fg focus:outline-none focus:border-a-accent/50"
          />
          <Btn primary type="submit" className="text-xs">查询</Btn>
        </form>
        <div className="mt-2 text-[10px] text-a-muted">
          任意 GET 路径都可以。每个节点在本地执行该 API，结果分节点展示。
        </div>
      </Card>

      {/* ── Per-node results ── */}
      {isLoading ? (
        <LoadingState />
      ) : error ? (
        <EmptyState title="聚合失败" description={(error as Error).message} />
      ) : results.length === 0 ? (
        <EmptyState title="无节点响应" description="distnode 未启用，或没有存活节点" />
      ) : (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          {results.map((r) => (
            <NodeResultCard key={r.node_id} nodeId={r.node_id} status={r.status} body={r.body} errorMsg={r.error} />
          ))}
        </div>
      )}
    </div>
  );
}

// ─── One node's result card ───
function NodeResultCard({ nodeId, status, body, errorMsg }: {
  nodeId: string;
  status: number;
  body?: any;
  errorMsg?: string;
}) {
  const ok = status >= 200 && status < 300;
  const count = Array.isArray(body) ? body.length
    : (body && Array.isArray(body.data)) ? body.data.length
    : (body && Array.isArray(body.results)) ? body.results.length
    : null;

  return (
    <Card>
      <div className="flex items-center gap-2 mb-3">
        <span className="font-medium text-sm text-a-fg font-mono">{nodeId}</span>
        <StatusBadge status={ok ? 'active' : 'error'} />
        <span className={cn('text-[11px] font-mono', ok ? 'text-[#4cd964]' : 'text-[#ff5c72]')}>
          HTTP {status}
        </span>
        {count !== null && <span className="ml-auto text-[10px] text-a-muted">{count} 条</span>}
      </div>
      {errorMsg ? (
        <div className="text-[11px] text-[#ff5c72] font-mono bg-[#ff5c72]/5 rounded px-2 py-1.5">{errorMsg}</div>
      ) : (
        <pre className="text-[10px] text-a-fg2 font-mono bg-a-bg rounded p-2 overflow-auto max-h-72 leading-relaxed">
          {JSON.stringify(body, null, 2)}
        </pre>
      )}
    </Card>
  );
}

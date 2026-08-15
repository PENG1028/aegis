// ─── Health ───
// Real API: nodes + endpoints health. Data loads automatically (the old
// "查看" button only gated rendering of already-loaded data).
import { useQuery } from '@tanstack/react-query';
import { Card, PageHeader, StatusBadge, HealthDot } from '@/components/shared';
import { fetchNodes, fetchEndpoints } from '@/lib/api-bridge';
import { cn } from '@/lib/utils';

export default function Health() {
  const { data: nodesData } = useQuery({
    queryKey: ['health-nodes'],
    queryFn: fetchNodes,
    refetchInterval: 30_000,
  });
  const nodes = Array.isArray(nodesData) ? nodesData : [];

  const { data: endpointsData } = useQuery({
    queryKey: ['health-endpoints'],
    queryFn: fetchEndpoints,
    refetchInterval: 30_000,
  });
  const endpoints = Array.isArray(endpointsData) ? endpointsData : [];

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="健康检查" subtitle={`${nodes.length} 节点 · ${endpoints.length} 端点`} />

      <Card title={`节点 (${nodes.length})`}>
        {nodes.length === 0 ? (
          <div className="text-center py-6 text-xs text-a-muted">无节点数据</div>
        ) : (
          <div className="space-y-2">
            {nodes.map(n => (
              <div key={n.node_id} className="flex items-center gap-3 px-3 py-2 rounded-a-sm bg-a-bg border border-a-border/40 text-xs">
                <HealthDot status={n.status === 'online' ? 'active' : 'failed'} />
                <span className="font-medium text-a-fg">{n.name || n.hostname || n.node_id}</span>
                <span className="text-a-muted">{n.public_ip || n.private_ip || '—'}</span>
                <span className="flex-1" />
                <StatusBadge status={n.status || 'unknown'} />
              </div>
            ))}
          </div>
        )}
      </Card>

      <Card title={`端点 (${endpoints.length})`}>
        {endpoints.length === 0 ? (
          <div className="text-center py-6 text-xs text-a-muted">无端点数据</div>
        ) : (
          <div className="space-y-2">
            {endpoints.map((ep: any) => (
              <div key={ep.id} className={cn('flex items-center gap-3 px-3 py-2 rounded-a-sm bg-a-bg border border-a-border/40 text-xs',
                ep.status === 'unhealthy' && 'border-l-2 border-l-[#ff5c72]')}>
                <HealthDot status={ep.status === 'healthy' ? 'active' : 'failed'} />
                <span className="font-mono text-a-fg">{ep.address || ep.host || '—'}</span>
                <span className="text-a-muted">{ep.type || '—'}</span>
                <span className="flex-1" />
                <StatusBadge status={ep.status || 'unknown'} />
              </div>
            ))}
          </div>
        )}
      </Card>
    </div>
  );
}

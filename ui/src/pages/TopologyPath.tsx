import { useQuery } from '@tanstack/react-query';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { fetchTopologyPath } from '@/lib/api-bridge';
import { PageHeader, Card, StatusBadge, Alert } from '@/components/shared';

export default function TopologyPathPage() {
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const from = params.get('from') || 'node-a';
  const to = params.get('to') || 'node-b';

  const { data, isLoading, error } = useQuery({
    queryKey: ['topology-path', from, to],
    queryFn: () => fetchTopologyPath(from, to),
    enabled: !!from && !!to,
  });

  return (
    <div>
      <button onClick={() => navigate('/topology')} className="inline-flex items-center gap-1 text-xs text-a-muted hover:text-a-fg mb-3 bg-transparent border-none cursor-pointer p-0">← 拓扑</button>
      <PageHeader title={`路径查询: ${from} → ${to}`} helpKey="topology" />

      {isLoading && <div className="text-center py-10 text-a-muted font-mono text-sm">查询中...</div>}
      {error && <div className="px-4 py-3 rounded-a-md text-xs border bg-[#ff5c72]/10 text-[#ff5c72] border-[#ff5c72]/20">查询失败: {error.message}</div>}

      {data && (
        <div className="space-y-4">
          <Card title="查询结果">
            <div className="flex items-center gap-2 mb-3">
              <StatusBadge status={data.reachable ? 'verified' : 'unreachable'} />
              <span className="text-xs text-a-fg2">{data.summary}</span>
            </div>
            {data.total_hops > 0 && (
              <div className="text-xs text-a-muted">Total hops: {data.total_hops}</div>
            )}
          </Card>

          {data.path.length > 0 && (
            <Card title="路径详情">
              <div className="space-y-0">
                {data.path.map((hop) => (
                  <div key={hop.hop} className="flex items-start gap-3 py-3 border-b border-a-border-soft last:border-b-0">
                    <div className={`w-7 h-7 rounded-full flex items-center justify-center text-xs font-bold shrink-0 ${
                      hop.status === 'ok' ? 'bg-[#4cd964]/20 text-[#4cd964]' : 'bg-[#ff5c72]/20 text-[#ff5c72]'
                    }`}>
                      {hop.hop}
                    </div>
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2">
                        <span className="font-mono text-xs font-semibold">{hop.node_name}</span>
                        <StatusBadge status={hop.via === 'local' ? 'local_gateway' : hop.via === 'private_gateway' ? 'private_gateway' : 'public_gateway'} />
                      </div>
                      <div className="text-[11px] text-a-muted mt-1">
                        {hop.gateway_url && <span className="mr-3 font-mono">{hop.gateway_url}</span>}
                        {hop.gateway_link_id && <span className="text-a-accent font-mono">{hop.gateway_link_id}</span>}
                      </div>
                      {hop.reason && <div className="text-[11px] text-a-fg2 mt-1">{hop.reason}</div>}
                    </div>
                    {hop.status === 'ok'
                      ? <span className="text-[#4cd964] text-sm">✓</span>
                      : <span className="text-a-danger text-sm">✗</span>}
                  </div>
                ))}
              </div>
            </Card>
          )}

          {!data.reachable && (
            <Alert type="warn">
              <span className="font-medium">路径不可达。原因：</span>
              <span>{data.summary || '未知'}</span>
            </Alert>
          )}
        </div>
      )}
    </div>
  );
}

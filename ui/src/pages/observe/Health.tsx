// ─── Health ───
// Real API: nodes + endpoints health. Click to run check.
import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Card, PageHeader, Btn, StatusBadge, HealthDot } from '@/components/shared';
import { fetchNodes, fetchEndpoints } from '@/lib/api-bridge';
import { cn } from '@/lib/utils';

export default function Health() {
  const [ran, setRan] = useState(false);

  const { data: nodesData } = useQuery({
    queryKey: ['health-nodes'],
    queryFn: fetchNodes,
    refetchInterval: ran ? 30_000 : false,
  });
  const nodes = Array.isArray(nodesData) ? nodesData : [];

  const { data: endpointsData } = useQuery({
    queryKey: ['health-endpoints'],
    queryFn: fetchEndpoints,
    refetchInterval: ran ? 30_000 : false,
  });
  const endpoints = Array.isArray(endpointsData) ? endpointsData : [];

  const runAll = () => setRan(true);

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="健康检查" subtitle={`${nodes.length} 节点 · ${endpoints.length} 端点`}
        actions={<Btn primary onClick={runAll} disabled={ran}>{ran ? '已加载' : '查看'}</Btn>} />

      {ran && (
        <>
          <Card title={`节点 (${nodes.length})`}>
            {nodes.length === 0 ? (
              <div className="text-center py-6 text-xs text-a-muted">无节点数据</div>
            ) : (
              <div className="space-y-2">
                {nodes.map(n => (
                  <div key={n.node_id} className={cn('flex items-center gap-3 p-3 rounded-a-sm border text-xs',
                    n.status === 'online' ? 'bg-[#4cd964]/3 border-[#4cd964]/10' :
                    n.status === 'degraded' ? 'bg-[#e8b830]/3 border-[#e8b830]/10' :
                    'bg-[#ff5c72]/3 border-[#ff5c72]/10')}>
                    <HealthDot status={n.status === 'online' ? 'healthy' : n.status === 'degraded' ? 'degraded' : 'failed'} />
                    <span className="font-medium text-a-fg w-32">{n.name}</span>
                    <span className="font-mono text-a-muted">{n.public_ip}</span>
                    <span className="flex-1" />
                    <span className="text-a-muted">心跳: {n.last_heartbeat_at ? new Date(n.last_heartbeat_at).toLocaleTimeString('zh-CN') : '—'}</span>
                    <StatusBadge status={n.status} />
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
                {endpoints.map(ep => (
                  <div key={ep.endpoint_id} className={cn('flex items-center gap-3 p-2.5 rounded-a-sm border text-xs',
                    ep.health_status === 'healthy' ? 'bg-[#4cd964]/3 border-[#4cd964]/10' :
                    ep.health_status === 'unhealthy' ? 'bg-[#ff5c72]/3 border-[#ff5c72]/10' :
                    'bg-a-border/10 border-a-border')}>
                    <HealthDot status={ep.health_status === 'healthy' ? 'healthy' : ep.health_status === 'unhealthy' ? 'failed' : 'unknown'} />
                    <span className="font-mono text-a-fg">{ep.target_local_host}:{ep.target_local_port}</span>
                    <span className="text-a-muted">{ep.node_name || ep.node_id}</span>
                    <span className="flex-1" />
                    <StatusBadge status={ep.health_status} />
                  </div>
                ))}
              </div>
            )}
          </Card>
        </>
      )}
    </div>
  );
}

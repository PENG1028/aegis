// ─── Nodes — list with leader/role/heartbeat ───
import { useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { useApiList } from '@/hooks/use-api';
import { fetchNodes, clusterHealthApi } from '@/lib/api-bridge';
import { Card, PageHeader, QueryGuard, StatusBadge, Btn } from '@/components/shared';
import { cn } from '@/lib/utils';

export default function Nodes() {
  const nav = useNavigate();
  const { items: nodes, isLoading, error, refetch } = useApiList<any>(['nodes'], () => fetchNodes());

  const { data: cluster } = useQuery({
    queryKey: ['cluster-health'],
    queryFn: () => clusterHealthApi.get(),
    refetchInterval: 30_000,
  });

  const leaderId = (cluster as any)?.leader_node_id || '';
  const clusterNodes: any[] = (cluster as any)?.nodes || [];

  const merged = useMemo(() => {
    return (nodes || []).map((n: any) => {
      const cn = clusterNodes.find((c: any) => c.node_id === n.node_id);
      return {
        ...n,
        is_leader: n.node_id === leaderId,
        role: cn?.role || n.roles?.[0] || 'worker',
        heartbeat_age: cn?.heartbeat_age || (n.last_heartbeat_at ? agoStr(n.last_heartbeat_at) : '—'),
      };
    });
  }, [nodes, clusterNodes, leaderId]);

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="节点列表" subtitle={`${nodes.length} 个节点`}
        actions={<Btn primary onClick={() => nav('/runtime/deploy')} className="text-xs">部署节点</Btn>} />
      <QueryGuard items={merged} isLoading={isLoading} error={error} refetch={refetch} emptyMessage="暂无节点">
        {(items) => (
          <Card>
            <table className="w-full text-xs">
              <thead>
                <tr className="border-b border-a-border text-a-muted text-left">
                  <th className="py-2 px-3">节点</th>
                  <th className="py-2 px-3">IP</th>
                  <th className="py-2 px-3">角色</th>
                  <th className="py-2 px-3 text-center">Leader</th>
                  <th className="py-2 px-3">心跳</th>
                  <th className="py-2 px-3">状态</th>
                  <th className="py-2 px-3">版本</th>
                </tr>
              </thead>
              <tbody>
                {items.map((n: any) => (
                  <tr key={n.node_id}
                    className="border-b border-a-border/50 hover:bg-a-border/10 cursor-pointer"
                    onClick={() => nav(`/runtime/node/${n.node_id}`)}>
                    <td className="py-2.5 px-3">
                      <div className="font-medium text-a-fg">{n.name || n.hostname || n.node_id}</div>
                      {n.hostname && n.hostname !== n.name && (
                        <div className="text-[10px] text-a-muted font-mono">{n.hostname}</div>
                      )}
                    </td>
                    <td className="py-2.5 px-3 font-mono text-a-fg2">
                      <div>{n.public_ip || '—'}</div>
                      {n.private_ip && <div className="text-[10px] text-a-muted">{n.private_ip}</div>}
                    </td>
                    <td className="py-2.5 px-3 text-a-muted text-[11px]">{n.role}</td>
                    <td className="py-2.5 px-3 text-center">
                      {n.is_leader ? <span className="text-[#e8b830]" title="Leader">⭐</span> : <span className="text-a-muted/30">—</span>}
                    </td>
                    <td className="py-2.5 px-3">
                      <span className={cn('font-mono text-[11px]',
                        n.heartbeat_age === '—' ? 'text-a-muted' :
                        n.heartbeat_age?.includes('h') || n.heartbeat_age?.includes('d') ? 'text-[#e8b830]' :
                        'text-[#4cd964]'
                      )}>{n.heartbeat_age}</span>
                    </td>
                    <td className="py-2.5 px-3"><StatusBadge status={n.status} /></td>
                    <td className="py-2.5 px-3">
                      {n.desired_revision ? (
                        n.desired_revision === n.applied_revision ? (
                          <span className="font-mono text-[#4cd964]">r{n.applied_revision}</span>
                        ) : (
                          <span className="font-mono">
                            <span className="text-a-fg">r{n.desired_revision}</span>
                            <span className="text-a-muted mx-1">→</span>
                            <span className="text-[#e8b830]">r{n.applied_revision}</span>
                          </span>
                        )
                      ) : (
                        <span className="text-a-muted">—</span>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </Card>
        )}
      </QueryGuard>
    </div>
  );
}

function agoStr(iso: string): string {
  const s = Math.floor((Date.now() - new Date(iso).getTime()) / 1000);
  if (s < 60) return `${s}s`;
  if (s < 3600) return `${Math.floor(s / 60)}min`;
  if (s < 86400) return `${Math.floor(s / 3600)}h`;
  return `${Math.floor(s / 86400)}d`;
}

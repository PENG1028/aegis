// ─── Cluster Health — multi-node monitoring ───
// Shows leader, split-brain detection, per-node heartbeat status.
// Backend: GET /api/admin/v1/cluster/health

import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { clusterHealthApi } from '@/lib/api-bridge';
import { Card, PageHeader, LoadingState, ErrorBanner } from '@/components/shared';
import { cn } from '@/lib/utils';

export default function ClusterHealth() {
  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['cluster-health'],
    queryFn: () => clusterHealthApi.get(),
    refetchInterval: 15_000,
  });

  if (isLoading) return <LoadingState />;
  if (error) return <ErrorBanner message="加载集群健康数据失败" onRetry={refetch} />;

  const d = data as any;
  const nodes: any[] = d?.nodes || [];
  const splitBrain = d?.split_brain || false;
  const healthy = d?.overall_healthy || false;
  const leaderId = d?.leader_node_id || '';
  const issues: string[] = d?.issues || [];

  return (
    <div className="p-6 space-y-5">
      <PageHeader title="集群健康" subtitle="多节点运行状态 · Leader · 脑裂检测" />

      {/* ── Overall status ── */}
      <div className={cn(
        'px-4 py-3 rounded-a-md border text-sm font-medium',
        splitBrain ? 'bg-[#ff5c72]/10 border-[#ff5c72]/30 text-[#ff5c72]' :
        healthy ? 'bg-[#4cd964]/10 border-[#4cd964]/30 text-[#4cd964]' :
        'bg-[#e8b830]/10 border-[#e8b830]/30 text-[#e8b830]'
      )}>
        {splitBrain ? '🔴 检测到脑裂 (Split-Brain) — 多个节点声称是 Leader' :
         healthy ? '🟢 集群正常' : '🟡 集群降级 — 部分节点异常'}
        {issues.length > 0 && (
          <span className="block text-xs mt-1 opacity-80">{issues.join(' · ')}</span>
        )}
      </div>

      {/* ── Summary cards ── */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        <StatCard label="节点数" value={d?.node_count || nodes.length} color="text-a-fg" />
        <StatCard label="健康" value={nodes.filter((n: any) => n.status === 'healthy').length} color="text-[#4cd964]" />
        <StatCard label="降级" value={nodes.filter((n: any) => n.status === 'degraded').length} color="text-[#e8b830]" />
        <StatCard label="离线" value={nodes.filter((n: any) => n.status === 'offline' || n.status === 'unknown').length} color="text-[#ff5c72]" />
      </div>

      {/* ── Leader info ── */}
      {leaderId && (
        <Card title="Leader">
          <div className="flex items-center gap-2 text-xs">
            <span className="text-[#e8b830] text-sm">⭐</span>
            <Link to={`/runtime/node/${leaderId}`} className="font-mono text-a-accent hover:underline">
              {leaderId}
            </Link>
            <span className="text-a-muted">— 负责协调集群配置同步</span>
          </div>
        </Card>
      )}

      {/* ── Node table ── */}
      <Card title={`节点列表 (${nodes.length})`}>
        {nodes.length === 0 ? (
          <div className="py-8 text-center text-a-muted text-xs">
            <p>暂无节点数据</p>
            <p className="mt-1">部署节点后，节点会自动注册并上报心跳</p>
            <Link to="/runtime/deploy" className="text-a-accent hover:underline mt-2 inline-block">部署节点 →</Link>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-xs">
              <thead>
                <tr className="border-b border-a-border/30 text-a-muted text-left">
                  <th className="py-1.5 pr-2 font-medium">节点</th>
                  <th className="py-1.5 px-2 font-medium">角色</th>
                  <th className="py-1.5 px-2 font-medium text-center">Leader</th>
                  <th className="py-1.5 px-2 font-medium">同步状态</th>
                  <th className="py-1.5 px-2 font-medium text-center">期望版本</th>
                  <th className="py-1.5 px-2 font-medium text-center">实际版本</th>
                  <th className="py-1.5 px-2 font-medium">心跳</th>
                  <th className="py-1.5 pl-2 font-medium text-center">状态</th>
                </tr>
              </thead>
              <tbody>
                {nodes.map((n: any, i: number) => (
                  <tr key={n.node_id || i} className="border-b border-a-border/20 hover:bg-a-border/10 transition-colors">
                    <td className="py-1.5 pr-2">
                      <Link to={`/runtime/node/${n.node_id}`} className="font-mono text-a-accent hover:underline text-[11px]">
                        {n.hostname || n.node_id}
                      </Link>
                    </td>
                    <td className="py-1.5 px-2 text-a-muted">{n.role || 'worker'}</td>
                    <td className="py-1.5 px-2 text-center">
                      {n.is_leader ? <span className="text-[#e8b830]" title="Leader">⭐</span> : <span className="text-a-muted/30">—</span>}
                    </td>
                    <td className="py-1.5 px-2 text-a-muted">{n.sync_status || '—'}</td>
                    <td className="py-1.5 px-2 text-center font-mono text-[10px] text-a-muted">{n.desired_revision ?? '—'}</td>
                    <td className="py-1.5 px-2 text-center font-mono text-[10px] text-a-muted">{n.applied_revision ?? '—'}</td>
                    <td className="py-1.5 px-2 text-a-muted text-[11px]">{n.heartbeat_age || '—'}</td>
                    <td className="py-1.5 pl-2 text-center">
                      <span className={cn(
                        'px-1.5 py-0.5 rounded text-[9px] font-medium',
                        n.status === 'healthy' ? 'bg-[#4cd964]/10 text-[#4cd964]' :
                        n.status === 'degraded' ? 'bg-[#e8b830]/10 text-[#e8b830]' :
                        'bg-[#ff5c72]/10 text-[#ff5c72]'
                      )}>{n.status === 'healthy' ? '健康' : n.status === 'degraded' ? '降级' : n.status || '未知'}</span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Card>
    </div>
  );
}

function StatCard({ label, value, color }: { label: string; value: number; color: string }) {
  return (
    <div className="bg-a-surface border border-a-border/30 rounded-a-sm px-3 py-2 text-center">
      <div className={cn('text-lg font-bold', color)}>{value}</div>
      <div className="text-[10px] text-a-muted">{label}</div>
    </div>
  );
}

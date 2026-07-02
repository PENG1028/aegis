// ─── Sync Status ───
// desired vs actual state per node. Uses fetchSyncStatus from real API.
import { useQuery } from '@tanstack/react-query';
import { Card, PageHeader, StatusBadge } from '@/components/shared';
import { fetchSyncStatus } from '@/lib/api-bridge';

export default function SyncStatus() {
  const { data, isLoading } = useQuery({
    queryKey: ['sync-status'],
    queryFn: () => fetchSyncStatus(),
    refetchInterval: 30_000,
  });
  const statuses = Array.isArray(data) ? data : [];

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="同步状态" subtitle={`${statuses.length} 个节点 · 期望状态 vs 实际状态`} />
      {isLoading ? (
        <div className="text-center py-12 text-a-muted text-sm">加载中...</div>
      ) : statuses.length === 0 ? (
        <div className="text-center py-12 text-a-muted text-sm">暂无节点数据</div>
      ) : (
        <Card>
          <table className="w-full text-xs">
            <thead>
              <tr className="border-b border-a-border text-a-muted text-left">
                <th className="py-2 px-3">节点</th>
                <th className="py-2 px-3">期望版本</th>
                <th className="py-2 px-3">实际版本</th>
                <th className="py-2 px-3">状态</th>
                <th className="py-2 px-3">最后应用</th>
                <th className="py-2 px-3">错误</th>
              </tr>
            </thead>
            <tbody>
              {statuses.map((s: any) => (
                <tr key={s.node_id} className="border-b border-a-border/50">
                  <td className="py-2.5 px-3">
                    <div className="font-medium text-a-fg">{s.node_name}</div>
                    {s.node_id && <div className="text-[10px] text-a-muted font-mono">{s.node_id}</div>}
                  </td>
                  <td className="py-2.5 px-3 font-mono text-a-fg">v{s.desired_revision}</td>
                  <td className="py-2.5 px-3 font-mono">{s.applied_revision !== s.desired_revision ? <span className="text-[#ff5c72]">v{s.applied_revision}</span> : <span className="text-a-muted">v{s.applied_revision}</span>}</td>
                  <td className="py-2.5 px-3"><StatusBadge status={s.status} /></td>
                  <td className="py-2.5 px-3 text-a-muted text-[11px]">{s.last_success_at ? new Date(s.last_success_at).toLocaleString('zh-CN') : '—'}</td>
                  <td className="py-2.5 px-3 text-[#ff5c72] text-[11px] truncate max-w-[240px]">{s.last_error || '—'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </Card>
      )}
    </div>
  );
}

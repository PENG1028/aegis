// ─── Sync Status ───
import { useApiList } from '@/hooks/use-api';
import { Card, PageHeader, QueryGuard, StatusBadge } from '@/components/shared';
import { fetchSyncStatus } from '@/lib/api-bridge';

export default function SyncStatus() {
  const { items: statuses, isLoading, error, refetch } = useApiList<any>(['sync-status'], () => fetchSyncStatus());

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="同步状态" subtitle={`${statuses.length} 个节点 · 期望状态 vs 实际状态`} />
      <QueryGuard items={statuses} isLoading={isLoading} error={error} refetch={refetch} emptyMessage="暂无节点数据">
        {(items) => (
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
                {items.map((s: any) => (
                  <tr key={s.node_id} className="border-b border-a-border/50">
                    <td className="py-2.5 px-3">
                      <div className="font-medium text-a-fg">{s.node_name}</div>
                      {s.node_id && <div className="text-[10px] text-a-muted font-mono">{s.node_id}</div>}
                    </td>
                    <td className="py-2.5 px-3 font-mono text-a-fg">v{s.desired_revision}</td>
                    <td className="py-2.5 px-3 font-mono">
                      {s.applied_revision !== s.desired_revision
                        ? <span className="text-[#ff5c72]">v{s.applied_revision}</span>
                        : <span className="text-a-muted">v{s.applied_revision}</span>}
                    </td>
                    <td className="py-2.5 px-3"><StatusBadge status={s.status} /></td>
                    <td className="py-2.5 px-3 text-a-muted text-[11px]">{s.last_success_at ? new Date(s.last_success_at).toLocaleString('zh-CN') : '—'}</td>
                    <td className="py-2.5 px-3 text-[#ff5c72] text-[11px] truncate max-w-[240px]">{s.last_error || '—'}</td>
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

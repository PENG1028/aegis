// ─── Sync Status — desired vs actual state across all nodes ───
import { Link } from 'react-router-dom';
import { useApiList } from '@/hooks/use-api';
import { Card, PageHeader, QueryGuard, StatusBadge } from '@/components/shared';
import { fetchSyncStatus } from '@/lib/api-bridge';

export default function SyncStatus() {
  const { items: statuses, isLoading, error, refetch } = useApiList<any>(['sync-status'], () => fetchSyncStatus());

  const drifted = statuses.filter((s: any) => s.desired_revision !== s.applied_revision);

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="同步状态" subtitle={
        `${statuses.length} 个节点 · ` +
        (drifted.length > 0 ? `${drifted.length} 个有配置漂移` : '全部同步')
      } />

      {drifted.length > 0 && (
        <Card title={`配置漂移 (${drifted.length})`}>
          {drifted.map((s: any) => (
            <div key={s.node_id} className="flex items-center gap-4 p-3 rounded-a-sm bg-[#e8b830]/5 border border-[#e8b830]/10 mb-2">
              <div className="flex-1">
                <Link to={`/runtime/node/${s.node_id}`} className="text-sm font-medium text-a-fg hover:text-a-accent hover:underline">
                  {s.node_name || s.node_id}
                </Link>
                <div className="text-xs text-a-muted mt-0.5">
                  期望 r{s.desired_revision} → 实际 r{s.applied_revision}
                </div>
              </div>
              <StatusBadge status="drifted" />
              {s.last_error && <span className="text-[10px] text-[#ff5c72] truncate max-w-[200px]">{s.last_error}</span>}
            </div>
          ))}
        </Card>
      )}

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
                  <tr key={s.node_id} className="border-b border-a-border/50 hover:bg-a-border/10">
                    <td className="py-2.5 px-3">
                      <Link to={`/runtime/node/${s.node_id}`} className="font-medium text-a-fg hover:text-a-accent hover:underline">
                        {s.node_name || s.node_id}
                      </Link>
                      {s.node_id && <div className="text-[10px] text-a-muted font-mono">{s.node_id}</div>}
                    </td>
                    <td className="py-2.5 px-3 font-mono text-a-fg">r{s.desired_revision}</td>
                    <td className="py-2.5 px-3 font-mono">
                      {s.applied_revision !== s.desired_revision
                        ? <span className="text-[#ff5c72]">r{s.applied_revision}</span>
                        : <span className="text-[#4cd964]">r{s.applied_revision}</span>}
                    </td>
                    <td className="py-2.5 px-3"><StatusBadge status={s.status || (s.desired_revision === s.applied_revision ? 'active' : 'drifted')} /></td>
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

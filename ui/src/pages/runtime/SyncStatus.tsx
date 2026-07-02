import { useQuery } from '@tanstack/react-query';
import { Card, PageHeader, StatusBadge } from '@/components/shared';
import { getScenario } from '@/mocks';
import { API_CONFIG } from '@/lib/api-config';

export default function SyncStatus() {
  const { data } = useQuery({
    queryKey: ['sync-status-mock'],
    queryFn: async () => {
      await new Promise(r => setTimeout(r, 200));
      return getScenario().syncStatuses;
    },
    enabled: API_CONFIG.useMock,
  });
  const statuses = API_CONFIG.useMock
    ? (data || getScenario().syncStatuses)
    : [];

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="同步状态" subtitle="期望状态 vs 实际状态" />
      <Card>
        <table className="w-full text-xs">
          <thead><tr className="border-b border-a-border text-a-muted text-left"><th className="py-2 px-3">节点</th><th className="py-2 px-3">期望版本</th><th className="py-2 px-3">实际版本</th><th className="py-2 px-3">状态</th><th className="py-2 px-3">错误</th></tr></thead>
          <tbody>
            {statuses.map((s: any) => (
              <tr key={s.node_id} className="border-b border-a-border/50">
                <td className="py-2 px-3 font-medium text-a-fg">{s.node_name}</td>
                <td className="py-2 px-3 font-mono text-a-muted">v{s.desired_revision}</td>
                <td className="py-2 px-3 font-mono text-a-muted">v{s.applied_revision}</td>
                <td className="py-2 px-3"><StatusBadge status={s.status} /></td>
                <td className="py-2 px-3 text-[#ff5c72] text-[11px] truncate max-w-[200px]">{s.last_error || '—'}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </Card>
    </div>
  );
}

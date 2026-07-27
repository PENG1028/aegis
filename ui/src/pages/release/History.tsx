import { Card, PageHeader, StatusBadge, Timestamp } from '@/components/shared';
export default function History() {
  const history = [
    { version: 'v43', status: 'success', created_at: '2026-07-02T10:30:00Z', routes: 3 },
    { version: 'v42', status: 'success', created_at: '2026-07-01T08:15:00Z', routes: 3 },
    { version: 'v41', status: 'success', created_at: '2026-06-30T14:00:00Z', routes: 2 },
  ];

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="发布历史" subtitle={`${history.length} 次发布`} />
      <Card>
        <table className="w-full text-xs">
          <thead><tr className="border-b border-a-border text-a-muted text-left"><th className="py-2 px-3">版本</th><th className="py-2 px-3">状态</th><th className="py-2 px-3">路由数</th><th className="py-2 px-3">时间</th></tr></thead>
          <tbody>
            {history.map(h => (
              <tr key={h.version} className="border-b border-a-border/50">
                <td className="py-2 px-3 font-mono font-medium text-a-fg">{h.version}</td>
                <td className="py-2 px-3"><StatusBadge status={h.status} /></td>
                <td className="py-2 px-3 text-a-muted">{h.routes}</td>
                <td className="py-2 px-3"><Timestamp iso={h.created_at} /></td>
              </tr>
            ))}
          </tbody>
        </table>
      </Card>
    </div>
  );
}

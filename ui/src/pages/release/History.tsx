import { useQuery } from '@tanstack/react-query';
import { Card, PageHeader, StatusBadge, Timestamp } from '@/components/shared';
import { adminApi } from '@/lib/api-bridge';

export default function History() {
  // Real apply history from the backend (previously hardcoded fake releases).
  const { data } = useQuery({
    queryKey: ['apply-history'],
    queryFn: () => adminApi.applyHistory().catch(() => [] as any[]),
    refetchInterval: 30_000,
  });

  const history: any[] = Array.isArray(data) ? data : (data as any)?.history || [];

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="发布历史" subtitle={`${history.length} 次发布`} />
      <Card>
        {history.length === 0 ? (
          <div className="px-3 py-6 text-center text-xs text-a-muted">暂无发布记录 — 执行一次 Apply 后这里会显示真实历史</div>
        ) : (
          <table className="w-full text-xs">
            <thead><tr className="border-b border-a-border text-a-muted text-left"><th className="py-2 px-3">版本</th><th className="py-2 px-3">状态</th><th className="py-2 px-3">配置</th><th className="py-2 px-3">时间</th></tr></thead>
            <tbody>
              {history.map((h: any, i: number) => (
                <tr key={h.version || h.id || i} className="border-b border-a-border/50">
                  <td className="py-2 px-3 font-mono font-medium text-a-fg">{h.version || '—'}</td>
                  <td className="py-2 px-3"><StatusBadge status={h.status || 'success'} /></td>
                  <td className="py-2 px-3 text-a-muted truncate max-w-[220px]" title={h.message || ''}>{h.message || h.config_path || '—'}</td>
                  <td className="py-2 px-3"><Timestamp iso={h.created_at || ''} /></td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </Card>
    </div>
  );
}

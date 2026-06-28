import { useQuery } from '@tanstack/react-query';
import { fetchListeners } from '@/lib/api-bridge';
import { PageHeader, Card, Alert, StatusBadge } from '@/components/shared';

export default function ListenersPage() {
  const { data, isLoading, error } = useQuery({
    queryKey: ['listeners'],
    queryFn: () => fetchListeners(),
  });

  if (isLoading) return <div className="text-center py-10 text-a-muted font-mono text-sm">加载中...</div>;
  if (error) return <Alert type="err">加载失败: {(error as any).message}</Alert>;

  return (
    <div>
      <PageHeader title="监听器" helpKey="listeners" sub={`${(data || []).length} 个监听器`} />
      <Card>
        <table className="w-full text-sm border-collapse">
          <thead>
            <tr>
              {['绑定 IP', '端口', '提供商', '用途', '状态'].map((h) => (
                <th key={h} className="text-left px-3.5 py-2.5 text-[11px] font-semibold uppercase tracking-[0.06em] text-a-muted bg-a-bg border-b border-a-border whitespace-nowrap">{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {(data || []).map((l: any, i: number) => (
              <tr key={i} className="hover:bg-white/[0.04] [&:last-child>td]:border-b-0">
                <td className="px-3.5 py-2.5 border-b border-a-border-soft font-mono text-xs">{l.bind_ip}</td>
                <td className="px-3.5 py-2.5 border-b border-a-border-soft font-mono text-xs">{l.port}</td>
                <td className="px-3.5 py-2.5 border-b border-a-border-soft font-mono text-xs">{l.provider}</td>
                <td className="px-3.5 py-2.5 border-b border-a-border-soft text-xs">{l.purpose}</td>
                <td className="px-3.5 py-2.5 border-b border-a-border-soft"><StatusBadge status={l.status} /></td>
              </tr>
            ))}
            {(!data || data.length === 0) && (
              <tr><td colSpan={5} className="text-center py-10 text-a-muted text-xs">暂无监听器数据</td></tr>
            )}
          </tbody>
        </table>
      </Card>
    </div>
  );
}

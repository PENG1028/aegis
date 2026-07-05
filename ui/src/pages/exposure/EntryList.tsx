// ─── Entry List — unified view of routes + exposures + managed domains ───
import { useQuery } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { routeApi, exposureApi } from '@/lib/api-bridge';
import { Card, StatusBadge, Btn } from '@/components/shared';

export default function EntryList() {
  const nav = useNavigate();

  const { data: rd, isLoading: rl } = useQuery({
    queryKey: ['routes'], queryFn: () => routeApi.list().catch(() => ({ routes: [] })), refetchInterval: 30_000,
  });
  const { data: ed, isLoading: el } = useQuery({
    queryKey: ['exposures'], queryFn: () => exposureApi.list().catch(() => ({ exposures: [] })), refetchInterval: 30_000,
  });

  const routes = (rd as any)?.routes || [];
  const exposures = (ed as any)?.exposures || [];

  const items = [
    ...routes.map((r: any) => ({ key: r.id, _t: 'route', domain: r.domain, status: r.status, target: r.target || '—', comp: r.composition || (r.tls_enabled ? 'HTTPS' : 'HTTP') })),
    ...exposures.map((e: any) => ({ key: e.id, _t: 'exposure', domain: `:${e.entry_port || e.port || '?'}`, status: e.status, target: `${e.target_host || '?'}:${e.target_port || '?'}`, comp: (e.type || 'TCP').toUpperCase() })),
  ];

  return (
    <div className="p-6 space-y-5">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-bold text-a-fg">流量管理</h2>
          <p className="text-xs text-a-muted mt-1">{routes.length + exposures.length} 个入口</p>
        </div>
        <Btn primary onClick={() => nav('/exposure/new')}>新建入口</Btn>
      </div>

      <Card>
        {rl || el ? <div className="text-sm text-a-muted py-8 text-center">加载中...</div>
        : items.length === 0 ? (
          <div className="text-center py-12 text-a-muted text-sm">
            <p className="text-lg mb-2 opacity-40">还没有任何入口</p>
            <Btn onClick={() => nav('/exposure/new')}>创建第一个入口</Btn>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-xs">
              <thead><tr className="border-b border-a-border text-a-muted text-left">
                <th className="py-2.5 px-3 font-medium">入口</th>
                <th className="py-2.5 px-3 font-medium">类型</th>
                <th className="py-2.5 px-3 font-medium">后端</th>
                <th className="py-2.5 px-3 font-medium">状态</th>
              </tr></thead>
              <tbody>
                {items.map(item => (
                  <tr key={item.key} className="border-b border-a-border/30 hover:bg-a-border/5 cursor-pointer"
                    onClick={() => nav(`/exposure/entry/${item.key}`)}>
                    <td className="py-2.5 px-3 font-mono text-[11px]">{item.domain}</td>
                    <td className="py-2.5 px-3 text-[10px] text-a-muted">{item.comp}</td>
                    <td className="py-2.5 px-3 font-mono text-[11px] text-a-muted">{item.target}</td>
                    <td className="py-2.5 px-3"><StatusBadge status={item.status === 'active' ? 'active' : 'disabled'} /></td>
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

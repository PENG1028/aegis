// ─── 域名与路由 — unified domain + port management ───
import { useMemo, useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { routeApi, exposureApi } from '@/lib/api-bridge';
import { Card, StatusBadge, Btn, useToast } from '@/components/shared';

export default function EntryList() {
  const nav = useNavigate(); const toast = useToast(); const qc = useQueryClient();
  const [scope, setScope] = useState('all');

  const { data: rd, isLoading: rl } = useQuery({
    queryKey: ['routes'], queryFn: () => routeApi.list().catch(() => ({ routes: [] })), refetchInterval: 30_000,
  });
  const { data: ed, isLoading: el } = useQuery({
    queryKey: ['exposures'], queryFn: () => exposureApi.list().catch(() => ({ exposures: [] })), refetchInterval: 30_000,
  });

  const disableRoute = useMutation({
    mutationFn: async (id: string) => {
      const res = await fetch(`/api/routes/${id}/disable`, { method: 'POST', credentials: 'include' });
      if (!res.ok) { const e = await res.json().catch(()=>({})); throw new Error((e as any).error?.message || `HTTP ${res.status}`); }
    },
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['routes'] }); toast('已禁用'); },
    onError: (e: any) => toast(e.message || '禁用失败', 'error'),
  });
  const enableRoute = useMutation({
    mutationFn: async (id: string) => {
      const res = await fetch(`/api/routes/${id}/enable`, { method: 'POST', credentials: 'include' });
      if (!res.ok) { const e = await res.json().catch(()=>({})); throw new Error((e as any).error?.message || `HTTP ${res.status}`); }
    },
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['routes'] }); toast('已启用'); },
    onError: (e: any) => toast(e.message || '启用失败', 'error'),
  });
  const disableExposure = useMutation({
    mutationFn: (id: string) => exposureApi.disable(id),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['exposures'] }); toast('已禁用'); },
    onError: (e: any) => toast(e.message || '禁用失败', 'error'),
  });

  const routes = (rd as any)?.data || (rd as any)?.routes || [];
  const exposures = (ed as any)?.data || (ed as any)?.exposures || [];

  const allItems = useMemo(() => [
    ...routes.map((r: any) => ({
      key: r.id, _t: 'route' as const, name: r.domain,
      type: r.tls_enabled ? 'HTTPS' : 'HTTP',
      target: r.service_id || '—', status: r.status,
      scope: r.owner_type === 'space' ? (r.space_id || r.owner_id || 'service') : 'admin',
    })),
    ...exposures.map((e: any) => ({
      key: e.id, _t: 'exposure' as const, name: `:${e.entry_port || e.port || '?'}`,
      type: (e.type || 'TCP').toUpperCase(),
      target: `${e.target_host || '?'}:${e.target_port || '?'}`, status: e.status, scope: 'admin',
    })),
  ], [routes, exposures]);

  // Dynamic scope filters from data
  const scopes = useMemo(() => {
    const set = new Set<string>(); set.add('all');
    allItems.forEach(i => set.add(i.scope));
    return [...set];
  }, [allItems]);

  const filtered = scope === 'all' ? allItems : allItems.filter(i => i.scope === scope);
  const isLoading = rl || el;

  return (
    <div className="p-6 space-y-5">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-bold text-a-fg">域名与路由</h2>
          <p className="text-xs text-a-muted mt-1">{filtered.length} 条记录</p>
        </div>
        <Btn primary onClick={() => nav('/exposure/new')}>添加域名</Btn>
      </div>

      <div className="flex items-center gap-1 flex-wrap">
        {scopes.map(s => (
          <button key={s} onClick={() => setScope(s)}
            className={`px-3 py-1 rounded-a-sm text-[10px] font-medium transition-colors border cursor-pointer ${
              scope === s ? 'bg-a-accent/10 text-a-accent border-a-accent/30' : 'bg-a-bg text-a-muted border-a-border/30 hover:text-a-fg'
            }`}>
            {s === 'all' ? `全部 (${allItems.length})` : s === 'admin' ? `管理员 (${allItems.filter(i=>i.scope==='admin').length})` : `${s} (${allItems.filter(i=>i.scope===s).length})`}
          </button>
        ))}
      </div>

      <Card>
        {isLoading ? <div className="text-sm text-a-muted py-8 text-center">加载中...</div>
         : filtered.length === 0 ? (
          <div className="text-center py-12 text-a-muted text-sm">
            <p className="text-lg mb-2 opacity-40">还没有域名或端口映射</p>
            <p className="mb-4 text-xs opacity-60">创建域名映射让外部通过域名访问服务，或创建端口转发放行 TCP/UDP 流量</p>
            <Btn onClick={() => nav('/exposure/new')}>添加第一个域名</Btn>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-xs">
              <thead><tr className="border-b border-a-border text-a-muted text-left">
                <th className="py-2.5 px-3 font-medium">域名 / 端口</th>
                <th className="py-2.5 px-3 font-medium">类型</th>
                <th className="py-2.5 px-3 font-medium">后端</th>
                <th className="py-2.5 px-3 font-medium">来源</th>
                <th className="py-2.5 px-3 font-medium">状态</th>
                <th className="py-2.5 px-3 font-medium"></th>
              </tr></thead>
              <tbody>
                {filtered.map(item => (
                  <tr key={item.key} className="border-b border-a-border/30 hover:bg-a-border/5 cursor-pointer"
                    onClick={() => nav(`/exposure/entry/${item.key}`)}>
                    <td className="py-2.5 px-3 font-mono text-[11px]">{item.name}</td>
                    <td className="py-2.5 px-3 text-[10px] text-a-muted">{item.type}</td>
                    <td className="py-2.5 px-3 font-mono text-[11px] text-a-muted">{item.target}</td>
                    <td className="py-2.5 px-3 text-[10px]">
                      {item.scope === 'admin'
                        ? <span className="text-a-muted">管理员</span>
                        : <span className="px-1.5 py-0.5 rounded text-[9px] bg-a-accent/5 text-a-accent border border-a-accent/20 font-medium">{item.scope}</span>}
                    </td>
                    <td className="py-2.5 px-3"><StatusBadge status={item.status === 'active' ? 'active' : 'disabled'} /></td>
                    <td className="py-2.5 px-3">
                      {item._t === 'route' ? (
                        item.status === 'active'
                          ? <button onClick={e => { e.stopPropagation(); disableRoute.mutate(item.key); }} className="text-[10px] px-2 py-0.5 rounded border border-[#e8b830]/30 text-[#e8b830] hover:bg-[#e8b830]/10 cursor-pointer">禁用</button>
                          : <button onClick={e => { e.stopPropagation(); enableRoute.mutate(item.key); }} className="text-[10px] px-2 py-0.5 rounded border border-[#4cd964]/30 text-[#4cd964] hover:bg-[#4cd964]/10 cursor-pointer">启用</button>
                      ) : (
                        item.status === 'active'
                          ? <button onClick={e => { e.stopPropagation(); disableExposure.mutate(item.key); }} className="text-[10px] px-2 py-0.5 rounded border border-[#e8b830]/30 text-[#e8b830] hover:bg-[#e8b830]/10 cursor-pointer">禁用</button>
                          : <span className="text-[10px] text-a-muted/50">—</span>
                      )}
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

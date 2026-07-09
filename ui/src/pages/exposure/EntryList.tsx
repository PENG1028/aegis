// ─── 域名与路由 — unified domain + port management ───
import { useMemo, useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { routeApi, exposureApi, runtimeModeApi } from '@/lib/api-bridge';
import { Card, Btn, useToast } from '@/components/shared';
import { cn } from '@/lib/utils';

function HealthBadge({ status }: { status: string }) {
  const st = status === 'active' ? 'bg-[#4cd964]/10 text-[#4cd964] border-[#4cd964]/20'
    : status === 'unhealthy' ? 'bg-[#e8b830]/10 text-[#e8b830] border-[#e8b830]/20'
    : 'bg-a-border/10 text-a-muted border-a-border/20';
  const label = status === 'active' ? '活跃' : status === 'unhealthy' ? '不健康' : '禁用';
  return <span className={cn('px-2 py-0.5 rounded text-[10px] font-medium border', st)}>{label}</span>;
}

export default function EntryList() {
  const nav = useNavigate(); const toast = useToast(); const qc = useQueryClient();
  const [scope, setScope] = useState('all');

  const { data: rm } = useQuery({
    queryKey: ['runtime-mode'], queryFn: () => runtimeModeApi.get().catch(() => null), refetchInterval: 60_000,
  });
  const { data: rd, isLoading: rl } = useQuery({
    queryKey: ['routes'], queryFn: () => routeApi.list().catch(() => ({ routes: [] })), refetchInterval: 30_000,
  });
  const { data: ed, isLoading: el } = useQuery({
    queryKey: ['exposures'], queryFn: () => exposureApi.list().catch(() => ({ exposures: [] })), refetchInterval: 30_000,
  });

  const compositions = rm?.current?.compositions || [];

  const disableRoute = useMutation({
    mutationFn: async (id: string) => {
      const res = await fetch(`/api/admin/v1/routes/${id}/disable`, { method: 'POST', credentials: 'include' });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
    },
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['routes'] }); toast('已禁用'); },
    onError: (e: any) => toast(e.message || '失败', 'error'),
  });
  const enableRoute = useMutation({
    mutationFn: async (id: string) => {
      const res = await fetch(`/api/admin/v1/routes/${id}/enable`, { method: 'POST', credentials: 'include' });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
    },
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['routes'] }); toast('已启用'); },
    onError: (e: any) => toast(e.message || '失败', 'error'),
  });
  const deleteRoute = useMutation({
    mutationFn: async (id: string) => {
      const res = await fetch(`/api/admin/v1/routes/${id}`, { method: 'DELETE', credentials: 'include' });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
    },
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['routes'] }); toast('已删除'); },
    onError: (e: any) => toast(e.message || '删除失败', 'error'),
  });
  const disableExposure = useMutation({
    mutationFn: (id: string) => exposureApi.disable(id),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['exposures'] }); toast('已禁用'); },
    onError: (e: any) => toast(e.message || '失败', 'error'),
  });

  const routes = (rd as any)?.data || (rd as any)?.routes || [];
  const exposures = (ed as any)?.data || (ed as any)?.exposures || [];

  const allItems = useMemo(() => [
    ...routes.map((r: any) => {
      const compName = r.tls_enabled ? 'HTTPS Route' : 'HTTP Route';
      const comp = compositions.find((c: any) => c.name === compName);
      const health = r.status === 'active'
        ? (comp?.status === 'available' ? 'active' : 'unhealthy')
        : 'disabled';
      return {
        key: r.id, _t: 'route' as const, name: r.domain, type: compName, health,
        target: r.service_id || '—', status: r.status,
        scope: r.owner_type === 'space' ? (r.space_id || r.owner_id || 'service') : 'admin',
      };
    }),
    ...exposures.map((e: any) => ({
      key: e.id, _t: 'exposure' as const, name: `:${e.entry_port || e.port || '?'}`,
      type: (e.type || 'TCP').toUpperCase(), health: e.status === 'active' ? 'active' : 'disabled',
      target: `${e.target_host || '?'}:${e.target_port || '?'}`, status: e.status, scope: 'admin',
    })),
  ], [routes, exposures, compositions]);

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

      {/* ── Quick Publish ──
           @design: 1-click form to publish a domain without the 5-step flow.
           Calls POST /api/admin/v1/quick-publish and shows result inline. */}
      <QuickPublishForm onPublished={() => qc.invalidateQueries({ queryKey: ['routes'] })} />

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
                <th className="py-2.5 px-3 font-medium">健康</th>
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
                    <td className="py-2.5 px-3"><HealthBadge status={item.health} /></td>
                    <td className="py-2.5 px-3">
                      <div className="flex items-center gap-1">
                        {item._t === 'route' ? (
                          <>
                            {item.status === 'active'
                              ? <button onClick={e => { e.stopPropagation(); disableRoute.mutate(item.key); }} className="text-[10px] px-2 py-0.5 rounded border border-[#e8b830]/30 text-[#e8b830] hover:bg-[#e8b830]/10 cursor-pointer">禁用</button>
                              : <button onClick={e => { e.stopPropagation(); enableRoute.mutate(item.key); }} className="text-[10px] px-2 py-0.5 rounded border border-[#4cd964]/30 text-[#4cd964] hover:bg-[#4cd964]/10 cursor-pointer">启用</button>}
                            <button onClick={e => { e.stopPropagation(); if (confirm('确认删除？')) deleteRoute.mutate(item.key); }} className="text-[10px] px-2 py-0.5 rounded border border-[#ff5c72]/30 text-[#ff5c72] hover:bg-[#ff5c72]/10 cursor-pointer">删除</button>
                          </>
                        ) : (
                          item.status === 'active'
                            ? <button onClick={e => { e.stopPropagation(); disableExposure.mutate(item.key); }} className="text-[10px] px-2 py-0.5 rounded border border-[#e8b830]/30 text-[#e8b830] hover:bg-[#e8b830]/10 cursor-pointer">禁用</button>
                            : <span className="text-[10px] text-a-muted/50">—</span>
                        )}
                      </div>
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

// ─── QuickPublishForm ────────────────────────────────────────────────────
// @design: Inline form that collapses the 5-step Apply flow into one click.
// Calls POST /api/admin/v1/quick-publish with {domain, target_host, target_port}.
//
// Backend: internal/httpapi/handlers/quick_publish.go
// The handler auto-creates project → service → endpoint → route → apply.
function QuickPublishForm({ onPublished }: { onPublished: () => void }) {
  const toast = useToast();
  const [open, setOpen] = useState(false);
  const [domain, setDomain] = useState('');
  const [host, setHost] = useState('');
  const [port, setPort] = useState('3000');
  const [loading, setLoading] = useState(false);

  const handlePublish = async () => {
    if (!domain || !host) { toast('请填写域名和后端地址', 'error'); return; }
    setLoading(true);
    try {
      const res = await fetch('/api/admin/v1/quick-publish', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ domain, target_host: host, target_port: parseInt(port) || 3000 }),
      });
      const data = await res.json();
      if (data.status === 'success') {
        toast(data.message);
        setDomain(''); setHost(''); setPort('3000');
        setOpen(false);
        onPublished();
      } else {
        toast(data.message || '发布失败', 'error');
      }
    } catch (e: any) {
      toast(e.message || '请求失败', 'error');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="border border-a-border/30 rounded-a-sm mb-4">
      <button onClick={() => setOpen(!open)}
        className="flex items-center gap-2 px-3 py-2 text-xs font-medium text-a-fg hover:bg-a-border/10 w-full text-left cursor-pointer">
        <span className="text-a-muted">{open ? '▾' : '▸'}</span>
        快速接入
        <span className="text-[10px] text-a-muted font-normal">— 域名 + 后端地址，一步发布</span>
      </button>
      {open && (
        <div className="px-3 pb-3 pt-1 border-t border-a-border/20">
          <div className="grid grid-cols-4 gap-2 mb-2">
            <input value={domain} onChange={e => setDomain(e.target.value)}
              placeholder="example.com" className="col-span-2 px-2 py-1.5 text-xs bg-a-bg border border-a-border/50 rounded-a-sm text-a-fg placeholder:text-a-muted/30 focus:outline-none focus:border-a-accent/50" />
            <input value={host} onChange={e => setHost(e.target.value)}
              placeholder="127.0.0.1" className="px-2 py-1.5 text-xs bg-a-bg border border-a-border/50 rounded-a-sm text-a-fg placeholder:text-a-muted/30 focus:outline-none focus:border-a-accent/50" />
            <div className="flex gap-2">
              <input value={port} onChange={e => setPort(e.target.value)}
                placeholder="3000" className="w-20 px-2 py-1.5 text-xs bg-a-bg border border-a-border/50 rounded-a-sm text-a-fg placeholder:text-a-muted/30 focus:outline-none focus:border-a-accent/50" />
              <button onClick={handlePublish} disabled={loading}
                className="px-3 py-1.5 text-xs font-medium rounded-a-sm bg-a-accent text-white hover:brightness-110 disabled:opacity-50 cursor-pointer whitespace-nowrap">
                {loading ? '发布中...' : '发布'}
              </button>
            </div>
          </div>
          <p className="text-[10px] text-a-muted">自动创建服务、端点、路由并执行 Apply。支持 HTTPS。</p>
        </div>
      )}
    </div>
  );
}

// ─── Service Entry — unified inbound gateway binding ───
// Replaces the old fragmented EntryPoints / Exposure / QuickConnect / ImportConfig tabs.
// User selects a composition → form adapts → backend creates route/exposure automatically.
import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { runtimeModeApi, compositionApi, routeApi, exposureApi } from '@/lib/api-bridge';
import type { RuntimeModeDef, Composition, CompDef } from '@/lib/api-bridge';
import { Card, PageHeader, StatusBadge, Btn, useToast } from '@/components/shared';
import { cn } from '@/lib/utils';

// ═══════════════════════════════════════════════════════════════
// Types
// ═══════════════════════════════════════════════════════════════

type EntryType = 'http' | 'tcp' | 'udp';

interface EntryForm {
  composition: string;
  domain: string;
  port: number;
  targetHost: string;
  targetPort: number;
}

// ═══════════════════════════════════════════════════════════════
// Helpers
// ═══════════════════════════════════════════════════════════════

function entryType(comp: Composition): EntryType {
  const name = comp.name;
  if (name.includes('HTTP') || name.includes('HTTPS')) return 'http';
  if (name.includes('UDP')) return 'udp';
  return 'tcp';
}

function defaultPort(comp: Composition): number {
  if (comp.name.includes('HTTPS') || comp.name.includes('HTTP/3')) return 443;
  if (comp.name.includes('HTTP')) return 80;
  return 0;
}

const COMP_CARD: Record<string, { card: string; text: string }> = {
  available:        { card: 'bg-a-surface border-[#4cd964]/40 hover:bg-[#4cd964]/5',   text: 'text-a-fg' },
  missing_provider: { card: 'bg-[#ff5c72]/5 border-[#ff5c72]/30',                       text: 'text-[#ff5c72]' },
  unsupported:      { card: 'bg-a-border/10 border-a-border/20 opacity-40',              text: 'text-a-muted/50' },
};

// ═══════════════════════════════════════════════════════════════
// Composition Selector
// ═══════════════════════════════════════════════════════════════

function CompSelector({ compositions, selected, onSelect }: {
  compositions: Composition[];
  selected: string;
  onSelect: (name: string) => void;
}) {
  const avail = compositions.filter(c => c.status === 'available');
  const unavail = compositions.filter(c => c.status !== 'available');

  return (
    <div>
      <div className="text-[10px] text-a-muted uppercase tracking-wider mb-2">选择服务类型</div>
      <div className="flex items-center gap-2 flex-wrap">
        {[...avail, ...unavail].map(comp => {
          const st = COMP_CARD[comp.status] || COMP_CARD.unsupported;
          const disabled = comp.status !== 'available';
          return (
            <button key={comp.name} disabled={disabled}
              onClick={() => !disabled && onSelect(comp.name)}
              className={cn(
                'px-3 py-2 rounded-a-sm text-xs transition-colors border cursor-pointer',
                st.card,
                selected === comp.name && 'ring-2 ring-a-accent/50',
                disabled && 'cursor-not-allowed',
              )}>
              <div className={cn('font-medium', st.text)}>{comp.name}</div>
              <div className="text-[10px] text-a-muted mt-0.5">{comp.chain}</div>
              {comp.status === 'missing_provider' && (
                <div className="text-[9px] text-[#ff5c72]/70 mt-0.5">需安装中间件</div>
              )}
            </button>
          );
        })}
      </div>
    </div>
  );
}

// ═══════════════════════════════════════════════════════════════
// Entry Form
// ═══════════════════════════════════════════════════════════════

function EntryForm({ comp, onSubmit, loading }: {
  comp: Composition;
  onSubmit: (f: EntryForm) => void;
  loading: boolean;
}) {
  const [domain, setDomain] = useState('');
  const [port, setPort] = useState(defaultPort(comp));
  const [targetHost, setTargetHost] = useState('');
  const [targetPort, setTargetPort] = useState(3000);
  const isHTTP = entryType(comp) === 'http';

  const canSubmit = isHTTP ? (domain && targetHost && targetPort > 0) : (port > 0 && targetHost && targetPort > 0);

  return (
    <div className="space-y-3 p-4 rounded-a-md border border-a-border/30 bg-a-surface/50">
      <div className="text-xs font-medium text-a-fg">{comp.name} 配置</div>
      <div className="grid grid-cols-2 gap-3">
        {isHTTP ? (
          <div>
            <label className="text-[10px] text-a-muted block mb-1">域名</label>
            <input value={domain} onChange={e => setDomain(e.target.value)}
              placeholder="api.example.com"
              className="w-full px-2.5 py-1.5 rounded-a-sm border border-a-border/50 bg-a-bg text-xs text-a-fg outline-none focus:border-a-accent/50" />
          </div>
        ) : (
          <div>
            <label className="text-[10px] text-a-muted block mb-1">入口端口</label>
            <input type="number" value={port} onChange={e => setPort(Number(e.target.value))}
              placeholder="5432"
              className="w-full px-2.5 py-1.5 rounded-a-sm border border-a-border/50 bg-a-bg text-xs text-a-fg outline-none focus:border-a-accent/50" />
          </div>
        )}
        <div>
          <label className="text-[10px] text-a-muted block mb-1">目标地址</label>
          <input value={targetHost} onChange={e => setTargetHost(e.target.value)}
            placeholder="127.0.0.1"
            className="w-full px-2.5 py-1.5 rounded-a-sm border border-a-border/50 bg-a-bg text-xs text-a-fg outline-none focus:border-a-accent/50" />
        </div>
        <div>
          <label className="text-[10px] text-a-muted block mb-1">目标端口</label>
          <input type="number" value={targetPort} onChange={e => setTargetPort(Number(e.target.value))}
            placeholder="3000"
            className="w-full px-2.5 py-1.5 rounded-a-sm border border-a-border/50 bg-a-bg text-xs text-a-fg outline-none focus:border-a-accent/50" />
        </div>
      </div>
      <Btn primary disabled={!canSubmit || loading}
        onClick={() => onSubmit({ composition: comp.name, domain, port, targetHost, targetPort })}>
        {loading ? '创建中...' : '创建入口'}
      </Btn>
    </div>
  );
}

// ═══════════════════════════════════════════════════════════════
// Entry List
// ═══════════════════════════════════════════════════════════════

function EntryList({ routes, exposures, isLoading }: {
  routes: any[];
  exposures: any[];
  isLoading: boolean;
}) {
  const queryClient = useQueryClient();
  const toast = useToast();

  const disableRoute = useMutation({
    mutationFn: (id: string) => routeApi.get(id).then(() => id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['routes'] });
      toast('已禁用');
    },
  });

  const disableExposure = useMutation({
    mutationFn: (id: string) => exposureApi.disable(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['exposures'] });
      toast('已禁用');
    },
  });

  if (isLoading) return <div className="text-sm text-a-muted py-6 text-center">加载中...</div>;

  const items = [
    ...routes.map((r: any) => ({ ...r, _type: 'route' as const })),
    ...exposures.map((e: any) => ({ ...e, _type: 'exposure' as const })),
  ];

  if (items.length === 0) {
    return <div className="text-center py-8 text-a-muted text-sm">暂无服务入口</div>;
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-xs">
        <thead>
          <tr className="border-b border-a-border text-a-muted text-left">
            <th className="py-2 px-3 font-medium">入口</th>
            <th className="py-2 px-3 font-medium">类型</th>
            <th className="py-2 px-3 font-medium">目标</th>
            <th className="py-2 px-3 font-medium">状态</th>
          </tr>
        </thead>
        <tbody>
          {items.map((item: any, i: number) => (
            <tr key={i} className="border-b border-a-border/30 hover:bg-a-border/5">
              <td className="py-2 px-3 font-mono text-[11px]">
                {item._type === 'route' ? item.domain : `:${item.port || item.entry_port || '?'}`}
              </td>
              <td className="py-2 px-3 text-[10px] text-a-muted">
                {item._type === 'route' ? 'HTTP/HTTPS' : (item.type || item.exposure_type || 'TCP/UDP').toUpperCase()}
              </td>
              <td className="py-2 px-3 font-mono text-[11px] text-a-muted">
                {item.target || item.target_host || `${item.target_host || '?'}:${item.target_port || '?'}`}
              </td>
              <td className="py-2 px-3">
                <StatusBadge status={item.status === 'active' ? 'active' : 'disabled'} />
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

// ═══════════════════════════════════════════════════════════════
// Main
// ═══════════════════════════════════════════════════════════════

export default function ServiceEntry() {
  const toast = useToast();
  const queryClient = useQueryClient();
  const [selectedComp, setSelectedComp] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const { data: runtimeMode } = useQuery({
    queryKey: ['runtime-mode'],
    queryFn: () => runtimeModeApi.get(),
    refetchInterval: 60_000,
  });

  const { data: routesData, isLoading: routesLoading } = useQuery({
    queryKey: ['routes'],
    queryFn: () => routeApi.list(),
    refetchInterval: 30_000,
  });

  const { data: exposuresData, isLoading: expLoading } = useQuery({
    queryKey: ['exposures'],
    queryFn: () => exposureApi.list(),
    refetchInterval: 30_000,
  });

  const compositions = runtimeMode?.current?.compositions || [];
  const routes = (routesData as any)?.routes || [];
  const exposures = (exposuresData as any)?.exposures || [];

  const handleSubmit = async (form: EntryForm) => {
    setSubmitting(true);
    try {
      const type = entryType(compositions.find(c => c.name === form.composition)!);
      if (type === 'http') {
        await fetch('/api/v1/actions/bind-http-domain', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ domain: form.domain, target_host: form.targetHost, target_port: form.targetPort }),
          credentials: 'include',
        }).then(r => { if (!r.ok) throw new Error(`HTTP ${r.status}`); return r.json(); });
      } else {
        await exposureApi.create({
          type: type,
          entry_port: form.port,
          target_host: form.targetHost,
          target_port: form.targetPort,
          description: `${form.composition} — ${form.domain || `port ${form.port}`}`,
        });
      }
      toast('入口创建成功');
      setSelectedComp('');
      queryClient.invalidateQueries({ queryKey: ['routes'] });
      queryClient.invalidateQueries({ queryKey: ['exposures'] });
    } catch (e: any) {
      toast(e.message || '创建失败', 'error');
    } finally {
      setSubmitting(false);
    }
  };

  const selected = compositions.find(c => c.name === selectedComp);

  return (
    <div className="space-y-5">
      <CompSelector compositions={compositions} selected={selectedComp} onSelect={setSelectedComp} />

      {selected && (
        <EntryForm comp={selected} onSubmit={handleSubmit} loading={submitting} />
      )}

      <Card title={`已有入口 (${routes.length + exposures.length})`}>
        <EntryList routes={routes} exposures={exposures} isLoading={routesLoading || expLoading} />
      </Card>
    </div>
  );
}

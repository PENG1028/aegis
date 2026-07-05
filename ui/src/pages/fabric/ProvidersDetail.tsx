// ─── Provider Detail — 独立页面 (从 Providers.tsx 拆分) ───

import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { providerApi, runtimeModeApi } from '@/lib/api-bridge';
import { PageHeader, HealthDot, StatusBadge, Card, Btn, useToast, LoadingState, ErrorBanner } from '@/components/shared';
import { cn } from '@/lib/utils';

interface ProviderState {
  id: string; name: string; gateway_type: string; status: string;
  installed: boolean; running: boolean; version: string;
  binary_path: string; config_path: string;
  capabilities: string[]; theoretical_capabilities: string[];
  ports: { port: number; owner: string; protocol: string; purpose: string; status: string }[];
  diagnostic?: any;
}

interface CapabilityDef {
  key: string; layer: string; label: string; description: string;
}

// ─── Helpers ───

function getCapStatus(prov: ProviderState, capKey: string): 'native' | 'theoretical' | 'unsupported' {
  if (prov.capabilities?.includes(capKey)) return 'native';
  if (prov.theoretical_capabilities?.includes(capKey)) return 'theoretical';
  return 'unsupported';
}

function CapBar({ native, theoretical, unsupported, total }: {
  native: number; theoretical: number; unsupported: number; total: number;
}) {
  const pct = (n: number) => total > 0 ? (n / total * 100).toFixed(1) : '0';
  return (
    <div className="flex items-center gap-2">
      <div className="flex-1 h-1.5 rounded-full bg-a-border/20 overflow-hidden flex">
        {native > 0 && <div className="h-full bg-[#4cd964]" style={{ width: `${pct(native)}%` }} />}
        {theoretical > 0 && <div className="h-full bg-[#e8b830]" style={{ width: `${pct(theoretical)}%` }} />}
      </div>
      <span className="text-[10px] font-mono text-a-muted w-8 text-right">{native}/{total}</span>
    </div>
  );
}

// ─── Provider Card ───

function ProviderCard({ provider, universe, toast }: {
  provider: ProviderState;
  universe: CapabilityDef[];
  toast: any;
}) {
  const [expanded, setExpanded] = useState(false);
  const isAvailable = provider.installed && provider.running;
  const nativeCount = provider.capabilities?.length || 0;
  const theoCount = provider.theoretical_capabilities?.filter(c => !provider.capabilities?.includes(c)).length || 0;
  const unsupCount = universe.length - nativeCount - theoCount;

  const layerOrder = ['L3', 'L4', 'L5', 'L6', 'L7'];
  const byLayer = new Map<string, CapabilityDef[]>();
  for (const c of universe) {
    const list = byLayer.get(c.layer) || [];
    list.push(c); byLayer.set(c.layer, list);
  }

  const LAYER_COLORS: Record<string, string> = {
    L3: 'border-l-purple-500/40 bg-purple-500/3', L4: 'border-l-blue-500/40 bg-blue-500/3',
    L5: 'border-l-teal-500/40 bg-teal-500/3', L6: 'border-l-amber-500/40 bg-amber-500/3',
    L7: 'border-l-green-500/40 bg-green-500/3',
  };

  return (
    <div className={cn('border rounded-a-md transition-all',
      isAvailable ? 'bg-a-surface border-a-border'
        : provider.installed ? 'bg-a-surface/70 border-[#e8b830]/20'
          : 'bg-[#ff5c72]/3 border-[#ff5c72]/15',
    )}>
      <button onClick={() => setExpanded(!expanded)}
        className="w-full flex items-center gap-3 px-4 py-3 text-left cursor-pointer hover:bg-a-border/5 transition-colors">
        <HealthDot status={provider.running ? 'healthy' : provider.installed ? 'degraded' : 'failed'} />
        <span className={cn('font-semibold text-sm', isAvailable ? 'text-a-fg' : 'text-[#ff5c72]')}>{provider.name}</span>
        <StatusBadge status={provider.status} />
        <span className={cn('text-[11px] font-mono ml-auto mr-2', isAvailable ? 'text-a-muted' : 'text-[#ff5c72]/60')}>
          {nativeCount}/{universe.length}
        </span>
        <svg className={cn('w-4 h-4 text-a-muted transition-transform shrink-0', expanded && 'rotate-180')}
          viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M6 9l6 6 6-6"/></svg>
      </button>
      {expanded && (
        <div className="px-4 pb-4 border-t border-a-border/30 pt-3 space-y-4">
          <div className="grid grid-cols-2 lg:grid-cols-4 gap-3 text-xs">
            <div><span className="text-a-muted">Gateway Type</span><div className="text-a-fg font-mono mt-0.5 font-medium">{provider.gateway_type}</div></div>
            <div><span className="text-a-muted">Binary</span><div className={cn('font-mono mt-0.5 truncate', provider.binary_path ? 'text-a-fg' : 'text-[#ff5c72]/60')}>{provider.binary_path || '未检测到'}</div></div>
            <div><span className="text-a-muted">Config</span><div className="text-a-fg font-mono mt-0.5 truncate">{provider.config_path || '—'}</div></div>
            <div><span className="text-a-muted">Ports</span><div className="text-a-fg font-mono mt-0.5">{provider.ports?.length ? provider.ports.map(b => `${b.port}/${b.protocol}(${b.purpose})`).join(', ') : '—'}</div></div>
          </div>
          <div>
            <div className="flex items-center gap-2 mb-1.5">
              <span className="text-[10px] text-a-muted uppercase tracking-wider">Capabilities</span>
              <span className={cn('text-[10px] font-mono', isAvailable ? 'text-a-muted' : 'text-[#ff5c72]/60')}>{nativeCount}/{universe.length}</span>
            </div>
            <CapBar native={nativeCount} theoretical={theoCount} unsupported={unsupCount} total={universe.length} />
            <div className="flex gap-3 mt-1 text-[9px]"><span className="text-[#4cd964]">{nativeCount} 原生</span><span className="text-[#e8b830]">{theoCount} 可实现</span><span className="text-a-muted">{unsupCount} 不适用</span></div>
          </div>
          <div className="space-y-1.5">
            {layerOrder.map(layer => {
              const caps = byLayer.get(layer);
              if (!caps?.length) return null;
              return (
                <div key={layer} className="flex items-start gap-2 text-[10px]">
                  <span className={cn('px-1 py-0.5 rounded font-mono w-8 text-center shrink-0 mt-0.5', LAYER_COLORS[layer])}>{layer}</span>
                  <div className="flex flex-wrap gap-1 flex-1">
                    {caps.map(cap => {
                      const s = getCapStatus(provider, cap.key);
                      return (
                        <span key={cap.key} title={`${cap.label}: ${cap.description}`}
                          className={cn('px-1.5 py-0.5 rounded text-[10px] font-mono',
                            s === 'native' ? 'bg-[#4cd964]/10 text-[#4cd964] border border-[#4cd964]/20' :
                            s === 'theoretical' ? 'bg-[#e8b830]/10 text-[#e8b830] border border-[#e8b830]/20' :
                            'bg-a-border/10 text-a-muted/50 border border-a-border/10')}>
                          {s === 'native' ? '✓' : s === 'theoretical' ? '△' : '—'} {cap.key}
                        </span>
                      );
                    })}
                  </div>
                </div>
              );
            })}
          </div>
          {provider.installed ? (
            <div className="flex gap-2 pt-1 border-t border-a-border/30">
              <Btn onClick={() => { providerApi.reload(provider.id).then(() => toast(`${provider.name} 重载成功`)).catch((e: Error) => toast(`失败: ${e.message}`, 'error')); }} className="text-[10px]">热重载</Btn>
              <Btn onClick={() => { providerApi.getConfig(provider.id).then(c => toast(JSON.stringify(c))).catch((e: Error) => toast(`失败: ${e.message}`, 'error')); }} className="text-[10px]">查看配置</Btn>
              <Btn onClick={() => { providerApi.diagnoseAll().then(() => toast('诊断完成')).catch((e: Error) => toast(`失败: ${e.message}`, 'error')); }} className="text-[10px]">诊断</Btn>
            </div>
          ) : (
            <div className="flex gap-2 pt-1 border-t border-a-border/30">
              <Btn primary onClick={() => { providerApi.install(provider.id).then(() => toast(`${provider.name} 安装成功`)).catch((e: Error) => toast(`失败: ${e.message}`, 'error')); }} className="text-[10px]">安装</Btn>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

// ══════════════════════════════════════════════════════════════════
// Main Page
// ══════════════════════════════════════════════════════════════════

export default function ProvidersDetail() {
  const toast = useToast();

  const { data: provData, isLoading, error, refetch } = useQuery({
    queryKey: ['providers-detail'],
    queryFn: async () => {
      const result = await (providerApi as any).list();
      return {
        providers: (result?.providers || []) as ProviderState[],
        universe: (result?.capability_universe || []) as CapabilityDef[],
      };
    },
  });

  const providers = provData?.providers || [];
  const universe = provData?.universe || [];

  return (
    <div className="space-y-5">
      <PageHeader
        title="Provider 详情"
        subtitle={`${providers.length} 个 Provider · ${universe.length} 项能力`}
      />

      <Card title="Provider 详情" subtitle="点击展开查看完整能力清单、诊断信息和操作">
        {isLoading ? (
          <LoadingState />
        ) : error ? (
          <ErrorBanner message={(error as any)?.message} onRetry={refetch} />
        ) : providers.length === 0 ? (
          <div className="text-center py-12 text-sm text-a-muted">
            没有检测到 Provider · 运行 <code className="text-a-accent">aegis doctor</code> 诊断
          </div>
        ) : (
          <div className="space-y-2">
            {providers.map(p => <ProviderCard key={p.id} provider={p} universe={universe} toast={toast} />)}
          </div>
        )}
      </Card>
    </div>
  );
}

// ─── Trace ───
// Core troubleshooting page: trace any domain/SNI/route, show full chain with failure layer highlighted.
// v2: Port policy awareness, SNI passthrough recognition, Unix socket detection.
//
// Pipeline visualization:
//   Legacy mode:    DNS → Caddy :80/:443 (TLS 终止) → Route → Gateway → Service → Endpoint (TCP/Unix)
//   EdgeMux mode:   DNS → HAProxy :443 (SNI 直通) → Caddy :8443 (TLS 终止) → Route → Gateway → Service → Endpoint (TCP/Unix)

import { useState, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { Card, PageHeader, Btn, StatusBadge, HealthDot } from '@/components/shared';
import Input from '@/components/ui/Input';
import { PathRibbon } from '@/components/workspace/PathRibbon';
import { useChain } from '@/hooks/useChain';
import { traceApi, runtimeModeApi } from '@/lib/api-bridge';

import { cn } from '@/lib/utils';

// ─── Port policy indicator ───

function PortPolicyPill({ mode, loading }: { mode: string | null; loading: boolean }) {
  if (loading) return <span className="text-[10px] text-a-muted">加载端口策略...</span>;
  if (!mode) return null;

  const isEdgeMux = mode === 'edge_mux';

  return (
    <div className={cn(
      'inline-flex items-center gap-2 px-3 py-1.5 rounded text-[11px]',
      isEdgeMux ? 'bg-a-accent/10 border border-a-accent/20' : 'bg-a-border/20 border border-a-border/30',
    )}>
      <span className="text-a-muted">端口策略:</span>
      <span className={cn('font-mono font-medium', isEdgeMux ? 'text-a-accent' : 'text-a-fg2')}>
        {isEdgeMux ? 'EdgeMux' : 'Legacy'}
      </span>
      <span className="text-[10px] text-a-muted">
        {isEdgeMux ? 'HAProxy :443 SNI → Caddy :8443 TLS' : 'Caddy :80 + :443 TLS'}
      </span>
    </div>
  );
}

// ─── Pipeline summary diagram ───

function PipelineSummary({ mode, domain }: { mode: string | null; domain: string }) {
  const isEdgeMux = mode === 'edge_mux';

  return (
    <div className="flex items-center gap-1 flex-wrap text-[11px] font-mono">
      <span className="text-a-fg">🌐 请求</span>
      <svg className="w-3 h-3 text-a-border" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M9 18l6-6-6-6"/></svg>
      <span className="text-a-fg2">DNS → {domain}</span>
      <svg className="w-3 h-3 text-a-border" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M9 18l6-6-6-6"/></svg>

      {isEdgeMux ? (
        <>
          <span className="px-1.5 py-0.5 rounded bg-a-accent/10 text-a-accent">HAProxy :443</span>
          <span className="text-[10px] text-a-muted">SNI 直通</span>
          <svg className="w-3 h-3 text-a-border" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M9 18l6-6-6-6"/></svg>
          <span className="px-1.5 py-0.5 rounded bg-[#4cd964]/10 text-[#4cd964]">Caddy :8443</span>
          <span className="text-[10px] text-a-muted">TLS 终止</span>
        </>
      ) : (
        <>
          <span className="px-1.5 py-0.5 rounded bg-[#4cd964]/10 text-[#4cd964]">Caddy :443</span>
          <span className="text-[10px] text-a-muted">TLS 终止</span>
        </>
      )}

      <svg className="w-3 h-3 text-a-border" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M9 18l6-6-6-6"/></svg>
      <span className="text-a-fg2">Route</span>
      <svg className="w-3 h-3 text-a-border" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M9 18l6-6-6-6"/></svg>
      <span className="text-a-fg2">Upstream</span>
    </div>
  );
}

// ─── Step renderer ───
function getStepColor(status: string) {
  switch (status) {
    case 'ok': return { dot: 'bg-[#4cd964] border-[#4cd964]', bg: 'bg-[#4cd964]/3 border-[#4cd964]/10', line: 'bg-[#4cd964]/30' };
    case 'failed': return { dot: 'bg-[#ff5c72] border-[#ff5c72] ring-2 ring-[#ff5c72]/30', bg: 'bg-[#ff5c72]/5 border-[#ff5c72]/20', line: 'bg-[#ff5c72]/30' };
    case 'skipped': return { dot: 'bg-a-border border-a-border', bg: 'bg-a-bg/30 border-a-border/30 opacity-50', line: 'bg-a-border/50' };
    default: return { dot: 'bg-a-bg border-a-border', bg: 'bg-a-bg border-a-border', line: 'bg-a-border/50' };
  }
}

// ─── Main Page ───

export default function Trace() {
  const nav = useNavigate();
  const [domain, setDomain] = useState('');
  const [inputType, setInputType] = useState<'domain' | 'sni' | 'route'>('domain');
  const [routeId, setRouteId] = useState('');
  const [result, setResult] = useState<any>(null);
  const [loading, setLoading] = useState(false);

  const chainRouteId = result?.route_id;
  const { data: chain } = useChain('route', chainRouteId);

  // Fetch port policy for pipeline visualization
  const { data: runtimeMode } = useQuery({
    queryKey: ['runtime-mode'],
    queryFn: () => runtimeModeApi.get().catch(() => null),
    refetchInterval: 120_000,
  });

  const portPolicyMode = runtimeMode?.current?.id || null;

  const handleTrace = async () => {
    setLoading(true);
    setResult(null);
    try {
      if (inputType === 'domain') {
        setResult(await traceApi.byDomain(domain));
      } else if (inputType === 'route') {
        setResult(await traceApi.byRoute(routeId));
      } else {
        setResult(await traceApi.bySNI(domain));
      }
    } catch (e) {
      setResult({ error: (e as Error).message });
    } finally {
      setLoading(false);
    }
  };

  const failedStepIndex = useMemo(() => {
    if (!result?.steps) return -1;
    return result.steps.findIndex((s: any) => s.status === 'failed');
  }, [result]);

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="链路追踪" subtitle="追踪完整请求路径 · 自动定位失败层 · 端口策略感知" />

      {/* Port Policy + Pipeline Summary */}
      <Card title="当前搭配">
        <div className="flex items-center gap-4 mb-3">
          <PortPolicyPill mode={portPolicyMode} loading={false} />
          {result?.hasSNIPassthrough && (
            <span className="text-[11px] px-2 py-1 rounded bg-a-accent/10 text-a-accent flex items-center gap-1">
              🔐 SNI 直通
            </span>
          )}
          {result?.unixSocketEndpoints > 0 && (
            <span className="text-[11px] px-2 py-1 rounded bg-[#e8b830]/10 text-[#e8b830] flex items-center gap-1">
              🔗 {result.unixSocketEndpoints} 个 Unix Socket
            </span>
          )}
        </div>
        <PipelineSummary mode={portPolicyMode} domain={domain || '(输入域名后显示)'} />
      </Card>

      {/* Input */}
      <Card title="输入">
        <div className="flex gap-2 mb-3">
          {(['domain', 'sni', 'route'] as const).map(t => (
            <button key={t} onClick={() => setInputType(t)}
              className={cn('px-3 py-1 text-xs rounded cursor-pointer transition-colors',
                inputType === t ? 'bg-a-accent/20 text-a-accent font-medium' : 'bg-a-bg text-a-muted hover:text-a-fg')}>
              {t === 'domain' ? '域名追踪' : t === 'sni' ? 'SNI 追踪' : 'Route ID 追踪'}
            </button>
          ))}
        </div>
        <div className="flex gap-2">
          {inputType === 'route' ? (
            <Input value={routeId} onChange={(e: React.ChangeEvent<HTMLInputElement>) => setRouteId(e.target.value)} placeholder="输入 Route ID" className="flex-1" />
          ) : (
            <Input value={domain} onChange={(e: React.ChangeEvent<HTMLInputElement>) => setDomain(e.target.value)} placeholder={inputType === 'sni' ? '输入 SNI 主机名' : '输入域名，如 api.proofnote.dev'} className="flex-1" />
          )}
          <Btn primary onClick={handleTrace} disabled={loading}>{loading ? '追踪中...' : '开始追踪'}</Btn>
        </div>
        <div className="flex gap-1.5 mt-2">
          <span className="text-[10px] text-a-muted">快捷:</span>
          {['api.proofnote.dev', 'auth.proofnote.dev', 'docs.proofnote.dev'].map(d => (
            <button key={d} onClick={() => { setDomain(d); setInputType('domain'); }}
              className="text-[10px] px-1.5 py-0.5 rounded bg-a-bg border border-a-border text-a-muted hover:text-a-fg cursor-pointer">{d}</button>
          ))}
        </div>
        <div className="text-[10px] text-a-muted mt-2">
          输入域名、SNI 主机名或 Route ID 进行全链路追踪 · 自动识别端口策略和传输协议
        </div>
      </Card>

      {/* Error */}
      {result?.error && (
        <Card title="追踪失败">
          <div className="p-4 rounded bg-[#ff5c72]/10 border border-[#ff5c72]/20 text-[#ff5c72] text-xs">{result.error}</div>
        </Card>
      )}

      {/* Steps with pipeline phase markers */}
      {result?.steps?.length > 0 && (
        <Card title="追踪步骤" subtitle={result.summary}>
          <div className="relative">
            {result.steps.map((step: any, i: number) => {
              const colors = getStepColor(step.status);
              return (
                <div key={i} className="flex items-stretch">
                  {/* Step connector */}
                  <div className="flex flex-col items-center mr-3">
                    <div className={cn('w-2.5 h-2.5 rounded-full border-2 shrink-0', colors.dot)} />
                    {i < result.steps.length - 1 && (
                      <div className={cn('w-0.5 flex-1 min-h-[20px]', colors.line)} />
                    )}
                  </div>
                  {/* Step content */}
                  <div className={cn('flex-1 pb-4', step.status === 'failed' && 'pb-6')}>
                    <div className={cn('p-3 rounded-a-sm border text-xs', colors.bg)}>
                      <div className="flex items-center gap-2 mb-1">
                        <span className="text-[10px] text-a-muted font-mono">#{i + 1}</span>
                        {/* Phase badge */}
                        {step.phase && (
                          <span className={cn(
                            'text-[10px] px-1 py-0.5 rounded font-medium',
                            step.phase === 'match' ? 'bg-a-accent/10 text-a-accent' :
                            step.phase === 'gateway' ? 'bg-[#4cd964]/10 text-[#4cd964]' :
                            step.phase === 'forwarding' ? 'bg-[#e8b830]/10 text-[#e8b830]' :
                            step.phase === 'target' ? 'bg-a-border/30 text-a-fg2' :
                            'bg-a-border/20 text-a-muted',
                          )}>
                            {step.phase === 'match' ? '匹配' :
                             step.phase === 'gateway' ? '网关' :
                             step.phase === 'forwarding' ? '转发' :
                             step.phase === 'target' ? '目标' :
                             step.phase === 'fallback' ? '回退' :
                             step.phase === 'health' ? '健康' : step.phase}
                          </span>
                        )}
                        <span className="font-semibold text-a-fg">{step.name}</span>
                        {/* Special markers */}
                        {step.passthrough && (
                          <span className="text-[10px] px-1 py-0.5 rounded bg-a-accent/10 text-a-accent font-medium">SNI 直通</span>
                        )}
                        {step.isUnixSocket && (
                          <span className="text-[10px] px-1 py-0.5 rounded bg-[#e8b830]/10 text-[#e8b830] font-medium">Unix Socket</span>
                        )}
                        <StatusBadge status={step.status === 'ok' ? 'active' : step.status === 'failed' ? 'error' : step.status === 'skipped' ? 'disabled' : 'pending'} />
                      </div>
                      <p className="text-a-fg2">{step.description}</p>
                      {step.detail && <p className="text-[10px] text-a-muted mt-0.5">{step.detail}</p>}
                      {step.error && (
                        <div className="mt-2 p-2 rounded bg-[#ff5c72]/10 border border-[#ff5c72]/20 text-[#ff5c72] text-[11px] font-medium">
                          ✕ {step.error}
                        </div>
                      )}
                      {step.status === 'failed' && chain && (
                        <button onClick={() => chain.entryPoint && nav(`/exposure/entry/${chain.entryPoint.route_id}`)}
                          className="mt-2 text-[10px] text-a-accent hover:underline cursor-pointer">
                          查看详情 →
                        </button>
                      )}
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        </Card>
      )}

      {/* Chain visualization overlay */}
      {chain && result?.steps?.length > 0 && (
        <Card title="完整链路">
          <PathRibbon chain={chain} focusType="route" focusId={chainRouteId} />
        </Card>
      )}

      {/* Anomalies */}
      {result?.anomalies?.length > 0 && (
        <Card title="关联异常">
          {result.anomalies.map((a: string, i: number) => (
            <div key={i} className="flex items-center gap-2 text-xs py-1">
              <HealthDot status="degraded" />
              <span className="text-a-fg2">{a}</span>
            </div>
          ))}
        </Card>
      )}
    </div>
  );
}

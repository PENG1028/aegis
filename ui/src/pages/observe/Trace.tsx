// ─── Trace ───
// Core troubleshooting page: trace any domain/SNI/route, show full chain with failure layer highlighted.

import { useState, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { Card, PageHeader, Btn, StatusBadge, HealthDot } from '@/components/shared';
import Input from '@/components/ui/Input';
import { PathRibbon } from '@/components/workspace/PathRibbon';
import { useChain } from '@/hooks/useChain';
import { traceApi } from '@/lib/api-bridge';
import { resolveChain } from '@/mocks/generators/chain-factory';
import { getScenario } from '@/mocks';
import { API_CONFIG } from '@/lib/api-config';
import { cn } from '@/lib/utils';

// In mock mode, build trace result from scenario data
function mockTrace(domain: string) {
  const route = getScenario().routes.find(r => r.domain === domain);
  if (!route) return { error: `未找到域名 ${domain} 对应的路由` };

  const chain = resolveChain('route', route.route_id);
  const anomalies = getScenario().anomalies.filter(a =>
    a.affectedObjects.some(o => o.id === route.route_id || o.id === route.service_id)
  );

  const steps = [
    { name: 'DNS 解析', status: 'ok', description: `${domain} → ${chain.nodes[0]?.public_ip || '43.160.211.232'}` },
    { name: 'TLS 终止', status: route.tls_mode !== 'http_only' ? 'ok' : 'skipped', description: route.tls_mode === 'terminate_local' ? 'Caddy 本地终止 TLS' : 'HTTP 明文' },
    { name: '路由匹配', status: 'ok', description: `匹配路由 ${route.route_id}` },
    { name: '网关转发', status: chain.gateway?.status === 'active' ? 'ok' : 'failed', description: chain.gateway ? `${chain.gateway.name} (${chain.gateway.provider})` : '无网关', error: chain.gateway?.status !== 'active' ? chain.gateway?.last_error : undefined },
    { name: '服务解析', status: chain.service?.health_status === 'healthy' ? 'ok' : chain.service?.health_status === 'unhealthy' ? 'failed' : 'ok', description: chain.service?.name || '—', error: chain.service?.health_status === 'unhealthy' ? '服务健康检查失败' : undefined },
    ...chain.endpoints.map(ep => ({
      name: `端点 ${ep.node_name || ep.node_id}`,
      status: ep.health_status === 'healthy' ? 'ok' : 'failed',
      description: `${ep.target_local_host}:${ep.target_local_port} (${ep.address_type})`,
      error: ep.health_status !== 'healthy' ? '端点不可达' : undefined,
    })),
  ];

  const failedStep = steps.find(s => s.status === 'failed');
  return {
    input: domain,
    input_type: 'domain',
    trace_status: failedStep ? 'degraded' : 'ok',
    route_id: route.route_id,
    steps,
    summary: failedStep ? `失败于: ${failedStep.name}` : '全链路正常',
    anomalies: anomalies.map(a => a.title),
  };
}

export default function Trace() {
  const nav = useNavigate();
  const [domain, setDomain] = useState('');
  const [inputType, setInputType] = useState<'domain' | 'sni' | 'route'>('domain');
  const [routeId, setRouteId] = useState('');
  const [result, setResult] = useState<any>(null);
  const [loading, setLoading] = useState(false);

  const chainRouteId = result?.route_id;
  const { data: chain } = useChain('route', chainRouteId);

  const handleTrace = async () => {
    setLoading(true);
    setResult(null);
    try {
      if (API_CONFIG.useMock && inputType === 'domain') {
        await new Promise(r => setTimeout(r, 400));
        setResult(mockTrace(domain || 'api.proofnote.dev'));
      } else if (inputType === 'domain') {
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
      <PageHeader title="链路追踪" subtitle="追踪完整请求路径 · 自动定位失败层" />

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
        {/* Quick test domains */}
        {API_CONFIG.useMock && (
          <div className="flex gap-1.5 mt-2">
            <span className="text-[10px] text-a-muted">快捷:</span>
            {['api.proofnote.dev', 'auth.proofnote.dev', 'docs.proofnote.dev'].map(d => (
              <button key={d} onClick={() => { setDomain(d); setInputType('domain'); }}
                className="text-[10px] px-1.5 py-0.5 rounded bg-a-bg border border-a-border text-a-muted hover:text-a-fg cursor-pointer">{d}</button>
            ))}
          </div>
        )}
      </Card>

      {/* Error */}
      {result?.error && (
        <Card title="追踪失败">
          <div className="p-4 rounded bg-[#ff5c72]/10 border border-[#ff5c72]/20 text-[#ff5c72] text-xs">{result.error}</div>
        </Card>
      )}

      {/* Steps with failure highlight */}
      {result?.steps?.length > 0 && (
        <Card title="追踪步骤" subtitle={result.summary}>
          <div className="relative">
            {result.steps.map((step: any, i: number) => (
              <div key={i} className="flex items-stretch">
                {/* Step connector line */}
                <div className="flex flex-col items-center mr-3">
                  <div className={cn(
                    'w-2.5 h-2.5 rounded-full border-2 shrink-0',
                    step.status === 'ok' ? 'bg-[#4cd964] border-[#4cd964]' :
                    step.status === 'failed' ? 'bg-[#ff5c72] border-[#ff5c72] ring-2 ring-[#ff5c72]/30' :
                    step.status === 'skipped' ? 'bg-a-border border-a-border' :
                    'bg-a-bg border-a-border',
                  )} />
                  {i < result.steps.length - 1 && (
                    <div className={cn('w-0.5 flex-1 min-h-[20px]',
                      step.status === 'failed' ? 'bg-[#ff5c72]/30' :
                      step.status === 'ok' ? 'bg-[#4cd964]/30' :
                      'bg-a-border/50',
                    )} />
                  )}
                </div>
                {/* Step content */}
                <div className={cn(
                  'flex-1 pb-4',
                  step.status === 'failed' && 'pb-6',
                )}>
                  <div className={cn(
                    'p-3 rounded-a-sm border text-xs',
                    step.status === 'ok' ? 'bg-[#4cd964]/3 border-[#4cd964]/10' :
                    step.status === 'failed' ? 'bg-[#ff5c72]/5 border-[#ff5c72]/20' :
                    step.status === 'skipped' ? 'bg-a-bg/30 border-a-border/30 opacity-50' :
                    'bg-a-bg border-a-border',
                  )}>
                    <div className="flex items-center gap-2 mb-1">
                      <span className="text-[10px] text-a-muted font-mono">#{i + 1}</span>
                      <span className="font-semibold text-a-fg">{step.name}</span>
                      <StatusBadge status={step.status === 'ok' ? 'active' : step.status === 'failed' ? 'error' : step.status === 'skipped' ? 'disabled' : 'pending'} />
                    </div>
                    <p className="text-a-fg2">{step.description}</p>
                    {step.error && (
                      <div className="mt-2 p-2 rounded bg-[#ff5c72]/10 border border-[#ff5c72]/20 text-[#ff5c72] text-[11px] font-medium">
                        ✕ {step.error}
                      </div>
                    )}
                    {/* Action button on failed step */}
                    {step.status === 'failed' && chain && (
                      <button onClick={() => chain.entryPoint && nav(`/exposure/entry/${chain.entryPoint.route_id}`)}
                        className="mt-2 text-[10px] text-a-accent hover:underline cursor-pointer">
                        查看详情 →
                      </button>
                    )}
                  </div>
                </div>
              </div>
            ))}
          </div>
        </Card>
      )}

      {/* Chain visualization overlay */}
      {chain && result?.steps?.length > 0 && (
        <Card title="完整链路">
          <PathRibbon chain={chain} focusType="route" focusId={chainRouteId} />
        </Card>
      )}

      {/* Anomalies from mock */}
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

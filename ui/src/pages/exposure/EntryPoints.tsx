// ─── Entry Points ───
// Core page: each row shows the full chain flow, not isolated columns.
// domain → listener → gateway → service → endpoints → [health] [release]

import { useQuery } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { fetchRoutes } from '@/lib/api-bridge';
import { PageHeader, HealthDot, StatusBadge } from '@/components/shared';
import { getScenario } from '@/mocks';
import { API_CONFIG } from '@/lib/api-config';
import { cn } from '@/lib/utils';
import type { EntryPointSummary } from '@/types/workspace';

function ChainRow({ ep }: { ep: EntryPointSummary }) {
  const nav = useNavigate();
  const failedEndpoints = ep.endpoints.filter(e => e.health === 'unhealthy');
  const hasFailure = ep.health === 'degraded' || ep.health === 'failed' || failedEndpoints.length > 0;
  const isPending = ep.release_state === 'pending' || ep.release_state === 'drifted';

  return (
    <div
      onClick={() => nav(`/exposure/entry/${ep.route_id}`)}
      className={cn(
        'flex items-center gap-1.5 px-4 py-3 cursor-pointer transition-colors text-xs',
        'border-b border-a-border/40 hover:bg-a-border/10',
        hasFailure && 'bg-[#ff5c72]/3 border-l-2 border-l-[#ff5c72]',
        isPending && !hasFailure && 'bg-[#e8b830]/3 border-l-2 border-l-[#e8b830]',
      )}
    >
      {/* Domain */}
      <span className="font-mono font-semibold text-a-fg w-40 shrink-0 truncate">{ep.domain}</span>

      {/* Arrow */}
      <svg className="w-3 h-3 text-a-border shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M9 18l6-6-6-6"/></svg>

      {/* Listener */}
      <span className="text-a-muted shrink-0 w-16 text-center">
        {ep.listener ? <span className="font-mono">:{ep.listener.port}<span className="text-[10px]">/{ep.listener.provider}</span></span> : <span className="text-a-border">—</span>}
      </span>

      <svg className="w-3 h-3 text-a-border shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M9 18l6-6-6-6"/></svg>

      {/* Gateway */}
      <span className="shrink-0 w-20 text-center">
        {ep.gateway_name ? (
          <span className="flex items-center justify-center gap-1">
            <HealthDot status={ep.health === 'healthy' ? 'healthy' : 'degraded'} size="sm" />
            <span className="text-a-fg2 truncate">{ep.gateway_name}</span>
          </span>
        ) : <span className="text-a-border">—</span>}
      </span>

      <svg className="w-3 h-3 text-a-border shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M9 18l6-6-6-6"/></svg>

      {/* Service */}
      <span className="font-medium text-a-fg w-28 shrink-0 truncate">{ep.service_name}</span>

      <svg className="w-3 h-3 text-a-border shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M9 18l6-6-6-6"/></svg>

      {/* Endpoints with health indicators */}
      <span className="flex-1 flex items-center gap-1.5 min-w-0">
        {ep.endpoints.length === 0 ? (
          <span className="text-a-border">—</span>
        ) : (
          ep.endpoints.map(e => (
            <span key={e.endpoint_id} className={cn(
              'inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px]',
              e.health === 'healthy' ? 'bg-[#4cd964]/10 text-[#4cd964]' :
              e.health === 'unhealthy' ? 'bg-[#ff5c72]/10 text-[#ff5c72]' :
              'bg-a-border/30 text-a-muted',
            )}>
              <HealthDot status={e.health === 'healthy' ? 'healthy' : e.health === 'unhealthy' ? 'failed' : 'unknown'} size="sm" />
              {e.node_name}
            </span>
          ))
        )}
      </span>

      {/* Status badges */}
      <span className="flex items-center gap-1.5 shrink-0">
        <StatusBadge status={ep.health} />
        <StatusBadge status={ep.release_state} />
        {ep.safety !== 'unknown' && ep.safety !== 'safe' && <StatusBadge status={ep.safety} />}
      </span>
    </div>
  );
}

export default function EntryPoints() {
  const { data: routesData } = useQuery({
    queryKey: ['routes'],
    queryFn: fetchRoutes,
  });

  const entryPoints: EntryPointSummary[] = API_CONFIG.useMock
    ? getScenario().entryPoints
    : ((routesData as any)?.routes || []).map((r: any) => ({
        route_id: r.route_id, domain: r.domain,
        protocol: 'http', tls_mode: r.tls_mode,
        listener: null, gateway_id: null, gateway_name: null,
        service_id: r.service_id, service_name: r.service_name,
        endpoints: [], health: 'unknown', safety: 'unknown', release_state: 'current',
      }));

  const degraded = entryPoints.filter(e => e.health === 'degraded' || e.health === 'failed').length;
  const pending = entryPoints.filter(e => e.release_state === 'pending' || e.release_state === 'drifted').length;

  return (
    <div className="flex flex-col h-full">
      <PageHeader
        title="入口总览"
        subtitle={`${entryPoints.length} 个入口${degraded > 0 ? ` · ${degraded} 异常` : ''}${pending > 0 ? ` · ${pending} 待发布` : ''}`}
        className="px-6 pt-6 pb-2"
      />

      {/* Column guide */}
      <div className="flex items-center gap-1.5 px-4 py-1.5 text-[10px] text-a-muted border-b border-a-border/30 shrink-0">
        <span className="w-40 shrink-0">域名</span>
        <span className="w-3 shrink-0" />
        <span className="w-16 shrink-0 text-center">监听器</span>
        <span className="w-3 shrink-0" />
        <span className="w-20 shrink-0 text-center">网关</span>
        <span className="w-3 shrink-0" />
        <span className="w-28 shrink-0">服务</span>
        <span className="w-3 shrink-0" />
        <span className="flex-1">端点</span>
        <span className="shrink-0 w-28 text-right">状态</span>
      </div>

      {/* Rows */}
      <div className="flex-1 overflow-y-auto">
        {entryPoints.length === 0 ? (
          <div className="text-center py-16 text-a-muted text-sm">暂无入口点 · 去 <a href="/exposure/connect" className="text-a-accent hover:underline">快速接入</a></div>
        ) : (
          entryPoints.map(ep => <ChainRow key={ep.route_id} ep={ep} />)
        )}
      </div>
    </div>
  );
}

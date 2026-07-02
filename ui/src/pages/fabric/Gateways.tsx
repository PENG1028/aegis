// ─── Gateways ───
import { useQuery } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { fetchGateways, fetchRoutes } from '@/lib/api-bridge';
import { PageHeader, HealthDot, StatusBadge } from '@/components/shared';
import { getScenario } from '@/mocks';
import { API_CONFIG } from '@/lib/api-config';
import { cn } from '@/lib/utils';

export default function Gateways() {
  const nav = useNavigate();
  const { data } = useQuery({ queryKey: ['gateways'], queryFn: fetchGateways });
  const { data: routesData } = useQuery({ queryKey: ['routes'], queryFn: fetchRoutes });
  const gws = Array.isArray(data) ? data : [];
  const routes = (routesData as any)?.routes || [];
  const scenario = API_CONFIG.useMock ? getScenario() : null;

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="网关列表" subtitle={`${gws.length} 个网关 · 关联路由与服务`} />

      <div className="space-y-2">
        {gws.map((g: any) => {
          // Find routes using this gateway
          const gwRoutes = API_CONFIG.useMock
            ? scenario!.routes.filter(r => r.gateway_policy?.primary_gateway_id === g.gateway_id)
            : routes.filter((r: any) => r.gateway_policy?.primary_gateway_id === g.gateway_id);

          const hasError = g.status === 'error';

          return (
            <div key={g.gateway_id}
              onClick={() => nav(`/fabric/gateway/${g.gateway_id}`)}
              className={cn(
                'p-4 rounded-a-md border cursor-pointer transition-colors hover:brightness-105',
                hasError ? 'bg-[#ff5c72]/3 border-[#ff5c72]/20 border-l-2 border-l-[#ff5c72]' :
                g.status === 'active' ? 'bg-a-surface border-a-border' :
                'bg-a-surface/50 border-a-border/50',
              )}>
              <div className="flex items-center gap-4">
                {/* Gateway identity */}
                <div className="w-48 shrink-0">
                  <div className="flex items-center gap-2 mb-1">
                    <HealthDot status={g.status === 'active' ? 'healthy' : g.status === 'error' ? 'failed' : 'unknown'} />
                    <span className="text-sm font-semibold text-a-fg">{g.name}</span>
                    <StatusBadge status={g.status} />
                  </div>
                  <div className="text-[10px] text-a-muted font-mono">
                    {g.provider} · {g.bind_addr}:{g.port} · {g.scheme}
                  </div>
                </div>

                {/* Arrow */}
                <svg className="w-4 h-4 text-a-border shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M9 18l6-6-6-6"/></svg>

                {/* Node */}
                <div className="w-32 shrink-0">
                  <div className="text-xs text-a-fg2">{g.node_name || g.node_id}</div>
                  <div className="text-[10px] text-a-muted">节点</div>
                </div>

                <svg className="w-4 h-4 text-a-border shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M9 18l6-6-6-6"/></svg>

                {/* Routes served */}
                <div className="flex-1 min-w-0">
                  {gwRoutes.length > 0 ? (
                    <div className="flex items-center gap-1.5 flex-wrap">
                      {gwRoutes.map((r: any) => (
                        <span key={r.route_id} className="inline-flex items-center gap-1 px-2 py-0.5 rounded text-[10px] bg-a-accent/10 text-a-accent font-mono">
                          {r.domain}
                        </span>
                      ))}
                    </div>
                  ) : (
                    <span className="text-[10px] text-a-muted">无关联路由</span>
                  )}
                </div>

                {/* Counts */}
                <div className="flex items-center gap-3 text-[10px] text-a-muted shrink-0">
                  <span>{gwRoutes.length} 路由</span>
                  {g.last_error && <span className="text-[#ff5c72] truncate max-w-[160px]" title={g.last_error}>{g.last_error}</span>}
                </div>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

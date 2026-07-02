// ─── Listeners ───
import { useQuery } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { fetchListeners, fetchRoutes } from '@/lib/api-bridge';
import { PageHeader, HealthDot, StatusBadge } from '@/components/shared';
import { getScenario } from '@/mocks';
import { API_CONFIG } from '@/lib/api-config';

export default function Listeners() {
  const nav = useNavigate();
  const { data } = useQuery({ queryKey: ['listeners'], queryFn: fetchListeners });
  const { data: routesData } = useQuery({ queryKey: ['routes'], queryFn: fetchRoutes });
  const listeners = API_CONFIG.useMock ? getScenario().listeners : (data || []);
  const routes = (routesData as any)?.routes || [];
  const scenario = API_CONFIG.useMock ? getScenario() : null;

  // Resolve node name from scenario nodes
  const nodeName = (nodeId: string) => {
    if (!scenario) return nodeId;
    const n = scenario.nodes.find(n => n.node_id === nodeId);
    return n?.name || nodeId;
  };

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="监听器列表" subtitle={`${listeners.length} 个监听端口`} />

      <div className="space-y-2">
        {listeners.map((l: any, i: number) => {
          const viaRoutes = scenario
            ? scenario.routes.filter(r => r.gateway_policy?.primary_gateway_id === l.gateway_id)
            : [];

          return (
            <div key={i}
              onClick={() => l.gateway_id && nav(`/fabric/gateway/${l.gateway_id}`)}
              className="p-4 rounded-a-md border bg-a-surface border-a-border cursor-pointer hover:brightness-105 transition-colors flex items-center gap-4">
              {/* Listener identity */}
              <div className="w-40 shrink-0">
                <div className="flex items-center gap-2 mb-1">
                  <HealthDot status={l.status === 'active' ? 'healthy' : 'unknown'} />
                  <span className="font-mono font-semibold text-a-fg">{l.bind_addr}:{l.port}</span>
                </div>
                <div className="text-[10px] text-a-muted">{l.provider} · {l.purpose}</div>
              </div>

              <svg className="w-4 h-4 text-a-border shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M9 18l6-6-6-6"/></svg>

              {/* Gateway name resolved */}
              <div className="w-28 shrink-0">
                <div className="text-xs font-mono text-a-fg2">{l.gateway_id}</div>
                <div className="text-[10px] text-a-muted">网关</div>
              </div>

              <svg className="w-4 h-4 text-a-border shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M9 18l6-6-6-6"/></svg>

              {/* Node name resolved */}
              <div className="w-24 shrink-0">
                <div className="text-xs text-a-fg2">{nodeName(l.node_id)}</div>
                <div className="text-[10px] text-a-muted">节点</div>
              </div>

              <svg className="w-4 h-4 text-a-border shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M9 18l6-6-6-6"/></svg>

              {/* Routes */}
              <div className="flex-1 min-w-0">
                {viaRoutes.length > 0 ? (
                  <div className="flex items-center gap-1.5 flex-wrap">
                    {viaRoutes.map((r: any) => (
                      <span key={r.route_id} className="inline-flex items-center gap-1 px-2 py-0.5 rounded text-[10px] bg-a-accent/10 text-a-accent font-mono">
                        {r.domain}
                      </span>
                    ))}
                  </div>
                ) : (
                  <span className="text-[10px] text-a-muted">—</span>
                )}
              </div>

              <StatusBadge status={l.status} />
            </div>
          );
        })}
      </div>
    </div>
  );
}

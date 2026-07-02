// ─── Routing Table ───
import { useQuery } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { PageHeader, StatusBadge, HealthDot } from '@/components/shared';
import { getScenario } from '@/mocks';
import { API_CONFIG } from '@/lib/api-config';
import { nodeApi } from '@/lib/api-bridge';
import { useState } from 'react';
import { cn } from '@/lib/utils';

export default function RoutingTable() {
  const nav = useNavigate();
  const [selectedNode, setSelectedNode] = useState<string>('node-a');
  const nodes = API_CONFIG.useMock ? getScenario().nodes : [];

  const { data } = useQuery({
    queryKey: ['routing-table', selectedNode],
    queryFn: () => nodeApi.routingTable(selectedNode),
    enabled: !API_CONFIG.useMock,
  });

  // In mock mode, build routing entries from scenario data
  const entries = API_CONFIG.useMock
    ? getScenario().routes.map(r => ({
        domain: r.domain,
        route_id: r.route_id,
        service_id: r.service_id,
        gateway: r.gateway_policy?.primary_gateway_id || '—',
        status: r.status === 'active' ? 'available' : 'unavailable',
      }))
    : ((data as any)?.entries || []);

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="路由表" subtitle="节点路由表 · 域 → 网关 → 服务" />

      {/* Node selector */}
      {API_CONFIG.useMock && (
        <div className="flex gap-1.5">
          {nodes.map(n => (
            <button key={n.node_id} onClick={() => setSelectedNode(n.node_id)}
              className={cn('px-3 py-1.5 text-xs rounded cursor-pointer transition-colors',
                selectedNode === n.node_id ? 'bg-a-accent/20 text-a-accent font-medium' : 'bg-a-bg text-a-muted hover:text-a-fg border border-a-border')}>
              {n.name}
            </button>
          ))}
        </div>
      )}

      <div className="space-y-2">
        {entries.map((e: any, i: number) => (
          <div key={i}
            onClick={() => nav(`/exposure/entry/${e.route_id}`)}
            className="p-3 rounded-a-md border bg-a-surface border-a-border cursor-pointer hover:brightness-105 flex items-center gap-4 text-xs">
            <span className="font-mono font-semibold text-a-fg w-44 shrink-0">{e.domain}</span>
            <svg className="w-3 h-3 text-a-border shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M9 18l6-6-6-6"/></svg>
            <span className="font-mono text-a-fg2 w-28 shrink-0">{e.gateway}</span>
            <svg className="w-3 h-3 text-a-border shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M9 18l6-6-6-6"/></svg>
            <span className="font-mono text-a-fg2 flex-1">{e.service_id}</span>
            <StatusBadge status={e.status} />
          </div>
        ))}
      </div>
    </div>
  );
}

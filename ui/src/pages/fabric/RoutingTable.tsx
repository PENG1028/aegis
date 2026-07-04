// ─── Routing Table ───
// Per-node routing table. Mock: built from null. Production: real API.
import { useQuery } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { useState } from 'react';
import { PageHeader, StatusBadge, Card } from '@/components/shared';
import { fetchNodes } from '@/lib/api-bridge';
import { nodeApi } from '@/lib/api-bridge';

import { cn } from '@/lib/utils';

export default function RoutingTable() {
  const nav = useNavigate();
  const [selectedNode, setSelectedNode] = useState<string>('');
  

  // Nodes (both modes)
  const { data: nodesData } = useQuery({
    queryKey: ['routing-nodes'],
    queryFn: fetchNodes,
    refetchInterval: 120_000,
  });
  const nodes = (Array.isArray(nodesData) ? nodesData : []);

  // Set default node on first load
  if (!selectedNode && nodes.length > 0) {
    setSelectedNode(nodes[0].node_id);
  }

  // Routing table entries
  const { data: rtData, isLoading } = useQuery({
    queryKey: ['routing-table', selectedNode],
    queryFn: () => nodeApi.routingTable(selectedNode),
    enabled: !!selectedNode,
    refetchInterval: 60_000,
  });

  // Build entries from real routing table data
  const entries: any[] = (() => {
    if (Array.isArray(rtData)) return rtData;
    return (rtData as any)?.entries || (rtData as any)?.routes || [];
  })();

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="路由表" subtitle="按节点查看路由表 · 域 → 网关 → 服务" />

      {/* Node selector */}
      <div className="flex items-center gap-1.5 flex-wrap">
        {nodes.map(n => (
          <button key={n.node_id} onClick={() => setSelectedNode(n.node_id)}
            className={cn('px-3 py-1.5 text-xs rounded cursor-pointer transition-colors',
              selectedNode === n.node_id ? 'bg-a-accent/20 text-a-accent font-medium' : 'bg-a-bg text-a-muted hover:text-a-fg border border-a-border')}>
            {n.name || n.hostname || n.node_id}
          </button>
        ))}
        {nodes.length === 0 && (
          <span className="text-xs text-a-muted">无节点数据</span>
        )}
      </div>

      {/* Entries */}
      {isLoading ? (
        <div className="text-center py-12 text-a-muted text-sm">加载路由表...</div>
      ) : entries.length === 0 ? (
        <Card title="路由条目">
          <div className="text-center py-8 text-xs text-a-muted">
            {selectedNode ? '该节点无路由条目' : '请选择节点查看路由表'}
          </div>
        </Card>
      ) : (
        <div className="space-y-2">
          {entries.map((e: any, i: number) => (
            <div key={i}
              onClick={() => e.route_id && nav(`/exposure/entry/${e.route_id}`)}
              className="p-3 rounded-a-md border bg-a-surface border-a-border cursor-pointer hover:brightness-105 flex items-center gap-4 text-xs">
              <span className="font-mono font-semibold text-a-fg w-44 shrink-0">{e.domain}</span>
              <svg className="w-3 h-3 text-a-border shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M9 18l6-6-6-6"/></svg>
              <span className="font-mono text-a-fg2 w-28 shrink-0">{e.gateway || e.gateway_id || e.target_gateway || '—'}</span>
              <svg className="w-3 h-3 text-a-border shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M9 18l6-6-6-6"/></svg>
              <span className="font-mono text-a-fg2 flex-1">{e.service_id || e.service || '—'}</span>
              <StatusBadge status={e.status || 'available'} />
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

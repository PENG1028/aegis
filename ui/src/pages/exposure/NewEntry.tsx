// ─── Service Entry — unified inbound gateway binding ───
import { useState, useMemo } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { runtimeModeApi, routeApi, exposureApi, nodeApi } from '@/lib/api-bridge';
import type { Composition } from '@/lib/api-bridge';
import { Card, StatusBadge, Btn, useToast } from '@/components/shared';
import { cn } from '@/lib/utils';

// ═══════════════════════════════════════════════════════
// Node helpers
// ═══════════════════════════════════════════════════════

interface NodeInfo {
  id: string; name: string;
  privateIP: string; publicIP: string;
  networkID: string; region: string;
}

function deriveLabel(n: NodeInfo, allNodes: NodeInfo[]): { label: string; color: string; forced: boolean } {
  // Use explicit NetworkID if set
  if (n.networkID) return { label: n.networkID, color: 'bg-blue-500/10 text-blue-400 border-blue-500/20', forced: false };

  // Detect duplicate private IPs → force a label to disambiguate
  const sameIP = allNodes.filter(o => o.id !== n.id && o.privateIP && o.privateIP === n.privateIP);
  if (sameIP.length > 0 && n.privateIP) {
    return { label: n.privateIP, color: 'bg-[#e8b830]/10 text-[#e8b830] border-[#e8b830]/20', forced: true };
  }

  // Derive from private IP prefix
  if (n.privateIP) {
    const parts = n.privateIP.split('.');
    if (parts.length === 4) {
      return { label: `${parts[0]}.${parts[1]}.x.x`, color: 'bg-a-border/10 text-a-muted border-a-border/20', forced: false };
    }
  }

  return { label: n.region || '', color: 'bg-a-border/10 text-a-muted border-a-border/20', forced: false };
}

// ═══════════════════════════════════════════════════════
// Composition card styles
// ═══════════════════════════════════════════════════════

function entryType(c: Composition) { return c.name.includes('UDP') ? 'udp' : c.name.includes('HTTP') ? 'http' : 'tcp'; }

const COMP_CARD: Record<string, string> = {
  available:        'bg-a-surface border-[#4cd964]/40 hover:bg-[#4cd964]/5 text-a-fg',
  missing_provider: 'bg-[#ff5c72]/5 border-[#ff5c72]/30 text-[#ff5c72]',
  unsupported:      'bg-a-border/10 border-a-border/20 opacity-40 text-a-muted/50',
};

// ═══════════════════════════════════════════════════════
// Step 1: Composition selector
// ═══════════════════════════════════════════════════════

function CompSelector({ compositions, selected, onSelect }: {
  compositions: Composition[]; selected: string; onSelect: (n: string) => void;
}) {
  return (
    <div>
      <div className="text-xs font-medium text-a-fg mb-3">1. 选择服务类型</div>
      <div className="flex gap-2 flex-wrap">
        {compositions.map(comp => {
          const ok = comp.status === 'available';
          return (
            <button key={comp.name} disabled={!ok} onClick={() => ok && onSelect(comp.name)}
              className={cn('px-3 py-2.5 rounded-a-sm text-xs border transition-colors min-w-[130px]',
                COMP_CARD[comp.status] || COMP_CARD.unsupported,
                selected === comp.name && 'ring-2 ring-a-accent/50', !ok && 'cursor-not-allowed')}>
              <div className="font-medium">{comp.name}</div>
              <div className="text-[10px] opacity-70 mt-0.5">{comp.chain}</div>
              {comp.status === 'missing_provider' && <div className="text-[9px] text-[#ff5c72]/70 mt-1">需安装中间件</div>}
            </button>
          );
        })}
      </div>
    </div>
  );
}

// ═══════════════════════════════════════════════════════
// Step 2: Config form
// ═══════════════════════════════════════════════════════

function EntryForm({ comp, nodes, onSubmit, loading }: {
  comp: Composition; nodes: NodeInfo[]; onSubmit: (f: any) => void; loading: boolean;
}) {
  const isHTTP = entryType(comp) === 'http';
  const [domain, setDomain] = useState('');
  const [internalOnly, setInternalOnly] = useState(false);
  const [nodeId, setNodeId] = useState('');
  const [targetHost, setTargetHost] = useState('127.0.0.1');
  const [targetPort, setTargetPort] = useState(3000);
  const [showPreview, setShowPreview] = useState(false);

  const selectedNode = nodes.find(n => n.id === nodeId);
  const nodeLabels = useMemo(() => {
    const m: Record<string, ReturnType<typeof deriveLabel>> = {};
    nodes.forEach(n => { m[n.id] = deriveLabel(n, nodes); });
    return m;
  }, [nodes]);

  const canSubmit = isHTTP ? (domain && targetHost && targetPort > 0) : (targetHost && targetPort > 0);

  return (
    <div className="space-y-5 p-5 rounded-a-md border border-a-border/30 bg-a-surface/50">
      <div className="text-xs font-medium text-a-fg">2. 配置 — {comp.name}</div>

      {/* Node picker */}
      <div>
        <label className="text-[10px] text-a-muted block mb-1.5 font-medium">目标节点</label>
        <select value={nodeId} onChange={e => { setNodeId(e.target.value); if (e.target.value) { const n = nodes.find(x => x.id === e.target.value); if (n) setTargetHost(n.privateIP || n.publicIP || '127.0.0.1'); } }}
          className="w-full px-3 py-2 rounded-a-sm border border-a-border/50 bg-a-bg text-xs outline-none focus:border-a-accent/50">
          <option value="">本节点 (当前)</option>
          {nodes.map(n => {
            const lb = nodeLabels[n.id];
            return <option key={n.id} value={n.id}>
              {n.name} · {n.privateIP || '—'}{n.publicIP ? ` · ${n.publicIP}` : ''}{lb.label ? ` · ${lb.label}` : ''}
            </option>;
          })}
        </select>
        {selectedNode && (
          <div className="flex items-center gap-2 mt-1.5">
            {selectedNode.privateIP && (
              <span className="px-1.5 py-0.5 rounded text-[9px] font-mono bg-blue-500/10 text-blue-400 border border-blue-500/20">
                内网 {selectedNode.privateIP}
              </span>
            )}
            {selectedNode.publicIP && (
              <span className="px-1.5 py-0.5 rounded text-[9px] font-mono bg-a-border/10 text-a-muted border border-a-border/20">
                公网 {selectedNode.publicIP}
              </span>
            )}
            {nodeLabels[selectedNode.id]?.label && (
              <span className={cn('px-1.5 py-0.5 rounded text-[9px] font-medium border', nodeLabels[selectedNode.id].color)}>
                {nodeLabels[selectedNode.id].forced ? '⚠ 重复IP: ' : ''}{nodeLabels[selectedNode.id].label}
              </span>
            )}
          </div>
        )}
      </div>

      {/* Domain (HTTP only) */}
      {isHTTP && (
        <div>
          <label className="text-[10px] text-a-muted block mb-1.5 font-medium">域名</label>
          <input value={domain} onChange={e => setDomain(e.target.value)} placeholder="api.example.com"
            className="w-full px-3 py-2 rounded-a-sm border border-a-border/50 bg-a-bg text-xs outline-none focus:border-a-accent/50" />
          <label className="flex items-center gap-1.5 mt-1.5 text-[10px] text-a-muted cursor-pointer">
            <input type="checkbox" checked={internalOnly} onChange={e => setInternalOnly(e.target.checked)} className="w-3 h-3" />
            仅内部使用（集群内服务间调用，不对外暴露 DNS）
          </label>
        </div>
      )}

      {/* Backend address */}
      <div className="grid grid-cols-2 gap-4">
        <div>
          <label className="text-[10px] text-a-muted block mb-1.5 font-medium">后端地址</label>
          <input value={targetHost} onChange={e => setTargetHost(e.target.value)}
            placeholder={selectedNode?.privateIP || '127.0.0.1'}
            className="w-full px-3 py-2 rounded-a-sm border border-a-border/50 bg-a-bg text-xs outline-none focus:border-a-accent/50 font-mono" />
          <div className="text-[9px] text-a-muted/50 mt-0.5">服务实际监听的 IP。本机用 127.0.0.1，远程用内网 IP</div>
        </div>
        <div>
          <label className="text-[10px] text-a-muted block mb-1.5 font-medium">后端端口</label>
          <input type="number" value={targetPort} onChange={e => setTargetPort(Number(e.target.value))}
            className="w-full px-3 py-2 rounded-a-sm border border-a-border/50 bg-a-bg text-xs outline-none focus:border-a-accent/50 font-mono" />
        </div>
      </div>

      {/* Preview + Submit */}
      <div className="flex gap-2">
        <Btn onClick={() => setShowPreview(!showPreview)} className="text-[10px]">{showPreview ? '收起' : '预览效果'}</Btn>
        <Btn primary disabled={!canSubmit || loading}
          onClick={() => onSubmit({ composition: comp.name, domain, port: 0, targetHost, targetPort, nodeId, internalOnly })}>
          {loading ? '创建中...' : '创建并 Apply'}
        </Btn>
      </div>

      {showPreview && canSubmit && (
        <div className="p-3 rounded-a-sm bg-a-bg border border-a-border/30 text-[10px] font-mono space-y-1">
          <div className="text-a-muted">流量路径预览</div>
          <div className="text-a-fg2">
            {`客户端 → ${domain || `:0`} → ${comp.name} → ${targetHost}:${targetPort}`}
          </div>
          {selectedNode && (
            <div className="text-a-muted/70">
              {selectedNode.id ? `目标节点: ${selectedNode.name} (${selectedNode.privateIP || selectedNode.publicIP})` : '本节点'}
              {selectedNode.id && ' — 跨节点走 Gateway Link 认证转发'}
            </div>
          )}
          {internalOnly && <div className="text-[#e8b830]/80">内部域名 — 不对外暴露 DNS，仅集群内解析</div>}
        </div>
      )}
    </div>
  );
}

// ═══════════════════════════════════════════════════════
// Entry list
// ═══════════════════════════════════════════════════════

function EntryList({ routes, exposures, isLoading }: { routes: any[]; exposures: any[]; isLoading: boolean }) {
  const qc = useQueryClient(); const toast = useToast();
  const items = [
    ...routes.map((r: any) => ({ ...r, _t: 'route' as const })),
    ...exposures.map((e: any) => ({ ...e, _t: 'exposure' as const })),
  ];
  if (isLoading) return <div className="text-sm text-a-muted py-6 text-center">加载中...</div>;
  if (items.length === 0) return <div className="text-center py-10 text-a-muted text-sm">暂无入口 — 选择组合能力开始创建</div>;
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-xs">
        <thead><tr className="border-b border-a-border text-a-muted text-left">
          <th className="py-2 px-3 font-medium">入口</th><th className="py-2 px-3 font-medium">类型</th>
          <th className="py-2 px-3 font-medium">后端</th><th className="py-2 px-3 font-medium">状态</th>
        </tr></thead>
        <tbody>{items.map((item: any, i: number) => (
          <tr key={i} className="border-b border-a-border/30 hover:bg-a-border/5">
            <td className="py-2 px-3 font-mono text-[11px]">{item._t === 'route' ? item.domain : `:${item.port || item.entry_port || '?'}`}</td>
            <td className="py-2 px-3 text-[10px] text-a-muted">{item._t === 'route' ? 'HTTP/HTTPS' : (item.type || 'TCP/UDP').toUpperCase()}</td>
            <td className="py-2 px-3 font-mono text-[11px] text-a-muted">{item.target || item.target_host || '—'}{item.target_port ? `:${item.target_port}` : ''}</td>
            <td className="py-2 px-3"><StatusBadge status={item.status === 'active' ? 'active' : 'disabled'} /></td>
          </tr>
        ))}</tbody>
      </table>
    </div>
  );
}

// ═══════════════════════════════════════════════════════
// Main
// ═══════════════════════════════════════════════════════

export default function ServiceEntry() {
  const toast = useToast(); const qc = useQueryClient();
  const [selectedComp, setSelectedComp] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const { data: rm } = useQuery({ queryKey: ['runtime-mode'], queryFn: () => runtimeModeApi.get(), refetchInterval: 60_000 });
  const { data: rd, isLoading: rl } = useQuery({ queryKey: ['routes'], queryFn: () => routeApi.list().catch(() => ({ routes: [] })), refetchInterval: 30_000 });
  const { data: ed, isLoading: el } = useQuery({ queryKey: ['exposures'], queryFn: () => exposureApi.list().catch(() => ({ exposures: [] })), refetchInterval: 30_000 });
  const { data: nd } = useQuery({ queryKey: ['nodes'], queryFn: () => nodeApi.list().catch(() => ({ nodes: [] })), refetchInterval: 120_000 });

  const compositions = rm?.current?.compositions || [];
  const routes = (rd as any)?.routes || [];
  const exposures = (ed as any)?.exposures || [];
  const nodes: NodeInfo[] = ((nd as any)?.nodes || []).map((n: any) => ({
    id: n.id || n.node_id, name: n.name || n.node_id || n.id,
    privateIP: n.private_ip || '', publicIP: n.public_ip || '',
    networkID: n.network_id || '', region: n.region || '',
  }));

  const handleSubmit = async (form: any) => {
    setSubmitting(true);
    try {
      const type = entryType(compositions.find(c => c.name === form.composition)!);
      if (type === 'http') {
        const res = await fetch('/api/v1/actions/bind-http-domain', {
          method: 'POST', headers: { 'Content-Type': 'application/json' }, credentials: 'include',
          body: JSON.stringify({ domain: form.domain, target_host: form.targetHost, target_port: form.targetPort }),
        });
        if (!res.ok) { const e = await res.json().catch(() => ({})); throw new Error((e as any).error?.message || `HTTP ${res.status}`); }
      } else {
        await exposureApi.create({ type, target_host: form.targetHost, target_port: form.targetPort });
      }
      toast('入口创建成功，配置已自动 Apply');
      setSelectedComp('');
      qc.invalidateQueries({ queryKey: ['routes'] }); qc.invalidateQueries({ queryKey: ['exposures'] });
    } catch (e: any) { toast(e.message || '创建失败', 'error'); }
    finally { setSubmitting(false); }
  };

  const selected = compositions.find(c => c.name === selectedComp);

  return (
    <div className="space-y-5">
      <CompSelector compositions={compositions} selected={selectedComp} onSelect={setSelectedComp} />
      {selected && <EntryForm comp={selected} nodes={nodes} onSubmit={handleSubmit} loading={submitting} />}
      <Card title={`已有入口 (${routes.length + exposures.length})`}>
        <EntryList routes={routes} exposures={exposures} isLoading={rl || el} />
      </Card>
    </div>
  );
}

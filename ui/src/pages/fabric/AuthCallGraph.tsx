// ─── Service Auth Topology (v1.9A) — 服务调用拓扑 ───
// SVG force-directed graph of inter-service call relationships.
// No external D3 dependency — pure React + SVG simulation.

import { useState, useMemo, useRef } from 'react';
import { useQuery } from '@tanstack/react-query';
import { adminApi } from '@/lib/api-bridge';
import { PageHeader, Drawer, Card, Btn, useToast, HealthDot, LoadingState, ErrorBanner, EmptyState } from '@/components/shared';
import { cn } from '@/lib/utils';

// ─── Types ───

interface TopoNode {
  name: string;
  host: string;
  port: number;
  node_host: string;
  status: string;
}

interface TopoEdge {
  caller: string;
  target: string;
  api: string;
  count: number;
  last_seen: string;
}

interface SimNode extends TopoNode {
  x: number;
  y: number;
  vx: number;
  vy: number;
  isCrossNode: boolean;
}

interface SimEdge {
  caller: string;
  target: string;
  api: string;
  count: number;
  last_seen: string;
  sourceIdx: number;
  targetIdx: number;
  thickness: number;
}

// ─── Constants ───

const W = 900;
const H = 560;
const NODE_R = 48;
const CHARGE = -2800;
const DAMPING = 0.85;
const MIN_V = 0.5;
const MAX_SPEED = 4;
const COLORS: Record<string, string> = {
  active: '#4cd964',
  blocked: '#ff5c72',
};

// ─── Force simulation (pure function, mutates nodes in place) ───

function runSimulation(nodes: SimNode[], edges: SimEdge[], iterations = 80): void {
  const cx = W / 2;
  const cy = H / 2;

  const angleStep = (2 * Math.PI) / nodes.length;
  const radius = Math.min(W, H) * 0.3;
  nodes.forEach((n, i) => {
    n.x = cx + radius * Math.cos(angleStep * i - Math.PI / 2);
    n.y = cy + radius * Math.sin(angleStep * i - Math.PI / 2);
    n.vx = 0;
    n.vy = 0;
  });

  for (let iter = 0; iter < iterations; iter++) {
    const alpha = 1 - iter / iterations;

    for (let i = 0; i < nodes.length; i++) {
      for (let j = i + 1; j < nodes.length; j++) {
        const a = nodes[i];
        const b = nodes[j];
        let dx = b.x - a.x;
        let dy = b.y - a.y;
        const dist = Math.max(Math.sqrt(dx * dx + dy * dy), 10);
        const force = (CHARGE * alpha) / (dist * dist);
        dx /= dist;
        dy /= dist;
        a.vx -= dx * force;
        a.vy -= dy * force;
        b.vx += dx * force;
        b.vy += dy * force;
      }
    }

    for (const e of edges) {
      const a = nodes[e.sourceIdx];
      const b = nodes[e.targetIdx];
      let dx = b.x - a.x;
      let dy = b.y - a.y;
      const dist = Math.max(Math.sqrt(dx * dx + dy * dy), 1);
      const ideal = NODE_R * 3;
      const force = (dist - ideal) * 0.1 * alpha;
      dx /= dist;
      dy /= dist;
      a.vx += dx * force;
      a.vy += dy * force;
      b.vx -= dx * force;
      b.vy -= dy * force;
    }

    for (const n of nodes) {
      n.vx += (cx - n.x) * 0.01 * alpha;
      n.vy += (cy - n.y) * 0.01 * alpha;
    }

    for (const n of nodes) {
      n.vx *= DAMPING;
      n.vy *= DAMPING;
      const spd = Math.sqrt(n.vx * n.vx + n.vy * n.vy);
      if (spd > MAX_SPEED) {
        n.vx = (n.vx / spd) * MAX_SPEED;
        n.vy = (n.vy / spd) * MAX_SPEED;
      }
      if (spd > MIN_V) {
        n.x += n.vx;
        n.y += n.vy;
      }
    }
  }
}

// ─── SVG helpers ───

function edgePath(a: { x: number; y: number }, b: { x: number; y: number }): string {
  const dx = b.x - a.x;
  const dy = b.y - a.y;
  const dist = Math.sqrt(dx * dx + dy * dy);
  if (dist < 1) return '';
  const nx = dx / dist;
  const ny = dy / dist;
  const sx = a.x + nx * (NODE_R + 6);
  const sy = a.y + ny * (NODE_R + 6);
  const tx = b.x - nx * (NODE_R + 6);
  const ty = b.y - ny * (NODE_R + 6);
  return `M${sx},${sy}L${tx},${ty}`;
}

function arrowHead(x: number, y: number, angle: number): string {
  const s = 8;
  return [
    `M${x},${y}`,
    `L${x - s * Math.cos(angle - 0.35)},${y - s * Math.sin(angle - 0.35)}`,
    `L${x - s * Math.cos(angle + 0.35)},${y - s * Math.sin(angle + 0.35)}`,
    'Z',
  ].join('');
}

// ─── Build simulation data ───

function buildSimulation(topoData: { nodes?: TopoNode[]; edges?: TopoEdge[] }): { nodes: SimNode[]; edges: SimEdge[] } | null {
  if (!topoData.nodes?.length) return null;

  const rawNodes = topoData.nodes;
  const rawEdges = topoData.edges || [];
  let maxCount = 0;
  for (const e of rawEdges) {
    if (e.count > maxCount) maxCount = e.count;
  }
  const norm = maxCount > 0 ? 1 / maxCount : 1;

  const nodeMap = new Map<string, number>();
  const simNodes: SimNode[] = rawNodes.map((n, i) => {
    nodeMap.set(n.name, i);
    return { ...n, x: 0, y: 0, vx: 0, vy: 0, isCrossNode: false };
  });

  const simEdges: SimEdge[] = [];
  for (const e of rawEdges) {
    const si = nodeMap.get(e.caller);
    const ti = nodeMap.get(e.target);
    if (si === undefined || ti === undefined) continue;
    simEdges.push({
      caller: e.caller,
      target: e.target,
      api: e.api,
      count: e.count,
      last_seen: e.last_seen,
      sourceIdx: si,
      targetIdx: ti,
      thickness: e.count * norm,
    });
  }

  // Mark cross-node services
  const nodeHostMap = new Map<string, string>();
  rawNodes.forEach(n => nodeHostMap.set(n.name, n.node_host));
  for (const e of rawEdges) {
    const aHost = nodeHostMap.get(e.caller);
    const bHost = nodeHostMap.get(e.target);
    if (aHost && bHost && aHost !== bHost) {
      const si = nodeMap.get(e.caller);
      const ti = nodeMap.get(e.target);
      if (si !== undefined) simNodes[si].isCrossNode = true;
      if (ti !== undefined) simNodes[ti].isCrossNode = true;
    }
  }

  runSimulation(simNodes, simEdges, 100);
  return { nodes: simNodes, edges: simEdges };
}

// ─── Main Component ───

export default function AuthTopology() {
  const toast = useToast();
  const [selectedNode, setSelectedNode] = useState<SimNode | null>(null);
  const [selectedEdge, setSelectedEdge] = useState<SimEdge | null>(null);
  const [hoveredEdge, setHoveredEdge] = useState<SimEdge | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [focusService, setFocusService] = useState<string | null>(null);
  const svgRef = useRef<SVGSVGElement>(null);

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['auth-topology'],
    queryFn: () => adminApi.getAuthTopology('1h'),
    refetchInterval: 60_000,
  });

  const sim = useMemo(() => (data ? buildSimulation(data) : null), [data]);

  const nodes = sim?.nodes || [];
  const edges = sim?.edges || [];
  const hasData = nodes.length > 0;

  // Cross-node edges
  const crossNodeNames = useMemo(() => {
    const set = new Set<string>();
    for (const n of nodes) {
      if (n.isCrossNode) set.add(n.name);
    }
    return set;
  }, [nodes]);

  // Focus filter
  const relatedNames = useMemo(() => {
    if (!focusService || !edges.length) return null;
    const s = new Set([focusService]);
    for (const e of edges) {
      if (e.caller === focusService) s.add(e.target);
      if (e.target === focusService) s.add(e.caller);
    }
    return s;
  }, [focusService, edges]);

  const handleNodeClick = (n: SimNode) => {
    setSelectedNode(n);
    setSelectedEdge(null);
    setDrawerOpen(true);
  };

  const handleEdgeClick = (e: SimEdge) => {
    setSelectedEdge(e);
    setSelectedNode(null);
    setDrawerOpen(true);
  };

  const crossNodeEdgeCount = edges.filter(e => {
    const aNode = nodes[e.sourceIdx];
    const bNode = nodes[e.targetIdx];
    return aNode && bNode && aNode.node_host && bNode.node_host && aNode.node_host !== bNode.node_host;
  }).length;

  return (
    <div className="p-6 space-y-5">
      <PageHeader
        title="服务拓扑 · Service Topology"
        subtitle={`${nodes.length} 个服务 · ${edges.length} 条调用边 · 过去 1h 窗口`}
      />

      {/* Stats row */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
        <div className="px-4 py-3 rounded-a-sm bg-a-surface border border-a-border/30">
          <div className="text-[10px] text-a-muted uppercase tracking-wider">注册服务</div>
          <div className="text-lg font-bold font-mono text-a-fg mt-0.5">{nodes.length}</div>
        </div>
        <div className="px-4 py-3 rounded-a-sm bg-a-surface border border-a-border/30">
          <div className="text-[10px] text-a-muted uppercase tracking-wider">活跃调用</div>
          <div className="text-lg font-bold font-mono text-a-fg mt-0.5">{edges.reduce((s, e) => s + e.count, 0)}</div>
        </div>
        <div className="px-4 py-3 rounded-a-sm bg-a-surface border border-a-border/30">
          <div className="text-[10px] text-a-muted uppercase tracking-wider">跨机建议</div>
          <div className="text-lg font-bold font-mono text-[#e8b830] mt-0.5">{crossNodeEdgeCount}</div>
        </div>
        <div className="px-4 py-3 rounded-a-sm bg-a-surface border border-a-border/30">
          <div className="text-[10px] text-a-muted uppercase tracking-wider">已封锁</div>
          <div className="text-lg font-bold font-mono text-[#ff5c72] mt-0.5">{nodes.filter(n => n.status === 'blocked').length}</div>
        </div>
      </div>

      {/* Focus filter pills */}
      {nodes.length > 0 && (
        <div className="flex items-center gap-1.5 flex-wrap">
          <span className="text-[10px] text-a-muted uppercase tracking-wider mr-1">聚焦:</span>
          <button onClick={() => setFocusService(null)}
            className={cn('px-2 py-0.5 rounded text-[10px] font-mono transition-colors',
              !focusService ? 'bg-a-accent/20 text-a-accent border border-a-accent/30' : 'bg-a-border/10 text-a-muted hover:text-a-fg border border-transparent',
            )}>全部</button>
          {nodes.map(n => (
            <button key={n.name} onClick={() => setFocusService(n.name === focusService ? null : n.name)}
              className={cn('px-2 py-0.5 rounded text-[10px] font-mono transition-colors',
                focusService === n.name ? 'bg-a-accent/20 text-a-accent border border-a-accent/30' : 'bg-a-border/10 text-a-muted hover:text-a-fg border border-transparent',
              )}>
              {n.name}
            </button>
          ))}
        </div>
      )}

      {/* SVG Canvas */}
      <div className="relative border border-a-border/30 rounded-a-md overflow-hidden bg-a-bg/50">
        {isLoading ? (
          <LoadingState />
        ) : error ? (
          <div className="flex items-center justify-center" style={{ height: H }}>
            <div className="text-center">
              <div className="text-sm text-[#ff5c72] mb-2">加载失败</div>
              <Btn onClick={() => refetch()} className="text-xs">重试</Btn>
            </div>
          </div>
        ) : !hasData ? (
          <div className="flex items-center justify-center" style={{ height: H }}>
            <div className="text-center">
              <div className="text-3xl mb-2 opacity-30">🔌</div>
              <div className="text-sm text-a-muted">尚未检测到服务间调用</div>
              <div className="text-xs text-a-muted/60 mt-1">部署 SDK 并注册服务后自动生成</div>
            </div>
          </div>
        ) : (
          <svg ref={svgRef} viewBox={`0 0 ${W} ${H}`} className="w-full" style={{ height: H }}>
            {/* Edges */}
            {edges.map((e, i) => {
              const a = nodes[e.sourceIdx];
              const b = nodes[e.targetIdx];
              if (!a || !b) return null;
              const path = edgePath(a, b);
              const isHovered = hoveredEdge === e;
              const isFocused = !focusService || (relatedNames?.has(e.caller) && relatedNames?.has(e.target));
              const isDimmed = focusService && !isFocused;
              const isCross = a.node_host && b.node_host && a.node_host !== b.node_host;
              return (
                <g key={i}
                  onMouseEnter={() => setHoveredEdge(e)}
                  onMouseLeave={() => setHoveredEdge(null)}
                  onClick={() => handleEdgeClick(e)}
                  className="cursor-pointer">
                  <path d={path} stroke="transparent" strokeWidth={16} fill="none" />
                  <path
                    d={path}
                    stroke={isCross ? '#e8b830' : isDimmed ? '#2d2447' : isHovered ? '#a865ff' : '#3a2e5e'}
                    strokeWidth={Math.max(1, Math.min(e.thickness * 3, 6))}
                    fill="none"
                    opacity={isDimmed ? 0.15 : isHovered ? 0.9 : 0.5}
                    className="transition-all duration-200"
                  />
                  <text
                    x={(a.x + b.x) / 2}
                    y={(a.y + b.y) / 2 - 8}
                    textAnchor="middle"
                    fill={isDimmed ? '#2d2447' : isHovered ? '#c9c0dc' : '#8b83a6'}
                    fontSize={9}
                    fontFamily="JetBrains Mono, monospace"
                    opacity={isDimmed ? 0.1 : 0.7}
                  >
                    {e.count}
                  </text>
                </g>
              );
            })}

            {/* Arrow heads */}
            {edges.map((e, i) => {
              const a = nodes[e.sourceIdx];
              const b = nodes[e.targetIdx];
              if (!a || !b) return null;
              const dx = b.x - a.x;
              const dy = b.y - a.y;
              const dist = Math.sqrt(dx * dx + dy * dy);
              if (dist < 1) return null;
              const nx = dx / dist;
              const ny = dy / dist;
              const ax = b.x - nx * (NODE_R + 6);
              const ay = b.y - ny * (NODE_R + 6);
              const angle = Math.atan2(ny, nx);
              const isCross = a.node_host && b.node_host && a.node_host !== b.node_host;
              const isFocused = !focusService || (relatedNames?.has(e.caller) && relatedNames?.has(e.target));
              return (
                <path key={`arrow-${i}`}
                  d={arrowHead(ax, ay, angle)}
                  fill={isCross ? '#e8b830' : '#3a2e5e'}
                  opacity={isFocused ? (isCross ? 0.6 : 0.4) : 0.1}
                />
              );
            })}

            {/* Nodes */}
            {nodes.map((n, i) => {
              const isFocused = !focusService || focusService === n.name || relatedNames?.has(n.name);
              const isDimmed = focusService && !isFocused;
              const isSelected = selectedNode === n;
              const color = n.status === 'blocked' ? '#ff5c72' : isSelected ? '#a865ff' : '#3a2e5e';
              return (
                <g key={i}
                  onClick={() => handleNodeClick(n)}
                  className="cursor-pointer">
                  {isSelected && (
                    <circle cx={n.x} cy={n.y} r={NODE_R + 6}
                      fill="none" stroke="#a865ff" strokeWidth={2} opacity={0.5}>
                      <animate attributeName="r" values={`${NODE_R + 4};${NODE_R + 10};${NODE_R + 4}`} dur="2s" repeatCount="indefinite" />
                      <animate attributeName="opacity" values="0.5;0.1;0.5" dur="2s" repeatCount="indefinite" />
                    </circle>
                  )}
                  <circle cx={n.x} cy={n.y} r={NODE_R}
                    fill={isDimmed ? '#181225' : '#1e1633'}
                    stroke={color}
                    strokeWidth={isSelected ? 2.5 : n.status === 'blocked' ? 2 : 1.5}
                    opacity={isDimmed ? 0.2 : 1}
                  />
                  <circle cx={n.x + NODE_R - 8} cy={n.y - NODE_R + 8} r={4}
                    fill={COLORS[n.status] || '#8b83a6'}
                    opacity={isDimmed ? 0.1 : 1}
                  />
                  <text x={n.x} y={n.y + 4}
                    textAnchor="middle"
                    fill={isDimmed ? '#2d2447' : '#f0ecf8'}
                    fontSize={11}
                    fontFamily="Inter, sans-serif"
                    fontWeight={600}
                    opacity={isDimmed ? 0.2 : 1}>
                    {n.name.length > 12 ? n.name.slice(0, 11) + '…' : n.name}
                  </text>
                  {n.isCrossNode && !isDimmed && (
                    <text x={n.x} y={n.y - NODE_R - 6} textAnchor="middle" fontSize={8} fill="#e8b830" fontFamily="monospace">
                      🌐
                    </text>
                  )}
                </g>
              );
            })}

            {/* Hovered edge tooltip */}
            {hoveredEdge && (() => {
              const a = nodes[hoveredEdge.sourceIdx];
              const b = nodes[hoveredEdge.targetIdx];
              if (!a || !b) return null;
              const mx = (a.x + b.x) / 2;
              const my = (a.y + b.y) / 2;
              return (
                <g>
                  <rect x={mx - 60} y={my - 40} width={120} height={28} rx={4}
                    fill="#1e1633" stroke="#a865ff" strokeWidth={1} opacity={0.95} />
                  <text x={mx} y={my - 24}
                    textAnchor="middle"
                    fill="#f0ecf8" fontSize={9}
                    fontFamily="JetBrains Mono, monospace">
                    {hoveredEdge.count} calls · {hoveredEdge.api}
                  </text>
                </g>
              );
            })()}
          </svg>
        )}
      </div>

      {/* Legend */}
      <div className="flex items-center gap-4 text-[10px] text-a-muted flex-wrap">
        <span className="flex items-center gap-1"><span className="w-2 h-2 rounded-full bg-[#4cd964]" /> 活跃</span>
        <span className="flex items-center gap-1"><span className="w-2 h-2 rounded-full bg-[#ff5c72]" /> 已封锁</span>
        <span className="flex items-center gap-1"><span className="w-6 h-0.5 bg-[#3a2e5e]" /> 同机调用</span>
        <span className="flex items-center gap-1"><span className="w-6 h-0.5 bg-[#e8b830]" /> 跨机调用</span>
        <span className="flex items-center gap-1 text-a-border">线粗=调用频率</span>
        <span className="flex items-center gap-1 text-a-border">点击节点查看详情</span>
        <span className="flex-1" />
        <Btn onClick={() => refetch()} className="text-[10px]" disabled={isLoading}>刷新</Btn>
      </div>

      {/* Drawer: Node detail */}
      <Drawer open={drawerOpen && !!selectedNode} onClose={() => { setDrawerOpen(false); setSelectedNode(null); }}
        title={selectedNode?.name || ''}
        subtitle={selectedNode ? `${selectedNode.host}:${selectedNode.port}` : ''}
        width="sm">
        {selectedNode && (
          <div className="space-y-4">
            <Card title="基本信息">
              <MetaRow label="状态" value={
                <span className={selectedNode.status === 'active' ? 'text-[#4cd964]' : 'text-[#ff5c72]'}>
                  {selectedNode.status === 'active' ? '活跃' : '已封锁'}
                </span>
              } />
              <MetaRow label="地址" mono>{selectedNode.host}:{selectedNode.port}</MetaRow>
              <MetaRow label="节点" mono>{selectedNode.node_host || '—'}</MetaRow>
            </Card>
            <Card title="调用关系">
              <MetaRow label="调用" value={
                edges.filter(e => e.caller === selectedNode.name).map(e =>
                  `${e.target}(${e.count}次)`
                ).join('、') || '无'
              } />
              <MetaRow label="被调" value={
                edges.filter(e => e.target === selectedNode.name).map(e =>
                  `${e.caller}(${e.count}次)`
                ).join('、') || '无'
              } />
            </Card>
            {selectedNode.node_host && edges.filter(e => {
              if (e.caller !== selectedNode.name && e.target !== selectedNode.name) return false;
              const aNode = nodes[e.sourceIdx];
              const bNode = nodes[e.targetIdx];
              return aNode && bNode && aNode.node_host && bNode.node_host && aNode.node_host !== bNode.node_host;
            }).length > 0 && (
              <div className="p-3 rounded-a-sm bg-[#e8b830]/5 border border-[#e8b830]/20">
                <div className="text-[11px] font-medium text-[#e8b830] mb-1">🌐 跨机通信</div>
                <div className="text-[10px] text-a-muted">
                  此服务与不同节点的服务存在调用关系。
                </div>
              </div>
            )}
          </div>
        )}
      </Drawer>

      {/* Drawer: Edge detail */}
      <Drawer open={drawerOpen && !!selectedEdge} onClose={() => { setDrawerOpen(false); setSelectedEdge(null); }}
        title={selectedEdge ? `${selectedEdge.caller} → ${selectedEdge.target}` : ''}
        subtitle={selectedEdge ? `API: ${selectedEdge.api}` : ''}
        width="sm">
        {selectedEdge && (() => {
          const aNode = nodes[selectedEdge.sourceIdx];
          const bNode = nodes[selectedEdge.targetIdx];
          const isCross = aNode && bNode && aNode.node_host && bNode.node_host && aNode.node_host !== bNode.node_host;
          return (
            <div className="space-y-4">
              <Card title="调用统计">
                <MetaRow label="调用方" value={selectedEdge.caller} />
                <MetaRow label="目标方" value={selectedEdge.target} />
                <MetaRow label="API" mono>{selectedEdge.api}</MetaRow>
                <MetaRow label="调用次数" value={`${selectedEdge.count} 次`} />
                <MetaRow label="最后调用" value={fmtTime(selectedEdge.last_seen)} />
              </Card>
              {isCross && (
                <div className="p-3 rounded-a-sm bg-[#e8b830]/5 border border-[#e8b830]/20">
                  <div className="text-[11px] font-medium text-[#e8b830] mb-1">🌐 跨机通信</div>
                  <div className="text-xs text-a-muted space-y-1">
                    <p><span className="text-a-fg">{selectedEdge.caller}</span> 在 {aNode.node_host}</p>
                    <p><span className="text-a-fg">{selectedEdge.target}</span> 在 {bNode.node_host}</p>
                    <p className="text-[#e8b830]/80 text-[10px] mt-2">建议为此链路创建 GatewayLink</p>
                  </div>
                </div>
              )}
            </div>
          );
        })()}
      </Drawer>
    </div>
  );
}

// ─── MetaRow inline (avoids import issues with component signature) ───

function MetaRow({ label, value, mono, children, className }: {
  label: string;
  value?: React.ReactNode;
  mono?: boolean;
  children?: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={cn('flex items-start gap-3 py-1.5 text-xs border-b border-a-border/20 last:border-0', className)}>
      <span className="text-a-muted w-16 shrink-0">{label}</span>
      <span className={cn('text-a-fg', mono && 'font-mono')}>{value || children}</span>
    </div>
  );
}

// ─── Helpers ───

function fmtTime(t: string | undefined): string {
  if (!t) return '—';
  try {
    return new Date(t).toLocaleString('zh-CN', {
      month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit',
    });
  } catch { return t; }
}

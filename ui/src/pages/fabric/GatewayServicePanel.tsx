// ─── Aegis 网关面板 (v1.9A) — 视角 B ───
// Aegis 作为网关服务的自检视图。以对象为中心：
//   - 调用方服务（谁在调我？还活着吗？）
//   - 受管资源（域名、路由）
//   - Aegis 自身
// 其他项目可参考此模式实现自己的服务面板。

import { useQuery } from '@tanstack/react-query';
import { adminApi } from '@/lib/api-bridge';
import { PageHeader, Card, HealthDot, LoadingState, EmptyState } from '@/components/shared';
import { cn } from '@/lib/utils';

// ─── Types ───

interface ServiceObj {
  name: string;
  status: string;
  lastSeen: string;
  instanceID: string;
  publicKey: string;
  callCount: number;
  lastCall: string;
}

interface DomainObj {
  domain: string;
  status: string;
  target: string;
  tls: boolean;
  serviceID: string;
}

// ─── Main ───

export default function GatewayServicePanel() {
  // ── Data ──

  const { data: topoData } = useQuery({
    queryKey: ['gw-topo'],
    queryFn: () => adminApi.getAuthTopology('24h'),
    refetchInterval: 60_000,
  });

  const { data: myRoutes } = useQuery({
    queryKey: ['gw-routes'],
    queryFn: () => adminApi.callMyRoutes(''),
    refetchInterval: 30_000,
  });

  const { data: sysStatus } = useQuery({
    queryKey: ['gw-sys'],
    queryFn: () => fetch('/api/system/status').then(r => r.json()),
    refetchInterval: 30_000,
  });

  // ── Build service objects from topology ──

  const topoNodes: any[] = topoData?.nodes || [];
  const topoEdges: any[] = topoData?.edges || [];
  const routes: any[] = myRoutes?.routes || [];

  // Callers: edges where target is aegis-gateway (someone called Aegis)
  // Deps: edges where caller is aegis-gateway (Aegis called someone)
  const callerEdgeMap = new Map<string, { count: number; last: string }>();
  const depEdgeMap = new Map<string, { count: number; last: string }>();

  for (const e of topoEdges) {
    if (e.target === 'aegis-gateway') {
      const existing = callerEdgeMap.get(e.caller) || { count: 0, last: '' };
      existing.count += e.count;
      if (e.last_seen > existing.last) existing.last = e.last_seen;
      callerEdgeMap.set(e.caller, existing);
    }
    if (e.caller === 'aegis-gateway') {
      const existing = depEdgeMap.get(e.target) || { count: 0, last: '' };
      existing.count += e.count;
      if (e.last_seen > existing.last) existing.last = e.last_seen;
      depEdgeMap.set(e.target, existing);
    }
  }

  // Merge topology nodes into ServiceObj[]
  const services: ServiceObj[] = [];
  const seen = new Set<string>();

  for (const n of topoNodes) {
    if (n.name === 'aegis-gateway') continue; // don't show self
    const edge = callerEdgeMap.get(n.name);
    services.push({
      name: n.name,
      status: n.status,
      lastSeen: '',
      instanceID: '',
      publicKey: '',
      callCount: edge?.count || 0,
      lastCall: edge?.last || '',
    });
    seen.add(n.name);
  }

  // Add callers from topology edges that aren't in nodes
  for (const [name, edge] of callerEdgeMap) {
    if (!seen.has(name)) {
      services.push({
        name,
        status: 'unknown',
        lastSeen: '',
        instanceID: '',
        publicKey: '',
        callCount: edge.count,
        lastCall: edge.last,
      });
    }
  }

  services.sort((a, b) => b.callCount - a.callCount);

  const activeServices = services.filter(s => {
    if (s.status === 'blocked') return false;
    const ls = new Date(s.lastSeen);
    return !isNaN(ls.getTime()) && (Date.now() - ls.getTime()) < 5 * 60 * 1000;
  });

  const isLoading = !topoData;

  // ── Domain objects ──

  const domains: DomainObj[] = (routes || []).map((r: any) => ({
    domain: r.domain,
    status: r.status,
    target: r.service_id,
    tls: !!r.tls_enabled,
    serviceID: r.service_id,
  }));

  // ── Render ──

  return (
    <div className="p-6 space-y-5">
      {/* ─── Header ─── */}
      <PageHeader
        title="Aegis · 网关面板"
        subtitle={`${services.length} 个关联服务 · ${activeServices.length} 在线 · ${domains.length} 个域名`}
      />

      {isLoading ? <LoadingState /> : (
        <>
          {/* ─── Aegis 自身 ─── */}
          <section>
            <div className="flex items-center gap-4 mb-3">
              <div className="w-10 h-10 rounded-lg bg-a-accent/20 flex items-center justify-center text-lg shrink-0"></div>
              <div>
                <div className="text-sm font-semibold text-a-fg">Aegis Gateway</div>
                <div className="text-[11px] text-a-muted font-mono">{sysStatus?.version || 'dev'} · {sysStatus?.build_time || ''}</div>
              </div>
              <div className="ml-auto flex items-center gap-2 text-xs">
                <HealthDot status="healthy" />
                <span className="text-[#4cd964]">运行中</span>
              </div>
            </div>

            <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
              <StatBox label="调用方服务" value={services.length} />
              <StatBox label="在线" value={activeServices.length} accent />
              <StatBox label="管理域名" value={domains.length} />
              <StatBox label="总调用" value={services.reduce((s, c) => s + c.callCount, 0)} />
            </div>
          </section>

          {/* ─── 调用方服务 ─── */}
          <section>
            <div className="flex items-center gap-2 mb-3">
              <h2 className="text-xs font-semibold text-a-fg uppercase tracking-wider">调用方服务</h2>
              <span className="text-[10px] text-a-muted">已注册的调用方 · 健康状态 · 调用活跃度</span>
            </div>

            {services.length === 0 ? (
              <EmptyState title="暂无调用方" description="还没有服务通过 ServiceAuth 调用 Action API" />
            ) : (
              <div className="grid grid-cols-1 lg:grid-cols-2 gap-2">
                {services.map(s => <CallerCard key={s.name} svc={s} />)}
              </div>
            )}
          </section>

          {/* ─── 受管域名 ─── */}
          <section>
            <div className="flex items-center gap-2 mb-3">
              <h2 className="text-xs font-semibold text-a-fg uppercase tracking-wider">受管域名</h2>
              <span className="text-[10px] text-a-muted">Aegis 管理的 HTTP 路由</span>
            </div>

            {domains.length === 0 ? (
              <div className="text-xs text-a-muted py-6 text-center border border-dashed border-a-border/30 rounded-a-sm">暂无绑定域名</div>
            ) : (
              <div className="overflow-x-auto border border-a-border/20 rounded-a-sm">
                <table className="w-full text-xs">
                  <thead>
                    <tr className="border-b border-a-border/30 text-a-muted text-left bg-a-surface/50">
                      <th className="py-2 px-3 font-medium">域名</th>
                      <th className="py-2 px-3 font-medium">状态</th>
                      <th className="py-2 px-3 font-medium">TLS</th>
                      <th className="py-2 px-3 font-medium text-right">后端</th>
                    </tr>
                  </thead>
                  <tbody>
                    {domains.map((d, i) => (
                      <tr key={i} className="border-b border-a-border/20 hover:bg-a-border/10">
                        <td className="py-2 px-3 font-mono text-a-fg font-medium">{d.domain}</td>
                        <td className="py-2 px-3"><span className={cn('px-1.5 py-0.5 rounded text-[10px]', d.status === 'active' ? 'bg-[#4cd964]/10 text-[#4cd964]' : 'bg-a-border/20 text-a-muted')}>{d.status}</span></td>
                        <td className="py-2 px-3 text-a-muted">{d.tls ? 'TLS ✓' : '—'}</td>
                        <td className="py-2 px-3 text-right font-mono text-a-muted text-[10px]">{d.target?.slice(0, 16) || '—'}…</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </section>
        </>
      )}
    </div>
  );
}

// ─── Caller Card ───

function CallerCard({ svc }: { svc: ServiceObj }) {
  const now = Date.now();
  const ls = new Date(svc.lastSeen);
  const isOnline = !isNaN(ls.getTime()) && (now - ls.getTime()) < 5 * 60 * 1000;
  const isBlocked = svc.status === 'blocked';
  const minutesAgo = isNaN(ls.getTime()) ? null : Math.round((now - ls.getTime()) / 60000);

  return (
    <div className={cn(
      'flex items-center gap-3 px-3 py-2.5 rounded-a-sm border transition-colors',
      isBlocked ? 'border-[#ff5c72]/20 bg-[#ff5c72]/5' : isOnline ? 'border-a-border/30 bg-a-surface' : 'border-a-border/20 bg-a-bg/50',
    )}>
      {/* 图标 */}
      <div className={cn(
        'w-8 h-8 rounded-md flex items-center justify-center text-xs font-bold shrink-0',
        isBlocked ? 'bg-[#ff5c72]/10 text-[#ff5c72]' : isOnline ? 'bg-[#4cd964]/10 text-[#4cd964]' : 'bg-a-border/20 text-a-muted',
      )}>
        {svc.name.charAt(0).toUpperCase()}
      </div>

      {/* 身份 */}
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-1.5">
          <span className="text-xs font-semibold text-a-fg">{svc.name}</span>
          <HealthDot status={isBlocked ? 'failed' : isOnline ? 'healthy' : 'failed'} />
          <span className={cn('text-[10px]', isBlocked ? 'text-[#ff5c72]' : isOnline ? 'text-[#4cd964]' : 'text-a-muted')}>
            {isBlocked ? '已封锁' : isOnline ? '在线' : '离线'}
          </span>
        </div>
        <div className="flex items-center gap-2 text-[10px] text-a-muted mt-0.5">
          {svc.callCount > 0 && <span>调用了 {svc.callCount} 次</span>}
          {svc.instanceID && <span className="font-mono">{svc.instanceID.slice(0, 12)}…</span>}
          {minutesAgo !== null && <span>{minutesAgo < 1 ? '刚才' : `${minutesAgo} 分钟前`}</span>}
        </div>
      </div>

      {/* 活动指示 */}
      {svc.callCount > 0 && (
        <div className="text-right shrink-0">
          <div className="text-[11px] font-mono text-a-fg">{svc.callCount}</div>
          <div className="text-[9px] text-a-muted">调用</div>
        </div>
      )}
    </div>
  );
}

// ─── StatBox ───

function StatBox({ label, value, accent }: { label: string; value: number; accent?: boolean }) {
  return (
    <div className={cn('px-3 py-2.5 rounded-a-sm border', accent ? 'bg-a-accent/5 border-a-accent/20' : 'bg-a-surface border-a-border/30')}>
      <div className={cn('text-[9px] uppercase tracking-wider', accent ? 'text-a-accent' : 'text-a-muted')}>{label}</div>
      <div className={cn('text-base font-bold font-mono mt-0.5', accent ? 'text-a-accent' : 'text-a-fg')}>{value}</div>
    </div>
  );
}


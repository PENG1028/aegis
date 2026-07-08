// ─── Aegis 网关面板 (v1.9A) — 视角 B ───
// 站在 Aegis 自己的角度：谁在调我？我依赖谁？我提供了什么？
// 其他项目可以照着这个页面实现自己的服务面板。
//
// 数据来源：
//   - 调用方 → topology API (call_logs 聚合)
//   - 健康状态 → service-auth 的 /api/service-auth/v1/services
//   - 我的资源 → /api/v1/my/routes 等
//   - 心跳 → Admin API 服务列表

import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { adminApi } from '@/lib/api-bridge';
import { PageHeader, StatCard, StatusBadge, Card, Btn, HealthDot, LoadingState, ErrorBanner, EmptyState } from '@/components/shared';
import { cn } from '@/lib/utils';

interface CallerInfo {
  name: string;
  api: string;
  count: number;
  lastSeen: string;
  status: string;   // active | inactive | blocked
  instanceCount: number;
}

export default function GatewayServicePanel() {
  const [serviceFilter, setServiceFilter] = useState('');

  // ── Data ──

  // Topology: who's calling Aegis's Action API
  const { data: topoData, isLoading: topoLoading } = useQuery({
    queryKey: ['gw-service-topology'],
    queryFn: () => adminApi.getAuthTopology('24h'),
    refetchInterval: 60_000,
  });
  const topoEdges: any[] = topoData?.edges || [];

  // Service list: health + status
  const { data: svcData, isLoading: svcLoading } = useQuery({
    queryKey: ['gw-service-services'],
    queryFn: () => adminApi.listAuthServices(),
    refetchInterval: 15_000,
  });
  const allServices: any[] = svcData?.services || [];

  // Service status map: name → status
  const statusMap = new Map(allServices.map((s: any) => [s.name, s.status]));
  const lastSeenMap = new Map(allServices.map((s: any) => [s.name, s.last_seen]));

  // ── Build caller list ──
  // Action API endpoints are the Aegis gateway's public interface.
  // Callers hitting these endpoints are "calling Aegis".
  const actionApiPaths = ['bind-http-domain', 'bind-tls-backend', 'update-target', 'disable-domain', 'delete-domain'];

  const callerMap = new Map<string, CallerInfo>();
  for (const e of topoEdges) {
    const isActionApi = actionApiPaths.some(p => e.api?.includes(p));
    if (!isActionApi) continue;
    const existing = callerMap.get(e.caller);
    if (existing) {
      existing.count += e.count;
      if (e.last_seen > existing.lastSeen) existing.lastSeen = e.last_seen;
    } else {
      callerMap.set(e.caller, {
        name: e.caller,
        api: e.api,
        count: e.count,
        lastSeen: e.last_seen,
        status: statusMap.get(e.caller) || 'unknown',
        instanceCount: 0,
      });
    }
  }

  const callers = Array.from(callerMap.values()).sort((a, b) => b.count - a.count);

  // Enrich with instance counts from service list
  for (const svc of allServices) {
    const caller = callers.find(c => c.name === svc.name);
    if (caller && svc.instance_id) caller.instanceCount++;
  }

  // ── Compute active/inactive ──
  const now = new Date();
  const fiveMinAgo = new Date(now.getTime() - 5 * 60 * 1000).toISOString();

  const activeCallers = callers.filter(c => {
    const ls = lastSeenMap.get(c.name);
    return ls && ls >= fiveMinAgo;
  });

  const filteredCallers = serviceFilter
    ? callers.filter(c => c.name.toLowerCase().includes(serviceFilter.toLowerCase()))
    : callers;

  // ── My resources (via admin API — Aegis itself is the admin) ──
  const { data: myRoutes } = useQuery({
    queryKey: ['gw-my-routes'],
    queryFn: () => adminApi.callMyRoutes(''),
    refetchInterval: 30_000,
  });
  const routeCount = myRoutes?.routes?.length || 0;

  // ── Gateway info ──
  const { data: systemStatus } = useQuery({
    queryKey: ['gw-system-status'],
    queryFn: () => fetch('/api/system/status').then(r => r.json()),
    refetchInterval: 30_000,
  });

  const isLoading = topoLoading || svcLoading;

  return (
    <div className="p-6 space-y-5">
      <PageHeader
        title="网关面板 · Gateway Panel"
        subtitle="Aegis 作为网关服务的自检视图 — 谁在调我？我依赖谁？其他项目可参考此模式"
      />

      {/* Stats bar */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
        <StatCard label="调用方服务" value={callers.length} accent />
        <StatCard label="活跃中" value={activeCallers.length} />
        <StatCard label="管理域名" value={routeCount} />
        <StatCard label="总调用数" value={callers.reduce((s, c) => s + c.count, 0)} />
      </div>

      {/* Who's calling me */}
      <Card
        title="谁在调我 · Callers"
        subtitle="过去 24h 内调用了 Aegis Action API 的服务 — 这是外部项目最直接的参考：你的服务注册后，会出现在这里"
      >
        <div className="mb-3">
          <input value={serviceFilter} onChange={e => setServiceFilter(e.target.value)}
            placeholder="搜索调用方服务名..."
            className="w-full bg-a-bg border border-a-border/50 rounded-a-sm px-3 py-1.5 text-xs font-mono text-a-fg placeholder:text-a-muted/40 focus:outline-none focus:border-a-accent/50" />
        </div>

        {isLoading ? (
          <LoadingState />
        ) : callers.length === 0 ? (
          <EmptyState
            title="暂无调用方"
            description="还没有服务通过 ServiceAuth 调用 Aegis Action API。部署 demo-service 或注册 SDK 后会自动出现"
          />
        ) : filteredCallers.length === 0 ? (
          <div className="text-xs text-a-muted py-3">没有匹配的服务</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-xs">
              <thead>
                <tr className="border-b border-a-border text-a-muted text-left">
                  <th className="py-2 px-3 font-medium">调用方</th>
                  <th className="py-2 px-3 font-medium">最后调用</th>
                  <th className="py-2 px-3 font-medium text-right">调用次数</th>
                  <th className="py-2 px-3 font-medium text-right">实例数</th>
                  <th className="py-2 px-3 font-medium">状态</th>
                </tr>
              </thead>
              <tbody>
                {filteredCallers.map((c, i) => {
                  const ls = lastSeenMap.get(c.name);
                  const isOnline = ls && ls >= fiveMinAgo;
                  return (
                    <tr key={i} className="border-b border-a-border/50 hover:bg-a-border/10 transition-colors">
                      <td className="py-2 px-3 font-semibold text-a-fg">{c.name}</td>
                      <td className="py-2 px-3 text-[10px] text-a-muted font-mono">{ls ? fmtTimeShort(ls) : '—'}</td>
                      <td className="py-2 px-3 text-right font-mono text-a-fg">{c.count}</td>
                      <td className="py-2 px-3 text-right font-mono text-a-muted">{c.instanceCount || 1}</td>
                      <td className="py-2 px-3">
                        <div className="flex items-center gap-1.5">
                          <HealthDot status={isOnline ? 'healthy' : 'failed'} />
                          <span className={cn('text-[11px]', isOnline ? 'text-[#4cd964]' : 'text-a-muted')}>
                            {isOnline ? '在线' : c.status === 'blocked' ? '已封锁' : '离线'}
                          </span>
                        </div>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}

        <div className="mt-3 pt-2 border-t border-a-border/20 text-[10px] text-a-muted/60 flex items-center gap-2">
          <HealthDot status="healthy" /> 在线（心跳 ≤5min）
          <HealthDot status="failed" /> 离线
          <span className="ml-auto">完整拓扑见 <a href="/auth/callgraph" className="text-a-accent hover:underline">ServiceAuth 控制台 →</a></span>
        </div>
      </Card>

      {/* Gateway info — 参考实现标注 */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <Card title="网关身份" subtitle="Aegis 运行信息">
          {systemStatus ? (
            <div className="space-y-2 text-xs">
              <Row label="版本" mono>{systemStatus.version || 'dev'}</Row>
              <Row label="API 地址">http://127.0.0.1:7380</Row>
              <Row label="运行时间">{systemStatus.uptime || '—'}</Row>
              <Row label="认证方式">Admin Token / ServiceAuth Ticket</Row>
              <Row label="Provider">{systemStatus.providers?.join(', ') || 'caddy'}</Row>
            </div>
          ) : (
            <div className="text-xs text-a-muted">加载中...</div>
          )}
        </Card>

        <Card title="已提供的能力" subtitle="Aegis 对外暴露的 Action API — 服务通过 ServiceAuth ticket 调用">
          <div className="space-y-1 text-xs font-mono">
            <div className="py-1 px-2 rounded-a-sm bg-a-bg/50 text-a-fg">POST /api/v1/actions/bind-http-domain</div>
            <div className="py-1 px-2 rounded-a-sm bg-a-bg/50 text-a-fg">POST /api/v1/actions/bind-tls-backend</div>
            <div className="py-1 px-2 rounded-a-sm bg-a-bg/50 text-a-fg">PATCH /api/v1/actions/update-target</div>
            <div className="py-1 px-2 rounded-a-sm bg-a-bg/50 text-a-fg">POST /api/v1/actions/disable-domain</div>
            <div className="py-1 px-2 rounded-a-sm bg-a-bg/50 text-a-fg">DELETE /api/v1/actions/domain</div>
            <div className="py-1 px-2 rounded-a-sm bg-a-bg/50 text-a-fg">GET /api/v1/my/routes | services | edge-rules</div>
          </div>
        </Card>
      </div>

      {/* Reference annotation */}
      <Card title="参考实现" subtitle="你的服务注册到 ServiceAuth 后，也可以做同样的面板" className="border border-[#a865ff]/30">
        <div className="text-xs text-a-muted space-y-2">
          <p>这个页面展示的是 <span className="text-a-fg font-semibold">Aegis 网关自身</span> 的视角。</p>
          <p>你的服务注册到 ServiceAuth 后，通过 SDK 可以获得同样的能力：</p>
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-3 mt-3">
            <div className="p-3 rounded-a-sm bg-a-bg/50 border border-a-border/20">
              <div className="text-[11px] font-semibold text-a-fg mb-1">查看谁在调我</div>
              <div className="text-[10px] leading-relaxed">
                调 <code className="text-a-accent">GET /api/service-auth/v1/services</code><br />
                或 SDK <code className="text-a-accent">FetchServiceStatus()</code><br />
                就能看到所有注册服务的在线状态
              </div>
            </div>
            <div className="p-3 rounded-a-sm bg-a-bg/50 border border-a-border/20">
              <div className="text-[11px] font-semibold text-a-fg mb-1">查看我的资源</div>
              <div className="text-[10px] leading-relaxed">
                用 ServiceAuth ticket 调<br />
                <code className="text-a-accent">GET /api/v1/my/routes</code><br />
                只能看到自己空间内的资源
              </div>
            </div>
            <div className="p-3 rounded-a-sm bg-a-bg/50 border border-a-border/20">
              <div className="text-[11px] font-semibold text-a-fg mb-1">调用关系</div>
              <div className="text-[10px] leading-relaxed">
                调 <code className="text-a-accent">GET /api/admin/v1/service-auth/topology</code><br />
                看到谁依赖谁（需要 admin token）
              </div>
            </div>
          </div>
          <p className="mt-3 text-[10px] border-t border-a-border/20 pt-2">详细文档见 <a href="/docs/external-api-guide.md" className="text-a-accent hover:underline">外部 API 指南</a></p>
        </div>
      </Card>
    </div>
  );
}

// ─── Helpers ───

function Row({ label, mono, children }: { label: string; mono?: boolean; children: React.ReactNode }) {
  return (
    <div className="flex items-center gap-3">
      <span className="text-a-muted w-20 shrink-0">{label}</span>
      <span className={cn('text-a-fg', mono && 'font-mono')}>{children}</span>
    </div>
  );
}

function fmtTimeShort(t: string | undefined): string {
  if (!t) return '—';
  try { return new Date(t).toLocaleString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' }); }
  catch { return t; }
}

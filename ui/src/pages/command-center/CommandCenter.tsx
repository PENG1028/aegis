// ─── Command Center ───
// System health at a glance: anomalies with affected chains, pending changes, quick actions.

import { useQuery } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { fetchDashboard } from '@/lib/api-bridge';
import { cn } from '@/lib/utils';
import { Card, StatCard, HealthDot, StatusBadge, Btn, Timestamp, PageHeader } from '@/components/shared';
import { getScenario } from '@/mocks';
import { API_CONFIG } from '@/lib/api-config';
import { resolveChain } from '@/mocks/generators/chain-factory';
import type { DashboardData } from '@/types';
import type { Anomaly } from '@/types/workspace';

const QUICK_ACTIONS = [
  { label: '快速接入', desc: '创建域名映射', path: '/exposure/connect' },
  { label: '链路追踪', desc: '诊断请求路径', path: '/observe' },
  { label: '推送配置', desc: 'Apply 变更', path: '/release' },
  { label: '部署节点', desc: '添加新节点', path: '/runtime/deploy' },
  { label: '系统诊断', desc: '运行 Doctor', path: '/observe/doctor' },
];

function MiniChain({ type, id }: { type: string; id: string }) {
  if (!API_CONFIG.useMock) return null;
  const chain = resolveChain(type, id);
  if (chain.status === 'broken' && !chain.entryPoint) return null;

  const parts: string[] = [];
  if (chain.entryPoint) parts.push(chain.entryPoint.domain);
  if (chain.gateway) parts.push(chain.gateway.name);
  if (chain.service) parts.push(chain.service.name);
  if (chain.endpoints.length > 0) {
    const unhealthy = chain.endpoints.filter(e => e.health_status === 'unhealthy');
    if (unhealthy.length > 0) {
      parts.push(unhealthy.map(e => `✕ ${e.node_name || e.node_id}`).join(', '));
    } else {
      parts.push(chain.endpoints.map(e => e.node_name || e.node_id).join(', '));
    }
  }

  return (
    <div className="flex items-center gap-1 text-[10px] text-a-muted mt-1.5 font-mono">
      {parts.map((p, i) => (
        <span key={i} className="flex items-center gap-1">
          {i > 0 && <svg className="w-2.5 h-2.5 text-a-border" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M9 18l6-6-6-6"/></svg>}
          <span className={p.startsWith('✕') ? 'text-[#ff5c72]' : ''}>{p}</span>
        </span>
      ))}
    </div>
  );
}

function AnomalyCard({ a }: { a: Anomaly }) {
  const nav = useNavigate();
  const primaryObj = a.affectedObjects[0];
  const targetPath = primaryObj
    ? primaryObj.type === 'node' ? `/runtime/node/${primaryObj.id}`
    : primaryObj.type === 'route' ? `/exposure/entry/${primaryObj.id}`
    : primaryObj.type === 'gateway' ? `/fabric/gateway/${primaryObj.id}`
    : primaryObj.type === 'service' ? `/exposure/service/${primaryObj.id}`
    : primaryObj.type === 'endpoint' ? `/exposure/service/${primaryObj.id}`
    : a.workspace === 'release' ? '/release'
    : a.workspace === 'fabric' ? '/fabric'
    : '/'
    : '/';

  return (
    <div
      onClick={() => nav(targetPath)}
      className={cn(
        'p-3 rounded-a-sm border cursor-pointer transition-colors hover:brightness-110',
        a.severity === 'critical' ? 'bg-[#ff5c72]/5 border-[#ff5c72]/20' :
        a.severity === 'warning' ? 'bg-[#e8b830]/5 border-[#e8b830]/20' :
        'bg-a-bg border-a-border',
      )}
    >
      <div className="flex items-start gap-2.5">
        <HealthDot status={a.severity === 'critical' ? 'failed' : 'degraded'} size="md" className="mt-0.5" />
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 mb-0.5">
            <span className="text-xs font-semibold text-a-fg">{a.title}</span>
            <span className="text-[10px] px-1 py-0.5 rounded bg-a-border/30 text-a-muted">{a.workspace}</span>
          </div>
          <p className="text-xs text-a-fg2">{a.description}</p>
          {primaryObj && <MiniChain type={primaryObj.type} id={primaryObj.id} />}
        </div>
        <Timestamp iso={a.timestamp} />
      </div>
    </div>
  );
}

export default function CommandCenter() {
  const navigate = useNavigate();
  const { data, isLoading } = useQuery({
    queryKey: ['command-center'],
    queryFn: () => fetchDashboard(),
    refetchInterval: 30_000,
  });

  const d = data as DashboardData | undefined;
  const anomalies = API_CONFIG.useMock ? getScenario().anomalies : [];

  if (isLoading) {
    return (
      <div className="p-6">
        <PageHeader title="总控台" subtitle="加载中..." />
      </div>
    );
  }

  const hasIssues = anomalies.length > 0 || (d?.routes_unavailable || 0) > 0 || (d?.outdated_nodes || 0) > 0;

  return (
    <div className="p-6 space-y-6">
      <PageHeader
        title="Command Center"
        subtitle={hasIssues ? '⚠️ 系统存在需要注意的问题' : '✅ 所有系统运行正常'}
      />

      {/* Status Cards */}
      <div className="grid grid-cols-4 gap-3">
        <StatCard label="节点" value={`${d?.nodes_online || 0}/${d?.nodes_total || 0}`} sub="在线/总数"
          success={d != null && d.nodes_online === d.nodes_total && d.nodes_total > 0}
          warn={d != null && d.nodes_online < d.nodes_total && d.nodes_online > 0}
          danger={d != null && d.nodes_online === 0} />
        <StatCard label="网关" value={`${d?.gateways_online || 0}/${d?.gateways_total || 0}`} sub="活跃/总数"
          success={d != null && d.gateways_online === d.gateways_total && d.gateways_total > 0}
          warn={d != null && d.gateways_online < d.gateways_total && d.gateways_online > 0} />
        <StatCard label="路由" value={String(d?.managed_routes || 0)}
          sub={d?.routes_unavailable ? `${d.routes_unavailable} 不可用` : '全部可用'}
          danger={!!(d?.routes_unavailable)} />
        <StatCard label="待发布" value={String(d?.pending_capabilities?.length || 0)} sub="项变更"
          warn={!!(d?.pending_capabilities?.length)} />
      </div>

      {/* Anomalies */}
      {anomalies.length > 0 && (
        <Card title={`异常事件 (${anomalies.length})`} subtitle="点击跳转到对应页面">
          <div className="space-y-2">
            {anomalies.map(a => <AnomalyCard key={a.id} a={a} />)}
          </div>
        </Card>
      )}

      {/* Quick Actions */}
      <Card title="快速操作">
        <div className="grid grid-cols-5 gap-2">
          {QUICK_ACTIONS.map(a => (
            <button key={a.path} onClick={() => navigate(a.path)}
              className="p-4 rounded-a-md border border-a-border bg-a-bg hover:bg-a-border/20 text-center transition-colors cursor-pointer">
              <div className="text-sm font-semibold text-a-fg mb-1">{a.label}</div>
              <div className="text-[10px] text-a-muted">{a.desc}</div>
            </button>
          ))}
        </div>
      </Card>
    </div>
  );
}

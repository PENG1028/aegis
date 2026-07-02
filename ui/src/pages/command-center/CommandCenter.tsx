// ─── Command Center ───
// System health at a glance. Derives issues from real dashboard data.
// In mock mode, uses scenario anomalies. In production, derives from API data.

import { useQuery } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { fetchDashboard } from '@/lib/api-bridge';
import { cn } from '@/lib/utils';
import { Card, StatCard, HealthDot, StatusBadge, Btn, PageHeader } from '@/components/shared';
import { getScenario } from '@/mocks';
import { API_CONFIG } from '@/lib/api-config';
import type { DashboardData } from '@/types';
import type { Anomaly } from '@/types/workspace';

const QUICK_ACTIONS = [
  { label: '快速接入', desc: '创建域名映射', path: '/exposure/connect' },
  { label: '链路追踪', desc: '诊断请求路径', path: '/observe' },
  { label: '推送配置', desc: 'Apply 变更', path: '/release' },
  { label: '部署节点', desc: '添加新节点', path: '/runtime/deploy' },
  { label: '系统诊断', desc: '运行 Doctor', path: '/observe/doctor' },
];

interface Issue {
  id: string;
  title: string;
  description: string;
  severity: 'critical' | 'warning';
  workspace: string;
  targetPath: string;
}

function deriveIssues(d: DashboardData | undefined): Issue[] {
  const issues: Issue[] = [];
  if (!d) return issues;

  // Offline nodes
  const offlineCount = d.nodes_total - d.nodes_online;
  if (offlineCount > 0) {
    issues.push({
      id: 'nodes-offline', title: `${offlineCount} 个节点离线`,
      description: `${d.nodes_online}/${d.nodes_total} 节点在线`,
      severity: offlineCount === d.nodes_total ? 'critical' : 'warning',
      workspace: 'runtime', targetPath: '/runtime',
    });
  }

  // Unavailable routes
  if (d.routes_unavailable) {
    issues.push({
      id: 'routes-down', title: `${d.routes_unavailable} 条路由不可用`,
      description: '路由健康检查失败，流量可能受影响',
      severity: 'critical', workspace: 'exposure', targetPath: '/exposure',
    });
  }

  // Missing gateway links
  if (d.missing_gateway_links) {
    issues.push({
      id: 'missing-links', title: `缺少 ${d.missing_gateway_links} 条 Gateway Link`,
      description: '跨节点转发认证通道未建立',
      severity: 'warning', workspace: 'fabric', targetPath: '/fabric/links',
    });
  }

  // Outdated nodes
  if (d.outdated_nodes) {
    issues.push({
      id: 'outdated-nodes', title: `${d.outdated_nodes} 个节点版本过旧`,
      description: '节点需要更新二进制或配置',
      severity: 'warning', workspace: 'runtime', targetPath: '/runtime/updates',
    });
  }

  // Recent errors
  if (d.recent_errors?.length) {
    d.recent_errors.forEach((e: any, i: number) => {
      issues.push({
        id: `error-${i}`, title: `节点错误: ${e.node_name || e.node_id}`,
        description: e.error || '未知错误',
        severity: 'warning' as const, workspace: 'runtime',
        targetPath: `/runtime/node/${e.node_id}`,
      });
    });
  }

  // Pending capabilities
  if (d.pending_capabilities?.length) {
    issues.push({
      id: 'pending-caps', title: `${d.pending_capabilities.length} 项待发布变更`,
      description: '配置已修改但未推送到节点',
      severity: 'warning', workspace: 'release', targetPath: '/release',
    });
  }

  return issues;
}

export default function CommandCenter() {
  const navigate = useNavigate();
  const { data, isLoading } = useQuery({
    queryKey: ['command-center'],
    queryFn: () => fetchDashboard(),
    refetchInterval: 30_000,
  });

  const d = data as DashboardData | undefined;

  // Mock: use scenario anomalies. Production: derive from real data.
  const mockAnomalies: Anomaly[] = API_CONFIG.useMock ? getScenario().anomalies : [];
  const issues = API_CONFIG.useMock
    ? mockAnomalies.map(a => ({
        id: a.id, title: a.title, description: a.description,
        severity: a.severity as 'critical' | 'warning',
        workspace: a.workspace,
        targetPath: (() => {
          const obj = a.affectedObjects[0];
          if (!obj) return '/';
          return obj.type === 'node' ? `/runtime/node/${obj.id}`
            : obj.type === 'route' ? `/exposure/entry/${obj.id}`
            : obj.type === 'gateway' ? `/fabric/gateway/${obj.id}`
            : obj.type === 'service' ? `/exposure/service/${obj.id}`
            : a.workspace === 'release' ? '/release'
            : a.workspace === 'fabric' ? '/fabric'
            : '/';
        })(),
      }))
    : deriveIssues(d);

  const hasIssues = issues.length > 0;

  return (
    <div className="p-6 space-y-6">
      <PageHeader
        title="Command Center"
        subtitle={isLoading ? '加载中...' : hasIssues ? '⚠️ 系统存在需要注意的问题' : '✅ 所有系统运行正常'}
      />

      {/* Status Cards */}
      <div className="grid grid-cols-4 gap-3">
        <StatCard label="节点" value={`${d?.nodes_online || 0}/${d?.nodes_total || 0}`} sub="在线/总数"
          success={!!(d && d.nodes_online === d.nodes_total && d.nodes_total > 0)}
          warn={!!(d && d.nodes_online < d.nodes_total && d.nodes_online > 0)}
          danger={!!(d && d.nodes_online === 0)} />
        <StatCard label="网关" value={`${d?.gateways_online || 0}/${d?.gateways_total || 0}`} sub="活跃/总数"
          success={!!(d && d.gateways_online === d.gateways_total && d.gateways_total > 0)}
          warn={!!(d && d.gateways_online < d.gateways_total && d.gateways_online > 0)} />
        <StatCard label="路由" value={String(d?.managed_routes || 0)}
          sub={d?.routes_unavailable ? `${d.routes_unavailable} 不可用` : '全部可用'}
          danger={!!(d?.routes_unavailable)} />
        <StatCard label="待发布" value={String(d?.pending_capabilities?.length || 0)} sub="项变更"
          warn={!!(d?.pending_capabilities?.length)} />
      </div>

      {/* Issues / Anomalies */}
      {issues.length > 0 && (
        <Card title={API_CONFIG.useMock ? `异常事件 (${issues.length})` : `系统问题 (${issues.length})`} subtitle="点击跳转到对应页面">
          <div className="space-y-2">
            {issues.map(issue => (
              <div key={issue.id}
                onClick={() => navigate(issue.targetPath)}
                className={cn(
                  'p-3 rounded-a-sm border cursor-pointer transition-colors hover:brightness-110',
                  issue.severity === 'critical' ? 'bg-[#ff5c72]/5 border-[#ff5c72]/20' :
                  'bg-[#e8b830]/5 border-[#e8b830]/20',
                )}>
                <div className="flex items-start gap-2.5">
                  <HealthDot status={issue.severity === 'critical' ? 'failed' : 'degraded'} size="md" className="mt-0.5" />
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2 mb-0.5">
                      <span className="text-xs font-semibold text-a-fg">{issue.title}</span>
                      <span className="text-[10px] px-1 py-0.5 rounded bg-a-border/30 text-a-muted">{issue.workspace}</span>
                    </div>
                    <p className="text-xs text-a-fg2">{issue.description}</p>
                  </div>
                </div>
              </div>
            ))}
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

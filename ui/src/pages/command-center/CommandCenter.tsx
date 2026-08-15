// ─── Command Center ───
// System health at a glance. Derives issues from real dashboard data.
// In mock mode, uses scenario anomalies. In production, derives from API data.

import { useQuery } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { fetchDashboard } from '@/lib/api-bridge';
import { cn } from '@/lib/utils';
import { Card, StatCard, HealthDot, StatusBadge, Btn, PageHeader } from '@/components/shared';

import type { DashboardData } from '@/types';
import type { Anomaly } from '@/types/workspace';

const QUICK_ACTIONS = [
  { label: '快速接入', desc: '创建域名映射', path: '/exposure/new' },
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

  // Recent errors (real data — from node last_error)
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

  // NOTE: routes_unavailable / missing_gateway_links / outdated_nodes /
  // pending_capabilities have NO backend data source (gateway endpoints and
  // the desired-state subsystem were never implemented/removed). Reporting
  // them as 0 or as "pending" would be fake signals — they are intentionally
  // not pushed here.

  return issues;
}

export default function CommandCenter() {
  const navigate = useNavigate();
  const { data, isLoading } = useQuery({
    queryKey: ['command-center'],
    queryFn: () => fetchDashboard(),
    refetchInterval: 30_000,
  });

  const { data: sysStatus } = useQuery({
    queryKey: ['sys-status'],
    queryFn: () => fetch('/api/system/status').then(r => r.json()),
    refetchInterval: 60_000,
  });

  const d = data as DashboardData | undefined;

  const issues = deriveIssues(d);

  const hasIssues = issues.length > 0;
  const noData = !isLoading && d && d.nodes_total === 0 && d.managed_routes === 0;

  return (
    <div className="p-6 space-y-6">
      <PageHeader
        title="Command Center"
        subtitle={isLoading ? '加载中...' : hasIssues ? '⚠️ 系统存在需要注意的问题' : '✅ 所有系统运行正常'}
      />

      {/* Version & Build Info */}
      {sysStatus && (
        <div className="flex items-center gap-3 text-[11px] text-a-muted -mt-3 mb-2">
          <span>版本 <span className="font-mono text-a-fg">{sysStatus.version || '—'}</span></span>
          <span className="text-a-border/40">|</span>
          <span>构建于 <span className="font-mono text-a-fg">{sysStatus.build_time ? new Date(sysStatus.build_time).toLocaleString() : '—'}</span></span>
        </div>
      )}

      {/* First-run empty state */}
      {noData && (
        <Card title="欢迎使用 Aegis" subtitle="当前系统还没有数据，快速开始">
          <div className="grid grid-cols-3 gap-3">
            {[
              { label: '创建域名映射', desc: '添加第一个 HTTP 路由', path: '/exposure/new', icon: '→' },
              { label: '部署远程节点', desc: '添加第二台 VPS', path: '/runtime/deploy', icon: '⛁' },
              { label: '查看服务认证', desc: '管理服务间通信', path: '/auth', icon: '◈' },
            ].map(item => (
              <button key={item.path} onClick={() => navigate(item.path)}
                className="p-4 rounded-a-md border border-dashed border-a-border/40 bg-a-bg hover:bg-a-border/10 hover:border-a-accent/30 text-center transition-all cursor-pointer">
                <div className="text-lg mb-1">{item.icon}</div>
                <div className="text-sm font-semibold text-a-fg mb-0.5">{item.label}</div>
                <div className="text-[10px] text-a-muted">{item.desc}</div>
              </button>
            ))}
          </div>
        </Card>
      )}

      {/* Status Cards — only cards with REAL data sources are shown.
          Gateways/routing-sync/pending-capabilities have no backend source
          and are intentionally omitted (0/0 or "all ok" would be fake. */}
      <div className="grid grid-cols-3 gap-3">
        <StatCard label="节点" value={`${d?.nodes_online || 0}/${d?.nodes_total || 0}`} sub="在线/总数"
          success={!!(d && d.nodes_online === d.nodes_total && d.nodes_total > 0)}
          warn={!!(d && d.nodes_online < d.nodes_total && d.nodes_online > 0)}
          danger={!!(d && d.nodes_online === 0)} />
        <StatCard label="路由" value={String(d?.managed_routes || 0)} sub="已配置" />
        <StatCard label="节点错误" value={String(d?.recent_errors?.length || 0)} sub="最近错误"
          warn={!!(d?.recent_errors?.length)} />
      </div>

      {/* Issues / Anomalies */}
      {issues.length > 0 && (
        <Card title={`系统问题 (${issues.length})`} subtitle="点击跳转到对应页面">
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

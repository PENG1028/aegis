import { useQuery } from '@tanstack/react-query';
import { fetchPolicies } from '@/lib/api-bridge';
import { PageHeader, Card, DataTable, StatusBadge, Alert } from '@/components/shared';
import type { DataTableColumn } from '@/components/shared';
import type { GatewayPolicy } from '@/types';

const columns: DataTableColumn<GatewayPolicy>[] = [
  { key: 'target_name', label: '目标', mono: true },
  {
    key: 'target_type',
    label: '类型',
    render: (row) => <span className="font-mono text-[11px] px-2 py-0.5 rounded bg-a-border/40 text-a-muted">{row.target_type}</span>,
  },
  {
    key: 'mode',
    label: '模式',
    render: (row) => <StatusBadge status={row.mode} />,
  },
  {
    key: 'allow_local',
    label: '本地',
    render: (r) => r.allow_local ? <span className="text-[#4cd964]">✓</span> : <span className="text-a-muted">—</span>,
  },
  {
    key: 'allow_private',
    label: '内网',
    render: (r) => r.allow_private ? <span className="text-[#4cd964]">✓</span> : <span className="text-a-muted">—</span>,
  },
  {
    key: 'allow_public',
    label: '公网',
    render: (r) => r.allow_public ? <span className="text-[#e8b830]">✓</span> : <span className="text-a-muted">—</span>,
  },
  { key: 'require_gateway_link', label: '需 GWLink', render: (r) => r.require_gateway_link ? '✓' : '—' },
  { key: 'require_relay', label: '需中继', render: (r) => r.require_relay ? '✓' : '—' },
  {
    key: 'tls_mode',
    label: 'TLS',
    render: (row) => <StatusBadge status={row.tls_mode} />,
  },
  {
    key: 'enabled',
    label: '启用',
    render: (row) => row.enabled ? <span className="text-[#4cd964]">✓</span> : <span className="text-a-muted">✗</span>,
  },
];

export default function GatewayPoliciesPage() {
  const { data, isLoading, error } = useQuery({
    queryKey: ['policies'],
    queryFn: fetchPolicies,
  });

  if (isLoading) return <div className="text-center py-10 text-a-muted font-mono text-sm">加载中...</div>;
  if (error) return <div className="px-4 py-3 rounded-a-md text-xs border bg-[#ff5c72]/10 text-[#ff5c72] border-[#ff5c72]/20">加载失败: {error.message}</div>;

  return (
    <div>
      <PageHeader title="网关策略" helpKey="policies" subtitle="Service / Route 网关策略配置"  />

      <Alert type="info">
        <div className="flex-1">
          <p className="font-medium mb-1">策略模式说明</p>
          <ul className="space-y-0.5 text-a-fg2">
            <li><strong className="text-a-fg">auto</strong> — Aegis 根据 topology / gateway inventory / GatewayLink 自动选择</li>
            <li><strong className="text-a-fg">fixed</strong> — 只走 primary_gateway，primary 不可用时 unavailable，不 fallback direct</li>
            <li><strong className="text-a-fg">multi</strong> — 先走 primary，失败后按 fallback 顺序，全部失败则 unavailable，不 fallback direct</li>
            <li><strong className="text-a-fg">disabled</strong> — 不生成可用 routing entry</li>
          </ul>
          <p className="mt-2 text-[#ff5c72]">任何模式都不能 fallback 到 remote target_host:target_port</p>
        </div>
      </Alert>

      <Card>
        <DataTable columns={columns} data={data || []} keyExtractor={(r) => r.id} />
      </Card>
    </div>
  );
}

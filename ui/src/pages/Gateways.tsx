import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { fetchGateways, gatewayApi } from '@/lib/api-bridge';
import { PageHeader, Card, DataTable, StatusBadge } from '@/components/shared';
import { useToast } from '@/components/shared/Toast';
import type { DataTableColumn } from '@/components/shared';
import type { Gateway } from '@/types';
import { fmtRel } from '@/lib/utils';

export default function GatewaysPage() {
  const navigate = useNavigate();
  const toast = useToast();
  const qc = useQueryClient();
  (window as any)._navigate = navigate;

  const { data, isLoading, error } = useQuery({
    queryKey: ['gateways'],
    queryFn: fetchGateways,
  });

  const toggleGateway = async (id: string, enabled: boolean) => {
    try {
      await gatewayApi.update(id, { enabled: !enabled });
      toast(`${enabled ? '已禁用' : '已启用'} ${id}`);
      qc.invalidateQueries({ queryKey: ['gateways'] });
    } catch (e: any) { toast(e.message, 'error'); }
  };

  const columns: DataTableColumn<Gateway>[] = [
    {
      key: 'gateway_id',
      label: '网关 ID',
      mono: true,
      render: (row) => (
        <button className="text-a-accent font-mono text-xs bg-transparent border-none cursor-pointer p-0 hover:underline"
          onClick={() => window._navigate?.(`/gateways/${row.gateway_id}`)}>
          {row.gateway_id}
        </button>
      ),
    },
    { key: 'name', label: '名称' },
    { key: 'node_name', label: '节点', muted: true },
    {
      key: 'type',
      label: '类型',
      render: (row) => <StatusBadge status={row.type === 'local' ? 'local_gateway' : row.type === 'private' ? 'private_gateway' : 'public_gateway'} />,
    },
    { key: 'provider', label: '提供商', render: (row) => <span className="font-mono text-[11px] px-2 py-0.5 rounded bg-a-border/40 text-a-muted">{row.provider}</span> },
    { key: 'host', label: '主机', mono: true, muted: true },
    { key: 'port', label: '端口', mono: true },
    {
      key: 'public_accessible',
      label: '公网',
      render: (row) => row.public_accessible ? <span className="text-[#e8b830]">✓</span> : <span className="text-a-muted">—</span>,
    },
    {
      key: 'status',
      label: '状态',
      render: (row) => <StatusBadge status={row.status} />,
    },
    {
      key: 'last_verified_at',
      label: '验证',
      mono: true,
      muted: true,
      render: (row) => fmtRel(row.last_verified_at),
    },
    {
      key: 'enabled',
      label: '开关',
      render: (row) => (
        <button
          onClick={() => toggleGateway(row.gateway_id, row.enabled)}
          className={`font-mono text-[11px] px-2.5 py-1 rounded-a-sm border cursor-pointer transition-colors ${
            row.enabled
              ? 'bg-[#4cd964]/15 text-[#4cd964] border-[#4cd964]/30 hover:bg-[#4cd964]/25'
              : 'bg-a-border/30 text-a-muted border-a-border hover:bg-a-border/50'
          }`}>
          {row.enabled ? 'ON' : 'OFF'}
        </button>
      ),
    },
  ];

  if (isLoading) return <div className="text-center py-10 text-a-muted font-mono text-sm">加载中...</div>;
  if (error) return <div className="px-4 py-3 rounded-a-md text-xs border bg-[#ff5c72]/10 text-[#ff5c72] border-[#ff5c72]/20">加载失败: {error.message}</div>;

  return (
    <div>
      <PageHeader title="网关" helpKey="gateways" subtitle={`${data?.length || 0} 个网关`}  />
      <Card>
        <DataTable columns={columns} data={data || []} keyExtractor={(r) => r.gateway_id} />
      </Card>
    </div>
  );
}

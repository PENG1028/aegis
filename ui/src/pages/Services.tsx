import { useQuery } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { fetchServices } from '@/lib/api-bridge';
import { PageHeader, Card, DataTable, StatusBadge } from '@/components/shared';
import type { DataTableColumn } from '@/components/shared';
import type { Service } from '@/types';

const columns: DataTableColumn<Service>[] = [
  {
    key: 'name',
    label: '名称',
    mono: true,
    render: (row) => (
      <button className="text-a-accent font-mono text-xs bg-transparent border-none cursor-pointer p-0 hover:underline"
        onClick={() => window._navigate?.(`/services/${row.service_id}`)}>
        {row.name}
      </button>
    ),
  },
  { key: 'service_id', label: 'Service ID', mono: true, muted: true },
  { key: 'kind', label: '类型', render: (row) => <span className="font-mono text-[11px] px-2 py-0.5 rounded bg-a-border/40 text-a-muted">{row.kind}</span> },
  { key: 'upstream_url', label: '上游', mono: true, muted: true },
  {
    key: 'health_status',
    label: '健康',
    render: (row) => <StatusBadge status={row.health_status} />,
  },
  {
    key: 'latency_ms',
    label: '延迟',
    mono: true,
    render: (row) => row.latency_ms ? `${row.latency_ms}ms` : '—',
  },
  {
    key: 'status',
    label: '状态',
    render: (row) => <StatusBadge status={row.status} />,
  },
  { key: 'routes_count', label: 'Routes', mono: true },
  { key: 'endpoints_count', label: 'Endpoints', mono: true },
];

export default function ServicesPage() {
  const navigate = useNavigate();
  (window as any)._navigate = navigate;

  const { data, isLoading, error } = useQuery({
    queryKey: ['services'],
    queryFn: fetchServices,
  });

  if (isLoading) return <div className="text-center py-10 text-a-muted font-mono text-sm">加载中...</div>;
  if (error) return <div className="px-4 py-3 rounded-a-md text-xs border bg-[#ff5c72]/10 text-[#ff5c72] border-[#ff5c72]/20">加载失败: {error.message}</div>;

  return (
    <div>
      <PageHeader title="Services" helpKey="services" subtitle={`${data?.length || 0} 个服务`}  />
      <Card>
        <DataTable columns={columns} data={data || []} keyExtractor={(r) => r.service_id} />
      </Card>
    </div>
  );
}

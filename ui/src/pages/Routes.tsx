import { useQuery } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { fetchRoutes } from '@/lib/api-bridge';
import { PageHeader, Card, DataTable, StatusBadge } from '@/components/shared';
import type { DataTableColumn } from '@/components/shared';
import type { Route } from '@/types';

const columns: DataTableColumn<Route>[] = [
  {
    key: 'domain',
    label: '域名',
    mono: true,
    render: (row) => (
      <button className="text-a-accent font-mono text-xs bg-transparent border-none cursor-pointer p-0 hover:underline"
        onClick={() => window._navigate?.(`/routes/${row.route_id}`)}>
        {row.domain}
      </button>
    ),
  },
  { key: 'route_id', label: '路由 ID', mono: true, muted: true },
  { key: 'service_name', label: '服务', muted: true },
  {
    key: 'tls_mode',
    label: 'TLS',
    render: (row) => <StatusBadge status={row.tls_mode} />,
  },
  {
    key: 'public_allowed',
    label: '公网允许',
    render: (row) => row.public_allowed
      ? <span className="text-[#e8b830] text-xs">✓</span>
      : <span className="text-a-muted text-xs">✗</span>,
  },
  { key: 'preserve_host', label: '保留 Host', render: (row) => row.preserve_host ? '✓' : '✗' },
  {
    key: 'status',
    label: '状态',
    render: (row) => <StatusBadge status={row.status} />,
  },
];

export default function RoutesPage() {
  const navigate = useNavigate();
  (window as any)._navigate = navigate;

  const { data, isLoading, error } = useQuery({
    queryKey: ['routes'],
    queryFn: fetchRoutes,
  });

  if (isLoading) return <div className="text-center py-10 text-a-muted font-mono text-sm">加载中...</div>;
  if (error) return <div className="px-4 py-3 rounded-a-md text-xs border bg-[#ff5c72]/10 text-[#ff5c72] border-[#ff5c72]/20">加载失败: {error.message}</div>;

  return (
    <div>
      <PageHeader title="路由" helpKey="routes" subtitle={`${data?.length || 0} 条路由`}  />
      <Card>
        <DataTable columns={columns} data={data || []} keyExtractor={(r) => r.route_id} />
      </Card>
    </div>
  );
}

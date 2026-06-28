import { useQuery } from '@tanstack/react-query';
import { useParams, useNavigate } from 'react-router-dom';
import { fetchRouteDetail } from '@/lib/api-bridge';
import { PageHeader, Card, MetaRow, StatusBadge } from '@/components/shared';

export default function RouteDetailPage() {
  const { routeId } = useParams<{ routeId: string }>();
  const navigate = useNavigate();
  const { data, isLoading, error } = useQuery({
    queryKey: ['route', routeId],
    queryFn: () => fetchRouteDetail(routeId!),
    enabled: !!routeId,
  });

  if (isLoading) return <div className="text-center py-10 text-a-muted font-mono text-sm">加载中...</div>;
  if (error) return <div className="text-center py-10 text-a-danger text-sm">加载失败: {error.message}</div>;
  if (!data) return <div className="text-center py-10 text-a-danger text-sm">Route not found</div>;

  return (
    <div>
      <button onClick={() => navigate('/routes')} className="inline-flex items-center gap-1 text-xs text-a-muted hover:text-a-fg mb-3 bg-transparent border-none cursor-pointer p-0">← Routes</button>
      <PageHeader title={data.domain} subtitle={`route_id: ${data.route_id}`} helpKey="routes" />

      <div className="grid grid-cols-2 gap-4 mb-4">
        <Card title="基本信息">
          <MetaRow label="Route ID" value={data.route_id} mono color="text-a-accent" />
          <MetaRow label="Domain" value={data.domain} mono />
          <MetaRow label="Service" value={
            <button className="text-a-accent font-mono bg-transparent border-none cursor-pointer p-0 hover:underline"
              onClick={() => navigate(`/services/${data.service_id}`)}>
              {data.service_name || data.service_id}
            </button>
          } />
          <MetaRow label="Scope" value={data.scope_id || '—'} />
          <MetaRow label="Status" value={<StatusBadge status={data.status} />} />
          <MetaRow label="TLS Mode" value={<StatusBadge status={data.tls_mode} />} />
          <MetaRow label="Public Allowed" value={data.public_allowed ? '✓' : '✗'} />
          <MetaRow label="Preserve Host" value={data.preserve_host ? '✓' : '✗'} />
        </Card>

        <Card title="路由状态">
          <MetaRow label="Routing Status" value={<StatusBadge status={data.routing_status} />} />
          <MetaRow label="Policy" value={data.policy_summary} />
        </Card>
      </div>

      {data.endpoint && (
        <Card title="关联 Endpoint" className="mb-4">
          <MetaRow label="Endpoint" value={
            <span className="font-mono text-a-muted">{data.endpoint.endpoint_id}</span>
          } mono={false} />
          <MetaRow label="Node" value={data.endpoint.node_name || data.endpoint.node_id} />
          <MetaRow label="Target" value={`${data.endpoint.target_local_host}:${data.endpoint.target_local_port}`} mono />
          <MetaRow label="Health" value={<StatusBadge status={data.endpoint.health_status} />} />
          <MetaRow label="Relay Eligible" value={data.endpoint.relay_eligible ? '✓' : '✗'} />
        </Card>
      )}
    </div>
  );
}

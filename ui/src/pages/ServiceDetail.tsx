import { useQuery } from '@tanstack/react-query';
import { useParams, useNavigate } from 'react-router-dom';
import { fetchServiceDetail } from '@/lib/api-bridge';
import { PageHeader, Card, MetaRow, StatusBadge } from '@/components/shared';

export default function ServiceDetailPage() {
  const { serviceId } = useParams<{ serviceId: string }>();
  const navigate = useNavigate();
  const { data, isLoading, error } = useQuery({
    queryKey: ['service', serviceId],
    queryFn: () => fetchServiceDetail(serviceId!),
    enabled: !!serviceId,
  });

  if (isLoading) return <div className="text-center py-10 text-a-muted font-mono text-sm">加载中...</div>;
  if (error) return <div className="text-center py-10 text-a-danger text-sm">加载失败: {error.message}</div>;
  if (!data) return <div className="text-center py-10 text-a-danger text-sm">服务未找到</div>;

  return (
    <div>
      <button onClick={() => navigate('/services')} className="inline-flex items-center gap-1 text-xs text-a-muted hover:text-a-fg mb-3 bg-transparent border-none cursor-pointer p-0">← 服务</button>
      <PageHeader title={data.name} subtitle={`service_id: ${data.service_id}`} helpKey="services" />

      <div className="grid grid-cols-2 gap-4 mb-4">
        <Card title="基本信息">
          <MetaRow label="服务 ID" value={data.service_id} mono color="text-a-accent" />
          <MetaRow label="名称" value={data.name} mono />
          <MetaRow label="类型" value={data.kind} />
          <MetaRow label="作用域" value={data.scope_id || '—'} mono />
          <MetaRow label="上游地址" value={data.upstream_url || '—'} mono />
          <MetaRow label="健康检查" value={data.health_check_url || '—'} mono />
          <MetaRow label="状态" value={<StatusBadge status={data.status} />} />
        </Card>

        <Card title="路由与端点">
          {data.routes.map((r) => (
            <div key={r.route_id} className="flex items-center gap-2 py-1.5 border-b border-a-border-soft text-xs">
              <button className="text-a-accent font-mono bg-transparent border-none cursor-pointer p-0 hover:underline"
                onClick={() => navigate(`/routes/${r.route_id}`)}>
                {r.route_id}
              </button>
              <span className="text-a-muted">{r.domain}</span>
              <span className="ml-auto"><StatusBadge status={r.status} /></span>
            </div>
          ))}
          <div className="mt-2 text-xs text-a-muted">{data.endpoints.length} 个端点</div>
        </Card>
      </div>

      <Card title="端点" className="mb-4">
        {data.endpoints.length > 0 ? (
          <div className="space-y-2">
            {data.endpoints.map((ep) => (
              <div key={ep.endpoint_id} className="flex items-center gap-3 py-2 border-b border-a-border-soft last:border-b-0 text-xs">
                <button className="font-mono text-a-accent bg-transparent border-none cursor-pointer p-0 hover:underline"
                  onClick={() => navigate(`/endpoints/${ep.endpoint_id}`)}>
                  {ep.endpoint_id}
                </button>
                <span className="text-a-muted font-mono">{ep.node_name || ep.node_id}</span>
                <span className="font-mono">{ep.target_local_host}:{ep.target_local_port}</span>
                <span className="ml-auto"><StatusBadge status={ep.health_status} /></span>
              </div>
            ))}
          </div>
        ) : (
          <div className="text-center py-6 text-a-muted text-xs">无 endpoints</div>
        )}
      </Card>

      {data.gateway_policy && (
        <Card title="网关策略">
          <MetaRow label="模式" value={data.gateway_policy.mode} />
          <MetaRow label="需网关链接" value={data.gateway_policy.require_gateway_link ? '✓' : '✗'} />
          <MetaRow label="需中继" value={data.gateway_policy.require_relay ? '✓' : '✗'} />
          <MetaRow label="允许本地/内网/公网" value={`${data.gateway_policy.allow_local ? '✓' : '✗'} / ${data.gateway_policy.allow_private ? '✓' : '✗'} / ${data.gateway_policy.allow_public ? '✓' : '✗'}`} />
          <MetaRow label="保留 Host" value={data.gateway_policy.preserve_host ? '✓' : '✗'} />
          <MetaRow label="TLS 模式" value={data.gateway_policy.tls_mode} />
        </Card>
      )}
    </div>
  );
}

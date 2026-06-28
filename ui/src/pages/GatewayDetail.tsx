import { useQuery } from '@tanstack/react-query';
import { useParams, useNavigate } from 'react-router-dom';
import { fetchGatewayDetail } from '@/lib/api-bridge';
import { PageHeader, Card, MetaRow, StatusBadge } from '@/components/shared';
import { fmtRel } from '@/lib/utils';

export default function GatewayDetailPage() {
  const { gatewayId } = useParams<{ gatewayId: string }>();
  const navigate = useNavigate();
  const { data, isLoading, error } = useQuery({
    queryKey: ['gateway', gatewayId],
    queryFn: () => fetchGatewayDetail(gatewayId!),
    enabled: !!gatewayId,
  });

  if (isLoading) return <div className="text-center py-10 text-a-muted font-mono text-sm">加载中...</div>;
  if (error) return <div className="text-center py-10 text-a-danger text-sm">加载失败: {error.message}</div>;
  if (!data) return <div className="text-center py-10 text-a-danger text-sm">网关未找到</div>;

  return (
    <div>
      <button onClick={() => navigate('/gateways')} className="inline-flex items-center gap-1 text-xs text-a-muted hover:text-a-fg mb-3 bg-transparent border-none cursor-pointer p-0">← 网关</button>
      <PageHeader title={data.name} subtitle={`gateway_id: ${data.gateway_id}`} helpKey="gateways" />

      <div className="grid grid-cols-2 gap-4 mb-4">
        <Card title="基本信息">
          <MetaRow label="网关 ID" value={data.gateway_id} mono color="text-a-accent" />
          <MetaRow label="名称" value={data.name} />
          <MetaRow label="节点" value={
            <button className="text-a-accent font-mono bg-transparent border-none cursor-pointer p-0 hover:underline"
              onClick={() => navigate(`/nodes/${data.node_id}`)}>
              {data.node_name || data.node_id}
            </button>
          } />
          <MetaRow label="类型" value={<StatusBadge status={data.type === 'local' ? 'local_gateway' : data.type === 'private' ? 'private_gateway' : 'public_gateway'} />} />
          <MetaRow label="提供商" value={data.provider} />
          <MetaRow label="绑定地址" value={`${data.bind_addr}:${data.port}`} mono />
          <MetaRow label="协议" value={data.scheme} mono />
        </Card>

        <Card title="访问控制">
          <MetaRow label="公网可访问" value={data.public_accessible ? '✓ 是' : '✗ 否'} color={data.public_accessible ? 'text-[#e8b830]' : ''} />
          <MetaRow label="内网可访问" value={data.private_accessible ? '✓ 是' : '✗ 否'} color={data.private_accessible ? 'text-[#4cd964]' : ''} />
          <MetaRow label="已启用" value={data.enabled ? '✓' : '✗'} />
          <MetaRow label="优先级" value={data.priority} mono />
          <MetaRow label="状态" value={<StatusBadge status={data.status} />} />
          <MetaRow label="服务路由数" value={data.routes_served} mono />
        </Card>
      </div>

      {data.gateway_links.length > 0 && (
        <Card title="关联 GatewayLinks" className="mb-4">
          {data.gateway_links.map((gl) => (
            <div key={gl.gateway_link_id} className="flex items-center gap-3 py-2 border-b border-a-border-soft last:border-b-0 text-xs">
              <span className="font-mono text-a-accent">{gl.gateway_link_id}</span>
              <span className="text-a-muted">{gl.source_node_id} → {gl.target_node_id}</span>
              <span className="ml-auto"><StatusBadge status={gl.status} /></span>
            </div>
          ))}
        </Card>
      )}

      {data.last_error && (
        <div className="px-4 py-3 rounded-a-md text-xs border bg-[#ff5c72]/10 text-[#ff5c72] border-[#ff5c72]/20">
          上次错误: {data.last_error}
        </div>
      )}
      <MetaRow label="上次验证" value={fmtRel(data.last_verified_at)} mono className="mt-2" />
    </div>
  );
}

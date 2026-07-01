import { useQuery } from '@tanstack/react-query';
import { useNavigate, useParams } from 'react-router-dom';
import { gatewayLinkApi } from '@/lib/api-bridge';
import { PageHeader, Card, Btn, Alert, StatusBadge, MetaRow } from '@/components/shared';
import { useToast } from '@/components/shared/Toast';
import { fmtDate } from '@/lib/utils';

export default function GatewayLinkDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const toast = useToast();

  const { data, isLoading, error } = useQuery({
    queryKey: ['gateway-link', id],
    queryFn: () => gatewayLinkApi.get(id!),
    enabled: !!id,
  });

  if (isLoading) return <div className="text-center py-10 text-a-muted font-mono text-sm">加载中...</div>;
  if (error) return <Alert type="err">加载失败: {(error as any).message}</Alert>;
  if (!data) return <div className="text-center py-10 text-a-muted text-xs">网关链接未找到</div>;

  return (
    <div>
      <button onClick={() => navigate('/gateway-links')}
        className="inline-flex items-center gap-1 text-xs text-a-muted hover:text-a-fg mb-3 bg-transparent border-none cursor-pointer p-0">
        ← 网关链接
      </button>
      <PageHeader title={data.name || data.id} sub={`Link: ${data.id}`} actions={
        <Btn sm danger onClick={async () => {
          try {
            const res = await gatewayLinkApi.rotate(data.id);
            toast('已轮换');
          } catch (e: any) { toast(e.message, 'error'); }
        }}>轮换</Btn>
      } />

      <div className="grid grid-cols-2 gap-4">
        <Card title="基本信息">
          <div className="p-[18px]">
            <MetaRow label="链接 ID" value={data.id} mono color="text-a-accent" />
            <MetaRow label="名称" value={data.name} />
            <MetaRow label="主机" value={data.host} mono />
            <MetaRow label="内网 IP" value={data.private_ip || '—'} mono />
            <MetaRow label="端口" value={data.port} mono />
            <MetaRow label="类型" value={data.gateway_type || data.auth_type} />
            <MetaRow label="目标节点" value={data.target_node_id || '—'} mono />
            <MetaRow label="自动路由" value={data.auto_route ? '✓ 是' : '✗ 否'} />
            <MetaRow label="状态" value={<StatusBadge status={data.status} />} />
          </div>
        </Card>
        <Card title="密钥">
          <div className="p-[18px]">
            <MetaRow label="认证方式" value={data.auth_type} />
            <MetaRow label="密钥版本" value={data.secret_version} mono />
            <MetaRow label="密钥创建时间" value={data.secret_created_at ? fmtDate(data.secret_created_at) : '—'} />
            <MetaRow label="上次轮换" value={data.secret_rotated_at ? fmtDate(data.secret_rotated_at) : '—'} />
          </div>
        </Card>
      </div>
    </div>
  );
}

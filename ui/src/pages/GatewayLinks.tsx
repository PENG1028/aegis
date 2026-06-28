import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate, useParams } from 'react-router-dom';
import { gatewayLinkApi } from '@/lib/api-bridge';
import { PageHeader, Card, StatCard, Btn, Alert, Modal, StatusBadge, MetaRow } from '@/components/shared';
import { useToast } from '@/components/shared/Toast';
import { fmtDate } from '@/lib/utils';

export function GatewayLinksPage() {
  const navigate = useNavigate();
  const toast = useToast();
  const [showCreate, setShowCreate] = useState(false);
  const [tokenModal, setTokenModal] = useState<any>(null);

  const { data, isLoading, error } = useQuery({
    queryKey: ['gateway-links'],
    queryFn: () => gatewayLinkApi.list(),
  });

  const links = data || [];

  return (
    <div>
      <PageHeader title="网关链接" helpKey="gateway-links" sub="跨节点网关认证" actions={
        <Btn primary onClick={() => setShowCreate(true)}>+ 创建</Btn>
      } />

      {isLoading && <div className="text-center py-10 text-a-muted font-mono text-sm">加载中...</div>}
      {error && <Alert type="err">加载失败: {(error as any).message}</Alert>}

      {!isLoading && (
        <>
          <div className="grid grid-cols-3 gap-3 mb-5">
            <StatCard label="总数" value={links.length} accent />
            <StatCard label="活跃" value={links.filter((l: any) => l.status === 'active').length} success />
            <StatCard label="错误" value={links.filter((l: any) => l.status === 'error').length} danger />
          </div>

          <Card>
            <table className="w-full text-sm border-collapse">
              <thead>
                <tr>
                  {['链接 ID', '名称', '主机', '类型', '版本', '状态'].map((h) => (
                    <th key={h} className="text-left px-3.5 py-2.5 text-[11px] font-semibold uppercase tracking-[0.06em] text-a-muted bg-a-bg border-b border-a-border whitespace-nowrap">{h}</th>
                  ))}
                  <th className="text-left px-3.5 py-2.5 text-[11px] font-semibold uppercase tracking-[0.06em] text-a-muted bg-a-bg border-b border-a-border whitespace-nowrap">操作</th>
                </tr>
              </thead>
              <tbody>
                {links.map((l: any) => (
                  <tr key={l.id} className="hover:bg-white/[0.04] [&:last-child>td]:border-b-0">
                    <td className="px-3.5 py-2.5 border-b border-a-border-soft">
                      <button className="font-mono text-xs text-a-accent bg-transparent border-none cursor-pointer p-0 hover:underline"
                        onClick={() => navigate(`/gateway-links/${l.id}`)}>
                        {l.id}
                      </button>
                    </td>
                    <td className="px-3.5 py-2.5 border-b border-a-border-soft text-xs">{l.name}</td>
                    <td className="px-3.5 py-2.5 border-b border-a-border-soft font-mono text-xs">{l.host}</td>
                    <td className="px-3.5 py-2.5 border-b border-a-border-soft text-xs">{l.gateway_type || l.auth_type}</td>
                    <td className="px-3.5 py-2.5 border-b border-a-border-soft font-mono text-xs">{l.secret_version || '-'}</td>
                    <td className="px-3.5 py-2.5 border-b border-a-border-soft"><StatusBadge status={l.status} /></td>
                    <td className="px-3.5 py-2.5 border-b border-a-border-soft">
                      <div className="flex gap-1">
                        <Btn sm onClick={() => navigate(`/gateway-links/${l.id}`)}>详情</Btn>
                        <Btn sm danger onClick={async () => {
                          try {
                            const res = await gatewayLinkApi.rotate(l.id);
                            setTokenModal(res);
                            toast('已轮换');
                          } catch (e: any) { toast(e.message, 'error'); }
                        }}>轮换</Btn>
                      </div>
                    </td>
                  </tr>
                ))}
                {links.length === 0 && (
                  <tr><td colSpan={7} className="text-center py-10 text-a-muted text-xs">暂无网关链接</td></tr>
                )}
              </tbody>
            </table>
          </Card>
        </>
      )}

      {showCreate && (
        <CreateLinkModal onClose={() => setShowCreate(false)} onCreated={(res) => { setTokenModal(res); setShowCreate(false); }} />
      )}

      {tokenModal && (
        <Modal title="Token — 仅显示一次" onClose={() => setTokenModal(null)}
          footer={<Btn primary onClick={() => { navigator.clipboard.writeText(tokenModal.secret || tokenModal.raw_join_token || ''); toast('已复制'); }}>复制</Btn>}>
          <Alert type="warn">此 token 仅显示一次。关闭后无法再次查看。</Alert>
          <div className="font-mono text-sm bg-a-bg border border-a-border rounded-a-sm p-3 break-all select-all mt-3">
            {tokenModal.secret || tokenModal.raw_join_token || '—'}
          </div>
        </Modal>
      )}
    </div>
  );
}

function CreateLinkModal({ onClose, onCreated }: { onClose: () => void; onCreated: (res: any) => void }) {
  const [source, setSource] = useState('node-a');
  const [target, setTarget] = useState('node-b');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function doCreate() {
    setLoading(true);
    setError(null);
    try {
      const res = await gatewayLinkApi.create({ source_node_id: source, target_node_id: target });
      onCreated(res);
    } catch (e: any) { setError(e.message); }
    setLoading(false);
  }

  return (
    <Modal title="创建 Gateway Link" onClose={onClose}
      footer={<><Btn onClick={onClose}>取消</Btn><Btn primary onClick={doCreate} disabled={loading}>创建</Btn></>}>
      {error && <Alert type="err">{error}</Alert>}
      <div className="mb-3">
        <label className="block text-xs font-medium text-a-muted mb-1.5">源节点</label>
        <input className="w-full font-mono text-sm px-3 py-2 rounded-a-sm border border-a-border bg-a-bg text-a-fg outline-none focus:border-a-accent"
          value={source} onChange={(e) => setSource(e.target.value)} />
      </div>
      <div className="mb-3">
        <label className="block text-xs font-medium text-a-muted mb-1.5">目标节点</label>
        <input className="w-full font-mono text-sm px-3 py-2 rounded-a-sm border border-a-border bg-a-bg text-a-fg outline-none focus:border-a-accent"
          value={target} onChange={(e) => setTarget(e.target.value)} />
      </div>
    </Modal>
  );
}

export function GatewayLinkDetailPage() {
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

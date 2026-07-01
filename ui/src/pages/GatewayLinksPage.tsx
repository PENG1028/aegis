import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { gatewayLinkApi } from '@/lib/api-bridge';
import { PageHeader, Card, StatCard, Btn, Alert, Modal, StatusBadge } from '@/components/shared';
import { useToast } from '@/components/shared/Toast';
import { CreateLinkModal } from '@/components/gateways/CreateLinkModal';

export default function GatewayLinksPage() {
  const navigate = useNavigate();
  const toast = useToast();
  const qc = useQueryClient();
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
                        <Btn sm danger onClick={async () => {
                          if (!confirm(`确定删除 ${l.id}？`)) return;
                          try {
                            await gatewayLinkApi.delete(l.id);
                            toast('已删除');
                            qc.invalidateQueries({ queryKey: ['gateway-links'] });
                          } catch (e: any) { toast(e.message, 'error'); }
                        }}>删除</Btn>
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

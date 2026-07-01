import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { adminApi } from '@/lib/api-bridge';
import { PageHeader, Card, Btn, Alert, Modal, StatusBadge } from '@/components/shared';
import { useToast } from '@/components/shared/Toast';
import { SelectScopeModal } from '@/components/scopes/SelectScopeModal';

export default function ApiKeysPage() {
  const toast = useToast();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [scopeFilter, setScopeFilter] = useState('');
  const [showCreate, setShowCreate] = useState(false);
  const [plainKey, setPlainKey] = useState<any>(null);

  const { data: keysData } = useQuery({
    queryKey: ['api-keys', scopeFilter],
    queryFn: () => adminApi.listApiKeys(scopeFilter || undefined),
  });

  const { data: scopesData } = useQuery({
    queryKey: ['scopes'],
    queryFn: () => adminApi.listScopes(),
  });

  const keys = keysData?.api_keys || [];
  const scopes = scopesData?.spaces || [];

  async function doRevoke(id: string) {
    if (!confirm('确定撤销此 API Key？')) return;
    try {
      await adminApi.revokeApiKey(id);
      toast('已撤销');
      queryClient.invalidateQueries({ queryKey: ['api-keys'] });
    } catch (e: any) { toast(e.message, 'error'); }
  }

  async function doCreate(scopeId: string, name: string) {
    try {
      const res = await adminApi.createApiKey(scopeId, name);
      setPlainKey(res);
      setShowCreate(false);
      queryClient.invalidateQueries({ queryKey: ['api-keys'] });
    } catch (e: any) { toast(e.message, 'error'); }
  }

  return (
    <div>
      <PageHeader title="API 密钥" helpKey="api-keys" sub="每个 API Key 绑定到一个 Scope，隔离资源访问" actions={
        <><select className="font-mono text-xs px-2.5 py-1.5 rounded-a-sm border border-a-border bg-a-bg text-a-fg outline-none mr-2"
          value={scopeFilter} onChange={(e) => setScopeFilter(e.target.value)}>
          <option value="">全部作用域</option>
          {scopes.map((s: any) => <option key={s.id} value={s.space_id || s.id}>{s.name}</option>)}
        </select><Btn primary onClick={() => setShowCreate(true)}>+ 创建</Btn></>
      } />

      <Card>
        <table className="w-full text-sm border-collapse">
          <thead>
            <tr>
              {['前缀', '作用域', '类型', '状态', '操作'].map((h) => (
                <th key={h} className="text-left px-3.5 py-2.5 text-[11px] font-semibold uppercase tracking-[0.06em] text-a-muted bg-a-bg border-b border-a-border whitespace-nowrap">{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {keys.map((k: any) => (
              <tr key={k.id} className="hover:bg-white/[0.04]">
                <td className="px-3.5 py-2.5 border-b border-a-border-soft font-mono text-xs">{k.key_prefix || k.name}</td>
                <td className="px-3.5 py-2.5 border-b border-a-border-soft text-xs">{k.space_id}</td>
                <td className="px-3.5 py-2.5 border-b border-a-border-soft text-xs">{k.token_type}</td>
                <td className="px-3.5 py-2.5 border-b border-a-border-soft"><StatusBadge status={k.status} /></td>
                <td className="px-3.5 py-2.5 border-b border-a-border-soft">
                  {k.status === 'active' && <Btn sm danger onClick={() => doRevoke(k.id)}>撤销</Btn>}
                </td>
              </tr>
            ))}
            {keys.length === 0 && (
              <tr><td colSpan={5} className="text-center py-10 text-a-muted text-xs">暂无 API 密钥</td></tr>
            )}
          </tbody>
        </table>
      </Card>

      {showCreate && (
        scopes.length > 0 ? (
          <SelectScopeModal scopes={scopes} onClose={() => setShowCreate(false)} onCreate={doCreate} />
        ) : (
          <Modal title="无法创建 API Key" onClose={() => setShowCreate(false)}
            footer={<Btn primary onClick={() => { setShowCreate(false); navigate('/scopes'); }}>去创建 Scope</Btn>}>
            <Alert type="warn">请先在 Scopes 页面创建一个 Scope，每个 API Key 必须绑定到一个 Scope 以实现资源隔离。</Alert>
          </Modal>
        )
      )}

      {plainKey && (
        <Modal title="新 API Key" onClose={() => setPlainKey(null)}
          footer={<Btn primary onClick={() => { navigator.clipboard.writeText(plainKey.token || ''); toast('已复制'); }}>复制</Btn>}>
          <Alert type="warn">请立即复制。关闭后无法再次查看。</Alert>
          <div className="font-mono text-sm bg-a-bg border border-a-border rounded-a-sm p-3 break-all select-all mt-3">
            {plainKey.token || '—'}
          </div>
        </Modal>
      )}
    </div>
  );
}

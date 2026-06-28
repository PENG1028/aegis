import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { adminApi } from '@/lib/api-bridge';
import { PageHeader, Card, Btn, Alert, Modal, StatusBadge } from '@/components/shared';
import { useToast } from '@/components/shared/Toast';
import { fmtDate } from '@/lib/utils';

export function ScopesPage() {
  const toast = useToast();
  const queryClient = useQueryClient();
  const [showCreate, setShowCreate] = useState(false);

  const { data, isLoading, error } = useQuery({
    queryKey: ['scopes'],
    queryFn: () => adminApi.listScopes(),
  });

  const scopes = data?.spaces || [];

  async function doCreate(name: string) {
    try {
      await adminApi.createScope({ name });
      toast('Scope 已创建');
      setShowCreate(false);
      queryClient.invalidateQueries({ queryKey: ['scopes'] });
    } catch (e: any) { toast(e.message, 'error'); }
  }

  return (
    <div>
      <PageHeader title="作用域" helpKey="scopes" sub="API 作用域隔离 — 每个 Scope 内的资源相互隔离" actions={
        <Btn primary onClick={() => setShowCreate(true)}>+ 创建</Btn>
      } />

      {isLoading && <div className="text-center py-10 text-a-muted font-mono text-sm">加载中...</div>}
      {error && <Alert type="err">加载失败: {(error as any).message}</Alert>}

      {!isLoading && (
        <Card>
          <table className="w-full text-sm border-collapse">
            <thead>
              <tr>
                {['名称', '描述', '状态', '创建时间'].map((h) => (
                  <th key={h} className="text-left px-3.5 py-2.5 text-[11px] font-semibold uppercase tracking-[0.06em] text-a-muted bg-a-bg border-b border-a-border whitespace-nowrap">{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {scopes.map((s: any) => (
                <tr key={s.id} className="hover:bg-white/[0.04]">
                  <td className="px-3.5 py-2.5 border-b border-a-border-soft font-mono text-xs">{s.name || s.id}</td>
                  <td className="px-3.5 py-2.5 border-b border-a-border-soft text-xs text-a-muted">{s.description || '—'}</td>
                  <td className="px-3.5 py-2.5 border-b border-a-border-soft"><StatusBadge status={s.status || 'active'} /></td>
                  <td className="px-3.5 py-2.5 border-b border-a-border-soft font-mono text-xs">{fmtDate(s.created_at)}</td>
                </tr>
              ))}
              {scopes.length === 0 && (
                <tr><td colSpan={4} className="text-center py-10 text-a-muted text-xs">暂无作用域 — 点击「+ 创建」新建一个</td></tr>
              )}
            </tbody>
          </table>
        </Card>
      )}

      {showCreate && (
        <CreateScopeModal onClose={() => setShowCreate(false)} onCreate={doCreate} />
      )}
    </div>
  );
}

function CreateScopeModal({ onClose, onCreate }: { onClose: () => void; onCreate: (name: string) => void }) {
  const [name, setName] = useState('');
  return (
    <Modal title="创建 Scope" onClose={onClose}
      footer={<><Btn onClick={onClose}>取消</Btn><Btn primary onClick={() => name.trim() && onCreate(name.trim())}>创建</Btn></>}>
      <div className="mb-3">
        <label className="block text-xs font-medium text-a-muted mb-1.5">名称</label>
        <input className="w-full font-mono text-sm px-3 py-2 rounded-a-sm border border-a-border bg-a-bg text-a-fg outline-none focus:border-a-accent"
          value={name} onChange={(e) => setName(e.target.value)} autoFocus />
      </div>
    </Modal>
  );
}

export function ApiKeysPage() {
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

function SelectScopeModal({ scopes, onClose, onCreate }: { scopes: any[]; onClose: () => void; onCreate: (scopeId: string, name: string) => void }) {
  const [selected, setSelected] = useState(scopes[0]?.space_id || scopes[0]?.id || '');
  const [keyName, setKeyName] = useState('');
  const [error, setError] = useState<string | null>(null);
  return (
    <Modal title="创建 API Key" onClose={onClose}
      footer={<><Btn onClick={onClose}>取消</Btn><Btn primary onClick={() => {
        if (!keyName.trim()) { setError('请输入名称'); return; }
        setError(null);
        onCreate(selected, keyName.trim());
      }}>创建</Btn></>}>
      <div className="mb-3">
        <label className="block text-xs font-medium text-a-muted mb-1.5">名称</label>
        <input className="w-full font-mono text-sm px-3 py-2 rounded-a-sm border border-a-border bg-a-bg text-a-fg outline-none focus:border-a-accent"
          value={keyName} onChange={(e) => setKeyName(e.target.value)} placeholder="例如：my-api-key" autoFocus />
      </div>
      <div className="mb-3">
        <label className="block text-xs font-medium text-a-muted mb-1.5">选择作用域</label>
        <select className="w-full font-mono text-sm px-3 py-2 rounded-a-sm border border-a-border bg-a-bg text-a-fg outline-none"
          value={selected} onChange={(e) => setSelected(e.target.value)}>
          {scopes.map((s: any) => <option key={s.id} value={s.space_id || s.id}>{s.name}</option>)}
        </select>
        <p className="text-[11px] text-a-muted mt-1.5">API Key 的权限范围由所选 Scope 限定。不同 Scope 之间的资源（Routes、Services 等）完全隔离。</p>
      </div>
      {error && <Alert type="err">{error}</Alert>}
    </Modal>
  );
}

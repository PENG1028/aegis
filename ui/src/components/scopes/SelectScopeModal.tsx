import { useState } from 'react';
import { Alert, Btn, Modal } from '@/components/shared';

interface SelectScopeModalProps {
  scopes: any[];
  onClose: () => void;
  onCreate: (scopeId: string, name: string) => void;
}

export function SelectScopeModal({ scopes, onClose, onCreate }: SelectScopeModalProps) {
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

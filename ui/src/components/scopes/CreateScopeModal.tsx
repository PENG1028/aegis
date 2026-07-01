import { useState } from 'react';
import { Btn, Modal } from '@/components/shared';

interface CreateScopeModalProps {
  onClose: () => void;
  onCreate: (name: string) => void;
}

export function CreateScopeModal({ onClose, onCreate }: CreateScopeModalProps) {
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

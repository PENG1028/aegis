import { useState } from 'react';
import { gatewayLinkApi } from '@/lib/api-bridge';
import { Alert, Btn, Modal } from '@/components/shared';

export function CreateLinkModal({ onClose, onCreated }: { onClose: () => void; onCreated: (res: any) => void }) {
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

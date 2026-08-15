import { useState } from 'react';
import { useApiList } from '@/hooks/use-api';
import { credentialApi } from '@/lib/api-bridge';
import { Card, PageHeader, QueryGuard, StatusBadge, Btn, Modal, useToast } from '@/components/shared';

export default function Credentials() {
  const toast = useToast();
  const { items: creds, isLoading, error, refetch } = useApiList<any>(['credentials'], () => credentialApi.list());
  const [createOpen, setCreateOpen] = useState(false);
  const [alias, setAlias] = useState('');
  const [connString, setConnString] = useState('');
  const [description, setDescription] = useState('');
  const [submitting, setSubmitting] = useState(false);

  const handleCreate = async () => {
    if (!alias || !connString) {
      toast('别名和连接串必填', 'error');
      return;
    }
    setSubmitting(true);
    try {
      await credentialApi.create(alias, connString, description || undefined);
      toast('凭据已创建');
      setCreateOpen(false);
      setAlias(''); setConnString(''); setDescription('');
      refetch();
    } catch (e: any) {
      toast(e.message || '创建失败', 'error');
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="凭据管理" subtitle={`${creds.length} 个凭据`}
        actions={<Btn primary onClick={() => setCreateOpen(true)}>创建凭据</Btn>} />
      <QueryGuard items={creds} isLoading={isLoading} error={error} refetch={refetch} emptyMessage="暂无凭据">
        {(items) => (
          <Card>
            <table className="w-full text-xs">
              <thead><tr className="border-b border-a-border text-a-muted text-left"><th className="py-2 px-3">别名</th><th className="py-2 px-3">类型</th><th className="py-2 px-3">状态</th></tr></thead>
              <tbody>
                {items.map((c: any) => (
                  <tr key={c.id} className="border-b border-a-border/50"><td className="py-2 px-3 font-mono font-medium text-a-fg">{c.alias}</td><td className="py-2 px-3 text-a-muted">{c.scheme || c.type}</td><td className="py-2 px-3"><StatusBadge status={c.status || 'active'} /></td></tr>
                ))}
              </tbody>
            </table>
          </Card>
        )}
      </QueryGuard>
      {createOpen && (
        <Modal onClose={() => setCreateOpen(false)} title="创建凭据">
          <div className="space-y-3">
            <div>
              <label className="text-xs text-a-muted block mb-1">别名（如 pg-db）</label>
              <input value={alias} onChange={e => setAlias(e.target.value)} placeholder="pg-db"
                className="w-full px-3 py-2 rounded-a-sm border border-a-border/50 bg-a-bg text-xs outline-none focus:border-a-accent/50" />
            </div>
            <div>
              <label className="text-xs text-a-muted block mb-1">连接串（AES-256-GCM 加密存储）</label>
              <input value={connString} onChange={e => setConnString(e.target.value)} placeholder="host:port:user:password"
                className="w-full px-3 py-2 rounded-a-sm border border-a-border/50 bg-a-bg text-xs outline-none focus:border-a-accent/50 font-mono" />
            </div>
            <div>
              <label className="text-xs text-a-muted block mb-1">描述（可选）</label>
              <input value={description} onChange={e => setDescription(e.target.value)}
                className="w-full px-3 py-2 rounded-a-sm border border-a-border/50 bg-a-bg text-xs outline-none focus:border-a-accent/50" />
            </div>
            <div className="flex justify-end gap-2 pt-2">
              <Btn onClick={() => setCreateOpen(false)}>取消</Btn>
              <Btn primary disabled={submitting} onClick={handleCreate}>{submitting ? '创建中...' : '创建'}</Btn>
            </div>
          </div>
        </Modal>
      )}
    </div>
  );
}

import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { adminApi } from '@/lib/api-bridge';
import { PageHeader, Card, Btn, Alert, StatusBadge } from '@/components/shared';
import { useToast } from '@/components/shared/Toast';
import { CreateScopeModal } from '@/components/scopes/CreateScopeModal';
import { fmtDate } from '@/lib/utils';

export default function ScopesPage() {
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

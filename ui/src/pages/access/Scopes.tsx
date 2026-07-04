import { useApiList } from '@/hooks/use-api';
import { adminApi } from '@/lib/api-bridge';
import { Card, PageHeader, QueryGuard, Btn } from '@/components/shared';

export default function Scopes() {
  const { items: scopes, isLoading, error, refetch } = useApiList<any>(['scopes'], () => adminApi.listScopes());

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="Scope 管理" subtitle={`${scopes.length} 个 Scope`} actions={<Btn primary>创建 Scope</Btn>} />
      <QueryGuard items={scopes} isLoading={isLoading} error={error} refetch={refetch} emptyMessage="暂无 Scope">
        {(items) => (
          <Card>
            <table className="w-full text-xs">
              <thead><tr className="border-b border-a-border text-a-muted text-left"><th className="py-2 px-3">名称</th><th className="py-2 px-3">ID</th><th className="py-2 px-3">创建时间</th></tr></thead>
              <tbody>
                {items.map((s: any) => (
                  <tr key={s.id} className="border-b border-a-border/50"><td className="py-2 px-3 font-medium text-a-fg">{s.name}</td><td className="py-2 px-3 font-mono text-a-muted">{s.id}</td><td className="py-2 px-3 text-a-muted">{s.created_at || '—'}</td></tr>
                ))}
              </tbody>
            </table>
          </Card>
        )}
      </QueryGuard>
    </div>
  );
}

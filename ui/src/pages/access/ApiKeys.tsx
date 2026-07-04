import { useApiList } from '@/hooks/use-api';
import { adminApi } from '@/lib/api-bridge';
import { Card, PageHeader, QueryGuard, StatusBadge, Btn } from '@/components/shared';

export default function ApiKeys() {
  const { items: keys, isLoading, error, refetch } = useApiList<any>(['api-keys'], () => adminApi.listApiKeys());

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="API 密钥" subtitle={`${keys.length} 个密钥`} actions={<Btn primary>创建密钥</Btn>} />
      <QueryGuard items={keys} isLoading={isLoading} error={error} refetch={refetch} emptyMessage="暂无 API 密钥">
        {(items) => (
          <Card>
            <table className="w-full text-xs">
              <thead><tr className="border-b border-a-border text-a-muted text-left"><th className="py-2 px-3">名称</th><th className="py-2 px-3">Scope</th><th className="py-2 px-3">前缀</th><th className="py-2 px-3">状态</th></tr></thead>
              <tbody>
                {items.map((k: any) => (
                  <tr key={k.id} className="border-b border-a-border/50"><td className="py-2 px-3 font-medium text-a-fg">{k.name}</td><td className="py-2 px-3 text-a-muted">{k.scope_id || '—'}</td><td className="py-2 px-3 font-mono text-a-muted">{k.token_prefix || '—'}</td><td className="py-2 px-3"><StatusBadge status={k.status || 'active'} /></td></tr>
                ))}
              </tbody>
            </table>
          </Card>
        )}
      </QueryGuard>
    </div>
  );
}

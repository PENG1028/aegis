import { useApiList } from '@/hooks/use-api';
import { credentialApi } from '@/lib/api-bridge';
import { Card, PageHeader, QueryGuard, StatusBadge, Btn } from '@/components/shared';

export default function Credentials() {
  const { items: creds, isLoading, error, refetch } = useApiList<any>(['credentials'], () => credentialApi.list());

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="凭据管理" subtitle={`${creds.length} 个凭据`} actions={<Btn primary>创建凭据</Btn>} />
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
    </div>
  );
}

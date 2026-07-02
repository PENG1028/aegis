import { useQuery } from '@tanstack/react-query';
import { adminApi } from '@/lib/api-bridge';
import { Card, PageHeader, StatusBadge, Btn } from '@/components/shared';

export default function ApiKeys() {
  const { data } = useQuery({ queryKey: ['api-keys'], queryFn: () => adminApi.listApiKeys() });
  const keys = Array.isArray(data) ? data : [];

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="API 密钥" subtitle={`${keys.length} 个密钥`} actions={<Btn primary>创建密钥</Btn>} />
      <Card>
        <table className="w-full text-xs">
          <thead><tr className="border-b border-a-border text-a-muted text-left"><th className="py-2 px-3">名称</th><th className="py-2 px-3">Scope</th><th className="py-2 px-3">前缀</th><th className="py-2 px-3">状态</th></tr></thead>
          <tbody>
            {keys.map((k: any) => (
              <tr key={k.id} className="border-b border-a-border/50"><td className="py-2 px-3 font-medium text-a-fg">{k.name}</td><td className="py-2 px-3 text-a-muted">{k.scope_id || '—'}</td><td className="py-2 px-3 font-mono text-a-muted">{k.token_prefix || '—'}</td><td className="py-2 px-3"><StatusBadge status={k.status || 'active'} /></td></tr>
            ))}
          </tbody>
        </table>
      </Card>
    </div>
  );
}

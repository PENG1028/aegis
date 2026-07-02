import { useQuery } from '@tanstack/react-query';
import { credentialApi } from '@/lib/api-bridge';
import { Card, PageHeader, StatusBadge, Btn } from '@/components/shared';

export default function Credentials() {
  const { data } = useQuery({ queryKey: ['credentials'], queryFn: () => credentialApi.list() });
  const creds = Array.isArray(data) ? data : [];

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="凭据管理" subtitle={`${creds.length} 个凭据`} actions={<Btn primary>创建凭据</Btn>} />
      <Card>
        <table className="w-full text-xs">
          <thead><tr className="border-b border-a-border text-a-muted text-left"><th className="py-2 px-3">别名</th><th className="py-2 px-3">类型</th><th className="py-2 px-3">状态</th></tr></thead>
          <tbody>
            {creds.map((c: any) => (
              <tr key={c.id} className="border-b border-a-border/50"><td className="py-2 px-3 font-mono font-medium text-a-fg">{c.alias}</td><td className="py-2 px-3 text-a-muted">{c.scheme || c.type}</td><td className="py-2 px-3"><StatusBadge status={c.status || 'active'} /></td></tr>
            ))}
          </tbody>
        </table>
      </Card>
    </div>
  );
}

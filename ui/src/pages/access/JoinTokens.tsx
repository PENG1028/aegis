import { useQuery } from '@tanstack/react-query';
import { fetchJoinTokens } from '@/lib/api-bridge';
import { Card, PageHeader, StatusBadge, Btn, Timestamp } from '@/components/shared';

export default function JoinTokens() {
  const { data } = useQuery({ queryKey: ['join-tokens'], queryFn: fetchJoinTokens });
  const tokens = Array.isArray(data) ? data : [];

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="加入令牌" subtitle={`${tokens.length} 个令牌`} actions={<Btn primary>创建令牌</Btn>} />
      <Card>
        <table className="w-full text-xs">
          <thead><tr className="border-b border-a-border text-a-muted text-left"><th className="py-2 px-3">名称</th><th className="py-2 px-3">前缀</th><th className="py-2 px-3">角色</th><th className="py-2 px-3">过期</th><th className="py-2 px-3">状态</th></tr></thead>
          <tbody>
            {tokens.map((t: any) => (
              <tr key={t.id} className="border-b border-a-border/50"><td className="py-2 px-3 font-medium text-a-fg">{t.name}</td><td className="py-2 px-3 font-mono text-a-muted">{t.token_prefix}</td><td className="py-2 px-3 text-a-muted">{(t.allowed_roles || []).join(', ')}</td><td className="py-2 px-3"><Timestamp iso={t.expires_at} /></td><td className="py-2 px-3"><StatusBadge status={t.status} /></td></tr>
            ))}
          </tbody>
        </table>
      </Card>
    </div>
  );
}

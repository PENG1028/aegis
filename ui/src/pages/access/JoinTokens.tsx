import { useApiList } from '@/hooks/use-api';
import { fetchJoinTokens } from '@/lib/api-bridge';
import { Card, PageHeader, QueryGuard, StatusBadge, Btn, Timestamp } from '@/components/shared';

export default function JoinTokens() {
  const { items: tokens, isLoading, error, refetch } = useApiList<any>(['join-tokens'], () => fetchJoinTokens());

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="加入令牌" subtitle={`${tokens.length} 个令牌`} actions={<Btn primary>创建令牌</Btn>} />
      <QueryGuard items={tokens} isLoading={isLoading} error={error} refetch={refetch} emptyMessage="暂无令牌">
        {(items) => (
          <Card>
            <table className="w-full text-xs">
              <thead><tr className="border-b border-a-border text-a-muted text-left"><th className="py-2 px-3">名称</th><th className="py-2 px-3">前缀</th><th className="py-2 px-3">角色</th><th className="py-2 px-3">过期</th><th className="py-2 px-3">状态</th></tr></thead>
              <tbody>
                {items.map((t: any) => (
                  <tr key={t.id} className="border-b border-a-border/50"><td className="py-2 px-3 font-medium text-a-fg">{t.name}</td><td className="py-2 px-3 font-mono text-a-muted">{t.token?.substring(0, 8) || '—'}</td><td className="py-2 px-3">{t.role || 'node'}</td><td className="py-2 px-3"><Timestamp iso={t.expires_at} /></td><td className="py-2 px-3"><StatusBadge status={t.status || 'active'} /></td></tr>
                ))}
              </tbody>
            </table>
          </Card>
        )}
      </QueryGuard>
    </div>
  );
}

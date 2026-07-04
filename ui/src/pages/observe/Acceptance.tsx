import { useApiItem } from '@/hooks/use-api';
import { fetchAcceptance } from '@/lib/api-bridge';
import { Card, PageHeader, QueryGuard, StatusBadge, LoadingState } from '@/components/shared';

export default function Acceptance() {
  const { item: a, isLoading, error, refetch } = useApiItem<any>(['acceptance'], () => fetchAcceptance());

  if (isLoading) return <div className="p-6"><LoadingState /></div>;
  const labels: any[] = a?.labels || [];
  return (
    <div className="p-6 space-y-6">
      <PageHeader title="验收状态" subtitle={`${a?.summary?.pass_count || 0}/${a?.summary?.total_labels || 0} 通过`} />
      <QueryGuard items={labels} isLoading={false} error={error} refetch={refetch} emptyMessage="暂无验证标签">
        {(items) => (
          <Card title="验证标签">
            <div className="space-y-2">
              {items.map((l: any) => (
                <div key={l.key} className="flex items-center justify-between p-2 rounded-a-sm bg-a-bg border border-a-border text-xs">
                  <span className="font-medium text-a-fg">{l.label}</span>
                  <div className="flex items-center gap-2">
                    <span className="text-a-muted">{l.evidence}</span>
                    <StatusBadge status={l.status} />
                  </div>
                </div>
              ))}
            </div>
          </Card>
        )}
      </QueryGuard>
    </div>
  );
}

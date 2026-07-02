import { useQuery } from '@tanstack/react-query';
import { fetchAcceptance } from '@/lib/api-bridge';
import { Card, PageHeader, StatusBadge } from '@/components/shared';

export default function Acceptance() {
  const { data } = useQuery({ queryKey: ['acceptance'], queryFn: fetchAcceptance });
  const a = data as any;

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="验收状态" subtitle={`${a?.summary?.pass_count || 0}/${a?.summary?.total_labels || 0} 通过`} />
      <Card title="验证标签">
        <div className="space-y-2">
          {(a?.labels || []).map((l: any) => (
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
    </div>
  );
}

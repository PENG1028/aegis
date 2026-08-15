// ─── Logs / Audit ───
import { useState } from 'react';
import { useLocation } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { Card, PageHeader, TabBar, Timestamp, StatusBadge } from '@/components/shared';
import { adminApi } from '@/lib/api-bridge';

const TABS = [
  { key: 'ops', label: '操作日志' },
  { key: 'audit', label: '审计日志' },
];

export default function Logs() {
  const location = useLocation();
  const initialTab = location.pathname.includes('audit') ? 'audit' : 'ops';
  const [tab, setTab] = useState(initialTab);

  // Real data from the backend — these pages previously showed hardcoded
  // fake events ("v43 apply" etc.) that never happened.
  const { data: opsData } = useQuery({
    queryKey: ['operations'], queryFn: () => adminApi.listOperations().catch(() => ({ operations: [] as any[] })),
    refetchInterval: 30_000,
  });
  const { data: auditData } = useQuery({
    queryKey: ['audit-logs'], queryFn: () => adminApi.listAuditLogs().catch(() => ({ audit_logs: [] as any[] })),
    refetchInterval: 30_000,
  });

  const ops = (opsData as any)?.operations || [];
  const audits = (auditData as any)?.audit_logs || [];
  const entries = tab === 'audit' ? audits : ops;

  return (
    <div className="p-6 space-y-6">
      <PageHeader title={tab === 'audit' ? '审计日志' : '操作日志'} subtitle={`${entries.length} 条记录`} />
      <TabBar tabs={TABS} active={tab} onChange={setTab} />
      <Card>
        <div className="space-y-1">
          {entries.length === 0 && (
            <div className="px-3 py-6 text-center text-xs text-a-muted">暂无记录</div>
          )}
          {entries.map((e: any, i: number) => (
            <div key={i} className="flex items-center gap-3 px-3 py-2.5 rounded-a-sm hover:bg-a-border/10 text-xs">
              <Timestamp iso={e.created_at || e.CreatedAt} />
              <span className="font-mono text-a-fg2 w-36 truncate">{e.action || e.event_type || e.Action}</span>
              <span className="text-a-fg flex-1 truncate">{e.message || e.detail || e.target_id || e.TargetID || '—'}</span>
              <StatusBadge status={e.result || e.status || 'success'} />
            </div>
          ))}
        </div>
      </Card>
    </div>
  );
}

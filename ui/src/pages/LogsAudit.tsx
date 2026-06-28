import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { adminApi } from '@/lib/api-bridge';
import { PageHeader, Card, TabBar, Alert } from '@/components/shared';
import { fmtDate } from '@/lib/utils';

export default function LogsPage() {
  const [tab, setTab] = useState('operations');

  const { data: ops, isLoading: opsLoading } = useQuery({
    queryKey: ['operations'],
    queryFn: () => adminApi.listOperations(),
  });

  const { data: audit, isLoading: auditLoading } = useQuery({
    queryKey: ['audit-logs'],
    queryFn: () => adminApi.listAuditLogs(),
  });

  return (
    <div>
      <PageHeader title="日志" helpKey="logs" />

      <TabBar
        tabs={[
          { key: 'operations', label: 'Op Logs' },
          { key: 'audit', label: 'Audit' },
        ]}
        active={tab}
        onChange={setTab}
      />

      {tab === 'operations' && (
        <Card>
          {opsLoading && <div className="text-center py-10 text-a-muted text-xs">加载中...</div>}
          {!opsLoading && (
            <table className="w-full text-sm border-collapse">
              <thead>
                <tr>
                  {['Action', 'Target', 'Result', 'Actor', 'Time'].map((h) => (
                    <th key={h} className="text-left px-3.5 py-2.5 text-[11px] font-semibold uppercase tracking-[0.06em] text-a-muted bg-a-bg border-b border-a-border whitespace-nowrap">{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {(ops?.operations || []).map((op: any) => (
                  <tr key={op.id} className="hover:bg-white/[0.04]">
                    <td className="px-3.5 py-2.5 border-b border-a-border-soft font-mono text-xs">{op.action}</td>
                    <td className="px-3.5 py-2.5 border-b border-a-border-soft text-xs">{op.target_type}: {op.target_id}</td>
                    <td className="px-3.5 py-2.5 border-b border-a-border-soft text-xs">{op.result}</td>
                    <td className="px-3.5 py-2.5 border-b border-a-border-soft text-xs">{op.actor || '—'}</td>
                    <td className="px-3.5 py-2.5 border-b border-a-border-soft font-mono text-xs">{fmtDate(op.created_at)}</td>
                  </tr>
                ))}
                {(!ops?.operations || ops.operations.length === 0) && (
                  <tr><td colSpan={5} className="text-center py-10 text-a-muted text-xs">暂无操作日志</td></tr>
                )}
              </tbody>
            </table>
          )}
        </Card>
      )}

      {tab === 'audit' && (
        <Card>
          {auditLoading && <div className="text-center py-10 text-a-muted text-xs">加载中...</div>}
          {!auditLoading && (
            <table className="w-full text-sm border-collapse">
              <thead>
                <tr>
                  {['Event', 'Actor', 'Target', 'Result', 'IP', 'Time'].map((h) => (
                    <th key={h} className="text-left px-3.5 py-2.5 text-[11px] font-semibold uppercase tracking-[0.06em] text-a-muted bg-a-bg border-b border-a-border whitespace-nowrap">{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {(audit?.audit_logs || []).map((l: any) => (
                  <tr key={l.id} className="hover:bg-white/[0.04]">
                    <td className="px-3.5 py-2.5 border-b border-a-border-soft font-mono text-xs"><span className={`${l.error_code ? 'text-[#ff5c72]' : 'text-[#4cd964]'}`}>{l.event_type}</span></td>
                    <td className="px-3.5 py-2.5 border-b border-a-border-soft text-xs">{l.actor_type}{l.actor_id ? '/' + l.actor_id.slice(0, 12) : ''}</td>
                    <td className="px-3.5 py-2.5 border-b border-a-border-soft text-xs text-a-muted">{l.target_type}{l.target_id ? ': ' + l.target_id : ''}</td>
                    <td className="px-3.5 py-2.5 border-b border-a-border-soft text-xs">{l.result}</td>
                    <td className="px-3.5 py-2.5 border-b border-a-border-soft font-mono text-[11px] text-a-muted">{l.ip ? l.ip.split(':')[0] : '—'}</td>
                    <td className="px-3.5 py-2.5 border-b border-a-border-soft font-mono text-xs">{fmtDate(l.created_at)}</td>
                  </tr>
                ))}
                {(!audit?.audit_logs || audit.audit_logs.length === 0) && (
                  <tr><td colSpan={6} className="text-center py-10 text-a-muted text-xs">暂无审计日志</td></tr>
                )}
              </tbody>
            </table>
          )}
        </Card>
      )}
    </div>
  );
}

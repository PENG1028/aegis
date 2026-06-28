import { useQuery } from '@tanstack/react-query';
import { fetchSyncStatus } from '@/lib/api-bridge';
import { PageHeader, Card, DataTable, StatusBadge, StatCard, Alert } from '@/components/shared';
import type { DataTableColumn } from '@/components/shared';
import type { SyncStatus as SyncStatusType } from '@/types';
import { fmtRel } from '@/lib/utils';

const columns: DataTableColumn<SyncStatusType>[] = [
  { key: 'node_name', label: '节点', mono: true },
  {
    key: 'status',
    label: 'Sync 状态',
    render: (r) => <StatusBadge status={r.status} />,
  },
  { key: 'desired_revision', label: '期望版本', mono: true },
  { key: 'applied_revision', label: '实际版本', mono: true },
  { key: 'desired_hash', label: '期望哈希', mono: true, muted: true, render: (r) => r.desired_hash ? r.desired_hash.slice(0, 12) : '—' },
  { key: 'actual_hash', label: '实际哈希', mono: true, muted: true, render: (r) => r.actual_hash ? r.actual_hash.slice(0, 12) : '—' },
  {
    key: 'last_apply_at',
    label: '上次推送',
    mono: true,
    muted: true,
    render: (r) => fmtRel(r.last_apply_at),
  },
  {
    key: 'last_error',
    label: '错误',
    render: (r) => r.last_error ? <span className="text-a-danger text-[11px]">{r.last_error}</span> : '—',
  },
];

export default function SyncStatusPage() {
  const { data, isLoading, error } = useQuery({
    queryKey: ['sync-status'],
    queryFn: () => fetchSyncStatus(),
  });

  if (isLoading) return <div className="text-center py-10 text-a-muted font-mono text-sm">加载中...</div>;
  if (error) return <div className="px-4 py-3 rounded-a-md text-xs border bg-[#ff5c72]/10 text-[#ff5c72] border-[#ff5c72]/20">加载失败: {error.message}</div>;

  const inSyncCount = data?.filter((s) => s.status === 'in_sync').length || 0;

  return (
    <div>
      <PageHeader title="期望 / 实际 / 同步状态" helpKey="sync" subtitle="控制面与节点间状态同步"  />

      <div className="grid grid-cols-3 gap-3 mb-5">
        <StatCard label="节点" value={data?.length || 0} accent />
        <StatCard label="已同步" value={inSyncCount} success={inSyncCount === data?.length} />
        <StatCard label="未同步 / 失败" value={(data?.length || 0) - inSyncCount} warn={(data?.length || 0) > inSyncCount} />
      </div>

      <Alert type="info">
        控制面希望 Node 是什么状态（Desired）？Node 实际应用到哪一版（Actual）？为什么还没同步？
      </Alert>

      <Card>
        <DataTable columns={columns} data={data || []} keyExtractor={(r) => r.node_id} />
      </Card>

      {data && data.length > 0 && (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mt-4">
          {data.map((node) => (
            <Card key={node.node_id} title={node.node_name} subtitle={`desired_rev: ${node.desired_revision} · applied_rev: ${node.applied_revision}`}>
              <div className="space-y-3 text-xs">
                <div className="grid grid-cols-2 gap-2">
                  <div>
                    <div className="text-a-muted mb-1">提供者</div>
                    <StatusBadge status={node.provider_status.status} />
                    <div className="text-a-muted mt-0.5">{node.provider_status.message}</div>
                  </div>
                  <div>
                    <div className="text-a-muted mb-1">中继</div>
                    <StatusBadge status={node.relay_status.status} />
                    <div className="text-a-muted mt-0.5">{node.relay_status.message}</div>
                  </div>
                  <div>
                    <div className="text-a-muted mb-1">网关</div>
                    <StatusBadge status={node.gateway_status.status} />
                    <div className="text-a-muted mt-0.5">{node.gateway_status.message}</div>
                  </div>
                  <div>
                    <div className="text-a-muted mb-1">诊断</div>
                    <StatusBadge status={node.diagnostics_status.status} />
                    <div className="text-a-muted mt-0.5">{node.diagnostics_status.message}</div>
                  </div>
                </div>
              </div>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}

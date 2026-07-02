// ─── Node Detail ───
// Must show desired vs actual state + drifted status
import { useParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { fetchNodeDetail } from '@/lib/api-bridge';
import { useChain } from '@/hooks/useChain';
import { PathRibbon } from '@/components/workspace/PathRibbon';
import { RelationshipMap } from '@/components/workspace/RelationshipMap';
import { PageHeader, Card, StatusBadge, MetaRow, LoadingState, ErrorBanner, Btn } from '@/components/shared';
import { getScenario } from '@/mocks';
import { API_CONFIG } from '@/lib/api-config';

export default function NodeDetail() {
  const { nodeId } = useParams<{ nodeId: string }>();
  const { data: node, isLoading, error } = useQuery({
    queryKey: ['node', nodeId],
    queryFn: () => fetchNodeDetail(nodeId!),
    enabled: !!nodeId,
  });
  const { data: chain } = useChain('node', nodeId);

  if (isLoading) return <div className="p-6"><LoadingState text="加载节点详情..." /></div>;
  if (error) return <div className="p-6"><ErrorBanner message={(error as Error).message} /></div>;

  const n = (node as any)?.node || node;
  // Get sync status from scenario in mock mode
  const syncStatus = API_CONFIG.useMock
    ? getScenario().syncStatuses.find(s => s.node_id === nodeId)
    : n?.sync;

  const drifted = n?.desired_revision !== n?.applied_revision;

  return (
    <div className="p-6 space-y-6">
      <PageHeader title={n?.name || nodeId} subtitle={`${n?.public_ip} · ${n?.os} · ${n?.agent_version}`} />

      {chain && <PathRibbon chain={chain} focusType="node" focusId={nodeId} />}

      {/* Desired vs Actual State */}
      <Card title="状态同步" subtitle={drifted ? '⚠️ 配置漂移' : '配置已同步'}>
        <div className="grid grid-cols-2 gap-4">
          <div className="p-3 rounded-a-sm bg-a-bg border border-a-border">
            <p className="text-[10px] text-a-muted uppercase mb-2">期望状态 (Desired)</p>
            <MetaRow label="版本" value={String(n?.desired_revision || '—')} mono />
            <MetaRow label="Hash" value={syncStatus?.desired_hash || '—'} mono />
          </div>
          <div className={drifted ? 'p-3 rounded-a-sm bg-[#e8b830]/5 border border-[#e8b830]/30' : 'p-3 rounded-a-sm bg-a-bg border border-a-border'}>
            <p className="text-[10px] text-a-muted uppercase mb-2">实际状态 (Actual)</p>
            <MetaRow label="版本" value={String(n?.applied_revision || '—')} mono />
            <MetaRow label="Hash" value={syncStatus?.actual_hash || '—'} mono />
          </div>
        </div>
        <div className="mt-3 flex items-center gap-2">
          <span className="text-xs text-a-muted">同步状态:</span>
          <StatusBadge status={drifted ? 'drifted' : syncStatus?.status || 'unknown'} />
          {drifted && <Btn className="text-xs">触发同步</Btn>}
        </div>
      </Card>

      {chain && <RelationshipMap chain={chain} focusType="node" focusId={nodeId || ''} />}
    </div>
  );
}

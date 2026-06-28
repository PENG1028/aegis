import { useQuery } from '@tanstack/react-query';
import { useParams, useNavigate } from 'react-router-dom';
import { fetchNodeDetail } from '@/lib/api-bridge';
import {
  PageHeader, Card, StatusBadge, CapabilityBadge,
  MetaRow, Alert,
} from '@/components/shared';
import { fmtRel } from '@/lib/utils';

export default function NodeDetailPage() {
  const { nodeId } = useParams<{ nodeId: string }>();
  const navigate = useNavigate();
  const { data, isLoading, error } = useQuery({
    queryKey: ['node', nodeId],
    queryFn: () => fetchNodeDetail(nodeId!),
    enabled: !!nodeId,
  });

  if (isLoading) return <div className="text-center py-10 text-a-muted font-mono text-sm">加载中...</div>;
  if (error || !data) return <div className="text-center py-10 text-a-danger text-sm">Node not found</div>;

  return (
    <div>
      <button onClick={() => navigate('/nodes')} className="inline-flex items-center gap-1 text-xs text-a-muted hover:text-a-fg mb-3 bg-transparent border-none cursor-pointer p-0">
        ← Nodes
      </button>
      <PageHeader title={data.name} subtitle={`node_id: ${data.node_id}`} helpKey="nodes" />

      <div className="grid grid-cols-2 gap-4 mb-4">
        <Card title="基本信息">
          <MetaRow label="Node ID" value={data.node_id} mono color="text-a-accent" />
          <MetaRow label="Name" value={data.name} />
          <MetaRow label="Hostname" value={data.hostname} mono />
          <MetaRow label="Public IP" value={data.public_ip} mono />
          <MetaRow label="Private IP" value={data.private_ip} mono />
          <MetaRow label="OS / Arch" value={`${data.os} / ${data.arch}`} mono />
          <MetaRow label="Agent Version" value={data.agent_version} mono />
          <MetaRow label="Status" value={<StatusBadge status={data.status} />} />
          <MetaRow label="Last Heartbeat" value={fmtRel(data.last_heartbeat_at)} mono />
        </Card>

        <Card title="Sync 状态">
          <MetaRow label="Status" value={<StatusBadge status={data.sync.status} />} />
          <MetaRow label="Desired Revision" value={data.sync.desired_revision} mono />
          <MetaRow label="Applied Revision" value={data.sync.applied_revision} mono />
          <MetaRow label="Desired Hash" value={data.sync.desired_hash} mono />
          <MetaRow label="Actual Hash" value={data.sync.actual_hash} mono />
          <MetaRow label="Last Apply" value={data.sync.last_apply_at ? fmtRel(data.sync.last_apply_at) : '—'} mono />
          <MetaRow label="Last Error" value={data.sync.last_error || '—'} color={data.sync.last_error ? 'text-a-danger' : ''} />
        </Card>
      </div>

      <Card title="Capabilities" subtitle="节点运行时能力" className="mb-4">
        <div className="flex flex-wrap gap-2">
          {Object.entries(data.capabilities).map(([key, val]) => (
            <CapabilityBadge key={key} name={key} enabled={val} />
          ))}
        </div>
      </Card>

      {data.gateways.length > 0 && (
        <Card title="Gateways" subtitle={`${data.gateways.length} gateways on this node`} className="mb-4">
          <div className="space-y-2">
            {data.gateways.map((gw) => (
              <div key={gw.gateway_id} className="flex items-center gap-3 py-2 border-b border-a-border-soft last:border-b-0 text-xs">
                <button className="text-a-accent font-mono bg-transparent border-none cursor-pointer p-0 hover:underline"
                  onClick={() => navigate(`/gateways/${gw.gateway_id}`)}>
                  {gw.gateway_id}
                </button>
                <span className="text-a-muted">{gw.name}</span>
                <StatusBadge status={gw.type === 'local' ? 'local_gateway' : gw.type === 'private' ? 'private_gateway' : 'public_gateway'} />
                <span className="ml-auto font-mono">{gw.host}:{gw.port}</span>
                <StatusBadge status={gw.status} />
              </div>
            ))}
          </div>
        </Card>
      )}

      <div className="grid grid-cols-2 gap-4 mb-4">
        {data.local_gateway && (
          <Card title="Local Gateway Runtime">
            <MetaRow label="Bind" value={`${data.local_gateway.bind_addr}:${data.local_gateway.port}`} mono />
            <MetaRow label="Status" value={<StatusBadge status={data.local_gateway.status} />} />
            <MetaRow label="Routing Table Loaded" value={data.local_gateway.routing_table_loaded ? '✓' : '✗'} />
            <MetaRow label="Routing Revision" value={data.local_gateway.routing_table_revision ?? '—'} mono />
            <MetaRow label="Cache" value={data.local_gateway.cache_status} />
          </Card>
        )}
        <Card title="Diagnostics">
          {data.diagnostics.map((d, i) => (
            <div key={i} className="flex items-center gap-2 py-1.5 border-b border-a-border-soft last:border-b-0 text-xs">
              <StatusBadge status={d.status === 'ok' ? 'ok' : d.status === 'warning' ? 'warning' : 'error'} />
              <span className="text-a-fg">{d.name}</span>
              <span className="text-a-muted ml-auto">{d.message}</span>
            </div>
          ))}
        </Card>
      </div>

      {data.last_error && (
        <Alert type="err">最近错误: {data.last_error}</Alert>
      )}

      <div className="flex gap-2 mt-2">
        <button
          onClick={() => navigate(`/sync?nodeId=${nodeId}`)}
          className="inline-flex items-center gap-1.5 text-xs px-3 py-1.5 rounded-a-md bg-a-surface border border-a-border text-a-fg hover:bg-a-border-soft cursor-pointer"
        >
          Sync Detail
        </button>
        <button
          onClick={() => navigate(`/routing?nodeId=${nodeId}`)}
          className="inline-flex items-center gap-1.5 text-xs px-3 py-1.5 rounded-a-md bg-a-surface border border-a-border text-a-fg hover:bg-a-border-soft cursor-pointer"
        >
          Routing Table
        </button>
        <button
          onClick={() => navigate(`/local-gateway?nodeId=${nodeId}`)}
          className="inline-flex items-center gap-1.5 text-xs px-3 py-1.5 rounded-a-md bg-a-surface border border-a-border text-a-fg hover:bg-a-border-soft cursor-pointer"
        >
          Local Gateway
        </button>
      </div>
    </div>
  );
}

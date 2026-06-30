import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useParams, useNavigate } from 'react-router-dom';
import { fetchNodeDetail, nodeApi } from '@/lib/api-bridge';
import {
  PageHeader, Card, StatusBadge, CapabilityBadge,
  MetaRow, Alert, Btn,
} from '@/components/shared';
import { fmtRel } from '@/lib/utils';
import { useToast } from '@/components/shared/Toast';

export default function NodeDetailPage() {
  const { nodeId } = useParams<{ nodeId: string }>();
  const navigate = useNavigate();
  const toast = useToast();
  const [updating, setUpdating] = useState(false);
  const { data, isLoading, error } = useQuery({
    queryKey: ['node', nodeId],
    queryFn: () => fetchNodeDetail(nodeId!),
    enabled: !!nodeId,
  });

  async function handleUpdate() {
    if (!nodeId) return;
    setUpdating(true);
    try {
      const res = await nodeApi.triggerUpdate(nodeId);
      toast(`更新已触发 — ${res.message || '节点将在下次心跳时自动更新'}`);
    } catch (e: any) {
      toast(`更新失败: ${e.message}`, 'error');
    }
    setUpdating(false);
  }

  if (isLoading) return <div className="text-center py-10 text-a-muted font-mono text-sm">加载中...</div>;
  if (error || !data) return <div className="text-center py-10 text-a-danger text-sm">节点未找到</div>;

  return (
    <div>
      <button onClick={() => navigate('/nodes')} className="inline-flex items-center gap-1 text-xs text-a-muted hover:text-a-fg mb-3 bg-transparent border-none cursor-pointer p-0">
        ← 节点
      </button>
      <PageHeader title={data.name} subtitle={`node_id: ${data.node_id}`} helpKey="nodes" actions={
        <Btn primary onClick={handleUpdate} disabled={updating}>
          {updating ? '触发中…' : '推送更新'}
        </Btn>
      } />

      <div className="grid grid-cols-2 gap-4 mb-4">
        <Card title="基本信息">
          <MetaRow label="节点 ID" value={data.node_id} mono color="text-a-accent" />
          <MetaRow label="名称" value={data.name} />
          <MetaRow label="主机名" value={data.hostname} mono />
          <MetaRow label="公网 IP" value={data.public_ip} mono />
          <MetaRow label="内网 IP" value={data.private_ip} mono />
          <MetaRow label="系统 / 架构" value={`${data.os} / ${data.arch}`} mono />
          <MetaRow label="代理版本" value={data.agent_version} mono />
          <MetaRow label="状态" value={<StatusBadge status={data.status} />} />
          <MetaRow label="上次心跳" value={fmtRel(data.last_heartbeat_at)} mono />
        </Card>

        <Card title="Sync 状态">
          <MetaRow label="状态" value={<StatusBadge status={data.sync.status} />} />
          <MetaRow label="期望版本" value={data.sync.desired_revision} mono />
          <MetaRow label="实际版本" value={data.sync.applied_revision} mono />
          <MetaRow label="期望哈希" value={data.sync.desired_hash} mono />
          <MetaRow label="实际哈希" value={data.sync.actual_hash} mono />
          <MetaRow label="上次推送" value={data.sync.last_apply_at ? fmtRel(data.sync.last_apply_at) : '—'} mono />
          <MetaRow label="上次错误" value={data.sync.last_error || '—'} color={data.sync.last_error ? 'text-a-danger' : ''} />
        </Card>
      </div>

      <Card title="能力" subtitle="节点运行时能力" className="mb-4">
        <div className="flex flex-wrap gap-2">
          {Object.entries(data.capabilities).map(([key, val]) => (
            <CapabilityBadge key={key} name={key} enabled={val} />
          ))}
        </div>
      </Card>

      {data.gateways.length > 0 && (
        <Card title="网关" subtitle={`${data.gateways.length} gateways on this node`} className="mb-4">
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
          <Card title="本地网关运行时">
            <MetaRow label="绑定地址" value={`${data.local_gateway.bind_addr}:${data.local_gateway.port}`} mono />
            <MetaRow label="状态" value={<StatusBadge status={data.local_gateway.status} />} />
            <MetaRow label="路由表已加载" value={data.local_gateway.routing_table_loaded ? '✓' : '✗'} />
            <MetaRow label="路由版本" value={data.local_gateway.routing_table_revision ?? '—'} mono />
            <MetaRow label="缓存" value={data.local_gateway.cache_status} />
          </Card>
        )}
        <Card title="诊断">
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
          同步详情
        </button>
        <button
          onClick={() => navigate(`/routing?nodeId=${nodeId}`)}
          className="inline-flex items-center gap-1.5 text-xs px-3 py-1.5 rounded-a-md bg-a-surface border border-a-border text-a-fg hover:bg-a-border-soft cursor-pointer"
        >
          路由表
        </button>
        <button
          onClick={() => navigate(`/local-gateway?nodeId=${nodeId}`)}
          className="inline-flex items-center gap-1.5 text-xs px-3 py-1.5 rounded-a-md bg-a-surface border border-a-border text-a-fg hover:bg-a-border-soft cursor-pointer"
        >
          本地网关
        </button>
      </div>
    </div>
  );
}

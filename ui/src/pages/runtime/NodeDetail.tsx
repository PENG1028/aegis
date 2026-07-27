// ─── Node Detail — full per-node monitoring & operations ───
// Shows diagnostics, gateways, roles, heartbeat, sync status.
// All action buttons wired to real APIs.

import { useParams, useNavigate } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { fetchNodeDetail, nodeApi, clusterHealthApi } from '@/lib/api-bridge';
import type { NodeDetail as ND, NodeDiagnostic, Gateway } from '@/types';
import { Btn, Card, PageHeader, StatusBadge, LoadingState, ErrorBanner, useToast } from '@/components/shared';
import { cn } from '@/lib/utils';

export default function NodeDetail() {
  const { nodeId } = useParams<{ nodeId: string }>();
  const nav = useNavigate(); const toast = useToast(); const qc = useQueryClient();

  const { data: node, isLoading, error, refetch } = useQuery({
    queryKey: ['node-detail', nodeId],
    queryFn: () => fetchNodeDetail(nodeId!),
    enabled: !!nodeId,
    refetchInterval: 15_000,
  });

  const { data: cluster } = useQuery({
    queryKey: ['cluster-health'],
    queryFn: () => clusterHealthApi.get(),
    refetchInterval: 30_000,
  });

  const isLeader = (cluster as any)?.leader_node_id === nodeId;

  const syncMut = useMutation({
    mutationFn: () => nodeApi.refreshCapabilities(nodeId!),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['node-detail', nodeId] }); toast('同步已触发'); },
    onError: (e: any) => toast(e.message || '同步失败', 'error'),
  });
  const updateMut = useMutation({
    mutationFn: () => nodeApi.triggerUpdate(nodeId!),
    onSuccess: () => { toast('更新已触发'); },
    onError: (e: any) => toast(e.message || '更新失败', 'error'),
  });
  const healthMut = useMutation({
    mutationFn: () => nodeApi.health(nodeId!),
    onSuccess: (d: any) => { toast(d?.status || '健康检查完成'); },
    onError: (e: any) => toast(e.message || '健康检查失败', 'error'),
  });

  if (isLoading) return <LoadingState />;
  if (error || !node) return <ErrorBanner message="加载节点详情失败" onRetry={refetch} />;

  const n = node as ND;
  const active = (n.status as string) === 'online' || (n.status as string) === 'healthy';

  return (
    <div className="p-6 space-y-5">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-bold text-a-fg">{n.name || n.hostname || n.node_id}</h2>
          <p className="text-xs text-a-muted mt-1">
            {n.node_id} · {n.os || '—'} {n.arch || ''} · v{n.agent_version || '—'}
            {isLeader && <span className="text-[#e8b830] ml-2">⭐ Leader</span>}
          </p>
        </div>
        <Btn onClick={() => nav('/runtime')} className="text-xs">返回列表</Btn>
      </div>

      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        <SBox label="状态" value={active ? '在线' : n.status || '未知'} color={active ? 'text-[#4cd964]' : 'text-[#ff5c72]'} />
        <SBox label="角色" value={n.roles?.join(', ') || (isLeader ? 'leader' : 'worker')} color="text-a-fg" />
        <SBox label="配置版本" value={`${n.desired_revision}/${n.applied_revision}`} color="text-a-muted" />
        <SBox label="心跳" value={n.last_heartbeat_at ? ago(n.last_heartbeat_at) : '—'} color="text-a-muted" />
      </div>

      <div className="grid grid-cols-2 gap-4">
        <Card title="基本信息">
          <div className="space-y-2 text-xs">
            <Row label="节点 ID" value={n.node_id} mono />
            <Row label="主机名" value={n.hostname || '—'} />
            <Row label="公网 IP" value={n.public_ip || '—'} mono />
            <Row label="内网 IP" value={n.private_ip || '—'} mono />
            <Row label="OS" value={`${n.os || '—'} ${n.arch || ''}`} />
            <Row label="版本" value={n.agent_version || '—'} mono />
          </div>
        </Card>

        <Card title="同步状态">
          <div className="space-y-2 text-xs">
            <Row label="同步状态" badge={<StatusBadge status={(n.sync?.status as string) === 'ok' ? 'active' : 'disabled'} />} />
            <Row label="期望版本" value={String(n.sync?.desired_revision ?? n.desired_revision)} />
            <Row label="实际版本" value={String(n.sync?.applied_revision ?? n.applied_revision)} />
            <Row label="最后应用" value={n.sync?.last_apply_at || '—'} />
            {n.sync?.last_error && (
              <div className="text-[#ff5c72] text-[11px] bg-[#ff5c72]/5 px-2 py-1 rounded">{n.sync.last_error}</div>
            )}
          </div>
        </Card>

        {n.diagnostics && n.diagnostics.length > 0 && (
          <Card title="诊断">
            <div className="space-y-2">
              {n.diagnostics.map((d: NodeDiagnostic, i: number) => (
                <div key={i} className={cn('flex items-center gap-2 px-2 py-1.5 rounded-a-sm text-xs',
                  d.status === 'ok' ? 'bg-[#4cd964]/5' : d.status === 'warning' ? 'bg-[#e8b830]/5' : 'bg-[#ff5c72]/5')}>
                  <span className={cn('font-mono text-sm shrink-0',
                    d.status === 'ok' ? 'text-[#4cd964]' : d.status === 'warning' ? 'text-[#e8b830]' : 'text-[#ff5c72]')}>
                    {d.status === 'ok' ? '✓' : d.status === 'warning' ? '⚠' : '✗'}
                  </span>
                  <span className="font-medium">{d.name}</span>
                  <span className="text-a-muted ml-auto">{d.message}</span>
                </div>
              ))}
            </div>
          </Card>
        )}

        <Card title={`网关 (${n.gateways?.length || 0})`}>
          {n.gateways && n.gateways.length > 0 ? (
            <div className="space-y-1.5">
              {n.gateways.map((g: Gateway, i: number) => (
                <div key={g.gateway_id || i} className="flex items-center gap-2 text-xs px-2 py-1.5 rounded bg-a-bg border border-a-border/20">
                  <span className="font-mono text-a-fg text-[11px]">{g.name}</span>
                  <span className="text-a-muted">{g.provider}</span>
                  <span className="text-a-muted/50 ml-auto">{g.type} · {g.bind_addr}</span>
                </div>
              ))}
            </div>
          ) : (
            <div className="text-xs text-a-muted py-4 text-center">无网关数据</div>
          )}
        </Card>
      </div>

      {n.last_error && (
        <Card title="最近错误">
          <div className="text-xs text-[#ff5c72] font-mono bg-[#ff5c72]/5 px-3 py-2 rounded">{n.last_error}</div>
        </Card>
      )}

      <Card title="操作">
        <div className="flex gap-2 flex-wrap">
          <Btn onClick={() => syncMut.mutate()} disabled={syncMut.isPending} className="text-xs">
            {syncMut.isPending ? '同步中...' : '触发同步'}
          </Btn>
          <Btn onClick={() => updateMut.mutate()} disabled={updateMut.isPending} className="text-xs">
            {updateMut.isPending ? '更新中...' : '触发更新'}
          </Btn>
          <Btn onClick={() => healthMut.mutate()} disabled={healthMut.isPending} className="text-xs">
            {healthMut.isPending ? '检查中...' : '健康检查'}
          </Btn>
        </div>
      </Card>
    </div>
  );
}

function Row({ label, value, badge, mono }: { label: string; value?: string; badge?: any; mono?: boolean }) {
  return (
    <div className="flex justify-between items-center gap-3">
      <span className="text-a-muted shrink-0">{label}</span>
      {badge || <span className={cn('text-right truncate', mono && 'font-mono text-[11px]')}>{value || '—'}</span>}
    </div>
  );
}

function SBox({ label, value, color }: { label: string; value: string; color: string }) {
  return (
    <div className="bg-a-surface border border-a-border/30 rounded-a-sm px-3 py-2 text-center">
      <div className={cn('text-sm font-bold truncate', color)}>{value}</div>
      <div className="text-[10px] text-a-muted">{label}</div>
    </div>
  );
}

function ago(iso: string): string {
  const s = Math.floor((Date.now() - new Date(iso).getTime()) / 1000);
  if (s < 60) return `${s}s`;
  if (s < 3600) return `${Math.floor(s / 60)}min`;
  if (s < 86400) return `${Math.floor(s / 3600)}h`;
  return `${Math.floor(s / 86400)}d`;
}

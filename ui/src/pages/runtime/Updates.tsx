// ─── Updates — pending node binary/config updates ───
import { Link } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { fetchNodes, nodeApi } from '@/lib/api-bridge';
import { PageHeader, Card, Btn, StatusBadge, LoadingState, ErrorBanner, useToast } from '@/components/shared';

export default function Updates() {
  const toast = useToast();
  const qc = useQueryClient();

  const { data: nodes, isLoading, error, refetch } = useQuery({
    queryKey: ['updates-nodes'],
    queryFn: fetchNodes,
    refetchInterval: 30_000,
  });

  const nodeList: any[] = (nodes as any) || [];
  const outdated = nodeList.filter((n: any) =>
    n.desired_revision != null && n.applied_revision != null && n.desired_revision !== n.applied_revision
  );
  // desired-state tracking was removed from the backend; when no node carries
  // revision fields the page must say so instead of "all up to date".
  const trackingAvailable = nodeList.some((n: any) => n.desired_revision != null);

  const updateMut = useMutation({
    mutationFn: (nodeId: string) => nodeApi.triggerUpdate(nodeId),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['updates-nodes'] }); toast('更新已触发'); },
    onError: (e: any) => toast(e.message || '更新触发失败', 'error'),
  });

  if (isLoading) return <LoadingState />;
  if (error) return <ErrorBanner message="加载节点数据失败" onRetry={refetch} />;

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="更新管理" subtitle={
        outdated.length > 0 ? `${outdated.length} 个节点需要更新`
        : trackingAvailable ? '所有节点已是最新'
        : '更新跟踪不可用 — desired-state 子系统已移除，节点版本以实际状态为准'
      } />

      {!trackingAvailable && outdated.length === 0 && (
        <Card>
          <div className="px-3 py-6 text-center text-xs text-a-muted">
            后端不再提供期望/实际版本跟踪（desired-state 子系统已移除）。节点在线状态见节点列表，二进制更新请通过部署脚本（make update-*）执行。
          </div>
        </Card>
      )}

      {outdated.length > 0 && (
        <Card title={`待更新 (${outdated.length})`}>
          {outdated.map((n: any) => (
            <div key={n.node_id} className="flex items-center gap-4 p-3 rounded-a-sm bg-[#e8b830]/5 border border-[#e8b830]/10 mb-2">
              <div className="flex-1">
                <Link to={`/runtime/node/${n.node_id}`} className="text-sm font-medium text-a-fg hover:text-a-accent hover:underline">
                  {n.name || n.hostname || n.node_id}
                </Link>
                <div className="text-xs text-a-muted mt-0.5">
                  <span className="font-mono">期望 r{n.desired_revision}</span>
                  <span className="mx-2">→</span>
                  <span className="font-mono text-[#e8b830]">实际 r{n.applied_revision}</span>
                  {n.agent_version && <span className="ml-3">v{n.agent_version}</span>}
                </div>
              </div>
              <StatusBadge status="drifted" />
              <Btn onClick={() => updateMut.mutate(n.node_id)} disabled={updateMut.isPending}>
                {updateMut.isPending ? '更新中...' : '触发更新'}
              </Btn>
            </div>
          ))}
        </Card>
      )}

      <Card title={`所有节点 (${nodeList.length})`}>
        <div className="space-y-1">
          {nodeList.map((n: any) => (
            <div key={n.node_id} className="flex items-center gap-3 px-3 py-2 text-xs">
              <Link to={`/runtime/node/${n.node_id}`} className="font-medium text-a-fg hover:text-a-accent hover:underline w-32">
                {n.name || n.hostname || n.node_id}
              </Link>
              <span className="font-mono text-a-muted">r{n.applied_revision || '?'}</span>
              <StatusBadge status={n.sync_status || n.status} />
              <span className="text-a-muted">{n.agent_version || '—'}</span>
              <span className="text-a-muted/50 ml-auto">{n.last_heartbeat_at ? new Date(n.last_heartbeat_at).toLocaleString('zh-CN') : '—'}</span>
            </div>
          ))}
        </div>
      </Card>
    </div>
  );
}

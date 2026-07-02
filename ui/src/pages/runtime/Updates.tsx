// ─── Updates ───
import { PageHeader, Card, StatusBadge, Btn, Timestamp } from '@/components/shared';
import { getScenario } from '@/mocks';
import { API_CONFIG } from '@/lib/api-config';

export default function Updates() {
  const nodes = API_CONFIG.useMock ? getScenario().nodes : [];
  const outdated = nodes.filter(n => n.sync_status === 'outdated' || n.desired_revision !== n.applied_revision);

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="更新管理" subtitle={outdated.length > 0 ? `${outdated.length} 个节点需要更新` : '所有节点已是最新'} />

      {outdated.length > 0 && (
        <Card title="待更新节点">
          {outdated.map(n => (
            <div key={n.node_id} className="flex items-center gap-4 p-3 rounded-a-sm bg-[#e8b830]/3 border border-[#e8b830]/10 mb-2">
              <div className="flex-1">
                <span className="text-sm font-medium text-a-fg">{n.name}</span>
                <div className="text-xs text-a-muted mt-0.5">
                  <span className="font-mono">期望 v{n.desired_revision}</span>
                  <span className="mx-2">→</span>
                  <span className="font-mono text-[#e8b830]">实际 v{n.applied_revision}</span>
                </div>
              </div>
              <StatusBadge status="drifted" />
              <Btn>触发更新</Btn>
            </div>
          ))}
        </Card>
      )}

      <Card title="所有节点">
        <div className="space-y-1">
          {nodes.map(n => (
            <div key={n.node_id} className="flex items-center gap-3 px-3 py-2 text-xs">
              <span className="font-medium text-a-fg w-24">{n.name}</span>
              <span className="font-mono text-a-muted">v{n.applied_revision}</span>
              <StatusBadge status={n.sync_status} />
              <span className="text-a-muted">{n.agent_version}</span>
              <Timestamp iso={n.last_heartbeat_at} />
            </div>
          ))}
        </div>
      </Card>
    </div>
  );
}

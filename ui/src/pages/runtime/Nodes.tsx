// ─── Nodes ───
import { useApiList } from '@/hooks/use-api';
import { useNavigate } from 'react-router-dom';
import { fetchNodes } from '@/lib/api-bridge';
import { Card, PageHeader, QueryGuard, StatusBadge, CapabilityBadge } from '@/components/shared';

export default function Nodes() {
  const nav = useNavigate();
  const { items: nodes, isLoading, error, refetch } = useApiList<any>(['nodes'], () => fetchNodes());

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="节点列表" subtitle={`${nodes.length} 个节点`} />
      <QueryGuard items={nodes} isLoading={isLoading} error={error} refetch={refetch} emptyMessage="暂无节点">
        {(items) => (
          <Card>
            <table className="w-full text-xs">
              <thead>
                <tr className="border-b border-a-border text-a-muted text-left">
                  <th className="py-2 px-3">节点</th>
                  <th className="py-2 px-3">公网 IP</th>
                  <th className="py-2 px-3">内网 IP</th>
                  <th className="py-2 px-3">运行状态</th>
                  <th className="py-2 px-3">配置版本</th>
                  <th className="py-2 px-3">软件版本</th>
                  <th className="py-2 px-3">能力</th>
                </tr>
              </thead>
              <tbody>
                {items.map((n: any) => (
                  <tr key={n.node_id}
                    className="border-b border-a-border/50 hover:bg-a-border/10 cursor-pointer"
                    onClick={() => nav(`/runtime/node/${n.node_id}`)}>
                    <td className="py-2.5 px-3">
                      <div className="font-medium text-a-fg">{n.name || n.hostname || n.node_id}</div>
                      {n.hostname && n.hostname !== n.name && (
                        <div className="text-[10px] text-a-muted font-mono">{n.hostname}</div>
                      )}
                    </td>
                    <td className="py-2.5 px-3 font-mono text-a-fg2">{n.public_ip || '—'}</td>
                    <td className="py-2.5 px-3 font-mono text-a-muted">{n.private_ip || '—'}</td>
                    <td className="py-2.5 px-3"><StatusBadge status={n.status} /></td>
                    <td className="py-2.5 px-3">
                      {n.desired_revision ? (
                        n.desired_revision === n.applied_revision ? (
                          <span className="font-mono text-[#4cd964]">r{n.applied_revision}</span>
                        ) : (
                          <span className="font-mono">
                            <span className="text-a-fg">r{n.desired_revision}</span>
                            <span className="text-a-muted mx-1">→</span>
                            <span className="text-[#e8b830]">r{n.applied_revision}</span>
                          </span>
                        )
                      ) : (
                        <span className="text-a-muted">—</span>
                      )}
                    </td>
                    <td className="py-2.5 px-3">
                      <span className="font-mono text-a-fg2">{n.agent_version || '—'}</span>
                    </td>
                    <td className="py-2.5 px-3">
                      <div className="flex gap-1 flex-wrap">
                        {n.capabilities?.gateway_enabled && <CapabilityBadge name="网关" enabled />}
                        {n.capabilities?.relay_capable && <CapabilityBadge name="中继" enabled />}
                        {n.capabilities?.caddy_installed && <CapabilityBadge name="Caddy" enabled />}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </Card>
        )}
      </QueryGuard>
    </div>
  );
}

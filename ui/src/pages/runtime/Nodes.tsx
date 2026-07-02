import { useQuery } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { fetchNodes } from '@/lib/api-bridge';
import { Card, PageHeader, StatusBadge, CapabilityBadge } from '@/components/shared';

export default function Nodes() {
  const nav = useNavigate();
  const { data } = useQuery({ queryKey: ['nodes'], queryFn: fetchNodes });
  const nodes = (data as any)?.nodes || [];

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="节点列表" subtitle={`${nodes.length} 个节点`} />
      <Card>
        <table className="w-full text-xs">
          <thead><tr className="border-b border-a-border text-a-muted text-left"><th className="py-2 px-3">名称</th><th className="py-2 px-3">IP</th><th className="py-2 px-3">状态</th><th className="py-2 px-3">同步</th><th className="py-2 px-3">能力</th></tr></thead>
          <tbody>
            {nodes.map((n: any) => (
              <tr key={n.node_id} className="border-b border-a-border/50 hover:bg-a-border/10 cursor-pointer" onClick={() => nav(`/runtime/node/${n.node_id}`)}>
                <td className="py-2.5 px-3 font-medium text-a-fg">{n.name}</td>
                <td className="py-2.5 px-3 font-mono text-a-muted">{n.public_ip}</td>
                <td className="py-2.5 px-3"><StatusBadge status={n.status} /></td>
                <td className="py-2.5 px-3"><StatusBadge status={n.sync_status} /></td>
                <td className="py-2.5 px-3 flex gap-1 flex-wrap">
                  {n.capabilities?.gateway_enabled && <CapabilityBadge name="网关" enabled />}
                  {n.capabilities?.relay_capable && <CapabilityBadge name="中继" enabled />}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </Card>
    </div>
  );
}

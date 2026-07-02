import { useQuery } from '@tanstack/react-query';
import { fetchEndpoints } from '@/lib/api-bridge';
import { Card, PageHeader, StatusBadge, HealthDot } from '@/components/shared';

export default function Endpoints() {
  const { data, isLoading } = useQuery({ queryKey: ['endpoints'], queryFn: fetchEndpoints });
  const endpoints = Array.isArray(data) ? data : [];

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="端点列表" subtitle={`${endpoints.length} 个端点`} />
      <Card>
        <table className="w-full text-xs">
          <thead><tr className="border-b border-a-border text-a-muted text-left"><th className="py-2 px-3">端点</th><th className="py-2 px-3">节点</th><th className="py-2 px-3">目标</th><th className="py-2 px-3">健康</th></tr></thead>
          <tbody>
            {endpoints.map((e: any) => (
              <tr key={e.endpoint_id} className="border-b border-a-border/50 hover:bg-a-border/10">
                <td className="py-2 px-3 font-mono text-a-fg">{e.endpoint_id}</td>
                <td className="py-2 px-3">{e.node_name || e.node_id}</td>
                <td className="py-2 px-3 font-mono text-a-muted">{e.target_local_host}:{e.target_local_port}</td>
                <td className="py-2 px-3"><StatusBadge status={e.health_status} /></td>
              </tr>
            ))}
          </tbody>
        </table>
      </Card>
    </div>
  );
}

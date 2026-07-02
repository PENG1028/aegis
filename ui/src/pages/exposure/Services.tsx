// ─── Services (Exposure) ───
import { useQuery } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { fetchServices } from '@/lib/api-bridge';
import { Card, PageHeader, StatusBadge } from '@/components/shared';

export default function Services() {
  const navigate = useNavigate();
  const { data, isLoading } = useQuery({ queryKey: ['services'], queryFn: fetchServices });
  const services = (data as any)?.services || [];

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="服务列表" subtitle={`${services.length} 个服务`} />
      <Card>
        <div className="overflow-x-auto">
          <table className="w-full text-xs">
            <thead>
              <tr className="border-b border-a-border text-a-muted text-left">
                <th className="py-2 px-3 font-medium">名称</th>
                <th className="py-2 px-3 font-medium">类型</th>
                <th className="py-2 px-3 font-medium">上游</th>
                <th className="py-2 px-3 font-medium">健康</th>
                <th className="py-2 px-3 font-medium">延迟</th>
              </tr>
            </thead>
            <tbody>
              {services.map((s: any) => (
                <tr key={s.service_id} className="border-b border-a-border/50 hover:bg-a-border/10 cursor-pointer" onClick={() => navigate(`/exposure/service/${s.service_id}`)}>
                  <td className="py-2.5 px-3 font-medium text-a-fg">{s.name}</td>
                  <td className="py-2.5 px-3 text-a-muted">{s.kind}</td>
                  <td className="py-2.5 px-3 font-mono text-a-muted">{s.upstream_url || '—'}</td>
                  <td className="py-2.5 px-3"><StatusBadge status={s.health_status} /></td>
                  <td className="py-2.5 px-3 font-mono text-a-muted">{s.latency_ms ? `${s.latency_ms}ms` : '—'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Card>
    </div>
  );
}

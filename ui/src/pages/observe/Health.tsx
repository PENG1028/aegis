// ─── Health ───
import { useState } from 'react';
import { Card, PageHeader, Btn, StatusBadge, HealthDot } from '@/components/shared';
import { getScenario } from '@/mocks';
import { API_CONFIG } from '@/lib/api-config';
import { cn } from '@/lib/utils';

export default function Health() {
  const [ran, setRan] = useState(false);
  const [loading, setLoading] = useState(false);
  const nodes = API_CONFIG.useMock ? getScenario().nodes : [];
  const endpoints = API_CONFIG.useMock ? getScenario().endpoints : [];

  const runAll = async () => {
    setLoading(true);
    await new Promise(r => setTimeout(r, 600));
    setRan(true);
    setLoading(false);
  };

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="健康检查" subtitle="系统 · 端口 · 端点"
        actions={<Btn primary onClick={runAll} disabled={loading}>{loading ? '检查中...' : ran ? '重新检查' : '运行全部检查'}</Btn>} />

      {ran && (
        <>
          <Card title="节点健康">
            <div className="space-y-2">
              {nodes.map(n => (
                <div key={n.node_id} className={cn('flex items-center gap-3 p-3 rounded-a-sm border text-xs',
                  n.status === 'online' ? 'bg-[#4cd964]/3 border-[#4cd964]/10' :
                  n.status === 'degraded' ? 'bg-[#e8b830]/3 border-[#e8b830]/10' :
                  'bg-[#ff5c72]/3 border-[#ff5c72]/10')}>
                  <HealthDot status={n.status === 'online' ? 'healthy' : n.status === 'degraded' ? 'degraded' : 'failed'} />
                  <span className="font-medium text-a-fg w-20">{n.name}</span>
                  <span className="font-mono text-a-muted">{n.public_ip}</span>
                  <span className="flex-1" />
                  <span className="text-a-muted">心跳: {n.last_heartbeat_at ? new Date(n.last_heartbeat_at).toLocaleTimeString('zh-CN') : '—'}</span>
                  <StatusBadge status={n.status} />
                </div>
              ))}
            </div>
          </Card>

          <Card title="端点健康">
            <div className="space-y-2">
              {endpoints.map(ep => (
                <div key={ep.endpoint_id} className={cn('flex items-center gap-3 p-2.5 rounded-a-sm border text-xs',
                  ep.health_status === 'healthy' ? 'bg-[#4cd964]/3 border-[#4cd964]/10' :
                  ep.health_status === 'unhealthy' ? 'bg-[#ff5c72]/3 border-[#ff5c72]/10' :
                  'bg-a-border/10 border-a-border')}>
                  <HealthDot status={ep.health_status === 'healthy' ? 'healthy' : ep.health_status === 'unhealthy' ? 'failed' : 'unknown'} />
                  <span className="font-mono text-a-fg">{ep.target_local_host}:{ep.target_local_port}</span>
                  <span className="text-a-muted">{ep.node_name || ep.node_id}</span>
                  <span className="flex-1" />
                  <StatusBadge status={ep.health_status} />
                </div>
              ))}
            </div>
          </Card>
        </>
      )}

      {!ran && (
        <Card title="全面健康检查">
          <div className="text-center py-8 text-a-muted text-sm">
            <div className="text-3xl mb-3">🩺</div>
            <p>运行全面健康检查：节点心跳、端点可达性、端口冲突扫描</p>
          </div>
        </Card>
      )}
    </div>
  );
}

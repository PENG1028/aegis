import { useQuery } from '@tanstack/react-query';
import { fetchDashboard } from '@/lib/api-bridge';
import { StatCard, Card, StatusBadge, Alert } from '@/components/shared';

export default function DashboardPage() {
  const { data, isLoading, error } = useQuery({
    queryKey: ['dashboard'],
    queryFn: fetchDashboard,
  });

  if (isLoading) {
    return <div className="text-center py-10 text-a-muted font-mono text-sm">加载中...</div>;
  }

  if (error) {
    return (
      <div>
        <div className="flex items-center justify-between mb-5">
          <div><h2 className="text-lg font-bold text-a-fg">总览</h2><p className="text-xs text-a-muted mt-0.5">多节点 Aegis 控制面运行状态</p></div>
          
        </div>
        <div className="px-4 py-3 rounded-a-md text-xs border bg-[#ff5c72]/10 text-[#ff5c72] border-[#ff5c72]/20">加载失败: {error.message}</div>
      </div>
    );
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-5">
        <div>
          <h2 className="text-lg font-bold text-a-fg">总览</h2>
          <p className="text-xs text-a-muted mt-0.5">多节点 Aegis 控制面运行状态</p>
        </div>
        
      </div>

      {data && (
        <>
          {/* Stats row */}
          <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-5">
            <StatCard label="Nodes 在线" value={`${data.nodes_online}/${data.nodes_total}`} sub={data.nodes_total - data.nodes_online > 0 ? `${data.nodes_total - data.nodes_online} offline` : '全部在线'} success={data.nodes_online === data.nodes_total} warn={data.nodes_online < data.nodes_total} />
            <StatCard label="Gateways 在线" value={`${data.gateways_online}/${data.gateways_total}`} accent />
            <StatCard label="Managed Routes" value={data.managed_routes} success />
            <StatCard label="Routing Tables" value={`${data.routing_tables_synced}/${data.routing_tables_total}`} sub={data.routing_tables_synced === data.routing_tables_total ? '全部同步' : '部分未同步'} success={data.routing_tables_synced === data.routing_tables_total} warn={data.routing_tables_synced < data.routing_tables_total} />
            <StatCard label="Local Gateway" value={`${data.local_gateway_online}/${data.local_gateway_total}`} sub={data.local_gateway_online > 0 ? `${data.local_gateway_online} running` : '无'} accent />
            <StatCard label="Relay 验收" value={data.relay_acceptance === 'real_two_node_local_gateway_verified' ? '通过' : data.relay_acceptance} success />
            <StatCard label="密钥运行时" value={data.secret_runtime === 'code_verified' ? '代码已验证' : data.secret_runtime} accent />
            <StatCard label="Pending 能力" value={data.pending_capabilities.length} warn />
          </div>

          {/* Attention areas */}
          <div className="grid grid-cols-2 gap-4 mb-4">
            <Card title="路由健康">
              <div className="space-y-2">
                <div className="flex justify-between text-xs py-1.5 border-b border-a-border-soft">
                  <span className="text-a-muted">Routes Unavailable</span>
                  <span className={data.routes_unavailable > 0 ? 'text-a-danger font-mono' : 'text-a-success font-mono'}>
                    {data.routes_unavailable === 0 ? '0 ✓' : data.routes_unavailable}
                  </span>
                </div>
                <div className="flex justify-between text-xs py-1.5 border-b border-a-border-soft">
                  <span className="text-a-muted">Missing GatewayLinks</span>
                  <span className={data.missing_gateway_links > 0 ? 'text-a-warn font-mono' : 'text-a-success font-mono'}>
                    {data.missing_gateway_links === 0 ? '0 ✓' : data.missing_gateway_links}
                  </span>
                </div>
                <div className="flex justify-between text-xs py-1.5 border-b border-a-border-soft">
                  <span className="text-a-muted">Outdated Nodes</span>
                  <span className={data.outdated_nodes > 0 ? 'text-a-warn font-mono' : 'text-a-success font-mono'}>
                    {data.outdated_nodes === 0 ? '0 ✓' : data.outdated_nodes}
                  </span>
                </div>
              </div>
            </Card>

            <Card title="Pending Capabilities">
              <div className="space-y-1.5">
                {data.pending_capabilities.map((cap) => (
                  <div key={cap} className="flex items-center gap-2">
                    <span className="w-1.5 h-1.5 rounded-full bg-[#e8b830] shrink-0" />
                    <StatusBadge status={cap} />
                  </div>
                ))}
                {data.pending_capabilities.length === 0 && (
                  <div className="text-xs text-a-muted">全部能力已验证 ✓</div>
                )}
              </div>
            </Card>
          </div>

          {/* Recent errors */}
          <Card title="最近错误">
            {data.recent_errors.length > 0 ? (
              <div className="space-y-2">
                {data.recent_errors.map((err, i) => (
                  <div key={i} className="flex items-start gap-2 text-xs bg-[#ff5c72]/5 px-3 py-2 rounded-a-sm">
                    <span className="text-a-danger shrink-0 mt-0.5">✗</span>
                    <div>
                      <span className="font-semibold">{err.node_name}</span>
                      <span className="text-a-muted ml-2">{err.error}</span>
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <div className="text-center py-6 text-a-muted text-xs">✓ 无最近错误</div>
            )}
          </Card>

          {/* Summary */}
          <Alert type="info" className="mt-4">
            <span className="font-medium mr-2">已验证链路:</span>
            <span className="font-mono text-xs">Node A Local Gateway → Node B /__aegis/relay → target HTTP 200 ✓</span>
          </Alert>
        </>
      )}
    </div>
  );
}

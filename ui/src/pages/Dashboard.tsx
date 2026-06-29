import { useQuery } from '@tanstack/react-query';
import { fetchDashboard, dnsApi, clusterHealthApi, systemHealthApi } from '@/lib/api-bridge';
import { StatCard, Card, StatusBadge, Alert } from '@/components/shared';

export default function DashboardPage() {
  const { data, isLoading, error } = useQuery({
    queryKey: ['dashboard'],
    queryFn: fetchDashboard,
  });

  const { data: dnsStatus } = useQuery({
    queryKey: ['dns-status'],
    queryFn: () => dnsApi.status(),
    refetchInterval: 15000,
  });

  const { data: clusterHealth } = useQuery({
    queryKey: ['cluster-health'],
    queryFn: () => clusterHealthApi.get(),
    refetchInterval: 15000,
  });

  const { data: sysHealth } = useQuery({
    queryKey: ['system-health'],
    queryFn: () => systemHealthApi.get(),
    refetchInterval: 30000,
  });

  function fmtDisk(bytes: number) {
    if (!bytes) return '—';
    const gb = bytes / (1024 * 1024 * 1024);
    return gb >= 1 ? `${gb.toFixed(1)} GB` : `${(bytes / (1024 * 1024)).toFixed(0)} MB`;
  }

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
            <StatCard label="节点在线" value={`${data.nodes_online}/${data.nodes_total}`} sub={data.nodes_total - data.nodes_online > 0 ? `${data.nodes_total - data.nodes_online} offline` : '全部在线'} success={data.nodes_online === data.nodes_total} warn={data.nodes_online < data.nodes_total} />
            <StatCard label="网关在线" value={`${data.gateways_online}/${data.gateways_total}`} accent />
            <StatCard label="管理路由" value={data.managed_routes} success />
            <StatCard label="路由表" value={`${data.routing_tables_synced}/${data.routing_tables_total}`} sub={data.routing_tables_synced === data.routing_tables_total ? '全部同步' : '部分未同步'} success={data.routing_tables_synced === data.routing_tables_total} warn={data.routing_tables_synced < data.routing_tables_total} />
            <StatCard label="本地网关" value={`${data.local_gateway_online}/${data.local_gateway_total}`} sub={data.local_gateway_online > 0 ? `${data.local_gateway_online} running` : '无'} accent />
            <StatCard label="中继验收" value={data.relay_acceptance === 'real_two_node_local_gateway_verified' ? '通过' : data.relay_acceptance} success />
            <StatCard label="密钥运行时" value={data.secret_runtime === 'code_verified' ? '代码已验证' : data.secret_runtime} accent />
            <StatCard label="待处理能力" value={data.pending_capabilities.length} warn />
            <StatCard label="DNS 解析" value={dnsStatus?.running ? '运行中' : '已停用'} sub={`本地 ${dnsStatus?.managed_count ?? 0} 域名`} success={!!dnsStatus?.running} accent={!!dnsStatus?.running} />
          </div>

          {/* System health bar */}
          {sysHealth && (
            <div className="grid grid-cols-2 md:grid-cols-5 gap-3 mb-4">
              <StatCard label="SQLite" value={sysHealth.sqlite_ok ? '正常' : '异常'} sub={fmtDisk(sysHealth.sqlite_size_bytes)} success={sysHealth.sqlite_ok} />
              <StatCard label="磁盘可用" value={fmtDisk(sysHealth.disk_free_bytes)} sub={`/ ${fmtDisk(sysHealth.disk_total_bytes)}`} accent />
              <StatCard label="内存" value={`${sysHealth.memory_used_mb} MB`} sub={`/ ${sysHealth.memory_total_mb} MB`} accent />
              <StatCard label="Go Routine" value={sysHealth.goroutines} sub={sysHealth.go_version} accent />
              <StatCard label="运行时间" value={sysHealth.uptime_seconds > 3600 ? `${Math.floor(sysHealth.uptime_seconds / 3600)}h` : `${Math.floor(sysHealth.uptime_seconds / 60)}m`} sub={sysHealth.uptime_seconds > 86400 ? `${Math.floor(sysHealth.uptime_seconds / 86400)}d` : 'today'} accent />
            </div>
          )}

          {/* Attention areas */}
          <div className="grid grid-cols-2 gap-4 mb-4">
            <Card title="路由健康">
              <div className="space-y-2">
                <div className="flex justify-between text-xs py-1.5 border-b border-a-border-soft">
                  <span className="text-a-muted">路由不可用</span>
                  <span className={data.routes_unavailable > 0 ? 'text-a-danger font-mono' : 'text-a-success font-mono'}>
                    {data.routes_unavailable === 0 ? '0 ✓' : data.routes_unavailable}
                  </span>
                </div>
                <div className="flex justify-between text-xs py-1.5 border-b border-a-border-soft">
                  <span className="text-a-muted">缺失网关链接</span>
                  <span className={data.missing_gateway_links > 0 ? 'text-a-warn font-mono' : 'text-a-success font-mono'}>
                    {data.missing_gateway_links === 0 ? '0 ✓' : data.missing_gateway_links}
                  </span>
                </div>
                <div className="flex justify-between text-xs py-1.5 border-b border-a-border-soft">
                  <span className="text-a-muted">过期节点</span>
                  <span className={data.outdated_nodes > 0 ? 'text-a-warn font-mono' : 'text-a-success font-mono'}>
                    {data.outdated_nodes === 0 ? '0 ✓' : data.outdated_nodes}
                  </span>
                </div>
              </div>
            </Card>

            <Card title="待处理能力">
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
          {/* Cluster Health */}
          {clusterHealth && (
            <Card title={`集群健康 · ${clusterHealth.node_count} 节点`}>
              {/* Split-brain warning */}
              {clusterHealth.split_brain && (
                <Alert type="error" className="mb-3">
                  <span className="font-bold">⚠ 裂脑检测:</span> 检测到多领导者！{clusterHealth.issues?.filter(i => i.includes('SPLIT_BRAIN')).join(', ')}
                </Alert>
              )}

              {/* Overall status */}
              <div className="flex items-center gap-2 mb-3 text-xs">
                <span className={`w-2 h-2 rounded-full ${clusterHealth.overall_healthy ? 'bg-[#4cd964]' : 'bg-[#ff5c72]'}`} />
                <span className="font-medium">{clusterHealth.overall_healthy ? '集群正常' : '集群异常'}</span>
                {clusterHealth.leader_node_id && (
                  <span className="text-a-muted ml-2">Leader: <code className="text-a-accent">{clusterHealth.leader_node_id}</code></span>
                )}
              </div>

              {/* Per-node table */}
              <div className="overflow-x-auto">
                <table className="w-full text-xs">
                  <thead>
                    <tr className="border-b border-a-border text-a-muted text-left">
                      <th className="py-2 px-2 font-medium">节点</th>
                      <th className="py-2 px-2 font-medium">角色</th>
                      <th className="py-2 px-2 font-medium">状态</th>
                      <th className="py-2 px-2 font-medium">同步</th>
                      <th className="py-2 px-2 font-medium">版本</th>
                      <th className="py-2 px-2 font-medium">心跳</th>
                    </tr>
                  </thead>
                  <tbody>
                    {clusterHealth.nodes.map((n) => (
                      <tr key={n.node_id} className="border-b border-a-border/50">
                        <td className="py-2 px-2 font-mono text-a-fg">
                          {n.hostname}
                          {n.is_leader && <span className="ml-1 text-[10px] text-[#e8b830]">LEADER</span>}
                        </td>
                        <td className="py-2 px-2 text-a-muted">{n.role}</td>
                        <td className="py-2 px-2">
                          <StatusBadge status={n.status} />
                        </td>
                        <td className="py-2 px-2">
                          <span className={n.sync_status === 'in_sync' ? 'text-[#4cd964]' : n.sync_status === 'out_of_sync' ? 'text-[#ff5c72]' : 'text-a-muted'}>
                            {n.sync_status === 'in_sync' ? '✓' : n.sync_status === 'out_of_sync' ? '✗' : '—'} {n.sync_status}
                          </span>
                        </td>
                        <td className="py-2 px-2 font-mono text-a-muted">
                          {n.applied_revision}/{n.desired_revision}
                        </td>
                        <td className="py-2 px-2 font-mono text-a-muted">
                          {n.heartbeat_age || '—'}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>

              {/* Issues list */}
              {clusterHealth.issues && clusterHealth.issues.length > 0 && (
                <div className="mt-3 pt-3 border-t border-a-border">
                  <div className="text-xs font-medium text-[#ff5c72] mb-2">问题 ({clusterHealth.issues.length})</div>
                  {clusterHealth.issues.map((issue, i) => (
                    <div key={i} className="text-xs py-1 text-a-muted font-mono">
                      <span className="text-[#ff5c72] mr-1.5">•</span>
                      {issue}
                    </div>
                  ))}
                </div>
              )}
            </Card>
          )}

          <Alert type="info" className="mt-4">
            <span className="font-medium mr-2">已验证链路:</span>
            <span className="font-mono text-xs">Node A Local Gateway → Node B /__aegis/relay → target HTTP 200 ✓</span>
          </Alert>
        </>
      )}
    </div>
  );
}

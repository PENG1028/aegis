import { useQuery } from '@tanstack/react-query';
import { fetchLocalGatewayStatus } from '@/lib/api-bridge';
import { PageHeader, Card, StatusBadge, StatCard, WarningCard } from '@/components/shared';
import type { LocalGatewayStatus } from '@/types';

export default function LocalGatewayRuntimePage() {
  const { data, isLoading, error } = useQuery({
    queryKey: ['local-gateway'],
    queryFn: () => fetchLocalGatewayStatus(),
  });

  if (isLoading) return <div className="text-center py-10 text-a-muted font-mono text-sm">加载中...</div>;
  if (error) return <div className="px-4 py-3 rounded-a-md text-xs border bg-[#ff5c72]/10 text-[#ff5c72] border-[#ff5c72]/20">加载失败: {error.message}</div>;

  const runningCount = data?.filter((g) => g.status === 'running').length || 0;

  return (
    <div>
      <PageHeader title="本地网关运行时" helpKey="local-gateway" subtitle="本地 HTTP 网关运行时状态"  />

      <div className="grid grid-cols-3 gap-3 mb-5">
        <StatCard label="节点" value={data?.length || 0} accent />
        <StatCard label="运行中" value={runningCount} success={runningCount > 0} />
        <StatCard label="已加载条目" value={data?.reduce((s, g) => s + g.entries_count, 0) || 0} />
      </div>

      {data && data.map((gw) => (
        <Card key={gw.node_id} title={gw.node_name} subtitle={`${gw.bind_addr}:${gw.port}`} className="mb-4">
          <div className="space-y-3">
            <div className="flex items-center gap-3">
              <StatusBadge status={gw.status} />
              <span className="text-xs text-a-muted font-mono">
                Routing: {gw.routing_table_loaded ? '✓ 已加载' : '✗ 未加载'}
                {gw.routing_table_revision !== null && ` (rev ${gw.routing_table_revision})`}
              </span>
              <span className="text-xs text-a-muted">{gw.entries_count} 条</span>
              <StatusBadge status={gw.cache_status === 'fresh' ? 'ok' : gw.cache_status === 'stale' ? 'warning' : 'error'} />
            </div>

            <div className="text-xs text-a-muted">
              Cache: {gw.cache_status} · 上次错误: {gw.last_error || '无'}
            </div>

            {gw.diagnostics.length > 0 && (
              <div className="bg-a-bg border border-a-border rounded-a-sm p-3">
                <div className="text-[11px] font-semibold text-a-fg mb-2">诊断</div>
                <div className="space-y-1">
                  {gw.diagnostics.map((d, i) => (
                    <div key={i} className="flex items-center gap-2 text-xs">
                      <StatusBadge status={d.status === 'ok' ? 'ok' : d.status === 'warning' ? 'warning' : 'error'} />
                      <span className="font-mono">{d.name}</span>
                      <span className="text-a-muted">{d.message}</span>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>
        </Card>
      ))}

      <Card title="开发者入口说明">
        <div className="space-y-3 text-xs">
          <div>
            <div className="font-semibold text-a-fg mb-1">Mode A: Host Header</div>
            <code className="block bg-a-bg border border-a-border rounded-a-sm p-2 font-mono text-[11px] text-a-accent mt-1">
              curl -H "Host: api-b.example.com" http://127.0.0.1:18080/health
            </code>
            <p className="text-a-muted mt-1">推荐模式，无需系统配置，跨平台</p>
          </div>
          <div>
            <div className="font-semibold text-a-fg mb-1">Mode B: Hosts File + Dev Port</div>
            <code className="block bg-a-bg border border-a-border rounded-a-sm p-2 font-mono text-[11px] text-a-muted mt-1">
              127.0.0.1 api-b.example.com{'\n'}
              curl http://api-b.example.com:18080/health
            </code>
          </div>
          <div>
            <div className="font-semibold text-a-fg mb-1">Mode C: Port 80 绑定</div>
            <p className="text-a-muted">绑定 80 端口需要 root/systemd，风险说明见文档。</p>
            <WarningCard title="安全提示" type="warn" className="mt-2">
              <p>UI 不会自动修改 /etc/hosts，不会自动安装 root CA，不会声称 HTTPS 已支持。</p>
            </WarningCard>
          </div>
        </div>
      </Card>
    </div>
  );
}

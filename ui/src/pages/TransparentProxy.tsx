import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { transparentApi } from '@/lib/api-bridge';
import { PageHeader, Card, StatusBadge } from '@/components/shared';
import { useState } from 'react';
import { fmtBytes, fmtShort } from '@/lib/utils';

interface TransparentRule {
  id: string;
  original_ip: string;
  original_port: number;
  local_proxy_port: number;
  target_service: string;
  target_node: string;
  description: string;
  active: boolean;
  bytes_in: number;
  bytes_out: number;
}

export default function TransparentProxyPage() {
  const queryClient = useQueryClient();
  const [deleting, setDeleting] = useState<string | null>(null);

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['transparent-rules'],
    queryFn: () => transparentApi.listRules(),
    refetchInterval: 10_000,
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => {
      setDeleting(id);
      return transparentApi.deleteRule(id);
    },
    onSettled: () => {
      setDeleting(null);
      queryClient.invalidateQueries({ queryKey: ['transparent-rules'] });
    },
  });

  const rules: TransparentRule[] = data?.rules || [];
  const activeCount = rules.filter((r) => r.active).length;
  const totalBytesIn = rules.reduce((s, r) => s + (r.bytes_in || 0), 0);
  const totalBytesOut = rules.reduce((s, r) => s + (r.bytes_out || 0), 0);

  if (isLoading) return <div className="text-center py-10 text-a-muted font-mono text-sm">加载中...</div>;
  if (error) return <div className="px-4 py-3 rounded-a-md text-xs border bg-[#ff5c72]/10 text-[#ff5c72] border-[#ff5c72]/20">加载失败: {(error as Error).message}</div>;

  return (
    <div>
      <PageHeader
        title="透明代理"
        helpKey="transparent-proxy"
        subtitle={
          rules.length === 0
            ? '无活跃规则'
            : `${rules.length} 条规则 · ${activeCount} 活跃 · 入 ${fmtBytes(totalBytesIn)} / 出 ${fmtBytes(totalBytesOut)}`
        }
        actions={
          <button
            onClick={() => refetch()}
            className="px-3 py-1.5 text-xs rounded-a-md border border-a-border text-a-muted hover:text-a-fg hover:border-a-accent transition-colors bg-transparent cursor-pointer"
          >
            刷新
          </button>
        }
      />

      {data?.message && rules.length === 0 && (
        <Card>
          <div className="text-center py-8 text-a-muted text-sm">
            <div className="text-3xl mb-3 opacity-30">🔀</div>
            <p>透明代理未配置或不可用</p>
            <p className="text-xs mt-1 opacity-60">{data.message}</p>
            <div className="mt-4 text-xs text-left max-w-lg mx-auto space-y-1 opacity-70">
              <p className="font-semibold text-a-fg/70">需要以下条件：</p>
              <p>1. Linux 系统 + root 权限（iptables DNAT）</p>
              <p>2. 数据库中有启用端点（含端口号，如 127.0.0.1:9100）</p>
              <p>3. 端点关联的节点已注册 public_ip</p>
              <p>4. 触发一次端点变更（API mutation）以生成规则</p>
            </div>
          </div>
        </Card>
      )}

      {rules.length > 0 && (
        <Card>
          <div className="overflow-x-auto">
            <table className="w-full text-xs">
              <thead>
                <tr className="border-b border-a-border text-a-muted text-left">
                  <th className="py-2 px-3 font-medium">目标</th>
                  <th className="py-2 px-3 font-medium">代理端口</th>
                  <th className="py-2 px-3 font-medium">状态</th>
                  <th className="py-2 px-3 font-medium">方向</th>
                  <th className="py-2 px-3 font-medium">流量</th>
                  <th className="py-2 px-3 font-medium">描述</th>
                  <th className="py-2 px-3 font-medium"></th>
                </tr>
              </thead>
              <tbody>
                {rules.map((rule) => (
                  <tr
                    key={rule.id}
                    className={`border-b border-a-border/50 hover:bg-a-border/10 transition-colors ${
                      !rule.active ? 'opacity-50' : ''
                    }`}
                  >
                    <td className="py-2 px-3 font-mono">
                      <span className="text-a-fg">{rule.original_ip}</span>
                      <span className="text-a-muted">:</span>
                      <span className="text-a-accent">{rule.original_port}</span>
                    </td>
                    <td className="py-2 px-3 font-mono text-a-muted">
                      :{rule.local_proxy_port}
                    </td>
                    <td className="py-2 px-3">
                      <StatusBadge status={rule.active ? 'active' : 'inactive'} />
                    </td>
                    <td className="py-2 px-3">
                      {rule.active ? (
                        rule.bytes_in > 0 || rule.bytes_out > 0 ? (
                          <span className="text-[#4cd964]">● 活跃</span>
                        ) : (
                          <span className="text-a-muted">○ 空闲</span>
                        )
                      ) : (
                        <span className="text-[#ff5c72]">✕ 停用</span>
                      )}
                    </td>
                    <td className="py-2 px-3 font-mono text-a-muted text-[11px]">
                      <span title="入站">↓{fmtBytes(rule.bytes_in)}</span>{' '}
                      <span title="出站">↑{fmtBytes(rule.bytes_out)}</span>
                    </td>
                    <td className="py-2 px-3 text-a-muted text-[11px] max-w-[200px] truncate" title={rule.description}>
                      {rule.description || '—'}
                    </td>
                    <td className="py-2 px-3">
                      <button
                        onClick={() => deleteMutation.mutate(rule.id)}
                        disabled={deleting === rule.id}
                        className="text-[10px] px-2 py-0.5 rounded border border-[#ff5c72]/30 text-[#ff5c72] hover:bg-[#ff5c72]/10 transition-colors disabled:opacity-30 bg-transparent cursor-pointer"
                      >
                        {deleting === rule.id ? '删除中...' : '删除'}
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Card>
      )}

      {/* Legend */}
      {rules.length > 0 && (
        <div className="mt-4 p-3 rounded-a-md bg-a-surface border border-a-border text-[11px] text-a-muted space-y-1">
          <p className="font-semibold text-a-fg/70 mb-2">工作方式</p>
          <p>· 本机进程发出的连接 → iptables OUTPUT 链 DNAT 劫持 → 本地代理端口</p>
          <p>· 同节点目标 → 直接转发到本地端口（不走网卡）</p>
          <p>· 跨节点目标 → 转发到 Caddy :80 → Gateway Link → 远程节点</p>
          <p>· 删除规则 = 移除 iptables DNAT + 停止本地代理监听</p>
        </div>
      )}
    </div>
  );
}

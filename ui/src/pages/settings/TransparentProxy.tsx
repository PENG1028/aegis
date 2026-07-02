// ─── Transparent Proxy Settings ───
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { transparentApi } from '@/lib/api-bridge';
import { Card, PageHeader, StatusBadge, Btn, useToast } from '@/components/shared';
import { fmtBytes } from '@/lib/utils';
import { useState } from 'react';

export default function TransparentProxyPage() {
  const queryClient = useQueryClient();
  const toast = useToast();
  const [deleting, setDeleting] = useState<string | null>(null);

  const { data, isLoading } = useQuery({
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
      toast('规则已删除');
    },
    onError: (e: any) => toast(e.message || '删除失败', 'error'),
  });

  const rules = Array.isArray(data) ? data : [];

  if (isLoading) return <div className="p-6 text-a-muted text-sm">加载中...</div>;

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="透明代理" subtitle="IP:端口 拦截规则" />

      <Card title={`规则列表 (${rules.length})`} subtitle="iptables DNAT 透明代理规则">
        {rules.length === 0 ? (
          <div className="text-center py-8 text-a-muted text-sm">
            <div className="text-3xl mb-3 opacity-30">🔀</div>
            <p>无活跃规则</p>
            <p className="text-xs mt-1 opacity-60">需要 Linux 系统 + root 权限</p>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-xs">
              <thead>
                <tr className="border-b border-a-border text-a-muted text-left">
                  <th className="py-2 px-3 font-medium">目标</th>
                  <th className="py-2 px-3 font-medium">代理端口</th>
                  <th className="py-2 px-3 font-medium">状态</th>
                  <th className="py-2 px-3 font-medium">流量</th>
                  <th className="py-2 px-3 font-medium"></th>
                </tr>
              </thead>
              <tbody>
                {rules.map((rule: any) => (
                  <tr key={rule.id} className="border-b border-a-border/50 hover:bg-a-border/10">
                    <td className="py-2 px-3 font-mono">
                      {rule.original_ip}:{rule.original_port}
                    </td>
                    <td className="py-2 px-3 font-mono text-a-muted">:{rule.local_proxy_port}</td>
                    <td className="py-2 px-3">
                      <StatusBadge status={rule.active ? 'active' : 'disabled'} />
                    </td>
                    <td className="py-2 px-3 font-mono text-[11px]">
                      ↓{fmtBytes(rule.bytes_in || 0)} ↑{fmtBytes(rule.bytes_out || 0)}
                    </td>
                    <td className="py-2 px-3">
                      <button
                        onClick={() => deleteMutation.mutate(rule.id)}
                        disabled={deleting === rule.id}
                        className="text-[10px] px-2 py-0.5 rounded border border-[#ff5c72]/30 text-[#ff5c72] hover:bg-[#ff5c72]/10 transition-colors disabled:opacity-30 cursor-pointer"
                      >
                        {deleting === rule.id ? '删除中...' : '删除'}
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Card>
    </div>
  );
}

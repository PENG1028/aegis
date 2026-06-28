import { useState } from 'react';
import { useQuery, useMutation } from '@tanstack/react-query';
import { dnsApi } from '@/lib/api-bridge';
import { Card, Btn, Alert, StatusBadge } from '@/components/shared';
import type { DnsStatus } from '@/types';

export default function DnsSettingsPanel() {
  const { data, isLoading, error, refetch } = useQuery<DnsStatus>({
    queryKey: ['dns-status'],
    queryFn: () => dnsApi.status(true),
    refetchInterval: 10000,
  });

  const enableMut = useMutation({
    mutationFn: dnsApi.enable,
    onSuccess: () => refetch(),
  });

  const disableMut = useMutation({
    mutationFn: dnsApi.disable,
    onSuccess: () => refetch(),
  });

  const refreshMut = useMutation({
    mutationFn: dnsApi.refresh,
    onSuccess: () => refetch(),
  });

  if (isLoading) return <Card title="DNS 解析器"><div className="p-[18px] text-xs text-a-muted">加载中...</div></Card>;
  if (error) return <Card title="DNS 解析器"><Alert type="err">加载失败</Alert></Card>;

  const running = data?.running ?? false;
  const enabled = data?.enabled ?? false;

  return (
    <Card title="DNS 解析器">
      <div className="p-[18px] space-y-4">
        {/* Status */}
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <span className={`w-2 h-2 rounded-full ${running ? 'bg-green-500' : 'bg-a-muted'}`} />
            <span className="text-sm font-medium">{running ? '运行中' : '已停止'}</span>
          </div>
          <div className="flex gap-2">
            {running ? (
              <Btn sm danger onClick={() => disableMut.mutate()} disabled={disableMut.isPending}>
                停用
              </Btn>
            ) : (
              <Btn sm primary onClick={() => enableMut.mutate()} disabled={enableMut.isPending}>
                启用
              </Btn>
            )}
            <Btn sm onClick={() => refreshMut.mutate()} disabled={refreshMut.isPending}>
              刷新
            </Btn>
          </div>
        </div>

        {/* Config */}
        <div className="grid grid-cols-2 gap-4 text-xs">
          <div>
            <span className="text-a-muted">监听地址</span>
            <div className="font-mono mt-0.5">{data?.listen_addr ?? ':5353'}</div>
          </div>
          <div>
            <span className="text-a-muted">上游 DNS</span>
            <div className="font-mono mt-0.5">{data?.upstream ?? '1.1.1.1:53'}</div>
          </div>
          <div>
            <span className="text-a-muted">配置文件</span>
            <div className="mt-0.5">{enabled ? '已启用' : '未启用'}</div>
          </div>
          <div>
            <span className="text-a-muted">本地解析域名</span>
            <div className="font-mono mt-0.5">{data?.managed_count ?? 0}</div>
          </div>
        </div>

        {/* Stats */}
        <div className="grid grid-cols-2 gap-4 text-xs pt-2 border-t border-a-border-soft">
          <div>
            <span className="text-a-muted">本地命中</span>
            <div className="font-mono mt-0.5">{data?.local_hits ?? 0}</div>
          </div>
          <div>
            <span className="text-a-muted">上游转发</span>
            <div className="font-mono mt-0.5">{data?.upstream_calls ?? 0}</div>
          </div>
        </div>

        {/* Resolution Chain */}
        <div className="pt-2 border-t border-a-border-soft">
          <div className="text-xs font-medium mb-2 text-a-muted">解析链路</div>
          <div className="bg-a-bg-muted rounded-a-md p-3 font-mono text-xs leading-relaxed">
            <div>App → Aegis DNS ({data?.listen_addr ?? ':53'})</div>
            <div className="pl-4">├─ 路由表命中 → 返回节点 IP</div>
            <div className="pl-4">│  ├─ 本机 → 127.0.0.1</div>
            <div className="pl-4">│  ├─ 私网可达 → 私网 IP</div>
            <div className="pl-4">│  └─ 公网 → 公网 IP</div>
            <div className="pl-4">└─ 未命中 → 上游 ({data?.upstream ?? '1.1.1.1:53'})</div>
          </div>
        </div>

        {/* Managed domains table */}
        {data?.entries && data.entries.length > 0 && (
          <div className="pt-2 border-t border-a-border-soft">
            <div className="text-xs font-medium mb-2 text-a-muted">已解析域名 ({data.entries.length})</div>
            <div className="overflow-x-auto">
              <table className="w-full text-xs">
                <thead>
                  <tr className="text-a-muted border-b border-a-border-soft">
                    <th className="text-left py-1 pr-3">域名</th>
                    <th className="text-left py-1 pr-3">目标 IP</th>
                    <th className="text-left py-1 pr-3">目标节点</th>
                    <th className="text-left py-1">类型</th>
                  </tr>
                </thead>
                <tbody>
                  {data.entries.map((e) => (
                    <tr key={e.domain} className="border-b border-a-border-soft/50">
                      <td className="py-1.5 pr-3 font-mono">{e.domain}</td>
                      <td className="py-1.5 pr-3 font-mono">{e.target_ip}</td>
                      <td className="py-1.5 pr-3 font-mono">{e.target_node}</td>
                      <td className="py-1.5">
                        <StatusBadge status={e.is_local ? 'local' : 'remote'} />
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        )}
      </div>
    </Card>
  );
}

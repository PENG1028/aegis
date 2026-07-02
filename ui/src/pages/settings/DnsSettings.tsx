// ─── DNS Settings ───
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { dnsApi } from '@/lib/api-bridge';
import { Card, PageHeader, Btn, StatusBadge, useToast } from '@/components/shared';

export default function DnsSettings() {
  const queryClient = useQueryClient();
  const toast = useToast();

  const { data, isLoading } = useQuery({
    queryKey: ['dns-status'],
    queryFn: () => dnsApi.status(),
    refetchInterval: 10_000,
  });

  const enableMut = useMutation({
    mutationFn: () => dnsApi.enable(),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['dns-status'] }); toast('DNS 已启用'); },
    onError: (e: any) => toast(e.message || '启用失败', 'error'),
  });

  const disableMut = useMutation({
    mutationFn: () => dnsApi.disable(),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['dns-status'] }); toast('DNS 已停用'); },
    onError: (e: any) => toast(e.message || '停用失败', 'error'),
  });

  const refreshMut = useMutation({
    mutationFn: () => dnsApi.refresh(),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['dns-status'] }); toast('DNS 已刷新'); },
    onError: (e: any) => toast(e.message || '刷新失败', 'error'),
  });

  if (isLoading) return <div className="p-6 text-a-muted text-sm">加载中...</div>;

  const d = data as any;

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="DNS 解析器" subtitle="内部 DNS 服务状态与控制" />

      <Card title="运行状态">
        <div className="grid grid-cols-2 gap-3 text-xs mb-4">
          <div><span className="text-a-muted">状态: </span><StatusBadge status={d?.running ? 'active' : 'disabled'} /></div>
          <div><span className="text-a-muted">启用: </span><StatusBadge status={d?.enabled ? 'active' : 'disabled'} /></div>
          <div><span className="text-a-muted">监听: </span><span className="font-mono text-a-fg">{d?.listen_addr || '—'}</span></div>
          <div><span className="text-a-muted">上游: </span><span className="font-mono text-a-fg">{d?.upstream || '—'}</span></div>
          <div><span className="text-a-muted">本地命中: </span><span className="text-a-fg">{d?.local_hits || 0}</span></div>
          <div><span className="text-a-muted">上游调用: </span><span className="text-a-fg">{d?.upstream_calls || 0}</span></div>
        </div>
        <div className="flex gap-2">
          {!d?.running && <Btn primary onClick={() => enableMut.mutate()} disabled={enableMut.isPending}>启用</Btn>}
          {d?.running && <Btn onClick={() => disableMut.mutate()} disabled={disableMut.isPending}>停用</Btn>}
          <Btn onClick={() => refreshMut.mutate()} disabled={refreshMut.isPending}>刷新记录</Btn>
        </div>
      </Card>
    </div>
  );
}

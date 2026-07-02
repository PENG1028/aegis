// ─── Providers ───
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { providerApi } from '@/lib/api-bridge';
import { PageHeader, StatusBadge, Btn, useToast, Card } from '@/components/shared';
import { cn } from '@/lib/utils';

export default function Providers() {
  const queryClient = useQueryClient();
  const toast = useToast();
  const { data, isLoading } = useQuery({
    queryKey: ['providers'],
    queryFn: () => providerApi.list(),
    refetchInterval: 120_000,
  });

  // For mock mode, show caddy + haproxy
  const providers = (data as any)?.providers || [
    { provider_id: 'caddy', name: 'Caddy', kind: 'caddy', status: 'active', version: 'v2.8.4', config_path: '/etc/caddy/Caddyfile' },
    { provider_id: 'haproxy', name: 'HAProxy', kind: 'haproxy', status: 'disabled', version: '—', config_path: '/etc/haproxy/haproxy.cfg' },
  ];

  if (isLoading) return <div className="p-6 text-a-muted text-sm">加载中...</div>;

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="Provider 管理" subtitle="中间件安装、配置、重载"
        actions={<Btn primary onClick={() => { providerApi.diagnoseAll(); toast('诊断已触发'); }}>诊断全部</Btn>} />

      <div className="space-y-3">
        {providers.map((p: any) => (
          <Card key={p.provider_id || p.name} title={p.name} subtitle={p.kind}>
            <div className="grid grid-cols-2 gap-2 text-xs mb-3">
              <div><span className="text-a-muted">状态: </span><StatusBadge status={p.status || 'unknown'} /></div>
              <div><span className="text-a-muted">版本: </span><span className="font-mono text-a-fg">{p.version || '—'}</span></div>
              <div className="col-span-2"><span className="text-a-muted">配置路径: </span><span className="font-mono text-a-fg">{p.config_path || '—'}</span></div>
            </div>
            <div className="flex gap-2">
              {p.status === 'active' ? (
                <>
                  <Btn onClick={() => providerApi.reload(p.provider_id || p.name).then(() => toast('重载成功')).catch((e: any) => toast(e.message, 'error'))}>重载</Btn>
                  <Btn onClick={() => providerApi.getConfig(p.provider_id || p.name).then(() => toast('配置已加载')).catch(() => {})}>查看配置</Btn>
                </>
              ) : (
                <Btn primary onClick={() => providerApi.install(p.provider_id || p.name).then(() => { queryClient.invalidateQueries({ queryKey: ['providers'] }); toast('安装已触发'); }).catch((e: any) => toast(e.message, 'error'))}>安装</Btn>
              )}
            </div>
          </Card>
        ))}
      </div>
    </div>
  );
}

import { useQuery, useQueryClient } from '@tanstack/react-query';
import { providerApi } from '@/lib/api-bridge';
import { PageHeader, Card, Btn, Alert, StatusBadge } from '@/components/shared';
import { useToast } from '@/components/shared/Toast';

export default function ProvidersPage() {
  const toast = useToast();
  const queryClient = useQueryClient();

  const { data, isLoading, error } = useQuery({
    queryKey: ['providers'],
    queryFn: () => providerApi.list(),
  });

  async function doDiagnose() {
    try {
      const res = await providerApi.diagnoseAll();
      toast(res.healthy ? '所有 Provider 正常' : `${res.issues} 个问题`);
      queryClient.invalidateQueries({ queryKey: ['providers'] });
    } catch (e: any) { toast(e.message, 'error'); }
  }

  if (isLoading) return <div className="text-center py-10 text-a-muted font-mono text-sm">加载中...</div>;
  if (error) return <Alert type="err">加载失败: {(error as any).message}</Alert>;

  const providers = data?.providers || [];

  return (
    <div>
      <PageHeader title="提供商" helpKey="providers" sub={`${providers.length} 个`} actions={
        <Btn primary onClick={doDiagnose}>重新诊断</Btn>
      } />

      {providers.map((p: any) => (
        <Card key={p.name || p.provider} title={p.name || p.provider} className="mb-4">
          <div className="p-[18px] grid grid-cols-3 gap-3">
            {['installed', 'version', 'service_running', 'config_valid', 'listener_ok', 'runtime_verify_ok'].map((k) => {
              const labels: Record<string, string> = {
                installed: '已安装',
                version: '版本',
                service_running: '服务运行',
                config_valid: '配置有效',
                listener_ok: '监听正常',
                runtime_verify_ok: '运行时验证',
              };
              return (
                <div key={k}>
                  <div className="text-[11px] text-a-muted uppercase tracking-[0.06em]">{labels[k] || k}</div>
                  <div className="text-sm mt-0.5">
                    {typeof p[k] === 'boolean'
                      ? <StatusBadge status={p[k] ? 'ok' : 'error'} />
                      : p[k] || '—'}
                  </div>
                </div>
              );
            })}
          </div>
          {p.last_error_message && (
            <div className="px-[18px] pb-[18px]">
              <div className="px-3 py-2 rounded-a-sm text-xs border bg-[#ff5c72]/10 text-[#ff5c72] border-[#ff5c72]/20">
                {p.last_error_code}: {p.last_error_message}
              </div>
            </div>
          )}
        </Card>
      ))}

      {providers.length === 0 && (
        <div className="text-center py-10 text-a-muted text-xs">暂无 Provider 数据</div>
      )}
    </div>
  );
}

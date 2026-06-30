/**
 * Providers — 中间件概览 (v1.8K)
 *
 * 简化版 — 快速状态一览。完整管理功能请至「中间件」页面。
 */

import { useQuery } from '@tanstack/react-query';
import { providerApi } from '@/lib/api-bridge';
import { useNavigate } from 'react-router-dom';
import { PageHeader, Card, Btn } from '@/components/shared';

export default function ProvidersPage() {
  const nav = useNavigate();

  const { data, isLoading } = useQuery({
    queryKey: ['provider-diagnostics'],
    queryFn: () => providerApi.diagnoseAll(),
    refetchInterval: 120_000,
  });

  const diagnostics = (data as any)?.diagnostics || [];
  const healthy = (data as any)?.healthy ?? true;

  return (
    <div>
      <PageHeader
        title="中间件状态"
        helpKey="providers"
        sub={healthy ? '全部正常' : `${(data as any)?.issues || 0} 个问题`}
        actions={
          <Btn primary onClick={() => nav('/middleware')}>完整管理 →</Btn>
        }
      />

      {isLoading ? (
        <div className="text-center py-10 text-a-muted font-mono text-sm">加载中…</div>
      ) : (
        <div className="space-y-3">
          {diagnostics.map((d: any) => (
            <Card
              key={d.provider}
              title={d.provider?.replace(/_/g, ' ').replace(/\b\w/g, (c: string) => c.toUpperCase())}
            >
              <div
                className="p-4 flex items-center gap-4 cursor-pointer hover:bg-a-bg/50"
                onClick={() => nav('/middleware')}
              >
                <span className={`w-2.5 h-2.5 rounded-full ${
                  d.service_running ? 'bg-[#4cd964]' : d.installed ? 'bg-[#e8b830]' : 'bg-a-muted'
                }`} />
                <div className="flex-1">
                  <div className="text-sm">
                    {d.service_running ? '运行中' : d.installed ? '已安装 · 未运行' : '未安装'}
                  </div>
                  <div className="text-[10px] text-a-muted mt-0.5">
                    {d.version} · {d.config_path}
                  </div>
                </div>
                <span className="text-a-muted text-xs">详情 →</span>
              </div>
            </Card>
          ))}
          {diagnostics.length === 0 && (
            <div className="text-center py-10 text-a-muted text-xs">暂无数据 · 前往 <a href="/middleware" className="text-a-accent hover:underline">中间件管理</a></div>
          )}
        </div>
      )}
    </div>
  );
}

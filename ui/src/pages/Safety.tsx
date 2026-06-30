import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { safetyApi } from '@/lib/api-bridge';
import { PageHeader, Card, StatCard, TabBar, Btn, Alert, StatusBadge } from '@/components/shared';

export default function SafetyPage() {
  const [tab, setTab] = useState('routes');

  const { data: safety, isLoading, error, refetch } = useQuery({
    queryKey: ['route-safety'],
    queryFn: () => safetyApi.checkAllRoutes(),
    refetchOnMount: true,
  });

  return (
    <div>
      <PageHeader title="路由安全" helpKey="safety" sub="安全检查与路径评估" actions={
        <Btn sm onClick={() => refetch()}>重新检查</Btn>
      } />

      {isLoading && <div className="text-center py-10 text-a-muted font-mono text-sm">加载中...</div>}
      {error && <Alert type="err">加载失败: {(error as any).message}</Alert>}

      <TabBar
        tabs={[
          { key: 'routes', label: 'Route Safety' },
          { key: 'summary', label: 'Summary' },
        ]}
        active={tab}
        onChange={setTab}
      />

      {tab === 'routes' && !isLoading && safety && (
        <Card>
          <table className="w-full text-sm border-collapse">
            <thead>
              <tr>
                {['域名', '目标', '网关链接', '风险', '建议'].map((h) => (
                  <th key={h} className="text-left px-3.5 py-2.5 text-[11px] font-semibold uppercase tracking-[0.06em] text-a-muted bg-a-bg border-b border-a-border whitespace-nowrap">{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {(safety || []).map((r: any, i: number) => (
                <tr key={r.route_id || i} className="hover:bg-white/[0.04] [&:last-child>td]:border-b-0">
                  <td className="px-3.5 py-2.5 border-b border-a-border-soft font-mono text-xs">{r.domain}</td>
                  <td className="px-3.5 py-2.5 border-b border-a-border-soft font-mono text-xs">{r.target_host}:{r.target_port}</td>
                  <td className="px-3.5 py-2.5 border-b border-a-border-soft">{r.has_gateway_link ? <span className="text-[#4cd964]">✓</span> : <span className="text-[#e8b830]">✗</span>}</td>
                  <td className="px-3.5 py-2.5 border-b border-a-border-soft">
                    <div className="flex flex-wrap gap-1">
                      {(r.risks || []).map((risk: any, j: number) => (
                        <span key={j} className={`inline-flex items-center font-mono text-[9px] px-1.5 py-0.5 rounded ${
                          risk.severity === 'error' ? 'bg-[#ff5c72]/20 text-[#ff5c72]'
                          : 'bg-[#e8b830]/20 text-[#e8b830]'
                        }`}>{risk.code}</span>
                      ))}
                    </div>
                  </td>
                  <td className="px-3.5 py-2.5 border-b border-a-border-soft text-xs text-a-muted max-w-[200px] truncate">{r.recommendation}</td>
                </tr>
              ))}
              {(!safety || safety.length === 0) && (
                <tr><td colSpan={5} className="text-center py-10 text-a-muted text-xs">无安全检查数据</td></tr>
              )}
            </tbody>
          </table>
        </Card>
      )}

      {tab === 'summary' && safety && (
        <div className="grid grid-cols-4 gap-3 mb-4">
          <StatCard label="总路由数" value={safety.length} accent />
          <StatCard label="安全" value={safety.filter((r: any) => !r.risks?.length).length} success />
          <StatCard label="有风险" value={safety.filter((r: any) => r.risks?.length).length} warn />
          <StatCard label="GWLink 绕过" value={safety.filter((r: any) => r.gateway_link_required && !r.has_gateway_link).length} danger />
        </div>
      )}
    </div>
  );
}

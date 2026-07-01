import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { adminApi, system, healthCheckApi, systemHealthApi } from '@/lib/api-bridge';
import { PageHeader, Card, StatCard, Btn, Alert, TabBar, MetaRow, StatusBadge } from '@/components/shared';
import { useToast } from '@/components/shared/Toast';
import { fmtDate } from '@/lib/utils';

export function ApplyConfigPage() {
  const toast = useToast();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const [tab, setTab] = useState('status');
  const [applying, setApplying] = useState(false);
  const [applyRes, setApplyRes] = useState<any>(null);
  const [applyErr, setApplyErr] = useState<string | null>(null);

  const { data: status } = useQuery({
    queryKey: ['system-status'],
    queryFn: () => system.status(),
  });

  const { data: history } = useQuery({
    queryKey: ['apply-history'],
    queryFn: () => adminApi.applyHistory(),
  });

  const { data: preview } = useQuery({
    queryKey: ['config-preview'],
    queryFn: () => adminApi.configPreview(),
  });

  const { data: sysHealth } = useQuery({
    queryKey: ['system-health'],
    queryFn: () => systemHealthApi.get().catch(() => null),
    refetchInterval: 60000,
  });

  const { data: healthData } = useQuery({
    queryKey: ['health-latest'],
    queryFn: () => healthCheckApi.getLatest().catch(() => null),
    refetchInterval: 30000,
  });

  const pending = status?.pending_apply?.pending;

  async function doApply() {
    if (!confirm('确定执行 Apply？')) return;
    setApplying(true);
    setApplyErr(null);
    setApplyRes(null);
    try {
      const res = await adminApi.applyConfig();
      setApplyRes(res);
      toast(res.message || 'Apply 完成');
      queryClient.invalidateQueries({ queryKey: ['config-preview'] });
      queryClient.invalidateQueries({ queryKey: ['system-status'] });
      queryClient.invalidateQueries({ queryKey: ['health-latest'] });
      queryClient.invalidateQueries({ queryKey: ['system-health'] });
    } catch (e: any) {
      setApplyErr(e.message);
      toast(e.message, 'error');
    }
    setApplying(false);
  }

  return (
    <div>
      <PageHeader title="推送配置" helpKey="apply" sub="配置部署与渲染" actions={
        <div className="flex gap-2">
          <Btn primary onClick={doApply} disabled={applying || !pending}>
            {applying ? '推送中…' : 'Apply'}
          </Btn>
          <Btn onClick={() => navigate('/health')}>健康检查 →</Btn>
        </div>
      } />

      {pending && <Alert type="warn">有待处理变更。执行 Apply 以应用。</Alert>}
      {!pending && !applyRes && (
        <Alert type="info">当前无待处理的变更，配置已是最新。</Alert>
      )}

      {/* Apply result feedback */}
      {applyRes && (
        <Alert type={applyRes.message?.includes('error') ? 'err' : 'success'} className="mb-4">
          <span className="font-medium mr-2">✓ 推送完成</span>
          <span className="font-mono text-xs">
            {applyRes.message || 'success'}
            {applyRes.routes != null && ` · ${applyRes.routes} routes`}
            {applyRes.warnings > 0 && ` · ${applyRes.warnings} warnings`}
          </span>
        </Alert>
      )}
      {applyErr && <Alert type="err" className="mb-4">{applyErr}</Alert>}

      {status && (
        <div className="grid grid-cols-4 gap-3 mb-4">
          <StatCard label="服务" value={status.counts.services} accent />
          <StatCard label="路由" value={status.counts.routes} success />
          <StatCard label="管理域名" value={status.counts.managed_domains} />
          <StatCard label="待处理" value={pending ? '是' : '否'} warn={!!pending} />
        </div>
      )}

      <TabBar
        tabs={[
          { key: 'status', label: '状态' },
          { key: 'preview', label: '预览' },
          { key: 'history', label: '历史' },
          { key: 'verify', label: '验证' },
        ]}
        active={tab}
        onChange={setTab}
      />

      {tab === 'status' && status && (
        <Card title="系统状态">
          <div className="p-[18px] grid grid-cols-2 gap-3">
            <MetaRow label="名称" value={status.name} mono />
            <MetaRow label="版本" value={status.version} mono />
            <MetaRow label="提供商" value={status.proxy?.provider || '—'} />
            <MetaRow label="配置路径" value={status.proxy?.config_path || '—'} mono />
            <MetaRow label="验证可用" value={status.proxy?.validate_available ? '✓' : '✗'} />
            <MetaRow label="重载命令" value={status.proxy?.reload_command_configured ? '✓' : '✗'} />
            {status.last_apply && (
              <MetaRow label="上次推送" value={`${status.last_apply.status} @ ${status.last_apply.version}`} />
            )}
          </div>
        </Card>
      )}

      {tab === 'preview' && preview && (
        <Card title="渲染配置预览">
          <div className="p-[18px]">
            <div className="text-xs text-a-muted mb-2">{preview.route_count} routes, {preview.managed_domain_count} managed domains</div>
            <pre className="bg-a-bg border border-a-border rounded-a-sm p-3 text-xs font-mono text-a-muted overflow-x-auto max-h-[500px] whitespace-pre-wrap">
              {preview.rendered_config || 'No config rendered'}
            </pre>
          </div>
        </Card>
      )}

      {tab === 'history' && (
        <Card title="推送历史">
          <table className="w-full text-sm border-collapse">
            <thead>
              <tr>
                {['版本', '状态', '创建时间'].map((h) => (
                  <th key={h} className="text-left px-3.5 py-2.5 text-[11px] font-semibold uppercase tracking-[0.06em] text-a-muted bg-a-bg border-b border-a-border whitespace-nowrap">{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {(Array.isArray(history) ? history : []).map((h: any) => (
                <tr key={h.id || h.version} className="hover:bg-white/[0.04]">
                  <td className="px-3.5 py-2.5 border-b border-a-border-soft font-mono text-xs">{h.version}</td>
                  <td className="px-3.5 py-2.5 border-b border-a-border-soft"><StatusBadge status={h.status} /></td>
                  <td className="px-3.5 py-2.5 border-b border-a-border-soft font-mono text-xs">{fmtDate(h.created_at)}</td>
                </tr>
              ))}
              {(!history || (Array.isArray(history) && history.length === 0)) && (
                <tr><td colSpan={3} className="text-center py-10 text-a-muted text-xs">暂无 Apply 历史</td></tr>
              )}
            </tbody>
          </table>
        </Card>
      )}

      {/* ─── Verify Tab ─── */}
      {tab === 'verify' && (
        <div className="space-y-4">
          {/* System health summary */}
          {sysHealth && (
            <Card title="系统资源">
              <div className="grid grid-cols-2 md:grid-cols-4 gap-3 p-3">
                <div className="text-xs">
                  <div className="text-a-muted mb-0.5">SQLite</div>
                  <div className="flex items-center gap-1.5">
                    <span className={`w-1.5 h-1.5 rounded-full ${sysHealth.sqlite_ok ? 'bg-[#4cd964]' : 'bg-[#ff5c72]'}`} />
                    <span className="font-medium">{sysHealth.sqlite_ok ? '正常' : '异常'}</span>
                  </div>
                </div>
                <div className="text-xs">
                  <div className="text-a-muted mb-0.5">磁盘可用</div>
                  <div className="font-medium">{(sysHealth.disk_free_bytes / (1024*1024*1024)).toFixed(1)} GB</div>
                  <div className="text-[10px] text-a-muted">/ {(sysHealth.disk_total_bytes / (1024*1024*1024)).toFixed(1)} GB</div>
                </div>
                <div className="text-xs">
                  <div className="text-a-muted mb-0.5">内存</div>
                  <div className="font-medium">{sysHealth.memory_used_mb} MB</div>
                  <div className="text-[10px] text-a-muted">/ {sysHealth.memory_total_mb} MB</div>
                </div>
                <div className="text-xs">
                  <div className="text-a-muted mb-0.5">运行时间</div>
                  <div className="font-medium">
                    {sysHealth.uptime_seconds > 3600
                      ? `${Math.floor(sysHealth.uptime_seconds / 3600)}h`
                      : `${Math.floor(sysHealth.uptime_seconds / 60)}m`}
                  </div>
                </div>
              </div>
            </Card>
          )}

          {/* Endpoint health */}
          <Card title="端点健康状态">
            {status ? (
              <div className="p-3">
                <div className="grid grid-cols-3 gap-3 mb-3">
                  <div className="text-center p-3 rounded-a-sm bg-[#4cd964]/5">
                    <div className="text-lg font-bold text-[#4cd964]">{status.health?.healthy_endpoints ?? '—'}</div>
                    <div className="text-[10px] text-a-muted">健康</div>
                  </div>
                  <div className="text-center p-3 rounded-a-sm bg-[#ff5c72]/5">
                    <div className="text-lg font-bold text-[#ff5c72]">{status.health?.unhealthy_endpoints ?? '—'}</div>
                    <div className="text-[10px] text-a-muted">异常</div>
                  </div>
                  <div className="text-center p-3 rounded-a-sm bg-a-border/10">
                    <div className="text-lg font-bold text-a-muted">{status.health?.unknown_endpoints ?? '—'}</div>
                    <div className="text-[10px] text-a-muted">未知</div>
                  </div>
                </div>
                {status.health?.unhealthy_endpoints > 0 && (
                  <Alert type="warn">
                    检测到 {status.health.unhealthy_endpoints} 个异常端点。
                    <button onClick={() => navigate('/health')} className="ml-2 text-a-accent hover:underline bg-transparent border-none cursor-pointer text-xs">
                      查看详情 →
                    </button>
                  </Alert>
                )}
              </div>
            ) : (
              <div className="p-3 text-xs text-a-muted">加载中…</div>
            )}
          </Card>

          {/* Last apply summary */}
          {status?.last_apply && (
            <Card title="最近推送">
              <div className="p-3 grid grid-cols-2 gap-2 text-xs">
                <MetaRow label="版本" value={status.last_apply.version} mono />
                <MetaRow label="状态" value={status.last_apply.status} mono />
                <MetaRow label="时间" value={fmtDate(status.last_apply.created_at)} mono />
              </div>
            </Card>
          )}

          {/* Action buttons */}
          <div className="flex gap-3">
            <Btn onClick={() => {
              queryClient.invalidateQueries({ queryKey: ['health-latest'] });
              queryClient.invalidateQueries({ queryKey: ['system-health'] });
              queryClient.invalidateQueries({ queryKey: ['system-status'] });
              toast('已刷新验证数据');
            }}>
              刷新验证数据
            </Btn>
            <Btn onClick={() => navigate('/health')}>完整健康检查 →</Btn>
          </div>
        </div>
      )}
    </div>
  );
}

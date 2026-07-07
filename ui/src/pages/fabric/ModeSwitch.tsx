// ─── Runtime Mode Management (v1.9E) ───
// Three-section layout: capability usage → provider status → switch action
// Preview → Confirm → Execute → Result

import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { adminApi, providerApi, runtimeModeApi } from '@/lib/api-bridge';
import { PageHeader, Card, Btn, HealthDot, StatusBadge, useToast, LoadingState, ErrorBanner, EmptyState, Modal } from '@/components/shared';
import { cn } from '@/lib/utils';

// ─── Types ───

interface CompSummary {
  key: string; name: string; route_count: number;
  current_mode_ok: boolean; target_mode_ok: boolean; reason?: string;
}

interface ModeSwitchPreview {
  current_mode: string; target_mode: string; total_routes: number;
  route_breakdown: CompSummary[];
  affected_routes: { kept: number; unsupported: number };
  provider_changes: { provider_id: string; action: string; detail: string }[];
  risks: string[];
}

interface ProviderState {
  id: string; name: string; status: string;
  installed: boolean; running: boolean; version: string;
  config_path: string; ports: { port: number; purpose: string }[];
}

interface CompStatus {
  name: string; status: string;
}

// ─── Main Component ───

export default function ModeSwitch() {
  const toast = useToast();
  const [showPreview, setShowPreview] = useState(false);
  const [previewData, setPreviewData] = useState<ModeSwitchPreview | null>(null);
  const [previewLoading, setPreviewLoading] = useState(false);
  const [confirmChecks, setConfirmChecks] = useState<Record<string, boolean>>({});
  const [executing, setExecuting] = useState(false);
  const [execResult, setExecResult] = useState<any>(null);
  const [execError, setExecError] = useState<string | null>(null);
  const [execStep, setExecStep] = useState('');

  // ── Data ──
  const { data: modeData, isLoading: modeLoading } = useQuery({
    queryKey: ['runtime-mode'],
    queryFn: () => runtimeModeApi.get(),
    refetchInterval: 30_000,
  });

  const { data: provData } = useQuery({
    queryKey: ['providers'],
    queryFn: () => providerApi.list() as Promise<{ providers: ProviderState[] }>,
    refetchInterval: 30_000,
  });

  const currentMode = modeData?.current;
  const availableModes = modeData?.available_modes || [];
  const providers = provData?.providers || [];
  const currentCompositions: CompStatus[] = currentMode?.compositions || [];

  // Find the mode to switch TO (the one that isn't current)
  const otherModes = availableModes.filter((m: any) => m.id !== currentMode?.id && m.implemented);

  // ── Preview ──
  const openPreview = async (targetModeId: string) => {
    setPreviewLoading(true);
    setShowPreview(true);
    setExecResult(null);
    setExecError(null);
    try {
      const data = await adminApi.modePreview(targetModeId);
      setPreviewData(data);
      // Reset confirm checks for each risk
      const checks: Record<string, boolean> = {};
      data.risks?.forEach((_: string, i: number) => { checks[`risk_${i}`] = false; });
      setConfirmChecks(checks);
    } catch (err: any) {
      toast(err.message || '预览加载失败', 'error');
      setShowPreview(false);
    } finally {
      setPreviewLoading(false);
    }
  };

  // ── Execute ──
  const executeSwitch = async () => {
    if (!previewData) return;
    setExecuting(true);
    setExecStep('正在备份当前配置...');
    try {
      const result = await adminApi.modeSwitch(previewData.target_mode, true);
      setExecResult(result);
      setExecStep('');
    } catch (err: any) {
      setExecError(err.message || '切换失败');
      setExecStep('');
    } finally {
      setExecuting(false);
    }
  };

  const allConfirmed = previewData?.risks?.every((_, i) => confirmChecks[`risk_${i}`]) ?? false;

  // Available actions based on mode status
  const switchActions = otherModes.map((m: any) => ({
    id: m.id,
    label: m.label,
    description: m.description,
    onClick: () => openPreview(m.id),
  }));

  // ── Render ──

  if (modeLoading) {
    return <div className="p-6"><LoadingState /></div>;
  }
  if (!currentMode) {
    return <div className="p-6"><EmptyState title="无法检测运行时模式" description="运行 aegis doctor 诊断" /></div>;
  }

  return (
    <div className="p-6 space-y-5">
      <PageHeader
        title="运行时模式"
        subtitle={`当前: ${currentMode.label} · ${currentCompositions.filter((c: CompStatus) => c.status === 'available').length} 个组合能力可用`}
      />

      {/* Section 1: Capability Usage */}
      <Card title="能力使用概览" subtitle="每个组合能力的路由数及当前状态">
        <div className="space-y-2">
          {currentCompositions.length === 0 ? (
            <div className="text-xs text-a-muted py-2">暂无组合能力数据</div>
          ) : (
            currentCompositions.map((comp: CompStatus, i: number) => (
              <div key={i} className="flex items-center gap-3 px-3 py-2 rounded-a-sm bg-a-bg border border-a-border/20 text-xs">
                <HealthDot status={comp.status === 'available' ? 'healthy' : comp.status === 'missing_provider' ? 'degraded' : 'failed'} />
                <span className="font-medium text-a-fg w-36">{comp.name}</span>
                <span className={cn('text-[11px] font-mono',
                  comp.status === 'available' ? 'text-[#4cd964]' : 'text-a-muted'
                )}>{comp.status === 'available' ? '✅ 可用' : comp.status === 'missing_provider' ? '⚠️ 缺少 Provider' : '⛔ 不支持'}</span>
              </div>
            ))
          )}
          <div className="text-[10px] text-a-muted/60 pt-1">数据来自运行模式实时检测</div>
        </div>
      </Card>

      {/* Section 2: Provider Status */}
      <Card title="Provider 状态" subtitle="当前模式下各中间件的运行情况">
        {providers.length === 0 ? (
          <div className="text-xs text-a-muted py-2">暂无 Provider</div>
        ) : (
          <div className="space-y-2">
            {providers.map((p: ProviderState) => {
              const isHealthy = p.installed && p.running;
              const portStr = (p.ports || []).map((b: any) => `${b.port}/${b.protocol || 'tcp'}(${b.purpose})`).join(', ');
              return (
                <div key={p.id} className="flex items-center gap-3 px-3 py-2 rounded-a-sm bg-a-bg border border-a-border/20 text-xs">
                  <HealthDot status={isHealthy ? 'healthy' : p.installed ? 'degraded' : 'failed'} />
                  <span className="font-semibold text-a-fg w-24">{p.name}</span>
                  <StatusBadge status={p.status} />
                  <span className="text-a-muted font-mono text-[10px] ml-2 truncate flex-1">{portStr || '—'}</span>
                  {p.version && <span className="text-a-muted text-[10px]">{p.version}</span>}
                </div>
              );
            })}
          </div>
        )}
      </Card>

      {/* Section 3: Switch Action */}
      <Card title="模式切换" subtitle="切换到其他运行模式（需要确认风险）">
        {switchActions.length === 0 ? (
          <div className="text-xs text-a-muted py-2">没有可切换的模式</div>
        ) : (
          <div className="space-y-2">
            {switchActions.map((action: any) => (
              <div key={action.id} className="flex items-center justify-between px-3 py-2.5 rounded-a-sm bg-a-bg border border-a-border/20">
                <div>
                  <div className="text-sm font-semibold text-a-fg">{action.label}</div>
                  <div className="text-[10px] text-a-muted mt-0.5">{action.description}</div>
                </div>
                <Btn primary onClick={action.onClick} className="text-xs whitespace-nowrap">
                  切换到 {action.label}
                </Btn>
              </div>
            ))}
          </div>
        )}
      </Card>

      {/* ═══════ Preview Modal ═══════ */}
      {showPreview && (
        <Modal onClose={() => { if (!executing) setShowPreview(false); }}
          title="模式切换预览"
          wide
          footer={
            executing ? (
              <div className="flex items-center gap-2 text-xs text-a-muted">
                <span className="w-3.5 h-3.5 border-2 border-a-accent/30 border-t-a-accent rounded-full animate-spin" />
                {execStep || '执行中...'}
              </div>
            ) : execResult ? (
              <div className="flex items-center gap-2 w-full justify-between">
                <span className="text-xs text-[#4cd964]">✅ 切换完成</span>
                <Btn onClick={() => setShowPreview(false)} className="text-xs">关闭</Btn>
              </div>
            ) : execError ? (
              <div className="flex items-center gap-2 w-full justify-between">
                <span className="text-xs text-[#ff5c72]">❌ {execError}</span>
                <div className="flex gap-2">
                  <Btn onClick={() => setShowPreview(false)} className="text-xs">关闭</Btn>
                  <Btn onClick={() => { setExecResult(null); setExecError(null); }} className="text-xs">重试</Btn>
                </div>
              </div>
            ) : (
              <div className="flex items-center gap-2 w-full justify-between">
                <Btn onClick={() => setShowPreview(false)} className="text-xs">取消</Btn>
                <Btn primary onClick={executeSwitch} disabled={!allConfirmed} className="text-xs">
                  确认切换
                </Btn>
              </div>
            )
          }>
          {previewLoading ? (
            <div className="py-8 text-center text-sm text-a-muted">加载预览数据...</div>
          ) : previewData ? (
            <div className="space-y-4 text-sm">
              {/* Summary */}
              <div className="flex items-center gap-3">
                <span className="font-semibold text-a-fg">{previewData.current_mode}</span>
                <span className="text-a-muted">→</span>
                <span className="font-semibold text-a-accent">{previewData.target_mode}</span>
              </div>

              {previewData.affected_routes.unsupported > 0 && (
                <div className="px-3 py-2 rounded-a-sm bg-[#e8b830]/5 border border-[#e8b830]/20">
                  <div className="text-[11px] font-medium text-[#e8b830] mb-1">
                    ⚠️ {previewData.affected_routes.unsupported} 条路由将不可用
                  </div>
                  <div className="text-[10px] text-a-muted">
                    {previewData.affected_routes.kept} 条保持可用
                  </div>
                </div>
              )}

              {/* Route breakdown */}
              <div>
                <div className="text-[11px] font-medium text-a-fg mb-1.5">各组合能力影响</div>
                <div className="space-y-1">
                  {previewData.route_breakdown.map((b, i) => (
                    <div key={i} className="flex items-center gap-2 px-2 py-1 rounded-a-sm bg-a-bg border border-a-border/20 text-[11px]">
                      <span className={b.target_mode_ok ? 'text-[#4cd964]' : 'text-[#e8b830]'}>
                        {b.target_mode_ok ? '✅' : '⛔'}
                      </span>
                      <span className="font-medium text-a-fg w-32">{b.name || b.key}</span>
                      <span className="text-a-muted font-mono">{b.route_count} 条</span>
                      {b.reason && <span className="text-a-muted text-[10px] ml-auto">{b.reason}</span>}
                    </div>
                  ))}
                </div>
              </div>

              {/* Provider changes */}
              <div>
                <div className="text-[11px] font-medium text-a-fg mb-1.5">Provider 变更</div>
                <div className="space-y-1">
                  {previewData.provider_changes.map((pc, i) => (
                    <div key={i} className="flex items-center gap-2 px-2 py-1 rounded-a-sm bg-a-bg border border-a-border/20 text-[11px]">
                      <span className={cn(
                        pc.action === 'stop' ? 'text-[#ff5c72]' : pc.action === 'start' ? 'text-[#4cd964]' : 'text-[#e8b830]'
                      )}>
                        {pc.action === 'stop' ? '⏹' : pc.action === 'start' ? '▶' : '🔄'}
                      </span>
                      <span className="font-medium text-a-fg w-20">{pc.provider_id}</span>
                      <span className="text-a-muted">{pc.detail}</span>
                    </div>
                  ))}
                </div>
              </div>

              {/* Risk confirmation */}
              {previewData.risks?.length > 0 && (
                <div className="px-3 py-2 rounded-a-sm bg-[#ff5c72]/5 border border-[#ff5c72]/20">
                  <div className="text-[11px] font-medium text-[#ff5c72] mb-2">确认风险</div>
                  <div className="space-y-1.5">
                    {previewData.risks.map((risk, i) => (
                      <label key={i} className="flex items-start gap-2 cursor-pointer">
                        <input type="checkbox" checked={confirmChecks[`risk_${i}`] || false}
                          onChange={(e) => setConfirmChecks({ ...confirmChecks, [`risk_${i}`]: e.target.checked })}
                          className="mt-0.5 cursor-pointer accent-a-accent" />
                        <span className="text-[11px] text-a-fg">{risk}</span>
                      </label>
                    ))}
                  </div>
                </div>
              )}

              {/* Result */}
              {execResult && (
                <div className="px-3 py-2 rounded-a-sm bg-[#4cd964]/5 border border-[#4cd964]/20">
                  <div className="text-[11px] text-[#4cd964] font-medium">✅ {execResult.message}</div>
                  {execResult.warnings?.length > 0 && (
                    <div className="mt-1 text-[10px] text-a-muted">
                      {execResult.warnings.map((w: any, i: number) => (
                        <div key={i}>{w.message || w}</div>
                      ))}
                    </div>
                  )}
                </div>
              )}
              {execError && (
                <div className="px-3 py-2 rounded-a-sm bg-[#ff5c72]/5 border border-[#ff5c72]/20">
                  <div className="text-[11px] text-[#ff5c72]">❌ {execError}</div>
                  <div className="mt-1 text-[10px] text-a-muted">可使用 POST /api/rollback 回滚</div>
                </div>
              )}
            </div>
          ) : (
            <div className="py-8 text-center text-sm text-a-muted">预览数据为空</div>
          )}
        </Modal>
      )}
    </div>
  );
}

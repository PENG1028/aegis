/**
 * Middleware Management — 中间件安装/检测/配置/状态 一体化面板 (v1.8H)
 */

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { providerApi, system } from '@/lib/api-bridge';
import { PageHeader, Card, StatusBadge, Btn } from '@/components/shared';
import { useState } from 'react';

// ─── helpers ───

const DIAG_CODES: Record<string, { label: string; color: string; desc: string }> = {
  PROVIDER_MISSING:        { label: '未安装', color: '#ff5c72', desc: '二进制不在系统 PATH 中' },
  PROVIDER_VERSION_UNSUPPORTED: { label: '版本过低', color: '#e8b830', desc: '当前版本不受支持' },
  CONFIG_FILE_MISSING:     { label: '配置缺失', color: '#e8b830', desc: '配置文件不存在' },
  CONFIG_VALIDATE_FAILED:  { label: '配置错误', color: '#ff5c72', desc: '配置文件语法验证失败' },
  SERVICE_NOT_RUNNING:     { label: '服务未运行', color: '#e8b830', desc: 'systemd 服务未启动' },
  LISTENER_CONFLICT:       { label: '端口冲突', color: '#ff5c72', desc: '绑定端口已被占用' },
  RUNTIME_VERIFY_FAILED:   { label: '运行时异常', color: '#ff5c72', desc: '端口监听但请求无响应' },
};

function DiagLevelBadge({ code }: { code?: string }) {
  if (!code) return <span className="text-[#4cd964] text-xs">✓ 正常</span>;
  const info = DIAG_CODES[code] || { label: code, color: '#888', desc: '' };
  return (
    <span className="text-xs" style={{ color: info.color }} title={info.desc}>
      ✗ {info.label}
    </span>
  );
}

function fmtBool(b?: boolean | null) {
  if (b === true) return <StatusBadge status="active" />;
  if (b === false) return <StatusBadge status="inactive" />;
  return <span className="text-a-muted">—</span>;
}

// ─── main page ───

export default function MiddlewarePage() {
  const queryClient = useQueryClient();
  const [activeTab, setActiveTab] = useState<'caddy' | 'haproxy'>('caddy');
  const [installing, setInstalling] = useState<string | null>(null);
  const [configPreview, setConfigPreview] = useState<Record<string, any>>({});

  // Fetch diagnostics
  const { data: diag, isLoading } = useQuery({
    queryKey: ['provider-diagnostics'],
    queryFn: async () => {
      const res = await providerApi.diagnoseAll();
      return res;
    },
    refetchInterval: 30000,
  });

  // Fetch provider list
  const { data: providers } = useQuery({
    queryKey: ['providers'],
    queryFn: () => providerApi.list(),
  });

  // Install mutation
  const installMutation = useMutation({
    mutationFn: async (p: string) => {
      setInstalling(p);
      return providerApi.install(p);
    },
    onSettled: () => {
      setInstalling(null);
      queryClient.invalidateQueries({ queryKey: ['provider-diagnostics'] });
      queryClient.invalidateQueries({ queryKey: ['providers'] });
    },
  });

  // Config preview
  async function loadConfig(p: string) {
    try {
      const data = await providerApi.getConfig(p);
      setConfigPreview((prev) => ({ ...prev, [p]: data }));
    } catch { /* ignore */ }
  }

  if (isLoading) return <div className="text-center py-10 text-a-muted font-mono text-sm">加载中...</div>;

  const caddyDiag = diag?.diagnostics?.find((d: any) =>
    d.provider?.toLowerCase().includes('caddy'));
  const haproxyDiag = diag?.diagnostics?.find((d: any) =>
    d.provider?.toLowerCase().includes('haproxy'));
  const activeDiag = activeTab === 'caddy' ? caddyDiag : haproxyDiag;

  // Auto-load config on tab switch if not already loaded
  const switchTab = (tab: 'caddy' | 'haproxy') => {
    setActiveTab(tab);
    if (!configPreview[tab]) loadConfig(tab);
  };

  return (
    <div>
      <PageHeader
        title="中间件管理"
        helpKey="middleware"
        sub="Caddy / HAProxy 安装、检测、配置预览与诊断"
        actions={
          <div className="flex gap-2">
            <button
              className="px-3 py-1.5 text-xs rounded-a-md border border-a-border text-a-muted hover:text-a-fg hover:border-a-accent transition-colors bg-transparent cursor-pointer"
              onClick={() => {
                queryClient.invalidateQueries({ queryKey: ['provider-diagnostics'] });
                providerApi.diagnoseAll();
              }}
            >
              重新诊断
            </button>
          </div>
        }
      />

      {/* Tab switch */}
      <div className="flex gap-0 mb-4 border-b border-a-border">
        {(['caddy', 'haproxy'] as const).map((t) => (
          <button
            key={t}
            onClick={() => switchTab(t)}
            className={`px-5 py-2.5 text-xs font-medium border-b-2 transition-colors bg-transparent cursor-pointer ${
              activeTab === t
                ? 'border-a-accent text-a-accent'
                : 'border-transparent text-a-muted hover:text-a-fg'
            }`}
          >
            {t === 'caddy' ? 'Caddy (HTTP/HTTPS)' : 'HAProxy (TLS/EdgeMux)'}
          </button>
        ))}
      </div>

      {/* Provider status card */}
      {activeDiag && (
        <div className="space-y-4">
          {/* Status overview */}
          <Card title={`${activeTab === 'caddy' ? 'Caddy' : 'HAProxy'} 状态`}>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-3 p-3">
              <div className="text-xs">
                <div className="text-a-muted mb-1">已安装</div>
                {fmtBool(activeDiag.installed)}
              </div>
              <div className="text-xs">
                <div className="text-a-muted mb-1">版本</div>
                <div className="font-mono">{activeDiag.version || '—'}</div>
              </div>
              <div className="text-xs">
                <div className="text-a-muted mb-1">服务运行</div>
                {fmtBool(activeDiag.service_running)}
              </div>
              <div className="text-xs">
                <div className="text-a-muted mb-1">配置有效</div>
                {fmtBool(activeDiag.config_valid)}
              </div>
              <div className="text-xs">
                <div className="text-a-muted mb-1">版本兼容</div>
                {fmtBool(activeDiag.version_supported)}
              </div>
              <div className="text-xs">
                <div className="text-a-muted mb-1">监听正常</div>
                {fmtBool(activeDiag.listener_ok)}
              </div>
              <div className="text-xs">
                <div className="text-a-muted mb-1">运行时</div>
                {fmtBool(activeDiag.runtime_verify_ok)}
              </div>
              <div className="text-xs">
                <div className="text-a-muted mb-1">诊断</div>
                <DiagLevelBadge code={activeDiag.last_error_code} />
              </div>
            </div>

            {/* Error detail */}
            {activeDiag.last_error_message && (
              <div className="mt-2 mx-3 p-3 rounded-a-sm bg-[#ff5c72]/5 border border-[#ff5c72]/15 text-xs">
                <span className="text-[#ff5c72] font-medium mr-2">错误:</span>
                <span className="text-a-muted font-mono">{activeDiag.last_error_message}</span>
              </div>
            )}

            {/* Action buttons */}
            <div className="flex gap-2 m-3 pt-3 border-t border-a-border">
              {!activeDiag.installed ? (
                <Btn
                  primary
                  onClick={() => installMutation.mutate(activeTab)}
                  disabled={installing === activeTab}
                >
                  {installing === activeTab ? '安装中…' : `安装 ${activeTab === 'caddy' ? 'Caddy' : 'HAProxy'}`}
                </Btn>
              ) : (
                <span className="text-xs text-a-muted flex items-center">
                  ✓ 已安装 · 如需重装请手动运行 apt-get install --reinstall
                </span>
              )}
            </div>

            {/* Install result */}
            {installMutation.data && (
              <div className={`mx-3 mb-3 p-3 rounded-a-sm text-xs border ${
                installMutation.data.status === 'installed' || installMutation.data.status === 'already_installed'
                  ? 'bg-[#4cd964]/5 border-[#4cd964]/15'
                  : 'bg-[#ff5c72]/5 border-[#ff5c72]/15'
              }`}>
                <div className="font-medium mb-1">{installMutation.data.message}</div>
                {installMutation.data.apt_install_out && (
                  <pre className="text-[10px] text-a-muted mt-1 max-h-32 overflow-auto">{installMutation.data.apt_install_out}</pre>
                )}
              </div>
            )}
          </Card>

          {/* 7-step diagnostic pipeline */}
          <Card title="诊断流水线 (7 级)">
            <div className="space-y-0">
              {[
                { code: 'PROVIDER_MISSING',        check: activeDiag.installed,            ok: true },
                { code: 'PROVIDER_VERSION_UNSUPPORTED', check: activeDiag.version_supported, ok: true },
                { code: 'CONFIG_FILE_MISSING',     check: activeDiag.config_exists !== false, ok: activeDiag.config_exists },
                { code: 'CONFIG_VALIDATE_FAILED',  check: activeDiag.config_valid,          ok: true },
                { code: 'SERVICE_NOT_RUNNING',     check: activeDiag.service_running,       ok: true },
                { code: 'LISTENER_CONFLICT',       check: activeDiag.listener_ok,           ok: true },
                { code: 'RUNTIME_VERIFY_FAILED',   check: activeTab === 'caddy' ? activeDiag.runtime_verify_ok : undefined, ok: activeTab === 'caddy' },
              ].map((step) => {
                const info = DIAG_CODES[step.code];
                const passed = step.check === true;
                const skipped = step.check === undefined || step.check === null;
                return (
                  <div key={step.code} className="flex items-center gap-3 py-2.5 px-3 border-b border-a-border/30 text-xs">
                    <span className={`w-5 h-5 rounded-full flex items-center justify-center shrink-0 text-[10px] ${
                      skipped ? 'bg-a-border/30 text-a-muted' :
                      passed ? 'bg-[#4cd964]/15 text-[#4cd964]' : 'bg-[#ff5c72]/15 text-[#ff5c72]'
                    }`}>
                      {skipped ? '·' : passed ? '✓' : '✗'}
                    </span>
                    <span className={`w-24 shrink-0 font-medium ${skipped ? 'text-a-muted' : passed ? 'text-a-fg' : 'text-[#ff5c72]'}`}>
                      {skipped ? '跳过' : info.label}
                    </span>
                    <span className="text-a-muted text-[11px] hidden md:inline">{info.desc}</span>
                    {!passed && !skipped && activeDiag.last_error_message && (
                      <span className="ml-auto text-[10px] text-a-muted truncate max-w-[200px]">{activeDiag.last_error_message}</span>
                    )}
                  </div>
                );
              })}
            </div>
          </Card>

          {/* Config preview */}
          <Card
            title={`配置文件 · ${activeDiag.config_path || '未配置'}`}
            actions={
              <button
                className="text-[11px] px-2 py-1 rounded border border-a-border text-a-muted hover:text-a-fg transition-colors bg-transparent cursor-pointer"
                onClick={() => loadConfig(activeTab)}
              >
                加载配置
              </button>
            }
          >
            {configPreview[activeTab]?.exists ? (
              <pre className="text-[11px] font-mono text-a-muted p-3 max-h-80 overflow-auto bg-a-surface/50 rounded">
                {configPreview[activeTab].content}
              </pre>
            ) : configPreview[activeTab]?.error ? (
              <div className="p-3 text-xs text-a-muted">
                无法读取配置文件: {configPreview[activeTab].error}
              </div>
            ) : (
              <div className="p-3 text-xs text-a-muted">
                点击「加载配置」查看当前 {activeTab === 'caddy' ? 'Caddyfile' : 'haproxy.cfg'} 内容
              </div>
            )}
          </Card>
        </div>
      )}
    </div>
  );
}

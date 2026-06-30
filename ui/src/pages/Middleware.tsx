/**
 * Middleware — 中间件管理页面 (v1.8K)
 *
 * 统一管理 Caddy 和 HAProxy 的全生命周期:
 *   检查 → 安装 → 配置编辑 → 重载 → 启停 → 卸载
 *
 * 合并了原 ProvidersPage 和 MiddlewarePage 的功能。
 */

import { useState, useEffect } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { providerApi } from '@/lib/api-bridge';
import { PageHeader, Card, Btn } from '@/components/shared';
import { useToast } from '@/components/shared/Toast';

// ─── Provider definitions ───

interface ProviderTab {
  key: string;
  label: string;
  desc: string;
  protocol: string;
  port: string;
  configPath: string;
}

const TABS: ProviderTab[] = [
  {
    key: 'caddy',
    label: 'Caddy',
    desc: 'HTTP/HTTPS 反代 + Let\'s Encrypt 自动证书',
    protocol: 'http',
    port: ':80',
    configPath: '/etc/caddy/Caddyfile',
  },
  {
    key: 'haproxy',
    label: 'HAProxy',
    desc: 'TLS SNI 透传 + TCP 端口转发',
    protocol: 'tls_mux',
    port: ':443',
    configPath: '/etc/haproxy/haproxy.cfg',
  },
];

// ─── Helpers ───

function statusBadge(ok: boolean | undefined | null) {
  if (ok === true) return <span className="text-[#4cd964] text-[11px] font-medium">✓</span>;
  if (ok === false) return <span className="text-[#ff5c72] text-[11px] font-medium">✗</span>;
  return <span className="text-a-muted text-[11px]">—</span>;
}

const diagSteps = [
  { code: 'PROVIDER_MISSING', label: '二进制安装', desc: (p: string) => `${p} 在 PATH 中` },
  { code: 'PROVIDER_VERSION_UNSUPPORTED', label: '版本兼容', desc: () => '版本 >= 最低要求' },
  { code: 'CONFIG_FILE_MISSING', label: '配置文件', desc: () => '配置文件存在' },
  { code: 'CONFIG_VALIDATE_FAILED', label: '配置语法', desc: () => '配置语法有效' },
  { code: 'SERVICE_NOT_RUNNING', label: '服务运行', desc: () => 'systemd 服务 active' },
  { code: 'LISTENER_CONFLICT', label: '端口监听', desc: () => '端口正常绑定' },
  { code: 'RUNTIME_VERIFY_FAILED', label: '运行时验证', desc: () => '请求-响应正常' },
];

function diagPassMap(d: any): boolean[] {
  return [
    d.installed === true,
    d.version_supported !== false,
    d.config_exists === true,
    d.config_valid === true || d.config_valid === undefined,
    d.service_running === true,
    d.listener_ok === true,
    d.runtime_verify_ok === true || d.runtime_verify_ok === undefined,
  ];
}

export default function MiddlewarePage() {
  const toast = useToast();
  const queryClient = useQueryClient();
  const [activeTab, setActiveTab] = useState('caddy');
  const [configContent, setConfigContent] = useState('');
  const [configLoaded, setConfigLoaded] = useState(false);
  const [editingConfig, setEditingConfig] = useState(false);

  // ─── Diagnostics (auto-refresh every 60s) ───
  const diagQuery = useQuery({
    queryKey: ['provider-diagnostics'],
    queryFn: () => providerApi.diagnoseAll(),
    refetchInterval: 60_000,
  });

  const activeDiag = (diagQuery.data as any)?.diagnostics?.find((d: any) =>
    d.provider?.toLowerCase().includes(activeTab)
  );

  // ─── Mutations ───
  const installMu = useMutation({
    mutationFn: (p: string) => providerApi.install(p),
    onSuccess: (data: any) => {
      toast(data.message || `${activeTab} 安装完成`);
      queryClient.invalidateQueries({ queryKey: ['provider-diagnostics'] });
    },
    onError: (e: any) => toast(e.message || '安装失败', 'error'),
  });

  const reloadMu = useMutation({
    mutationFn: (p: string) => providerApi.reload(p),
    onSuccess: () => toast(`${activeTab} 已重载`),
    onError: (e: any) => toast(e.message || '重载失败', 'error'),
  });

  const svcMu = useMutation({
    mutationFn: ({ provider, action }: { provider: string; action: string }) =>
      providerApi.serviceControl(provider, action as any),
    onSuccess: (data: any) => {
      toast(`${activeTab} ${data.action === 'stop' ? '已停止' : data.action === 'start' ? '已启动' : '已重启'}`);
      queryClient.invalidateQueries({ queryKey: ['provider-diagnostics'] });
    },
    onError: (e: any) => toast(e.message || '操作失败', 'error'),
  });

  const saveConfigMu = useMutation({
    mutationFn: ({ provider, content }: { provider: string; content: string }) =>
      providerApi.saveConfig(provider, content),
    onSuccess: (data: any) => {
      toast(data.validation_warning ? `已保存 (⚠ ${data.validation_warning})` : '配置已保存');
      setEditingConfig(false);
      queryClient.invalidateQueries({ queryKey: ['provider-diagnostics'] });
    },
    onError: (e: any) => toast(e.message || '保存失败', 'error'),
  });

  const uninstallMu = useMutation({
    mutationFn: (p: string) => providerApi.uninstall(p),
    onSuccess: (data: any) => {
      toast(data.message || `${activeTab} 已卸载`);
      queryClient.invalidateQueries({ queryKey: ['provider-diagnostics'] });
    },
    onError: (e: any) => toast(e.message || '卸载失败', 'error'),
  });

  // ─── Load config on tab switch ───
  useEffect(() => {
    setConfigLoaded(false);
    setConfigContent('');
    setEditingConfig(false);
    providerApi.getConfig(activeTab).then((data: any) => {
      setConfigContent(data.content || '');
      setConfigLoaded(true);
    }).catch(() => setConfigLoaded(true));
  }, [activeTab]);

  const tab = TABS.find(t => t.key === activeTab)!;

  return (
    <div>
      <PageHeader
        title="中间件"
        helpKey="middleware"
        sub="Caddy (HTTP :80) + HAProxy (TLS SNI :443) 管理"
        actions={
          <Btn
            onClick={() => {
              providerApi.diagnoseAll().then(() =>
                queryClient.invalidateQueries({ queryKey: ['provider-diagnostics'] })
              );
            }}
          >
            重新诊断
          </Btn>
        }
      />

      {/* ─── Tab bar ─── */}
      <div className="flex gap-0.5 mb-4 p-0.5 rounded-a-sm bg-a-border/10 w-fit">
        {TABS.map(t => (
          <button
            key={t.key}
            onClick={() => setActiveTab(t.key)}
            className={`px-4 py-1.5 text-xs font-medium rounded-a-sm transition-colors ${
              activeTab === t.key
                ? 'bg-a-bg text-a-fg shadow-sm'
                : 'text-a-muted hover:text-a-fg'
            }`}
          >
            {t.label}
          </button>
        ))}
      </div>

      <div className="text-xs text-a-muted mb-4">{tab.desc} · 端口 {tab.port}</div>

      <div className="space-y-4">
        {/* ─── Status card ─── */}
        <Card title={`${tab.label} 状态`}>
          <div className="p-4 space-y-3">
            {activeDiag && (
              <div className={`p-3 rounded-a-sm text-xs flex items-center gap-3 border ${
                activeDiag.service_running
                  ? 'bg-[#4cd964]/5 border-[#4cd964]/15'
                  : activeDiag.installed
                    ? 'bg-[#e8b830]/5 border-[#e8b830]/15'
                    : 'bg-a-border/5 border-a-border/15'
              }`}>
                <span className={`w-2 h-2 rounded-full ${
                  activeDiag.service_running ? 'bg-[#4cd964]' : activeDiag.installed ? 'bg-[#e8b830]' : 'bg-a-muted'
                }`} />
                <span className="flex-1">
                  {activeDiag.service_running
                    ? `${tab.label} 正在运行 · ${activeDiag.version || ''}`
                    : activeDiag.installed
                      ? `${tab.label} 已安装但未运行`
                      : `${tab.label} 未安装`}
                </span>
                {activeDiag.config_path && (
                  <span className="text-a-muted font-mono text-[10px]">{activeDiag.config_path}</span>
                )}
              </div>
            )}

            {activeDiag && (
              <div className="grid grid-cols-3 gap-2">
                {[
                  ['已安装', activeDiag.installed],
                  ['版本', activeDiag.version],
                  ['版本兼容', activeDiag.version_supported],
                  ['服务运行', activeDiag.service_running],
                  ['配置有效', activeDiag.config_valid],
                  ['监听正常', activeDiag.listener_ok],
                ].map(([label, val]) => (
                  <div key={label as string} className="flex items-center justify-between p-2 rounded-a-sm bg-a-bg border border-a-border/30">
                    <span className="text-[10px] text-a-muted">{label as string}</span>
                    <span>{statusBadge(typeof val === 'boolean' ? val : undefined)}</span>
                  </div>
                ))}
              </div>
            )}

            {activeDiag?.last_error_message && (
              <div className="p-2.5 rounded-a-sm text-xs bg-[#ff5c72]/8 text-[#ff5c72] border border-[#ff5c72]/15">
                {activeDiag.last_error_message}
              </div>
            )}
          </div>
        </Card>

        {/* ─── Action buttons ─── */}
        <Card title="操作">
          <div className="p-4">
            <div className="flex flex-wrap gap-2">
              {!activeDiag?.installed ? (
                <Btn primary onClick={() => installMu.mutate(activeTab)} disabled={installMu.isPending}>
                  {installMu.isPending ? '安装中…' : '安装'}
                </Btn>
              ) : (
                <>
                  <Btn onClick={() => reloadMu.mutate(activeTab)} disabled={reloadMu.isPending}>
                    {reloadMu.isPending ? '重载中…' : '重载配置'}
                  </Btn>
                  <Btn onClick={() => svcMu.mutate({ provider: activeTab, action: 'restart' })} disabled={svcMu.isPending}>
                    {svcMu.isPending ? '重启中…' : '重启'}
                  </Btn>
                  {activeDiag?.service_running ? (
                    <Btn onClick={() => svcMu.mutate({ provider: activeTab, action: 'stop' })} disabled={svcMu.isPending}>
                      停止
                    </Btn>
                  ) : (
                    <Btn onClick={() => svcMu.mutate({ provider: activeTab, action: 'start' })} disabled={svcMu.isPending}>
                      启动
                    </Btn>
                  )}
                  <span className="flex-1" />
                  <Btn
                    onClick={() => {
                      if (confirm(`确定要卸载 ${tab.label} 吗？配置文件将保留在 ${tab.configPath}。`)) {
                        uninstallMu.mutate(activeTab);
                      }
                    }}
                    disabled={uninstallMu.isPending}
                  >
                    {uninstallMu.isPending ? '卸载中…' : '卸载'}
                  </Btn>
                </>
              )}
            </div>

            {(installMu.data as any)?.status && (
              <div className="mt-3 p-2.5 rounded-a-sm text-xs bg-[#4cd964]/5 text-a-fg border border-[#4cd964]/15 font-mono whitespace-pre-wrap max-h-32 overflow-y-auto">
                {(installMu.data as any).message}
                {(installMu.data as any).apt_install_out && (
                  <div className="text-a-muted mt-1">{(installMu.data as any).apt_install_out}</div>
                )}
              </div>
            )}
          </div>
        </Card>

        {/* ─── Diagnostic pipeline ─── */}
        <Card title="诊断流水线">
          <div className="p-4">
            {activeDiag ? (
              <div className="space-y-1">
                {diagSteps.map((step, i) => {
                  const passed = diagPassMap(activeDiag)[i];
                  const skipped = activeTab === 'haproxy' && step.code === 'RUNTIME_VERIFY_FAILED';
                  return (
                    <div key={step.code} className="flex items-center gap-3 p-2 rounded-a-sm hover:bg-a-bg/50">
                      <span className={skipped ? 'text-a-muted' : passed ? 'text-[#4cd964]' : 'text-[#ff5c72]'}>
                        {skipped ? '○' : passed ? '✓' : '✗'}
                      </span>
                      <span className={`text-xs ${skipped ? 'text-a-muted' : 'text-a-fg'}`}>
                        {step.label}
                      </span>
                      <span className="text-[10px] text-a-muted flex-1">{step.desc(activeTab)}</span>
                      {skipped && <span className="text-[10px] text-a-muted">跳过 (非 HTTP)</span>}
                    </div>
                  );
                })}
              </div>
            ) : (
              <div className="text-xs text-a-muted py-4 text-center">等待诊断数据…</div>
            )}
          </div>
        </Card>

        {/* ─── Config editor ─── */}
        <Card
          title="配置文件"
          actions={
            configLoaded && !editingConfig ? (
              <Btn onClick={() => setEditingConfig(true)}>编辑</Btn>
            ) : editingConfig ? (
              <div className="flex gap-2">
                <Btn onClick={() => { setEditingConfig(false); }}>取消</Btn>
                <Btn
                  primary
                  onClick={() => saveConfigMu.mutate({ provider: activeTab, content: configContent })}
                  disabled={saveConfigMu.isPending}
                >
                  {saveConfigMu.isPending ? '保存中…' : '保存'}
                </Btn>
              </div>
            ) : null
          }
        >
          <div className="p-4">
            {!configLoaded ? (
              <div className="text-xs text-a-muted py-4 text-center">加载中…</div>
            ) : editingConfig ? (
              <textarea
                className="w-full font-mono text-[11px] p-3 rounded-a-sm border border-a-border bg-a-bg text-a-fg outline-none focus:border-a-accent resize-y"
                rows={20}
                value={configContent}
                onChange={(e) => setConfigContent(e.target.value)}
                spellCheck={false}
              />
            ) : configContent ? (
              <pre className="text-[11px] font-mono text-a-fg bg-a-bg p-3 rounded-a-sm border border-a-border/30 overflow-x-auto max-h-96 overflow-y-auto whitespace-pre">
                {configContent}
              </pre>
            ) : (
              <div className="text-xs text-a-muted py-4 text-center">
                配置文件不存在或为空
                <div className="mt-1">
                  <Btn onClick={() => setEditingConfig(true)}>创建配置</Btn>
                </div>
              </div>
            )}

            {configLoaded && (
              <div className="text-[10px] text-a-muted mt-2">
                路径: <span className="font-mono">{tab.configPath}</span>
                {editingConfig && ' · 编辑后点击保存，备份自动存为 .bak'}
              </div>
            )}
          </div>
        </Card>
      </div>
    </div>
  );
}

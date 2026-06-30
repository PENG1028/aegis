/**
 * Settings — 系统配置页面 (v1.8J)
 *
 * 域名配置: 设置后面板自动启用 HTTPS（Let's Encrypt via Caddy）
 * DNS: DNS 解析器开关和状态
 */

import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { fetchSettings, updateSettings } from '@/lib/api-bridge';
import { PageHeader, Card, MetaRow, Btn, Alert } from '@/components/shared';
import { useToast } from '@/components/shared/Toast';
import DnsSettingsPanel from '@/components/dns/DnsSettingsPanel';

export default function SettingsPage() {
  const toast = useToast();
  const queryClient = useQueryClient();
  const [domain, setDomain] = useState('');
  const [email, setEmail] = useState('');
  const [saving, setSaving] = useState(false);
  const [result, setResult] = useState<any>(null);
  const [domainLoaded, setDomainLoaded] = useState(false);

  const { data, isLoading, error } = useQuery({
    queryKey: ['settings'],
    queryFn: async () => {
      const s = await fetchSettings();
      // Initialize domain/email from loaded settings
      if (!domainLoaded) {
        const d = s?.managed_domain?.gateway_domain || '';
        const e = s?.proxy?.email || '';
        setDomain(d);
        setEmail(e);
        setDomainLoaded(true);
      }
      return s;
    },
  });

  async function saveDomain() {
    setSaving(true);
    setResult(null);
    try {
      const res = await updateSettings({
        managed_domain: { gateway_domain: domain.trim() },
        proxy: { email: email.trim() },
      });
      setResult(res);
      queryClient.invalidateQueries({ queryKey: ['settings'] });
      toast(res.panel_url ? `面板地址: ${res.panel_url}` : '设置已保存');
    } catch (e: any) {
      toast(e.message, 'error');
    }
    setSaving(false);
  }

  if (isLoading) return <div className="text-center py-10 text-a-muted font-mono text-sm">加载中...</div>;
  if (error) return <div className="px-4 py-3 rounded-a-md text-xs border bg-[#ff5c72]/10 text-[#ff5c72] border-[#ff5c72]/20">加载失败: {error.message}</div>;

  const currentDomain = data?.managed_domain?.gateway_domain || '';
  const hasTLS = !!currentDomain;

  return (
    <div>
      <PageHeader title="设置" helpKey="settings" sub="系统配置" />

      <div className="space-y-4">
        {/* ─── Domain / TLS Panel ─── */}
        <Card title="面板域名 · TLS">
          <div className="p-[18px] space-y-4">
            {/* Current status */}
            <div className="flex items-center gap-3 p-3 rounded-a-sm border text-xs"
              style={{
                borderColor: hasTLS ? 'rgba(76,217,100,0.25)' : 'rgba(232,184,48,0.25)',
                backgroundColor: hasTLS ? 'rgba(76,217,100,0.06)' : 'rgba(232,184,48,0.06)',
              }}>
              <span className={`w-2 h-2 rounded-full ${hasTLS ? 'bg-[#4cd964]' : 'bg-[#e8b830]'}`} />
              <div className="flex-1">
                <span className="font-medium">
                  {hasTLS ? 'TLS 已启用' : '未配置 TLS'}
                </span>
                <span className="text-a-muted ml-2">
                  {hasTLS
                    ? `Let's Encrypt 自动证书 · https://${currentDomain}`
                    : '设置域名后自动启用 HTTPS'}
                </span>
              </div>
              {hasTLS && (
                <a href={`https://${currentDomain}`} target="_blank" rel="noopener"
                  className="text-a-accent hover:underline text-[11px]">
                  打开面板 →
                </a>
              )}
            </div>

            {/* Domain input */}
            <div>
              <label className="block text-xs font-medium text-a-muted mb-1.5">域名</label>
              <input
                className="w-full font-mono text-sm px-3 py-2 rounded-a-sm border border-a-border bg-a-bg text-a-fg outline-none focus:border-a-accent"
                value={domain}
                onChange={(e) => setDomain(e.target.value)}
                placeholder="例如: panel.yourdomain.com（留空 = 仅 HTTP）"
              />
              <div className="text-[10px] text-a-muted mt-1">
                需先将域名 DNS 解析到此服务器的公网 IP。Caddy 自动申请 Let's Encrypt 证书。
              </div>
            </div>

            {/* Email (for Let's Encrypt) */}
            <div>
              <label className="block text-xs font-medium text-a-muted mb-1.5">
                Let's Encrypt 通知邮箱
                <span className="text-a-muted font-normal ml-1">（可选）</span>
              </label>
              <input
                className="w-full font-mono text-sm px-3 py-2 rounded-a-sm border border-a-border bg-a-bg text-a-fg outline-none focus:border-a-accent"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder="admin@yourdomain.com"
              />
            </div>

            {/* Save */}
            <div className="flex items-center gap-3">
              <Btn primary onClick={saveDomain} disabled={saving}>
                {saving ? '保存中…' : '保存并重载 Caddy'}
              </Btn>
              {domain.trim() && domain.trim() !== currentDomain && (
                <span className="text-xs text-a-accent">
                  将启用 HTTPS: https://{domain.trim()}
                </span>
              )}
            </div>

            {/* Result */}
            {result && (
              <div className={`p-3 rounded-a-sm text-xs border ${
                result.caddy_reloaded
                  ? 'bg-[#4cd964]/5 border-[#4cd964]/15'
                  : 'bg-[#e8b830]/5 border-[#e8b830]/15'
              }`}>
                {result.panel_url && (
                  <div className="font-medium mb-1">
                    面板地址: <a href={result.panel_url} target="_blank" rel="noopener"
                      className="text-a-accent hover:underline font-mono">{result.panel_url}</a>
                  </div>
                )}
                <div className="text-a-muted space-y-0.5">
                  <div>TLS: {result.tls || '—'}</div>
                  {result.caddyfile_regenerated && <div>✓ Caddyfile 已更新</div>}
                  {result.caddy_reloaded && <div>✓ Caddy 已重载</div>}
                  {result.caddy_reload_warning && (
                    <div className="text-[#e8b830]">⚠ {result.caddy_reload_warning}</div>
                  )}
                </div>
              </div>
            )}
          </div>
        </Card>

        {/* DNS Resolver Panel */}
        <DnsSettingsPanel />

        {/* Other settings (read-only) */}
        {data && Object.entries(data)
          .filter(([group]) => !['managed_domain', 'proxy'].includes(group))
          .map(([group, values]) => (
            <Card key={group} title={group.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase())}>
              <div>
                {Object.entries(values as Record<string, any>).map(([key, val]) => (
                  <MetaRow key={key} label={key.replace(/_/g, ' ')} value={String(val)} mono />
                ))}
              </div>
            </Card>
          ))}
      </div>
    </div>
  );
}

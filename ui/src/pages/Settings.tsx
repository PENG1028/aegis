/**
 * Settings — 系统配置页面 (v1.8J)
 *
 * 域名配置: 设置后面板自动启用 HTTPS（Let's Encrypt via Caddy）
 * 修改密码: 修改管理员登录密码
 * DNS: DNS 解析器开关和状态
 */

import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { fetchSettings, updateSettings, auth } from '@/lib/api-bridge';
import { PageHeader, Card, MetaRow, Btn, Alert } from '@/components/shared';
import { useToast } from '@/components/shared/Toast';
import DnsSettingsPanel from '@/components/dns/DnsSettingsPanel';

export default function SettingsPage() {
  const toast = useToast();
  const queryClient = useQueryClient();
  const [domain, setDomain] = useState('');
  const [email, setEmail] = useState('');
  const [tlsCertFile, setTlsCertFile] = useState('');
  const [tlsKeyFile, setTlsKeyFile] = useState('');
  const [saving, setSaving] = useState(false);
  const [result, setResult] = useState<any>(null);
  const [domainLoaded, setDomainLoaded] = useState(false);

  const { data, isLoading, error } = useQuery({
    queryKey: ['settings'],
    queryFn: async () => {
      const s = await fetchSettings();
      if (!domainLoaded) {
        setDomain(s?.managed_domain?.gateway_domain || '');
        setEmail(s?.proxy?.email || '');
        setTlsCertFile(s?.proxy?.tls_cert_file || '');
        setTlsKeyFile(s?.proxy?.tls_key_file || '');
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
        proxy: {
          email: email.trim(),
          tls_cert_file: tlsCertFile.trim(),
          tls_key_file: tlsKeyFile.trim(),
        },
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

            {/* Custom TLS certificate (Cloudflare Origin CA, etc.) */}
            <details className="text-xs">
              <summary className="text-a-muted cursor-pointer hover:text-a-fg py-1">
                自定义 TLS 证书（如 Cloudflare Origin CA）…
              </summary>
              <div className="mt-2 space-y-3 pl-2 border-l-2 border-a-border/30">
                <div>
                  <label className="block text-xs font-medium text-a-muted mb-1">证书文件路径 (.pem)</label>
                  <input
                    className="w-full font-mono text-sm px-3 py-2 rounded-a-sm border border-a-border bg-a-bg text-a-fg outline-none focus:border-a-accent"
                    value={tlsCertFile}
                    onChange={(e) => setTlsCertFile(e.target.value)}
                    placeholder="/etc/ssl/certs/panel.example.com.pem"
                  />
                </div>
                <div>
                  <label className="block text-xs font-medium text-a-muted mb-1">私钥文件路径 (.pem)</label>
                  <input
                    className="w-full font-mono text-sm px-3 py-2 rounded-a-sm border border-a-border bg-a-bg text-a-fg outline-none focus:border-a-accent"
                    value={tlsKeyFile}
                    onChange={(e) => setTlsKeyFile(e.target.value)}
                    placeholder="/etc/ssl/private/panel.example.com.key"
                  />
                </div>
                <div className="text-[10px] text-a-muted">
                  设置后 Caddy 将使用此证书替代 Let's Encrypt。适用于 Cloudflare Origin CA 等场景。
                  清空两个路径即可恢复 Let's Encrypt。
                </div>
              </div>
            </details>

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

        {/* ─── Change Password Panel ─── */}
        <ChangePasswordPanel />

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

// ─── Change Password Panel ───

function ChangePasswordPanel() {
  const toast = useToast();
  const [currentPw, setCurrentPw] = useState('');
  const [newPw, setNewPw] = useState('');
  const [confirmPw, setConfirmPw] = useState('');
  const [saving, setSaving] = useState(false);
  const [result, setResult] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function doChange() {
    setError(null);
    setResult(null);

    if (!currentPw || !newPw || !confirmPw) {
      setError('所有字段不能为空');
      return;
    }
    if (newPw !== confirmPw) {
      setError('两次输入的新密码不一致');
      return;
    }
    if (newPw.length < 8) {
      setError('新密码至少 8 个字符');
      return;
    }

    setSaving(true);
    try {
      const res = await auth.changePassword(currentPw, newPw);
      setResult(res.message || '密码已修改');
      setCurrentPw('');
      setNewPw('');
      setConfirmPw('');
      toast('密码修改成功');
    } catch (e: any) {
      setError(e.message || '修改失败');
    }
    setSaving(false);
  }

  return (
    <Card title="修改密码">
      <div className="p-[18px] space-y-3">
        <div>
          <label className="block text-xs font-medium text-a-muted mb-1">当前密码</label>
          <input
            type="password"
            className="w-full font-mono text-sm px-3 py-2 rounded-a-sm border border-a-border bg-a-bg text-a-fg outline-none focus:border-a-accent"
            value={currentPw}
            onChange={(e) => setCurrentPw(e.target.value)}
            placeholder="输入当前密码"
          />
        </div>
        <div className="flex gap-3">
          <div className="flex-1">
            <label className="block text-xs font-medium text-a-muted mb-1">新密码</label>
            <input
              type="password"
              className="w-full font-mono text-sm px-3 py-2 rounded-a-sm border border-a-border bg-a-bg text-a-fg outline-none focus:border-a-accent"
              value={newPw}
              onChange={(e) => setNewPw(e.target.value)}
              placeholder="至少 8 个字符"
            />
          </div>
          <div className="flex-1">
            <label className="block text-xs font-medium text-a-muted mb-1">确认新密码</label>
            <input
              type="password"
              className="w-full font-mono text-sm px-3 py-2 rounded-a-sm border border-a-border bg-a-bg text-a-fg outline-none focus:border-a-accent"
              value={confirmPw}
              onChange={(e) => setConfirmPw(e.target.value)}
              placeholder="再次输入新密码"
            />
          </div>
        </div>

        {error && <div className="text-xs text-[#ff5c72]">{error}</div>}
        {result && <div className="text-xs text-[#4cd964]">{result}</div>}

        <Btn primary onClick={doChange} disabled={saving}>
          {saving ? '修改中…' : '修改密码'}
        </Btn>
      </div>
    </Card>
  );
}

import { useState, useEffect } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { fetchSettings, updateSettings, providerApi, certApi, adminApi, type CertificateItem } from '@/lib/api-bridge';
import { useToast, Card, PageHeader, Btn } from '@/components/shared';
import Input from '@/components/ui/Input';

interface ProviderInfo {
  id: string;
  name: string;
  capabilities: string[];
}

interface ProviderListResponse {
  providers: ProviderInfo[];
  capability_universe: Array<{ key: string; layer: string; label: string; description: string }>;
}

export default function TlsSettings() {
  const toast = useToast();
  const qc = useQueryClient();

  const { data: settings } = useQuery({
    queryKey: ['settings'],
    queryFn: fetchSettings,
  });
  const { data: provData } = useQuery({
    queryKey: ['providers'],
    queryFn: () => providerApi.list() as Promise<ProviderListResponse>,
  });
  const { data: certData } = useQuery({
    queryKey: ['certificates'],
    queryFn: () => certApi.list(),
  });

  const s = settings as any;
  const providers: ProviderInfo[] = provData?.providers || [];
  const certs: CertificateItem[] = certData?.certificates || [];

  const autoCertProv = providers.find((p) => p.capabilities?.includes('auto_cert'));
  const loadCertProv = providers.find((p) => p.capabilities?.includes('load_cert'));

  const domainConfigured = s?.managed_domain?.gateway_domain || '';

  // ── Email state ──
  const [email, setEmail] = useState('');
  const [emailSaved, setEmailSaved] = useState(false);
  useEffect(() => {
    if (s) setEmail(s.proxy?.email || '');
  }, [s]);

  const emailMut = useMutation({
    mutationFn: (val: string) => updateSettings({ proxy: { email: val } }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['settings'] });
      setEmailSaved(true);
      toast('邮箱已保存');
      setTimeout(() => setEmailSaved(false), 3000);
    },
    onError: (e: any) => toast(e.message || '保存失败', 'error'),
  });

  const emailConfigured = s?.proxy?.email || '';

  // ── Cert binding ──
  function parseDomains(item: CertificateItem): string[] {
    try {
      const arr = JSON.parse(item.domains);
      return Array.isArray(arr) ? arr : [String(arr)];
    } catch {
      return [item.domains];
    }
  }

  function primaryDomain(item: CertificateItem): string {
    const domains = parseDomains(item);
    return domains[0] || '';
  }

  function expiryLabel(item: CertificateItem): string {
    const d = new Date(item.not_after);
    const days = Math.floor((d.getTime() - Date.now()) / 86400000);
    if (days < 0) return '已过期';
    if (days <= 30) return `${days} 天后到期`;
    if (days <= 90) return `${days} 天后到期`;
    return '有效';
  }

  const [selectedCertId, setSelectedCertId] = useState('');
  const selectedCert = certs.find((c) => c.id === selectedCertId);
  const certDomain = selectedCert ? primaryDomain(selectedCert) : '';

  const bindMut = useMutation({
    mutationFn: async () => {
      if (!certDomain) return;
      await updateSettings({ managed_domain: { gateway_domain: certDomain } });
      await adminApi.applyConfig();
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['settings'] });
      qc.invalidateQueries({ queryKey: ['certificates'] });
      toast(`证书已绑定到面板。域名: ${certDomain}`);
    },
    onError: (e: any) => toast(e.message || '绑定失败', 'error'),
  });

  const handleUnbind = async () => {
    await updateSettings({ managed_domain: { gateway_domain: '' } });
    setSelectedCertId('');
    qc.invalidateQueries({ queryKey: ['settings'] });
    toast('已取消。面板域名已清空');
  };

  // ── TLS status ──
  const certConfigured = certs.length > 0 && domainConfigured
    ? certs.find((c) => {
        const domains = parseDomains(c);
        return domains.includes(domainConfigured);
      })
    : null;

  let tlsStatus = 'HTTP only（未配置 TLS）';
  if (certConfigured) tlsStatus = `自定义证书（${new Date(certConfigured.not_after).toLocaleDateString('zh-CN')} 到期）`;
  else if (domainConfigured && emailConfigured) tlsStatus = "自动 Let's Encrypt";
  else if (domainConfigured) tlsStatus = '域名已设置（需配置邮箱或绑定证书）';

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="TLS 证书配置" subtitle="Let's Encrypt 邮箱 · 证书绑定 · 状态" />

      {/* Section 1: ACME email */}
      {autoCertProv && (
        <Card title="Let's Encrypt 自动证书" subtitle={`通过 ${autoCertProv.name} 自动申请和续签`}>
          {!emailConfigured && (
            <div className="bg-[#e8b830]/10 border border-[#e8b830]/30 rounded-a-sm px-3 py-2 mb-3 text-xs text-[#e8b830]">
              未配置注册邮箱。虽然 Let's Encrypt 不强制要求邮箱，但证书到期时将无法收到通知。
            </div>
          )}

          <div className="space-y-3">
            <div>
              <label className="text-xs text-a-muted block mb-1">Let's Encrypt 注册邮箱（选填）</label>
              <div className="flex gap-2">
                <Input
                  value={email}
                  onChange={(e: React.ChangeEvent<HTMLInputElement>) => setEmail(e.target.value)}
                  placeholder="admin@example.com"
                />
                <Btn
                  primary
                  onClick={() => emailMut.mutate(email)}
                  disabled={emailMut.isPending}
                  className="shrink-0"
                >
                  {emailMut.isPending ? '...' : emailSaved ? '已保存' : '保存'}
                </Btn>
              </div>
              <p className="text-[11px] text-a-muted mt-1">
                仅用于 ACME 证书注册和到期通知。可随时补充，不影响面板正常运行。
              </p>
            </div>

            <div>
              <label className="text-xs text-a-muted block mb-1">面板域名</label>
              <div className="text-sm font-mono text-a-fg bg-a-bg border border-a-border rounded-a-sm px-3 py-2">
                {domainConfigured || '（未配置 — 前往「面板」标签设置）'}
              </div>
            </div>
          </div>
        </Card>
      )}

      {/* Section 2: Bind certificate via domain + Apply */}
      {loadCertProv && (
        <Card
          title="绑定已有证书"
          subtitle={`选择一个已入库的证书绑定到面板域名。保存后自动执行 Apply 并匹配证书。`}
        >
          {certs.length > 0 ? (
            <div className="space-y-4">
              <div>
                <label className="text-xs text-a-muted block mb-1">选择证书</label>
                <select
                  className="w-full font-mono text-xs px-3 py-2 rounded-a-sm border border-a-border bg-a-bg text-a-fg outline-none focus:border-a-accent"
                  value={selectedCertId}
                  onChange={(e) => setSelectedCertId(e.target.value)}
                >
                  <option value="">— 不绑定 —</option>
                  {certs.map((c) => {
                    const domains = parseDomains(c);
                    return (
                      <option key={c.id} value={c.id}>
                        {domains.join(', ') || c.id} ({expiryLabel(c)})
                      </option>
                    );
                  })}
                </select>
              </div>

              {selectedCert && (
                <div className="bg-a-bg border border-a-border rounded-a-sm p-3 text-xs space-y-1">
                  <div className="flex justify-between">
                    <span className="text-a-muted">证书域名</span>
                    <span className="font-mono text-a-fg">{parseDomains(selectedCert).join(', ')}</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-a-muted">签发者</span>
                    <span className="font-mono text-a-fg">
                      {selectedCert.issuer?.split(',')[0]?.replace('CN=', '') || '—'}
                    </span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-a-muted">到期</span>
                    <span className="font-mono text-a-fg">
                      {new Date(selectedCert.not_after).toLocaleDateString('zh-CN')} ({expiryLabel(selectedCert)})
                    </span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-a-muted">将绑定域名</span>
                    <span className="font-mono text-a-fg font-medium text-a-accent">
                      {certDomain || '—'}
                    </span>
                  </div>
                </div>
              )}

              <div className="flex gap-2">
                <Btn primary onClick={() => bindMut.mutate()} disabled={!selectedCert || bindMut.isPending}>
                  {bindMut.isPending ? '绑定中...' : '绑定证书到面板'}
                </Btn>
                {domainConfigured && (
                  <Btn onClick={handleUnbind} className="text-xs">
                    取消绑定
                  </Btn>
                )}
              </div>
              <p className="text-[10px] text-a-muted">
                点击后自动将面板域名设为证书域名，并执行 Apply。Apply 管线会自动将证书匹配到域名路由。
              </p>
            </div>
          ) : (
            <div className="py-6 text-center text-a-muted">
              <p className="text-sm">暂无已导入的证书</p>
              <p className="text-xs mt-1 opacity-60">
                前往「访问控制 → TLS 证书」上传 PEM 证书或等待 Caddy 自动签发
              </p>
            </div>
          )}
        </Card>
      )}

      {!autoCertProv && !loadCertProv && (
        <Card title="TLS 未就绪">
          <p className="text-sm text-a-muted">
            当前没有网关提供 TLS 能力。请先安装一个支持证书的网关中间件。
          </p>
        </Card>
      )}

      {/* Section 3: Current status — consistent with Panel page */}
      <Card title="当前 TLS 状态">
        <div className="space-y-2 text-sm">
          <div className="flex justify-between">
            <span className="text-a-muted">面板域名</span>
            <span className="font-mono text-a-fg">{domainConfigured || '未配置'}</span>
          </div>
          <div className="flex justify-between">
            <span className="text-a-muted">Let's Encrypt 注册邮箱</span>
            <span className="font-mono text-a-fg">{emailConfigured || '未配置'}</span>
          </div>
          <div className="flex justify-between">
            <span className="text-a-muted">已绑定证书</span>
            <span className="font-mono text-a-fg text-xs">
              {certConfigured
                ? `${parseDomains(certConfigured).join(', ')} (${expiryLabel(certConfigured)})`
                : '未绑定'}
            </span>
          </div>
          <div className="flex justify-between border-t border-a-border pt-2 mt-2">
            <span className="text-a-muted">TLS 状态</span>
            <span className={`font-medium ${tlsStatus.startsWith('HTTP only') ? 'text-a-muted' : 'text-a-accent'}`}>
              {tlsStatus}
            </span>
          </div>
        </div>
      </Card>
    </div>
  );
}

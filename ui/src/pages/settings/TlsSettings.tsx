import { useState, useEffect } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { fetchSettings, updateSettings, providerApi, certApi, acmeApi, system, type CertificateItem } from '@/lib/api-bridge';
import { useToast, Card, PageHeader, Btn } from '@/components/shared';
import Input from '@/components/ui/Input';

interface ProviderInfo { id: string; name: string; capabilities: string[]; }
interface ProviderListResponse {
  providers: ProviderInfo[];
  capability_universe: Array<{ key: string; layer: string; label: string; description: string }>;
}

function parseDomains(item: CertificateItem): string[] {
  try { const arr = JSON.parse(item.domains); return Array.isArray(arr) ? arr : [String(arr)]; }
  catch { return [item.domains]; }
}

function expiryLabel(item: CertificateItem): string {
  const d = new Date(item.not_after);
  const days = Math.floor((d.getTime() - Date.now()) / 86400000);
  if (days < 0) return '已过期';
  if (days <= 30) return `${days} 天后到期`;
  return '有效';
}

export default function TlsSettings() {
  const toast = useToast();
  const qc = useQueryClient();

  const { data: settings } = useQuery({ queryKey: ['settings'], queryFn: fetchSettings });
  const { data: provData } = useQuery({ queryKey: ['providers'], queryFn: () => providerApi.list() as Promise<ProviderListResponse> });
  const { data: certData } = useQuery({ queryKey: ['certificates'], queryFn: () => certApi.list() });
  const { data: acmeStatus } = useQuery({ queryKey: ['acme-status'], queryFn: () => acmeApi.status(), refetchInterval: 60_000 });

  const s = settings as any;
  const providers: ProviderInfo[] = provData?.providers || [];
  const certs: CertificateItem[] = certData?.certificates || [];
  const autoCertProv = providers.find((p) => p.capabilities?.includes('auto_cert'));
  const loadCertProv = providers.find((p) => p.capabilities?.includes('load_cert'));
  const domainConfigured = s?.managed_domain?.gateway_domain || '';
  const acmeAvailable = acmeStatus?.available ?? false;

  // ── Email ──
  const [email, setEmail] = useState('');
  const [emailSaved, setEmailSaved] = useState(false);
  useEffect(() => { if (s) setEmail(s.proxy?.email || ''); }, [s]);
  const emailMut = useMutation({
    mutationFn: (val: string) => updateSettings({ proxy: { email: val } }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['settings'] }); setEmailSaved(true); toast('邮箱已保存'); setTimeout(() => setEmailSaved(false), 3000); },
    onError: (e: any) => toast(e.message || '保存失败', 'error'),
  });
  const emailConfigured = s?.proxy?.email || '';

  // ── ACME one-click create ──
  const [acmeDomain, setAcmeDomain] = useState('');
  useEffect(() => { setAcmeDomain(domainConfigured); }, [domainConfigured]);

  const acmeMut = useMutation({
    mutationFn: async (domain: string) => {
      // 1. Obtain cert via ACME
      await acmeApi.obtain([domain]);
      // 2. Set domain in settings to create route (if panel domain not set)
      if (!domainConfigured) {
        await updateSettings({ managed_domain: { gateway_domain: domain } });
      }
      // 3. Apply — BindAutoCerts matches cert to route
      await system.apply();
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['certificates'] });
      qc.invalidateQueries({ queryKey: ['acme-status'] });
      qc.invalidateQueries({ queryKey: ['settings'] });
      qc.invalidateQueries({ queryKey: ['routes'] });
      toast(`证书已签发并绑定到域名 ${acmeDomain}。路由和 TLS 均已就绪。`);
    },
    onError: (e: any) => toast(e.message || '申请失败', 'error'),
  });

  // ── Cert binding ──
  const matchingCerts = certs.filter((c) => {
    if (!domainConfigured) return false;
    return parseDomains(c).includes(domainConfigured);
  });
  const [selectedCertId, setSelectedCertId] = useState('');
  useEffect(() => {
    if (matchingCerts.length > 0 && !selectedCertId) setSelectedCertId(matchingCerts[0].id);
  }, [domainConfigured, certData]);
  const selectedCert = matchingCerts.find((c) => c.id === selectedCertId);

  const bindMut = useMutation({
    mutationFn: async () => { await system.apply(); },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['settings'] });
      qc.invalidateQueries({ queryKey: ['certificates'] });
      toast(`已执行 Apply。证书将自动匹配到域名 ${domainConfigured}`);
    },
    onError: (e: any) => toast(e.message || 'Apply 失败', 'error'),
  });

  const handleUnbind = async () => {
    await updateSettings({ managed_domain: { gateway_domain: '' } });
    setSelectedCertId('');
    qc.invalidateQueries({ queryKey: ['settings'] });
    toast('已取消。面板域名已清空');
  };

  const boundCert = matchingCerts.length > 0 ? matchingCerts[0] : null;
  let tlsStatus = 'HTTP only（未配置 TLS）';
  if (boundCert) tlsStatus = `证书已就绪（${new Date(boundCert.not_after).toLocaleDateString('zh-CN')} 到期）`;
  else if (domainConfigured && emailConfigured) tlsStatus = "域名已设置，Apply 后将自动申请 Let's Encrypt";
  else if (domainConfigured) tlsStatus = '域名已设置（需配置邮箱或绑定证书）';

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="TLS 证书配置" subtitle="创建 · 上传 · 绑定 · 状态" />

      {/* Section 1: ACME one-click create */}
      {autoCertProv && (
        <Card title="创建证书" subtitle={`通过 ${autoCertProv.name} 的 Let's Encrypt 自动签发`}>
          {!emailConfigured && (
            <div className="bg-[#e8b830]/10 border border-[#e8b830]/30 rounded-a-sm px-3 py-2 mb-3 text-xs text-[#e8b830]">
              未配置注册邮箱。可正常申请证书，但到期时将无法收到邮件通知。
            </div>
          )}

          <div className="space-y-3">
            <div>
              <label className="text-xs text-a-muted block mb-1">Let's Encrypt 注册邮箱（选填）</label>
              <div className="flex gap-2">
                <Input value={email} onChange={(e) => setEmail(e.target.value)} placeholder="admin@example.com" />
                <Btn primary onClick={() => emailMut.mutate(email)} disabled={emailMut.isPending} className="shrink-0">
                  {emailMut.isPending ? '...' : emailSaved ? '已保存' : '保存'}
                </Btn>
              </div>
              <p className="text-[11px] text-a-muted mt-1">仅用于证书到期通知。可随时补充。</p>
            </div>

            <div className="border-t border-a-border pt-3">
              <label className="text-xs text-a-muted block mb-1">申请域名</label>
              <div className="flex gap-2">
                <Input value={acmeDomain} onChange={(e) => setAcmeDomain(e.target.value)} placeholder="example.com" />
                <Btn primary onClick={() => acmeMut.mutate(acmeDomain)} disabled={!acmeDomain || acmeMut.isPending}>
                  {acmeMut.isPending ? '申请中...' : '一键申请并绑定'}
                </Btn>
              </div>
              <p className="text-[11px] text-a-muted mt-1">
                自动签发 Let's Encrypt 证书，同时创建域名路由并绑定。需域名 DNS 已解析到本机。
              </p>
            </div>

            <div className="border-t border-a-border pt-3">
              <p className="text-[11px] text-a-muted">
                上传 PEM 证书或管理已有证书，请前往「
                <a href="/access/certificates" className="text-a-accent hover:underline">证书管理中心</a>
                」。
              </p>
            </div>
          </div>
        </Card>
      )}

      {/* Section 2: Bind existing certificate */}
      {loadCertProv && (
        <Card title="绑定已有证书" subtitle={boundCert ? `已绑定: ${parseDomains(boundCert).join(', ')} · ${expiryLabel(boundCert)}` : "从证书库选择已导入的证书绑定到面板域名"}>
          {!domainConfigured ? (
            <div className="py-4 text-center text-a-muted">
              <p className="text-sm">未配置面板域名</p>
              <p className="text-xs mt-1">
                请先在「<a href="/settings" className="text-a-accent hover:underline">面板设置</a>」中配置域名。
              </p>
            </div>
          ) : matchingCerts.length > 0 ? (
            <div className="space-y-4">
              <div>
                <label className="text-xs text-a-muted block mb-1">
                  匹配域名 <span className="text-a-accent font-mono">{domainConfigured}</span> 的证书
                </label>
                <select className="w-full font-mono text-xs px-3 py-2 rounded-a-sm border border-a-border bg-a-bg text-a-fg outline-none focus:border-a-accent"
                  value={selectedCertId} onChange={(e) => setSelectedCertId(e.target.value)}>
                  <option value="">— 不绑定 —</option>
                  {matchingCerts.map((c) => (
                    <option key={c.id} value={c.id}>{parseDomains(c).join(', ')} ({expiryLabel(c)})</option>
                  ))}
                </select>
              </div>
              {selectedCert && (
                <div className="bg-a-bg border border-a-border rounded-a-sm p-3 text-xs space-y-1">
                  <div className="flex justify-between"><span className="text-a-muted">证书域名</span><span className="font-mono text-a-fg">{parseDomains(selectedCert).join(', ')}</span></div>
                  <div className="flex justify-between"><span className="text-a-muted">签发者</span><span className="font-mono text-a-fg">{selectedCert.issuer?.split(',')[0]?.replace('CN=', '') || '—'}</span></div>
                  <div className="flex justify-between"><span className="text-a-muted">到期</span><span className="font-mono text-a-fg">{new Date(selectedCert.not_after).toLocaleDateString('zh-CN')} ({expiryLabel(selectedCert)})</span></div>
                </div>
              )}
              <div className="flex gap-2">
                <Btn primary onClick={() => bindMut.mutate()} disabled={bindMut.isPending}>{bindMut.isPending ? 'Apply 中...' : '执行 Apply（绑定证书）'}</Btn>
                {domainConfigured && <Btn onClick={handleUnbind} className="text-xs">取消域名</Btn>}
              </div>
            </div>
          ) : (
            <div className="py-4 text-center text-a-muted">
              <p className="text-sm">没有匹配域名 <span className="font-mono text-a-fg">{domainConfigured}</span> 的证书</p>
              <p className="text-xs mt-1 opacity-60">前往「<a href="/access/certificates" className="text-a-accent hover:underline">证书管理中心</a>」上传 PEM 证书。</p>
            </div>
          )}
        </Card>
      )}

      {!autoCertProv && !loadCertProv && (
        <Card title="TLS 未就绪"><p className="text-sm text-a-muted">当前没有网关提供 TLS 能力。请先安装网关中间件。</p></Card>
      )}

      {/* Section 3: Status */}
      <Card title="当前 TLS 状态">
        <div className="space-y-2 text-sm">
          <div className="flex justify-between"><span className="text-a-muted">面板域名</span><span className="font-mono text-a-fg">{domainConfigured || '未配置'}</span></div>
          <div className="flex justify-between"><span className="text-a-muted">Let's Encrypt 邮箱</span><span className="font-mono text-a-fg">{emailConfigured || '未配置'}</span></div>
          <div className="flex justify-between">
            <span className="text-a-muted">匹配的证书</span>
            <span className="font-mono text-a-fg text-xs">{boundCert ? `${parseDomains(boundCert).join(', ')} (${expiryLabel(boundCert)})` : matchingCerts.length > 0 ? `${matchingCerts.length} 个可用` : '无'}</span>
          </div>
          <div className="flex justify-between border-t border-a-border pt-2 mt-2">
            <span className="text-a-muted">TLS 状态</span>
            <span className={`font-medium ${tlsStatus.startsWith('HTTP only') ? 'text-a-muted' : 'text-a-accent'}`}>{tlsStatus}</span>
          </div>
        </div>
      </Card>
    </div>
  );
}

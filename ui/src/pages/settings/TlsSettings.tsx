import { useState, useEffect } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { fetchSettings, updateSettings, providerApi, certApi, type CertificateItem } from '@/lib/api-bridge';
import { useToast, Card, PageHeader, Btn } from '@/components/shared';

interface ProviderInfo {
  id: string;
  name: string;
  capabilities: string[];
}

interface ProviderListResponse {
  providers: ProviderInfo[];
  capability_universe: CapMeta[];
}

interface CapMeta {
  key: string;
  layer: string;
  label: string;
  description: string;
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
  const capUniverse: CapMeta[] = provData?.capability_universe || [];
  const certs: CertificateItem[] = certData?.certificates || [];

  function capMeta(key: string): CapMeta | undefined {
    return capUniverse.find((c) => c.key === key);
  }

  const autoCertProv = providers.find((p) => p.capabilities?.includes('auto_cert'));
  const loadCertProv = providers.find((p) => p.capabilities?.includes('load_cert'));
  const autoCap = capMeta('auto_cert');
  const loadCap = capMeta('load_cert');

  const [selectedCertId, setSelectedCertId] = useState('');
  const currentCertFile = s?.proxy?.tls_cert_file || '';
  const currentCertId = certs.find((c) => c.cert_path === currentCertFile)?.id || '';

  useEffect(() => {
    if (currentCertId) setSelectedCertId(currentCertId);
  }, [currentCertId]);

  const selectedCert = certs.find((c) => c.id === selectedCertId);

  const bindMut = useMutation({
    mutationFn: async () => {
      if (!selectedCert) return;
      const proxy: Record<string, any> = {};
      if (selectedCert.cert_path) proxy.tls_cert_file = selectedCert.cert_path;
      if (selectedCert.key_path) proxy.tls_key_file = selectedCert.key_path;
      return updateSettings({ proxy });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['settings'] });
      toast('证书已绑定到面板');
    },
    onError: (e: any) => toast(e.message || '绑定失败', 'error'),
  });

  const handleUnbind = async () => {
    await updateSettings({ proxy: { tls_cert_file: '', tls_key_file: '' } });
    setSelectedCertId('');
    qc.invalidateQueries({ queryKey: ['settings'] });
    toast('已取消证书绑定');
  };

  const certConfigured = s?.proxy?.tls_cert_file || s?.proxy?.tls_key_file;
  const domainConfigured = s?.managed_domain?.gateway_domain;
  const emailConfigured = s?.proxy?.email;

  let tlsStatus = 'HTTP only（未配置证书）';
  if (certConfigured) tlsStatus = '自定义证书';
  else if (domainConfigured && emailConfigured) tlsStatus = "自动 Let's Encrypt";

  const certFileName = certConfigured
    ? String(certConfigured).split('/').pop() || certConfigured
    : '未配置';

  function parseDomains(item: CertificateItem): string {
    try {
      const arr = JSON.parse(item.domains);
      return Array.isArray(arr) ? arr.join(', ') : item.domains;
    } catch {
      return item.domains;
    }
  }

  function expiryLabel(item: CertificateItem): string {
    const d = new Date(item.not_after);
    const days = Math.floor((d.getTime() - Date.now()) / 86400000);
    if (days < 0) return '已过期';
    if (days <= 30) return `${days} 天后到期`;
    if (days <= 90) return `${days} 天后到期`;
    return '有效';
  }

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="TLS 证书配置" subtitle={loadCap?.description || '基于网关能力自动适配'} />

      {/* Section 1: Auto ACME */}
      {autoCertProv && (
        <Card
          title={autoCap?.label || '自动证书'}
          subtitle={autoCap?.description || '通过 ACME 自动获取和续期 TLS 证书'}
        >
          <p className="text-sm text-a-muted mb-3">
            <span className="text-a-fg font-medium">{autoCertProv.name}</span>{' '}
            支持自动申请证书，无需手动管理。
          </p>
          <div className="space-y-3">
            <div>
              <label className="text-xs text-a-muted block mb-1">面板域名</label>
              <div className="text-sm font-mono text-a-fg bg-a-bg border border-a-border rounded-a-sm px-3 py-2">
                {domainConfigured || '（未配置 — 前往「面板」标签设置）'}
              </div>
            </div>
            <div>
              <label className="text-xs text-a-muted block mb-1">Let's Encrypt 注册邮箱</label>
              <div className="text-sm font-mono text-a-fg bg-a-bg border border-a-border rounded-a-sm px-3 py-2">
                {emailConfigured || '（未配置 — 前往「面板」标签设置）'}
              </div>
            </div>
            <p className="text-[11px] text-a-muted">
              在「面板」标签中设置域名和邮箱后，{autoCertProv.name} 将自动获取证书。
            </p>
          </div>
        </Card>
      )}

      {/* Section 2: Bind existing certificate */}
      {loadCertProv && (
        <Card
          title="绑定已有证书"
          subtitle="从证书库中选择已导入的证书绑定到面板"
        >
          <p className="text-sm text-a-muted mb-4">
            <span className="text-a-fg font-medium">{loadCertProv.name}</span>{' '}
            支持加载 PEM 证书文件。如需导入新证书，请前往「访问控制 → TLS 证书」页面操作。
          </p>

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
                  {certs.map((c) => (
                    <option key={c.id} value={c.id}>
                      {parseDomains(c)} ({expiryLabel(c)})
                    </option>
                  ))}
                </select>
              </div>

              {selectedCert && (
                <div className="bg-a-bg border border-a-border rounded-a-sm p-3 text-xs space-y-1">
                  <div className="flex justify-between">
                    <span className="text-a-muted">域名</span>
                    <span className="font-mono text-a-fg">{parseDomains(selectedCert)}</span>
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
                </div>
              )}

              <div className="flex gap-2">
                <Btn primary onClick={() => bindMut.mutate()} disabled={!selectedCert || bindMut.isPending}>
                  {bindMut.isPending ? '绑定中...' : '绑定证书'}
                </Btn>
                {currentCertId && (
                  <Btn onClick={handleUnbind} className="text-xs">
                    取消绑定
                  </Btn>
                )}
              </div>
            </div>
          ) : (
            <div className="py-6 text-center text-a-muted">
              <p className="text-sm">暂无已导入的证书</p>
              <p className="text-xs mt-1 opacity-60">
                前往「访问控制 → TLS 证书」上传 PEM 证书或通过 ACME 申请
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

      {/* Section 3: Current status */}
      <Card title="当前状态">
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
            <span className="text-a-muted">绑定的证书</span>
            <span className="font-mono text-a-fg text-xs">{certFileName}</span>
          </div>
          <div className="flex justify-between border-t border-a-border pt-2 mt-2">
            <span className="text-a-muted">TLS 状态</span>
            <span className="font-medium text-a-accent">{tlsStatus}</span>
          </div>
        </div>
      </Card>
    </div>
  );
}

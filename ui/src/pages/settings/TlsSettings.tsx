import { useState, useEffect } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { fetchSettings, updateSettings, providerApi } from '@/lib/api-bridge';
import { useToast, Card, PageHeader, Btn } from '@/components/shared';

interface CapMeta {
  key: string;
  layer: string;
  label: string;
  description: string;
}

interface ProviderInfo {
  id: string;
  name: string;
  capabilities: string[];
}

interface ProviderListResponse {
  providers: ProviderInfo[];
  capability_universe: CapMeta[];
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

  const s = settings as any;
  const providers: ProviderInfo[] = provData?.providers || [];
  const capUniverse: CapMeta[] = provData?.capability_universe || [];

  function capMeta(key: string): CapMeta | undefined {
    return capUniverse.find((c) => c.key === key);
  }

  function providerName(capKey: string): string {
    const p = providers.find((p) => p.capabilities?.includes(capKey));
    return p?.name || '当前网关';
  }

  const autoCertProv = providers.find((p) => p.capabilities?.includes('auto_cert'));
  const loadCertProv = providers.find((p) => p.capabilities?.includes('load_cert'));
  const autoCap = capMeta('auto_cert');
  const loadCap = capMeta('load_cert');

  const [certContent, setCertContent] = useState('');
  const [keyContent, setKeyContent] = useState('');
  const [certFile, setCertFile] = useState('');
  const [keyFile, setKeyFile] = useState('');

  useEffect(() => {
    if (s) {
      setCertFile(s.proxy?.tls_cert_file || '');
      setKeyFile(s.proxy?.tls_key_file || '');
    }
  }, [s]);

  const saveMut = useMutation({
    mutationFn: (updates: Record<string, any>) => updateSettings(updates),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['settings'] });
      toast('证书配置已保存');
    },
    onError: (e: any) => toast(e.message || '保存失败', 'error'),
  });

  const handleSaveCert = () => {
    const proxy: Record<string, any> = {};
    if (certContent.trim()) proxy.tls_cert_content = certContent.trim();
    if (keyContent.trim()) proxy.tls_key_content = keyContent.trim();
    if (!certContent.trim() && certFile) proxy.tls_cert_file = certFile;
    if (!keyContent.trim() && keyFile) proxy.tls_key_file = keyFile;
    saveMut.mutate({ proxy });
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
              <label className="text-xs text-a-muted block mb-1">通知邮箱</label>
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

      {/* Section 2: Manual cert */}
      {loadCertProv && (
        <Card
          title={loadCap?.label || '自定义证书'}
          subtitle={loadCap?.description || '加载 PEM 格式的证书文件'}
        >
          <p className="text-sm text-a-muted mb-4">
            粘贴 PEM 证书内容，或指定服务器上已有的证书文件路径。
            <span className="text-a-fg font-medium"> {loadCertProv.name}</span> 将加载该证书用于 TLS 终结。
          </p>

          <div className="space-y-3 mb-4">
            <div>
              <label className="text-xs text-a-muted block mb-1">证书内容 (PEM)</label>
              <textarea
                className="w-full font-mono text-xs px-3 py-2 rounded-a-sm border border-a-border bg-a-bg text-a-fg outline-none focus:border-a-accent resize-y"
                rows={5}
                value={certContent}
                onChange={(e) => setCertContent(e.target.value)}
                placeholder={'-----BEGIN CERTIFICATE-----\nMIID...\n-----END CERTIFICATE-----'}
              />
            </div>
            <div>
              <label className="text-xs text-a-muted block mb-1">私钥内容 (PEM)</label>
              <textarea
                className="w-full font-mono text-xs px-3 py-2 rounded-a-sm border border-a-border bg-a-bg text-a-fg outline-none focus:border-a-accent resize-y"
                rows={5}
                value={keyContent}
                onChange={(e) => setKeyContent(e.target.value)}
                placeholder={'-----BEGIN PRIVATE KEY-----\nMIIE...\n-----END PRIVATE KEY-----'}
              />
            </div>
          </div>

          <div className="border-t border-a-border pt-4 mb-4">
            <p className="text-[11px] text-a-muted mb-3">或指定服务器上已有的证书文件路径</p>
            <div className="space-y-3">
              <div>
                <label className="text-xs text-a-muted block mb-1">证书文件路径</label>
                <input
                  type="text"
                  className="w-full font-mono text-xs px-3 py-2 rounded-a-sm border border-a-border bg-a-bg text-a-fg outline-none focus:border-a-accent"
                  value={certFile}
                  onChange={(e) => setCertFile(e.target.value)}
                  placeholder="/etc/aegis/certs/panel.crt"
                />
              </div>
              <div>
                <label className="text-xs text-a-muted block mb-1">私钥文件路径</label>
                <input
                  type="text"
                  className="w-full font-mono text-xs px-3 py-2 rounded-a-sm border border-a-border bg-a-bg text-a-fg outline-none focus:border-a-accent"
                  value={keyFile}
                  onChange={(e) => setKeyFile(e.target.value)}
                  placeholder="/etc/aegis/certs/panel.key"
                />
              </div>
            </div>
          </div>

          <Btn primary onClick={handleSaveCert} disabled={saveMut.isPending}>
            {saveMut.isPending ? '保存中...' : '保存证书配置'}
          </Btn>
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
            <span className="text-a-muted">通知邮箱</span>
            <span className="font-mono text-a-fg">{emailConfigured || '未配置'}</span>
          </div>
          <div className="flex justify-between">
            <span className="text-a-muted">自定义证书文件</span>
            <span className="font-mono text-a-fg text-xs">{certFileName}</span>
          </div>
          <div className="flex justify-between border-t border-a-border pt-2 mt-2">
            <span className="text-a-muted">生效方式</span>
            <span className="font-medium text-a-accent">{tlsStatus}</span>
          </div>
        </div>
      </Card>
    </div>
  );
}

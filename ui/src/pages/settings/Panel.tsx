import { useState, useEffect } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { fetchSettings, updateSettings } from '@/lib/api-bridge';
import { useToast, Card, PageHeader, Btn } from '@/components/shared';
import Input from '@/components/ui/Input';
import ChangePasswordPanel from '@/components/settings/ChangePasswordPanel';

export default function PanelSettings() {
  const { data, isLoading } = useQuery({
    queryKey: ['settings'],
    queryFn: fetchSettings,
  });
  const queryClient = useQueryClient();
  const toast = useToast();

  const [domain, setDomain] = useState('');
  const [email, setEmail] = useState('');

  useEffect(() => {
    if (data) {
      setDomain((data as any).managed_domain?.gateway_domain || '');
      setEmail((data as any).proxy?.email || '');
    }
  }, [data]);

  const saveMutation = useMutation({
    mutationFn: (updates: Record<string, any>) => updateSettings(updates),
    onSuccess: (result: any) => {
      queryClient.invalidateQueries({ queryKey: ['settings'] });
      const msg = result?.tls_warning
        ? `已保存（注意：${result.tls_warning}）`
        : '设置已保存，执行 Apply 后生效';
      toast(msg);
    },
    onError: (e: any) => toast(e.message || '保存失败', 'error'),
  });

  if (isLoading) return <div className="p-6 text-a-muted text-sm">加载中...</div>;

  const s = data as any;
  const certConfigured = s?.proxy?.tls_cert_file || s?.proxy?.tls_key_file;
  const domainConfigured = s?.managed_domain?.gateway_domain;
  const emailConfigured = s?.proxy?.email;

  let tlsStatus = 'HTTP only（未配置 TLS）';
  if (certConfigured) tlsStatus = '自定义证书';
  else if (domainConfigured && emailConfigured) tlsStatus = "自动 Let's Encrypt";
  else if (domainConfigured) tlsStatus = '域名已设置（需配置邮箱或证书）';

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="面板设置" subtitle="域名、TLS 证书与管理员密码" />

      <Card
        title="面板域名"
        subtitle="配置面板的访问域名，网关将自动处理 TLS 证书"
      >
        <div className="space-y-3">
          <div>
            <label className="text-xs text-a-muted block mb-1">面板域名</label>
            <Input
              value={domain}
              onChange={(e: React.ChangeEvent<HTMLInputElement>) => setDomain(e.target.value)}
              placeholder="aegis.example.com"
            />
            <p className="text-[11px] text-a-muted mt-1">
              设置后可通过域名访问面板，而非 IP 地址。留空则仅允许 IP 直连。
            </p>
          </div>
          <div>
            <label className="text-xs text-a-muted block mb-1">通知邮箱</label>
            <Input
              value={email}
              onChange={(e: React.ChangeEvent<HTMLInputElement>) => setEmail(e.target.value)}
              placeholder="admin@example.com"
            />
            <p className="text-[11px] text-a-muted mt-1">
              Let's Encrypt 证书到期提醒和紧急通知将发送至该邮箱。
            </p>
          </div>

          <div className="border-t border-a-border pt-3">
            <p className="text-[11px] text-a-muted mb-2">
              <span className="text-a-accent">提示：</span>
              自定义 TLS 证书请在「TLS 证书」标签中配置。域名 + 邮箱 + 证书至少满足一项，面板才能启用 HTTPS。
            </p>
          </div>

          <Btn
            primary
            onClick={() =>
              saveMutation.mutate({
                managed_domain: { gateway_domain: domain },
                proxy: { email },
              })
            }
            disabled={saveMutation.isPending}
          >
            {saveMutation.isPending ? '保存中...' : '保存域名配置'}
          </Btn>
        </div>
      </Card>

      <Card title="当前 TLS 状态">
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
            <span className="text-a-muted">自定义证书</span>
            <span className="font-mono text-a-fg text-xs">
              {certConfigured
                ? String(certConfigured).split('/').pop() || certConfigured
                : '未配置'}
            </span>
          </div>
          <div className="flex justify-between border-t border-a-border pt-2 mt-2">
            <span className="text-a-muted">TLS 状态</span>
            <span
              className={`font-medium ${
                tlsStatus.startsWith('HTTP only') ? 'text-a-muted' : 'text-a-accent'
              }`}
            >
              {tlsStatus}
            </span>
          </div>
        </div>
      </Card>

      <Card title="修改管理员密码">
        <ChangePasswordPanel />
      </Card>
    </div>
  );
}

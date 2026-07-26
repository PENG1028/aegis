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

  useEffect(() => {
    if (data) {
      setDomain((data as any).managed_domain?.gateway_domain || '');
    }
  }, [data]);

  const saveMutation = useMutation({
    mutationFn: (updates: Record<string, any>) => updateSettings(updates),
    onSuccess: (result: any) => {
      queryClient.invalidateQueries({ queryKey: ['settings'] });
      const msg = result?.tls_warning ? `已保存（${result.tls_warning}）` : '设置已保存，执行 Apply 后生效';
      toast(msg);
    },
    onError: (e: any) => toast(e.message || '保存失败', 'error'),
  });

  if (isLoading) return <div className="p-6 text-a-muted text-sm">加载中...</div>;

  const s = data as any;
  const domainConfigured = s?.managed_domain?.gateway_domain;

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="面板设置" subtitle="域名与管理员密码" />

      <Card
        title="面板域名"
        subtitle="设置面板的访问域名，保存后 Apply 即可生效"
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
              设置后可通过域名访问面板。TLS 证书与邮箱请在「TLS 证书」标签中配置。
            </p>
          </div>

          <Btn
            primary
            onClick={() =>
              saveMutation.mutate({ managed_domain: { gateway_domain: domain } })
            }
            disabled={saveMutation.isPending}
          >
            {saveMutation.isPending ? '保存中...' : '保存域名配置'}
          </Btn>
        </div>
      </Card>

      <Card title="当前状态">
        <div className="space-y-2 text-sm">
          <div className="flex justify-between">
            <span className="text-a-muted">面板域名</span>
            <span className="font-mono text-a-fg">{domainConfigured || '未配置'}</span>
          </div>
          <div className="flex justify-between border-t border-a-border pt-2 mt-2">
            <span className="text-a-muted">提示</span>
            <span className="text-[11px] text-a-muted">
              前往「TLS 证书」标签配置证书和邮箱
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

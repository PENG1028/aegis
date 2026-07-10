// ─── Panel Settings ───
// Domain, TLS, and password configuration.

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

  // Init form from server data — useEffect avoids render-time setState issues
  useEffect(() => {
    if (data) {
      setDomain((data as any).managed_domain?.gateway_domain || '');
      setEmail((data as any).proxy?.email || '');
    }
  }, [data]);

  const saveMutation = useMutation({
    mutationFn: (updates: Record<string, any>) => updateSettings(updates),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['settings'] });
      toast('设置已保存');
    },
    onError: (e: any) => toast(e.message || '保存失败', 'error'),
  });

  if (isLoading) return <div className="p-6 text-a-muted text-sm">加载中...</div>;

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="面板设置" subtitle="域名 · TLS · 密码" />

      <Card title="面板域名">
        <div className="space-y-3">
          <div>
            <label className="text-xs text-a-muted block mb-1">面板域名</label>
            <Input
              value={domain}
              onChange={(e: React.ChangeEvent<HTMLInputElement>) => setDomain(e.target.value)}
              placeholder="aegis.example.com"
            />
          </div>
          <div>
            <label className="text-xs text-a-muted block mb-1">Let's Encrypt 邮箱</label>
            <Input
              value={email}
              onChange={(e: React.ChangeEvent<HTMLInputElement>) => setEmail(e.target.value)}
              placeholder="admin@example.com"
            />
          </div>
          <Btn
            primary
            onClick={() => saveMutation.mutate({
              managed_domain: { gateway_domain: domain },
              proxy: { email: email }
            })}
            disabled={saveMutation.isPending}
          >
            {saveMutation.isPending ? '保存中...' : '保存'}
          </Btn>
        </div>
      </Card>

      <Card title="修改管理员密码">
        <ChangePasswordPanel />
      </Card>
    </div>
  );
}

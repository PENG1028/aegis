// ─── Infra Management — unified middleware install/status/config ───
// All providers + infra deps in one table. Actions driven by capabilities, not hardcoded.

import { useState, useMemo } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { providerApi, infraApi } from '@/lib/api-bridge';
import { Card, PageHeader, Btn, useToast, Modal } from '@/components/shared';
import ProviderConfigModal from '@/components/settings/ProviderConfigModal';
import { cn } from '@/lib/utils';

interface MiddlewareItem {
  id: string;
  name: string;
  category: 'provider' | 'infra';
  installed: boolean;
  running: boolean;
  version: string;
  path: string;
  status: string;
  message: string;
  // provider-specific
  hasConfig: boolean;
  hasReload: boolean;
  hasService: boolean;
  canInstall: boolean;
  canUninstall: boolean;
}

export default function InfraManagement() {
  const toast = useToast();
  const qc = useQueryClient();
  const [configId, setConfigId] = useState<string | null>(null);

  // Fetch provider states
  const { data: providers } = useQuery({
    queryKey: ['providers'],
    queryFn: () => providerApi.list(),
    refetchInterval: 30_000,
  });

  // Fetch infra status
  const { data: infraData } = useQuery({
    queryKey: ['infra-status'],
    queryFn: () => infraApi.status(),
    refetchInterval: 30_000,
  });

  const items: MiddlewareItem[] = useMemo(() => {
    const list: MiddlewareItem[] = [];

    // Providers
    const provList = (providers as any)?.providers || [];
    for (const p of provList) {
      if (p.id === 'caddy' && p.installed && !p.running) continue; // caddy: show only if issue
      list.push({
        id: p.id,
        name: p.name || p.id,
        category: 'provider',
        installed: p.installed || false,
        running: p.running || false,
        version: p.version?.split('\n')[0] || '—',
        path: p.binary_path || p.config_path || '—',
        status: p.status || 'unknown',
        message: p.message || '',
        hasConfig: true,
        hasReload: p.capabilities?.includes('hot_reload'),
        hasService: true,
        canInstall: !p.installed && p.id !== 'caddy',
        canUninstall: p.installed && p.id !== 'caddy',
      });
    }

    // Infra deps
    const infras: any[] = (infraData as any)?.items || [];
    for (const inf of infras) {
      list.push({
        id: inf.name,
        name: inf.label || inf.name,
        category: 'infra',
        installed: inf.installed || false,
        running: inf.available || false,
        version: inf.version || '—',
        path: inf.path || '—',
        status: inf.available ? 'ready' : 'missing',
        message: inf.message || '',
        hasConfig: false,
        hasReload: false,
        hasService: false,
        canInstall: !inf.installed,
        canUninstall: inf.installed,
      });
    }

    return list;
  }, [providers, infraData]);

  // Mutations
  const installMut = useMutation({
    mutationFn: async ({ id, cat }: { id: string; cat: string }) => {
      const url = cat === 'provider'
        ? `/api/admin/v1/providers/${id}/install`
        : `/api/admin/v1/infra/${id}/install`;
      const res = await fetch(url, { method: 'POST', credentials: 'include' });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      return res.json();
    },
    onSuccess: () => { qc.invalidateQueries(); toast('安装成功'); },
    onError: (e: any) => toast(e.message || '安装失败', 'error'),
  });

  const serviceMut = useMutation({
    mutationFn: async ({ id, action }: { id: string; action: string }) => {
      const res = await fetch(`/api/admin/v1/providers/${id}/service`, {
        method: 'POST', credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ action }),
      });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
    },
    onSuccess: () => { qc.invalidateQueries(); toast('操作成功'); },
    onError: (e: any) => toast(e.message || '失败', 'error'),
  });

  const uninstallMut = useMutation({
    mutationFn: async ({ id, cat }: { id: string; cat: string }) => {
      const url = cat === 'provider'
        ? `/api/admin/v1/providers/${id}`
        : `/api/admin/v1/infra/${id}`;
      const res = await fetch(url, { method: 'DELETE', credentials: 'include' });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
    },
    onSuccess: () => { qc.invalidateQueries(); toast('已卸载'); },
    onError: (e: any) => toast(e.message || '卸载失败', 'error'),
  });

  return (
    <div className="p-6 space-y-5">
      <PageHeader title="中间件管理" subtitle="Provider + 基础设施依赖 · 安装 / 启停 / 配置查看" />

      <Card>
        <div className="overflow-x-auto">
          <table className="w-full text-xs">
            <thead>
              <tr className="border-b border-a-border/30 text-a-muted text-left">
                <th className="py-2 px-3">中间件</th>
                <th className="py-2 px-3">类型</th>
                <th className="py-2 px-3">状态</th>
                <th className="py-2 px-3">版本</th>
                <th className="py-2 px-3">路径</th>
                <th className="py-2 px-3">操作</th>
              </tr>
            </thead>
            <tbody>
              {items.map((item) => (
                <tr key={item.id} className="border-b border-a-border/20 hover:bg-a-border/10 transition-colors">
                  <td className="py-2 px-3">
                    <span className="font-medium text-a-fg">{item.name}</span>
                    <span className="text-[10px] text-a-muted ml-1.5">{item.id}</span>
                  </td>
                  <td className="py-2 px-3">
                    <span className={cn('px-1.5 py-0.5 rounded text-[9px] font-medium',
                      item.category === 'provider' ? 'bg-blue-500/10 text-blue-400' : 'bg-purple-500/10 text-purple-400')}>
                      {item.category === 'provider' ? 'Provider' : 'Infra'}
                    </span>
                  </td>
                  <td className="py-2 px-3">
                    <span className={cn('px-1.5 py-0.5 rounded text-[9px] font-medium',
                      item.status === 'ready' || item.status === 'available' ? 'bg-[#4cd964]/10 text-[#4cd964]' :
                      item.status === 'degraded' ? 'bg-[#e8b830]/10 text-[#e8b830]' :
                      'bg-[#ff5c72]/10 text-[#ff5c72]')}>
                      {item.status === 'ready' || item.status === 'available' ? '就绪' :
                       item.status === 'degraded' ? '降级' :
                       item.status === 'missing' ? '未安装' : item.status}
                    </span>
                    {item.message && <span className="text-[10px] text-a-muted ml-1.5">{item.message}</span>}
                  </td>
                  <td className="py-2 px-3 font-mono text-[10px] text-a-muted max-w-[140px] truncate">{item.version}</td>
                  <td className="py-2 px-3 font-mono text-[10px] text-a-muted max-w-[160px] truncate">{item.path}</td>
                  <td className="py-2 px-3">
                    <div className="flex gap-1 flex-wrap">
                      {!item.installed && item.canInstall && (
                        <Btn onClick={() => installMut.mutate({ id: item.id, cat: item.category })}
                          disabled={installMut.isPending} className="text-[9px]" primary>
                          安装
                        </Btn>
                      )}
                      {item.installed && item.hasReload && (
                        <Btn onClick={() => fetch(`/api/admin/v1/providers/${item.id}/reload`, { method: 'POST', credentials: 'include' })
                          .then(() => toast('已重载')).catch(() => toast('重载失败','error'))}
                          className="text-[9px]">重载</Btn>
                      )}
                      {item.installed && item.hasService && (
                        <>
                          {item.running ? (
                            <Btn onClick={() => serviceMut.mutate({ id: item.id, action: 'stop' })} className="text-[9px]">停止</Btn>
                          ) : (
                            <Btn onClick={() => serviceMut.mutate({ id: item.id, action: 'start' })} className="text-[9px]" primary>启动</Btn>
                          )}
                          {item.running && (
                            <Btn onClick={() => serviceMut.mutate({ id: item.id, action: 'restart' })} className="text-[9px]">重启</Btn>
                          )}
                        </>
                      )}
                      {item.installed && item.hasConfig && (
                        <Btn onClick={() => setConfigId(item.id)} className="text-[9px]">查看配置</Btn>
                      )}
                      {item.installed && item.canUninstall && (
                        <Btn onClick={() => { if (confirm(`确认卸载 ${item.name}?`)) uninstallMut.mutate({ id: item.id, cat: item.category }); }}
                          className="text-[9px]" danger>卸载</Btn>
                      )}
                      {item.installed && !item.hasConfig && !item.hasService && item.category === 'infra' && (
                        <span className="text-[9px] text-a-muted/50">—</span>
                      )}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Card>

      {configId && (
        <ProviderConfigModal
          providerId={configId}
          providerName={items.find(i => i.id === configId)?.name || configId}
          onClose={() => setConfigId(null)}
        />
      )}
    </div>
  );
}

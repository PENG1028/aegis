import { useQuery } from '@tanstack/react-query';
import { fetchSettings } from '@/lib/api-bridge';
import { PageHeader, Card, MetaRow } from '@/components/shared';
import DnsSettingsPanel from '@/components/dns/DnsSettingsPanel';

export default function SettingsPage() {
  const { data, isLoading, error } = useQuery({
    queryKey: ['settings'],
    queryFn: fetchSettings,
  });

  if (isLoading) return <div className="text-center py-10 text-a-muted font-mono text-sm">加载中...</div>;
  if (error) return <div className="px-4 py-3 rounded-a-md text-xs border bg-[#ff5c72]/10 text-[#ff5c72] border-[#ff5c72]/20">加载失败: {error.message}</div>;

  return (
    <div>
      <PageHeader title="设置" helpKey="settings" subtitle="系统配置" />
      <div className="space-y-4">
        {/* DNS Resolver Panel */}
        <DnsSettingsPanel />

        {/* Existing settings groups */}
        {data && Object.entries(data).map(([group, values]) => (
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

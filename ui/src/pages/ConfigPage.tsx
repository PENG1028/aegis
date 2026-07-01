import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { adminApi, system } from '@/lib/api-bridge';
import { PageHeader, Card, TabBar } from '@/components/shared';

export function ConfigPage() {
  const navigate = useNavigate();
  const [tab, setTab] = useState('current');

  const { data: status } = useQuery({
    queryKey: ['system-status'],
    queryFn: () => system.status(),
  });

  const { data: current } = useQuery({
    queryKey: ['config-current'],
    queryFn: () => adminApi.configCurrent(),
  });

  const { data: preview } = useQuery({
    queryKey: ['config-preview'],
    queryFn: () => adminApi.configPreview(),
  });

  const data = tab === 'current' ? current : preview;

  return (
    <div>
      <PageHeader title="配置预览 / 对比" helpKey="config" sub="配置证据页面，非编辑器" />
      <div className="grid grid-cols-4 gap-3 mb-4">
        <div><div className="text-[11px] text-a-muted uppercase tracking-[0.06em]">提供者</div><div className="font-mono text-sm mt-0.5">{status?.proxy?.provider || '—'}</div></div>
        <div><div className="text-[11px] text-a-muted uppercase tracking-[0.06em]">配置路径</div><div className="font-mono text-sm mt-0.5">{status?.proxy?.config_path || '—'}</div></div>
        <div><div className="text-[11px] text-a-muted uppercase tracking-[0.06em]">模式版本</div><div className="font-mono text-sm mt-0.5">{status?.store?.schema_version || '—'}</div></div>
        <div><div className="text-[11px] text-a-muted uppercase tracking-[0.06em]">路由数</div><div className="font-mono text-sm mt-0.5">{status?.counts?.routes || '—'}</div></div>
      </div>
      <TabBar
        tabs={[
          { key: 'current', label: '当前配置' },
          { key: 'preview', label: '预览' },
        ]}
        active={tab}
        onChange={setTab}
      />
      <Card>
        <div className="p-[18px]">
          <pre className="bg-a-bg border border-a-border rounded-a-sm p-3 font-mono text-xs text-a-muted overflow-x-auto max-h-[600px] whitespace-pre-wrap">
            {typeof data === 'object' && data !== null
              ? JSON.stringify(data, null, 2)
              : String(data || 'No config data')}
          </pre>
        </div>
      </Card>
    </div>
  );
}

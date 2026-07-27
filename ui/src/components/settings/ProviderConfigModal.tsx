// ─── ProviderConfigModal — view live middleware config ───
// Fetches GET /api/admin/v1/providers/{id}/config via the ConfigReader interface.
// Works for any provider that implements ConfigReader (Caddy, HAProxy, etc.).

import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Modal, Btn, LoadingState, ErrorBanner } from '@/components/shared';

interface Props {
  providerId: string;
  providerName: string;
  onClose: () => void;
}

export default function ProviderConfigModal({ providerId, providerName, onClose }: Props) {
  const [copied, setCopied] = useState(false);

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['provider-config', providerId],
    queryFn: async () => {
      const res = await fetch(`/api/admin/v1/providers/${providerId}/config`, { credentials: 'include' });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      return res.json();
    },
    enabled: !!providerId,
  });

  const config = data as any;
  const content: string = config?.content || '';
  const configPath: string = config?.config_path || '';

  const handleCopy = () => {
    navigator.clipboard.writeText(content);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <Modal onClose={onClose} title={`${providerName} 运行配置`}
      footer={
        <div className="flex items-center gap-2 justify-between w-full">
          {configPath && (
            <code className="text-[10px] text-a-muted font-mono truncate max-w-[320px]">{configPath}</code>
          )}
          <div className="flex gap-2 ml-auto">
            <Btn onClick={handleCopy} className="text-xs">{copied ? '已复制' : '复制'}</Btn>
            <Btn onClick={onClose} className="text-xs">关闭</Btn>
          </div>
        </div>
      }>
      {isLoading ? (
        <LoadingState />
      ) : error ? (
        <ErrorBanner message="加载配置失败" onRetry={refetch} />
      ) : (
        <pre className="bg-a-bg border border-a-border/30 rounded-a-sm p-3 text-[11px] font-mono text-a-fg leading-relaxed overflow-auto max-h-[60vh] whitespace-pre select-all">
          {content || '(空配置)'}
        </pre>
      )}
    </Modal>
  );
}

// ─── Dry-run ───
import { useState } from 'react';
import { Card, PageHeader, Btn, StatusBadge, useToast } from '@/components/shared';
import { adminApi } from '@/lib/api-bridge';
import { getScenario } from '@/mocks';
import { API_CONFIG } from '@/lib/api-config';

export default function DryRun() {
  const toast = useToast();
  const [result, setResult] = useState<any>(null);
  const [loading, setLoading] = useState(false);

  const handleDryRun = async () => {
    setLoading(true);
    try {
      if (API_CONFIG.useMock) {
        await new Promise(r => setTimeout(r, 800));
        const pending = getScenario().entryPoints.filter(ep => ep.release_state === 'pending' || ep.release_state === 'drifted');
        setResult({
          status: 'ok',
          routes_affected: pending.length || 1,
          warnings: pending.length > 0 ? 0 : 0,
          config_preview: `# Preview — Caddyfile\napi.proofnote.dev {\n  reverse_proxy node-a:3000 node-b:3001\n}\nauth.proofnote.dev {\n  reverse_proxy node-a:4000 node-b:4001\n}\n${pending.length > 0 ? 'docs.proofnote.dev {\n  reverse_proxy node-c:8080\n}\n' : ''}`,
          message: pending.length > 0 ? `${pending.length} 条新路由将生效` : '配置无变化',
        });
      } else {
        setResult(await adminApi.dryRun());
      }
    } catch (e) {
      toast((e as Error).message, 'error');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="Dry-run 预演" subtitle="模拟推送 · 不修改实际配置" />

      <Card title="预演" actions={<Btn primary onClick={handleDryRun} disabled={loading}>{loading ? '执行中...' : '执行 Dry-run'}</Btn>}>
        {result ? (
          <div className="space-y-4">
            <div className="flex items-center gap-3">
              <StatusBadge status={result.status === 'ok' ? 'active' : 'error'} />
              <span className="text-sm font-medium text-a-fg">{result.message}</span>
              <span className="text-xs text-a-muted">{result.routes_affected} 条路由受影响</span>
              {result.warnings > 0 && <span className="text-xs text-[#e8b830]">{result.warnings} 个警告</span>}
            </div>
            {result.config_preview && (
              <div className="bg-a-bg border border-a-border rounded-a-sm p-3">
                <div className="text-[10px] text-a-muted mb-2 uppercase">配置预览</div>
                <pre className="text-xs font-mono text-a-fg2 whitespace-pre-wrap">{result.config_preview}</pre>
              </div>
            )}
          </div>
        ) : (
          <div className="text-center py-8 text-a-muted text-sm">点击执行 Dry-run 预览配置变更效果</div>
        )}
      </Card>
    </div>
  );
}

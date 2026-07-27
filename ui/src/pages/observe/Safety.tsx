import { Card, PageHeader, Btn, StatusBadge } from '@/components/shared';
import { useState } from 'react';
import { safetyApi } from '@/lib/api-bridge';

export default function Safety() {
  const [results, setResults] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);

  const runCheck = async () => {
    setLoading(true);
    try {
      const r = await safetyApi.checkAllRoutes();
      setResults(Array.isArray(r) ? r : []);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="安全检查" subtitle="路由安全检测 · 风险评分" />
      <Card title="路由安全检查" actions={<Btn primary onClick={runCheck} disabled={loading}>{loading ? '检查中...' : '运行安全检查'}</Btn>}>
        {results.length > 0 ? (
          <div className="space-y-2">
            {results.map((r: any, i: number) => (
              <div key={i} className="flex items-center gap-2 p-2 rounded-a-sm bg-a-bg border border-a-border text-xs">
                <StatusBadge status={r.status || 'unknown'} />
                <span className="font-medium text-a-fg">{r.domain || r.route_id}</span>
                <span className="text-a-muted">{r.issue || r.message || '—'}</span>
              </div>
            ))}
          </div>
        ) : (
          <div className="text-center py-6 text-a-muted text-sm">点击运行安全检查</div>
        )}
      </Card>
    </div>
  );
}

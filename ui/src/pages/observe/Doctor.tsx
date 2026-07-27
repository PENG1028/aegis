import { Card, PageHeader, Btn, StatusBadge } from '@/components/shared';
import { useState } from 'react';
import { system } from '@/lib/api-bridge';

export default function Doctor() {
  const [results, setResults] = useState<any>(null);
  const [loading, setLoading] = useState(false);

  const runDoctor = async () => {
    setLoading(true);
    try {
      const r = await system.doctor();
      setResults(r);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="p-6 space-y-6">
      <PageHeader title="系统诊断" subtitle="运行全系统诊断检查" />
      <Card title="Doctor" actions={<Btn primary onClick={runDoctor} disabled={loading}>{loading ? '诊断中...' : '运行诊断'}</Btn>}>
        {results ? (
          <div className="p-3 rounded-a-sm bg-a-bg border border-a-border text-xs">
            <pre className="text-a-fg whitespace-pre-wrap">{JSON.stringify(results, null, 2)}</pre>
          </div>
        ) : (
          <div className="text-center py-6 text-a-muted text-sm">点击运行 Doctor 诊断</div>
        )}
      </Card>
    </div>
  );
}

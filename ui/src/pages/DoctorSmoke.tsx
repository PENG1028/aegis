import { useState } from 'react';
import { system } from '@/lib/api-bridge';
import { PageHeader, Card, Btn, Alert, StatusBadge } from '@/components/shared';
import { useToast } from '@/components/shared/Toast';

export function DoctorPage() {
  const toast = useToast();
  const [result, setResult] = useState<any>(null);
  const [loading, setLoading] = useState(false);

  async function doCheck() {
    setLoading(true);
    try {
      const res = await system.doctor();
      setResult(res);
      toast('诊断完成');
    } catch (e: any) { toast(e.message, 'error'); }
    setLoading(false);
  }

  return (
    <div>
      <PageHeader title="Doctor" sub="系统诊断与一致性检查" actions={
        <Btn primary onClick={doCheck} disabled={loading}>运行诊断</Btn>
      } />

      <Card title="诊断结果">
        <div className="p-[18px]">
          {result ? (
            <div className="space-y-2">
              <div className="font-mono text-xs text-a-accent">{result.message}</div>
              {result.checks?.map((c: any, i: number) => (
                <div key={i} className="flex items-center gap-2 text-xs">
                  <StatusBadge status={c.status} />
                  <span className="font-mono">{c.name}</span>
                  {c.source && <span className={`text-[10px] px-1.5 py-0.5 rounded ${c.source === 'real' ? 'bg-[#4cd964]/20 text-[#4cd964]' : 'bg-a-border/60 text-a-muted'}`}>{c.source}</span>}
                </div>
              )) || <div className="text-xs text-a-muted">检查 server 日志...</div>}
            </div>
          ) : (
            <div className="text-xs text-a-muted">点击"运行诊断"启动检查</div>
          )}

          {!loading && !result && (
            <div className="mt-3 text-[11px] text-a-muted">检查项包括：Provider 安装、配置合法性、运行时验证、端口监听</div>
          )}
        </div>
      </Card>
    </div>
  );
}

export function SmokePage() {
  const checks = [
    { name: 'Golden Path', source: 'real', status: 'pass' },
    { name: 'Provider Smoke', source: 'real', status: 'pass' },
    { name: 'Relay Resolve', source: 'real', status: 'pass' },
    { name: 'Relay Positive/Negative', source: 'unit', status: 'pass' },
    { name: 'Failure Matrix', source: 'fake_only', status: 'pass' },
  ];

  return (
    <div>
      <PageHeader title="Smoke" sub="冒烟测试状态" helpKey="smoke" />
      <Card>
        <table className="w-full text-sm border-collapse">
          <thead>
            <tr>
              {['Check', 'Source', 'Status'].map((h) => (
                <th key={h} className="text-left px-3.5 py-2.5 text-[11px] font-semibold uppercase tracking-[0.06em] text-a-muted bg-a-bg border-b border-a-border whitespace-nowrap">{h}</th>
              ))}
            </tr>
          </thead>
          <tbody>
            {checks.map((c, i) => (
              <tr key={i} className="hover:bg-white/[0.04]">
                <td className="px-3.5 py-2.5 border-b border-a-border-soft text-xs">{c.name}</td>
                <td className="px-3.5 py-2.5 border-b border-a-border-soft">
                  <StatusBadge status={c.source === 'real' ? 'verified' : c.source === 'unit' ? 'unit_verified' : 'fake_only'} />
                </td>
                <td className="px-3.5 py-2.5 border-b border-a-border-soft"><StatusBadge status={c.status === 'pass' ? 'ok' : 'error'} /></td>
              </tr>
            ))}
          </tbody>
        </table>
      </Card>
    </div>
  );
}

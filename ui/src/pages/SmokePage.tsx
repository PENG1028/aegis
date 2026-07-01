import { useState } from 'react';
import { system } from '@/lib/api-bridge';
import { PageHeader, Card, Btn, StatusBadge } from '@/components/shared';
import { useToast } from '@/components/shared/Toast';

export function SmokePage() {
  const toast = useToast();
  const [checks, setChecks] = useState<Array<{ name: string; source: string; status: string }>>([]);
  const [loading, setLoading] = useState(false);
  const [hasRun, setHasRun] = useState(false);

  async function doRun() {
    setLoading(true);
    try {
      const res = await system.doctor();
      setHasRun(true);
      const items = res.checks || [];
      if (items.length > 0) {
        setChecks(items.map((c: any) => ({
          name: c.name || c.check || '未知',
          source: c.source || 'real',
          status: c.status || 'unknown',
        })));
      } else {
        setChecks([
          { name: '系统诊断', source: 'real', status: res.message ? 'pass' : 'unknown' },
        ]);
      }
      toast('冒烟测试完成');
    } catch (e: any) {
      toast(e.message, 'error');
      setChecks([{ name: '诊断失败', source: 'real', status: 'error' }]);
    }
    setLoading(false);
  }

  return (
    <div>
      <PageHeader title="冒烟测试" sub="冒烟测试状态" helpKey="smoke" actions={
        <Btn primary onClick={doRun} disabled={loading}>{loading ? '运行中…' : '运行冒烟测试'}</Btn>
      } />
      <Card>
        {!hasRun ? (
          <div className="p-[18px] text-xs text-a-muted">点击"运行冒烟测试"启动诊断检查</div>
        ) : (
          <table className="w-full text-sm border-collapse">
            <thead>
              <tr>
                {['检查项', '来源', '状态'].map((h) => (
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
                  <td className="px-3.5 py-2.5 border-b border-a-border-soft"><StatusBadge status={c.status === 'pass' ? 'ok' : c.status === 'error' ? 'error' : 'warn'} /></td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </Card>
    </div>
  );
}

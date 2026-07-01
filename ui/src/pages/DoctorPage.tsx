import { useState } from 'react';
import { system } from '@/lib/api-bridge';
import { PageHeader, Card, Btn, StatusBadge } from '@/components/shared';
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
      <PageHeader title="诊断工具" sub="系统诊断与一致性检查" actions={
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

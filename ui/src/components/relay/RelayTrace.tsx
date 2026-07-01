import { useState } from 'react';
import { traceApi } from '@/lib/api-bridge';
import { Btn, Alert } from '@/components/shared';

export default function RelayTrace() {
  const [domain, setDomain] = useState('relay.example.com');
  const [result, setResult] = useState<any>(null);
  const [error, setError] = useState<string | null>(null);

  async function doTrace() {
    setError(null);
    setResult(null);
    try {
      const r = await traceApi.byDomain(domain.trim());
      setResult(r);
    } catch (e: any) {
      setError(e.message);
    }
  }

  return (
    <>
      <div className="flex gap-2 mb-4">
        <input className="flex-1 font-mono text-sm px-3 py-2 rounded-a-sm border border-a-border bg-a-bg text-a-fg outline-none focus:border-a-accent"
          value={domain} onChange={(e) => setDomain(e.target.value)} placeholder="域名" />
        <Btn primary onClick={doTrace}>跟踪中继</Btn>
      </div>
      {error && <Alert type="err">{error}</Alert>}
      {result?.steps?.map((s: any, i: number) => (
        <div key={i} className="flex items-start gap-3 py-2 border-b border-a-border-soft text-xs">
          <div className={`w-2 h-2 rounded-full mt-1.5 shrink-0 ${s.status === 'matched' ? 'bg-[#4cd964]' : 'bg-[#ff5c72]'}`} />
          <div>
            <div className="font-medium">{s.name || s.component}</div>
            <div className="text-a-muted text-[11px]">{s.detail}</div>
          </div>
        </div>
      )) || <div className="text-center py-10 text-a-muted text-xs">输入域名点击 Trace</div>}
    </>
  );
}

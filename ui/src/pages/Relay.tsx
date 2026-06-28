import { useState } from 'react';
import { relayApi, traceApi } from '@/lib/api-bridge';
import { PageHeader, Card, TabBar, Btn, Alert, StatusBadge } from '@/components/shared';

export default function RelayPage() {
  const [tab, setTab] = useState('resolve');

  return (
    <div>
      <PageHeader title="受管中继" helpKey="relay" sub="中继路径解析与运行时" />
      <TabBar
        tabs={[
          { key: 'resolve', label: '解析' },
          { key: 'trace', label: '跟踪' },
        ]}
        active={tab}
        onChange={setTab}
      />
      {tab === 'resolve' && <RelayResolve />}
      {tab === 'trace' && <RelayTrace />}
    </div>
  );
}

function RelayResolve() {
  const [domain, setDomain] = useState('relay.example.com');
  const [fromNode, setFromNode] = useState('node-a');
  const [result, setResult] = useState<any>(null);
  const [error, setError] = useState<string | null>(null);

  async function doResolve() {
    setError(null);
    setResult(null);
    try {
      const r = await relayApi.resolve(domain.trim(), fromNode.trim());
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
        <input className="w-32 font-mono text-sm px-3 py-2 rounded-a-sm border border-a-border bg-a-bg text-a-fg outline-none focus:border-a-accent"
          value={fromNode} onChange={(e) => setFromNode(e.target.value)} placeholder="源节点" />
        <Btn primary onClick={doResolve}>解析</Btn>
      </div>

      {error && <Alert type="err">{error}</Alert>}

      {result && (
        <Card title={result.domain}>
          <div className="p-[18px] grid grid-cols-2 gap-3 text-xs">
            <div><span className="text-a-muted">模式:</span> <StatusBadge status={result.mode} /></div>
            <div><span className="text-a-muted">目标节点:</span> <span className="font-mono">{result.target_node_id || '—'}</span></div>
            <div><span className="text-a-muted">网关 URL:</span> <span className="font-mono text-[11px] break-all">{result.gateway_url || '—'}</span></div>
            <div><span className="text-a-muted">网关链接:</span> <span className="font-mono">{result.gateway_link_id || '—'}</span></div>
            {result.direct_target_suppressed && (
              <div className="col-span-2"><Alert type="warn">远端真实端口已被隐藏</Alert></div>
            )}
            {result.final_local_target && (
              <div className="col-span-2">
                <span className="text-a-muted">最终目标:</span>
                <span className="font-mono text-a-accent ml-1">{result.final_local_target}</span>
              </div>
            )}
            {result.recommendation && (
              <div className="col-span-2">
                <span className="text-a-muted">建议:</span>
                <div className="text-a-accent mt-0.5">{result.recommendation}</div>
              </div>
            )}
          </div>
        </Card>
      )}
    </>
  );
}

function RelayTrace() {
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

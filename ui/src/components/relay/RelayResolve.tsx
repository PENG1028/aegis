import { useState } from 'react';
import { relayApi } from '@/lib/api-bridge';
import { Card, Btn, Alert, StatusBadge } from '@/components/shared';

export default function RelayResolve() {
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

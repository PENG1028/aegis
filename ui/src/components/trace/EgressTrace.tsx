import { useState } from 'react';
import { traceApi } from '@/lib/api-bridge';
import { Card, Btn, Alert } from '@/components/shared';

export default function EgressTrace() {
  const [domain, setDomain] = useState('api.example.com');
  const [fromNode, setFromNode] = useState('node-a');
  const [result, setResult] = useState<any>(null);
  const [error, setError] = useState<string | null>(null);

  async function doEgress() {
    setError(null);
    setResult(null);
    try {
      const r = await traceApi.egress(domain.trim(), fromNode.trim());
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
        <Btn primary onClick={doEgress}>跟踪</Btn>
      </div>

      {error && <Alert type="err">{error}</Alert>}

      {result && (
        <Card title={`Egress: ${result.domain}`}>
          <div className="p-[18px] grid grid-cols-2 gap-3 text-xs">
            <div><span className="text-a-muted">解析 IP:</span> <span className="font-mono">{(result.resolved_ips || []).join(', ')}</span></div>
            <div><span className="text-a-muted">分类:</span> {result.ip_classification || '—'}</div>
            <div><span className="text-a-muted">Aegis 管理:</span> {result.is_aegis_managed_domain ? '✓' : '✗'}</div>
            <div><span className="text-a-muted">网关链接:</span> {result.gateway_link_id || '—'}</div>
            {result.internal_target_available && (
              <div className="col-span-2"><Alert type="warn">目标指向本机内网地址；考虑加 GatewayLink 约束。</Alert></div>
            )}
            {result.recommendation && (
              <div className="col-span-2">
                <span className="text-a-muted">建议:</span>
                <div className="text-a-accent mt-0.5 text-[11px]">{result.recommendation}</div>
              </div>
            )}
          </div>
        </Card>
      )}
    </>
  );
}

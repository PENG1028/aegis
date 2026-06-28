import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { traceApi } from '@/lib/api-bridge';
import { PageHeader, Card, TabBar, Btn, Alert, PathChain, StatusBadge } from '@/components/shared';

export default function TracePage() {
  const [tab, setTab] = useState('ingress');
  const [domain, setDomain] = useState('policy.example.com');
  const [traceResult, setTraceResult] = useState<any>(null);
  const [error, setError] = useState<string | null>(null);

  async function doTrace() {
    setError(null);
    setTraceResult(null);
    try {
      const r = await traceApi.byDomain(domain.trim());
      setTraceResult(r);
    } catch (e: any) {
      setError(e.message);
    }
  }

  const steps = traceResult?.steps?.map((s: any) => ({
    label: s.name || s.component,
    tooltip: s.detail,
    color: s.status === 'matched' ? 'accent' as const : s.status === 'error' ? 'danger' as const : 'default' as const,
  })) || [];

  return (
    <div>
      <PageHeader title="Trace" helpKey="trace" sub="跟踪请求路径 — domain / SNI / route" />

      <TabBar
        tabs={[
          { key: 'ingress', label: 'Ingress Trace' },
          { key: 'egress', label: 'Egress Trace' },
        ]}
        active={tab}
        onChange={setTab}
      />

      {tab === 'ingress' && (
        <>
          <div className="flex gap-2 mb-4">
            <input
              className="flex-1 font-mono text-sm px-3 py-2 rounded-a-sm border border-a-border bg-a-bg text-a-fg outline-none focus:border-a-accent"
              value={domain}
              onChange={(e) => setDomain(e.target.value)}
              placeholder="输入域名..."
              onKeyDown={(e) => e.key === 'Enter' && doTrace()}
            />
            <Btn primary onClick={doTrace}>Trace</Btn>
          </div>

          {error && <Alert type="err">{error}</Alert>}

          {traceResult && (
            <>
              <Card title={`Trace: ${traceResult.input}`} className="mb-4">
                <div className="p-[18px]">
                  <div className="grid grid-cols-2 gap-3 text-xs mb-4">
                    <div><span className="text-a-muted">Input Type:</span> {traceResult.input_type}</div>
                    <div><span className="text-a-muted">Status:</span> <StatusBadge status={traceResult.trace_status} /></div>
                  </div>

                  {steps.length > 0 && <PathChain steps={steps} className="mb-4" />}

                  {traceResult.steps?.map((s: any, i: number) => (
                    <div key={i} className="flex items-start gap-3 py-2 border-b border-a-border-soft last:border-b-0">
                      <div className={`w-2 h-2 rounded-full mt-1.5 shrink-0 ${
                        s.status === 'matched' ? 'bg-[#4cd964]'
                        : s.status === 'error' ? 'bg-[#ff5c72]'
                        : 'bg-a-border'
                      }`} />
                      <div>
                        <div className="text-xs font-medium">{s.name || s.component}</div>
                        <div className="text-[11px] text-a-muted">{s.detail}</div>
                        {s.address && <div className="text-[11px] font-mono text-a-accent">{s.address}</div>}
                      </div>
                    </div>
                  ))}
                </div>
              </Card>

              {traceResult.final_target && (
                <Card title="Final Target">
                  <div className="p-[18px] grid grid-cols-2 gap-3 text-xs">
                    <div><span className="text-a-muted">Host:</span> <span className="font-mono">{traceResult.final_target.host}</span></div>
                    <div><span className="text-a-muted">Port:</span> <span className="font-mono">{traceResult.final_target.port}</span></div>
                    <div><span className="text-a-muted">Protocol:</span> {traceResult.final_target.protocol}</div>
                    <div><span className="text-a-muted">Reachable:</span> {traceResult.final_target.reachable ? '✓' : '—'}</div>
                  </div>
                </Card>
              )}
            </>
          )}
        </>
      )}

      {tab === 'egress' && (
        <EgressTrace />
      )}
    </div>
  );
}

function EgressTrace() {
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
        <Btn primary onClick={doEgress}>Trace</Btn>
      </div>

      {error && <Alert type="err">{error}</Alert>}

      {result && (
        <Card title={`Egress: ${result.domain}`}>
          <div className="p-[18px] grid grid-cols-2 gap-3 text-xs">
            <div><span className="text-a-muted">Resolved IPs:</span> <span className="font-mono">{(result.resolved_ips || []).join(', ')}</span></div>
            <div><span className="text-a-muted">Classified:</span> {result.ip_classification || '—'}</div>
            <div><span className="text-a-muted">Aegis Managed:</span> {result.is_aegis_managed_domain ? '✓' : '✗'}</div>
            <div><span className="text-a-muted">Gateway Link:</span> {result.gateway_link_id || '—'}</div>
            {result.internal_target_available && (
              <div className="col-span-2"><Alert type="warn">目标指向本机内网地址；考虑加 GatewayLink 约束。</Alert></div>
            )}
            {result.recommendation && (
              <div className="col-span-2">
                <span className="text-a-muted">Recommendation:</span>
                <div className="text-a-accent mt-0.5 text-[11px]">{result.recommendation}</div>
              </div>
            )}
          </div>
        </Card>
      )}
    </>
  );
}

import { useQuery } from '@tanstack/react-query';
import { fetchAcceptance } from '@/lib/api-bridge';
import {
  PageHeader, Card, StatusBadge, StatCard, Alert,
} from '@/components/shared';
import { fmtFull } from '@/lib/utils';

export default function AcceptancePage() {
  const { data, isLoading, error } = useQuery({
    queryKey: ['acceptance'],
    queryFn: fetchAcceptance,
  });

  if (isLoading) return <div className="text-center py-10 text-a-muted font-mono text-sm">加载中...</div>;
  if (error) return <div className="px-4 py-3 rounded-a-md text-xs border bg-[#ff5c72]/10 text-[#ff5c72] border-[#ff5c72]/20">加载失败: {error.message}</div>;
  if (!data) return null;

  const { labels, summary, last_acceptance: last, negative_smoke: smoke } = data;

  return (
    <div>
      <PageHeader title="验收 / 验证状态" helpKey="acceptance" subtitle={`${summary.pass_count} pass · ${summary.pending_count} pending · ${summary.deferred_count} deferred`}  />

      <Alert type="info">
        <div>
          <p className="font-medium">展示 Aegis v1.8C 真实验收结果。</p>
          <p className="text-a-fg2 text-[11px] mt-0.5">
            注意：real_secret_runtime_code_verified ≠ real_secret_runtime_deploy_verified
            · real_two_node_verified ≠ real_three_node_verified
            · local gateway verified ≠ system-wide transparent proxy verified
          </p>
        </div>
      </Alert>

      {/* Summary stats */}
      <div className="grid grid-cols-4 gap-3 mb-5">
        <StatCard label="All Labels" value={summary.total_labels} accent />
        <StatCard label="Pass" value={summary.pass_count} success />
        <StatCard label="Pending" value={summary.pending_count} warn />
        <StatCard label="Deferred" value={summary.deferred_count} />
      </div>

      {/* Verification Labels */}
      <Card title="Verification Labels" subtitle="各能力维度的验证状态" className="mb-4">
        <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
          {labels.map((l) => (
            <div key={l.key} className="bg-a-bg border border-a-border rounded-a-sm p-3.5 flex items-start gap-3">
              <div className="shrink-0 mt-0.5">
                {l.status === 'pass' ? (
                  <span className="w-5 h-5 rounded-full bg-[#4cd964]/20 flex items-center justify-center text-[#4cd964] text-xs">✓</span>
                ) : l.status === 'pending' ? (
                  <span className="w-5 h-5 rounded-full bg-[#e8b830]/20 flex items-center justify-center text-[#e8b830] text-xs">○</span>
                ) : (
                  <span className="w-5 h-5 rounded-full bg-a-border/40 flex items-center justify-center text-a-muted text-xs">—</span>
                )}
              </div>
              <div className="flex-1 min-w-0">
                <div className="text-xs font-semibold text-a-fg">{l.label}</div>
                <div className="font-mono text-[10px] text-a-muted mt-0.5">{l.key}</div>
                <div className="text-[11px] text-a-fg2 mt-1">{l.evidence}</div>
              </div>
              <StatusBadge status={l.status} />
            </div>
          ))}
        </div>
      </Card>

      {/* Last Acceptance */}
      {last && (
        <Card title="最近一次验收" className="mb-4">
          <div className="grid grid-cols-2 gap-3">
            <div>
              <div className="text-[11px] text-a-muted">Command</div>
              <code className="block bg-a-bg border border-a-border rounded-a-sm p-2 mt-1 font-mono text-[11px] text-a-accent">{last.command}</code>
            </div>
            <div>
              <div className="text-[11px] text-a-muted">HTTP Status</div>
              <div className="text-2xl font-mono font-bold text-[#4cd964]">{last.http_status}</div>
            </div>
            <div>
              <div className="text-[11px] text-a-muted">Response</div>
              <code className="block bg-a-bg border border-a-border rounded-a-sm p-2 mt-1 font-mono text-[10px] text-a-fg2">{last.response_summary}</code>
            </div>
            <div>
              <div className="text-[11px] text-a-muted">Candidate</div>
              <div className="text-xs mt-1">{last.selected_candidate}</div>
              <div className="text-[11px] text-a-muted mt-2">GatewayLink</div>
              <div className="font-mono text-xs text-a-accent">{last.gateway_link_id}</div>
            </div>
            <div>
              <div className="text-[11px] text-a-muted">Token Leak Scan</div>
              <StatusBadge status={last.token_leak_scan === 'clean' ? 'pass' : 'fail'} />
            </div>
            <div>
              <div className="text-[11px] text-a-muted">Negative Smoke</div>
              <StatusBadge status={last.negative_smoke_result === 'pass' ? 'pass' : last.negative_smoke_result === 'partial' ? 'warning' : 'fail'} />
            </div>
            <div className="col-span-2">
              <div className="text-[11px] text-a-muted">执行时间</div>
              <div className="text-xs mt-0.5 font-mono">{fmtFull(last.executed_at)}</div>
            </div>
            <div className="col-span-2">
              <div className="text-[11px] text-a-muted">文档</div>
              <div className="text-xs mt-0.5 font-mono text-a-accent">{last.docs_link}</div>
            </div>
          </div>
        </Card>
      )}

      {/* Negative Smoke */}
      <Card title="Negative Security Smoke" subtitle="安全边界测试结果">
        <div className="space-y-2">
          {smoke.map((t) => (
            <div key={t.id} className="flex items-start gap-3 py-2 border-b border-a-border-soft last:border-b-0 text-xs">
              <div className="w-6 shrink-0 font-mono text-a-muted">{t.id}</div>
              <div className="flex-1 min-w-0">
                <div className="font-medium">{t.desc}</div>
                <div className="text-a-muted mt-0.5">
                  Expected: <span className="font-mono">{t.expected}</span>
                  {' · '}Actual: <span className="font-mono">{t.actual}</span>
                </div>
              </div>
              <StatusBadge status={t.status} />
            </div>
          ))}
        </div>
      </Card>
    </div>
  );
}

import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { fetchRoutingTable, validateRouting, previewRouting, nodeApi } from '@/lib/api-bridge';
import { PageHeader, Card, DataTable, StatusBadge, Alert, WarningCard, StatCard, Btn } from '@/components/shared';
import { useToast } from '@/components/shared/Toast';
import type { DataTableColumn } from '@/components/shared';
import type { RoutingEntry } from '@/types';

export default function RoutingTablePage() {
  const [tab, setTab] = useState<'table' | 'preview' | 'validate'>('table');
  const [previewDomain, setPreviewDomain] = useState('api-b.example.com');
  const [previewNode, setPreviewNode] = useState('node-a');
  const [previewResult, setPreviewResult] = useState<any>(null);
  const [generating, setGenerating] = useState(false);
  const toast = useToast();
  const qc = useQueryClient();

  const { data: entries, isLoading, error } = useQuery({
    queryKey: ['routing-table'],
    queryFn: () => fetchRoutingTable(),
  });

  const handleGenerate = async (nodeId: string) => {
    setGenerating(true);
    try {
      await nodeApi.generateRoutingTable(nodeId);
      toast(`路由表已重新生成 (${nodeId})`);
      qc.invalidateQueries({ queryKey: ['routing-table'] });
    } catch (e: any) { toast(e.message, 'error'); }
    setGenerating(false);
  };

  const { data: validation } = useQuery({
    queryKey: ['routing-validate'],
    queryFn: () => validateRouting(),
  });

  const columns: DataTableColumn<RoutingEntry>[] = [
    { key: 'domain', label: '域名', mono: true },
    { key: 'route_id', label: '路由', mono: true, muted: true },
    { key: 'from_node_id', label: '来源', mono: true },
    { key: 'target_node_id', label: '目标节点', mono: true },
    { key: 'policy_mode', label: '策略', render: (r) => <StatusBadge status={r.policy_mode} /> },
    {
      key: 'candidates',
      label: '候选',
      render: (r) => (
        <div className="flex gap-1 flex-wrap">
          {r.candidates.map((c, i) => (
            <span key={i} className={`font-mono text-[10px] px-1.5 py-0.5 rounded ${
              c.mode === 'local_gateway' ? 'bg-a-accent/15 text-a-accent' :
              c.mode === 'private_gateway' ? 'bg-[#4cd964]/15 text-[#4cd964]' :
              'bg-[#e8b830]/15 text-[#e8b830]'
            }`}>{c.mode}</span>
          ))}
          {r.candidates.length === 0 && <span className="text-a-muted text-[11px]">无</span>}
        </div>
      ),
    },
    {
      key: 'status',
      label: '状态',
      render: (r) => <StatusBadge status={r.status} />,
    },
    {
      key: 'unavailable_reason',
      label: '原因',
      muted: true,
      render: (r) => r.unavailable_reason ? <span className="text-a-danger text-[11px]">{r.unavailable_reason}</span> : '—',
    },
  ];

  const availableCount = entries?.filter((e) => e.status === 'available').length || 0;

  if (isLoading) return <div className="text-center py-10 text-a-muted font-mono text-sm">加载中...</div>;
  if (error) return <div className="px-4 py-3 rounded-a-md text-xs border bg-[#ff5c72]/10 text-[#ff5c72] border-[#ff5c72]/20">加载失败: {error.message}</div>;

  return (
    <div>
      <PageHeader title="路由表" helpKey="routing" subtitle="按 Node / Domain 查看路由条目" actions={
        <div className="flex gap-2">
          <Btn sm onClick={() => handleGenerate('node-a')} disabled={generating}>重新生成 (A)</Btn>
          <Btn sm onClick={() => handleGenerate('node-b')} disabled={generating}>重新生成 (B)</Btn>
        </div>
      } />

      <div className="grid grid-cols-4 gap-3 mb-5">
        <StatCard label="总条目" value={entries?.length || 0} accent />
        <StatCard label="可用" value={availableCount} success />
        <StatCard label="不可用" value={(entries?.length || 0) - availableCount} danger={(entries?.length || 0) > availableCount} />
        <StatCard label="验证错误" value={validation?.error_count || 0} danger={(validation?.error_count || 0) > 0} />
      </div>

      {/* Tabs */}
      <div className="flex gap-0 mb-4 border-b border-a-border">
        {[
          { key: 'table', label: '路由表' },
          { key: 'preview', label: '预览' },
          { key: 'validate', label: '验证' },
        ].map((t: { key: 'table' | 'preview' | 'validate'; label: string }) => (
          <button key={t.key}
            className={`px-4 py-2 text-xs font-medium border-b-2 transition-all whitespace-nowrap bg-transparent cursor-pointer ${
              tab === t.key ? 'border-a-accent text-a-accent' : 'border-transparent text-a-muted hover:text-a-fg'
            }`}
            onClick={() => setTab(t.key)}>
            {t.label}
          </button>
        ))}
      </div>

      {tab === 'table' && (
        <Card>
          <DataTable columns={columns} data={entries || []} keyExtractor={(r) => `${r.domain}-${r.from_node_id}`} />
        </Card>
      )}

      {tab === 'preview' && (
        <div>
          <div className="flex gap-2 mb-4">
            <input className="flex-1 font-mono text-sm px-3 py-2 rounded-a-sm border border-a-border bg-a-bg text-a-fg outline-none focus:border-a-accent"
              value={previewDomain} onChange={(e) => setPreviewDomain(e.target.value)} placeholder="domain" />
            <select className="font-mono text-xs px-3 py-2 rounded-a-sm border border-a-border bg-a-bg text-a-fg outline-none"
              value={previewNode} onChange={(e) => setPreviewNode(e.target.value)}>
              <option value="node-a">Server A</option>
              <option value="node-b">Server B</option>
            </select>
            <button
              className="inline-flex items-center gap-1 text-xs px-3 py-2 rounded-a-md bg-a-accent text-white hover:opacity-90 cursor-pointer border-none font-medium"
              onClick={async () => {
                try {
                  const r = await previewRouting(previewDomain, previewNode);
                  setPreviewResult(r);
                } catch (e) {
                  setPreviewResult({ available: false, domain: previewDomain, entries: [], summary: `查询失败: ${e}`, unavailable_reason: String(e) });
                }
              }}>
              预览
            </button>
          </div>
          {previewResult && (
            <Card title={`Preview: ${previewResult.domain}`}>
              <div className="space-y-3">
                <div className="flex items-center gap-2">
                  <StatusBadge status={previewResult.available ? 'available' : 'unavailable'} />
                  <span className="text-xs text-a-fg2">{previewResult.summary}</span>
                </div>
                {previewResult.entries.map((entry: any) => (
                  <div key={entry.route_id} className="bg-a-bg border border-a-border rounded-a-sm p-3">
                    <div className="flex items-center gap-2 mb-2">
                      <span className="font-mono text-xs font-semibold">{entry.domain}</span>
                      <StatusBadge status={entry.status} />
                    </div>
                    <div className="text-[11px] text-a-muted space-y-1">
                      <div>路由: {entry.route_id} → Endpoint: {entry.endpoint_id}</div>
                      <div>目标: {entry.target_local_host}:{entry.target_local_port}</div>
                      <div>策略模式: {entry.policy_mode}</div>
                    </div>
                    {entry.candidates.length > 0 && (
                      <div className="mt-2 space-y-1">
                        <div className="text-[11px] font-semibold text-a-fg">候选:</div>
                        {entry.candidates.map((c: any, i: number) => (
                          <div key={i} className="flex items-center gap-2 text-[11px] font-mono text-a-muted pl-3">
                            <StatusBadge status={c.mode} />
                            <span>{c.gateway_url || '—'}</span>
                            {c.requires_gateway_link && <span className={c.gateway_link_id ? 'text-a-accent' : 'text-a-danger'}>{c.gateway_link_id || 'missing link'}</span>}
                            <span className="ml-auto">优先级: {c.priority}</span>
                          </div>
                        ))}
                      </div>
                    )}
                    {entry.unavailable_reason && (
                      <WarningCard title="Unavailable" type="err" className="mt-2">
                        <p>{entry.unavailable_reason}</p>
                      </WarningCard>
                    )}
                  </div>
                ))}
              </div>
            </Card>
          )}
        </div>
      )}

      {tab === 'validate' && (
        <div>
          {validation && (
            <div className="space-y-4">
              <Card title="验证结果">
                <div className="flex items-center gap-2 mb-3">
                  <StatusBadge status={validation.valid ? 'pass' : 'fail'} />
                  <span className="text-xs text-a-fg2">{validation.total_entries} entries, {validation.valid_count} valid</span>
                </div>
              </Card>

              {validation.errors.length > 0 && (
                <Card title={`错误 (${validation.error_count})`}>
                  {validation.errors.map((e, i) => (
                    <div key={i} className="flex items-start gap-2 py-1.5 border-b border-a-border-soft last:border-b-0 text-xs">
                      <span className="text-a-danger shrink-0">✗</span>
                      <span className="font-mono text-a-accent">{e.domain}</span>
                      <span className="text-a-muted">{e.code}</span>
                      <span className="text-a-fg2">{e.message}</span>
                    </div>
                  ))}
                </Card>
              )}

              {validation.warnings.length > 0 && (
                <Card title={`警告 (${validation.warning_count})`}>
                  {validation.warnings.map((w, i) => (
                    <div key={i} className="flex items-start gap-2 py-1.5 border-b border-a-border-soft last:border-b-0 text-xs">
                      <span className="text-[#e8b830] shrink-0">⚠</span>
                      <span className="font-mono text-a-accent">{w.domain}</span>
                      <span className="text-a-muted">{w.code}</span>
                      <span className="text-a-fg2">{w.message}</span>
                    </div>
                  ))}
                </Card>
              )}

              {validation.errors.length === 0 && validation.warnings.length === 0 && (
                <Alert type="success">✓ 所有条目有效</Alert>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

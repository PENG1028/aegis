import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate, useParams } from 'react-router-dom';
import { adminApi, system } from '@/lib/api-bridge';
import { PageHeader, Card, StatCard, Btn, Alert, TabBar, MetaRow, StatusBadge } from '@/components/shared';
import { useToast } from '@/components/shared/Toast';
import { fmtDate } from '@/lib/utils';

export function ApplyConfigPage() {
  const toast = useToast();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const [tab, setTab] = useState('status');

  const { data: status } = useQuery({
    queryKey: ['system-status'],
    queryFn: () => system.status(),
  });

  const { data: history } = useQuery({
    queryKey: ['apply-history'],
    queryFn: () => adminApi.applyHistory(),
  });

  const { data: preview } = useQuery({
    queryKey: ['config-preview'],
    queryFn: () => adminApi.configPreview(),
  });

  const pending = status?.pending_apply?.pending;

  async function doApply() {
    if (!confirm('确定执行 Apply？')) return;
    try {
      const res = await adminApi.applyConfig();
      toast(res.message || 'Apply 完成');
      queryClient.invalidateQueries({ queryKey: ['config-preview'] });
      queryClient.invalidateQueries({ queryKey: ['system-status'] });
    } catch (e: any) { toast(e.message, 'error'); }
  }

  return (
    <div>
      <PageHeader title="Apply" helpKey="apply" sub="配置部署与渲染" actions={
        <Btn primary onClick={doApply} disabled={!pending}>Apply</Btn>
      } />

      {pending && <Alert type="warn">有待处理变更。执行 Apply 以应用。</Alert>}

      {status && (
        <div className="grid grid-cols-4 gap-3 mb-4">
          <StatCard label="Services" value={status.counts.services} accent />
          <StatCard label="Routes" value={status.counts.routes} success />
          <StatCard label="Managed Domains" value={status.counts.managed_domains} />
          <StatCard label="Pending" value={pending ? '是' : '否'} warn={!!pending} />
        </div>
      )}

      <TabBar
        tabs={[
          { key: 'status', label: 'Status' },
          { key: 'preview', label: 'Preview' },
          { key: 'history', label: 'History' },
        ]}
        active={tab}
        onChange={setTab}
      />

      {tab === 'status' && status && (
        <Card title="System Status">
          <div className="p-[18px] grid grid-cols-2 gap-3">
            <MetaRow label="Name" value={status.name} mono />
            <MetaRow label="Version" value={status.version} mono />
            <MetaRow label="Provider" value={status.proxy?.provider || '—'} />
            <MetaRow label="Config Path" value={status.proxy?.config_path || '—'} mono />
            <MetaRow label="Validate Available" value={status.proxy?.validate_available ? '✓' : '✗'} />
            <MetaRow label="Reload Command" value={status.proxy?.reload_command_configured ? '✓' : '✗'} />
            {status.last_apply && (
              <MetaRow label="Last Apply" value={`${status.last_apply.status} @ ${status.last_apply.version}`} />
            )}
          </div>
        </Card>
      )}

      {tab === 'preview' && preview && (
        <Card title="Rendered Config Preview">
          <div className="p-[18px]">
            <div className="text-xs text-a-muted mb-2">{preview.route_count} routes, {preview.managed_domain_count} managed domains</div>
            <pre className="bg-a-bg border border-a-border rounded-a-sm p-3 text-xs font-mono text-a-muted overflow-x-auto max-h-[500px] whitespace-pre-wrap">
              {preview.rendered_config || 'No config rendered'}
            </pre>
          </div>
        </Card>
      )}

      {tab === 'history' && (
        <Card title="Apply History">
          <table className="w-full text-sm border-collapse">
            <thead>
              <tr>
                {['Version', 'Status', 'Created'].map((h) => (
                  <th key={h} className="text-left px-3.5 py-2.5 text-[11px] font-semibold uppercase tracking-[0.06em] text-a-muted bg-a-bg border-b border-a-border whitespace-nowrap">{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {(Array.isArray(history) ? history : []).map((h: any) => (
                <tr key={h.id || h.version} className="hover:bg-white/[0.04]">
                  <td className="px-3.5 py-2.5 border-b border-a-border-soft font-mono text-xs">{h.version}</td>
                  <td className="px-3.5 py-2.5 border-b border-a-border-soft"><StatusBadge status={h.status} /></td>
                  <td className="px-3.5 py-2.5 border-b border-a-border-soft font-mono text-xs">{fmtDate(h.created_at)}</td>
                </tr>
              ))}
              {(!history || (Array.isArray(history) && history.length === 0)) && (
                <tr><td colSpan={3} className="text-center py-10 text-a-muted text-xs">暂无 Apply 历史</td></tr>
              )}
            </tbody>
          </table>
        </Card>
      )}
    </div>
  );
}

export function ConfigPage() {
  const navigate = useNavigate();
  const [tab, setTab] = useState('current');

  const { data: status } = useQuery({
    queryKey: ['system-status'],
    queryFn: () => system.status(),
  });

  const { data: current } = useQuery({
    queryKey: ['config-current'],
    queryFn: () => adminApi.configCurrent(),
  });

  const { data: preview } = useQuery({
    queryKey: ['config-preview'],
    queryFn: () => adminApi.configPreview(),
  });

  const data = tab === 'current' ? current : preview;

  return (
    <div>
      <PageHeader title="Config Preview / Diff" helpKey="config" sub="配置证据页面，非编辑器" helpKey="doctor" />
      <div className="grid grid-cols-4 gap-3 mb-4">
        <div><div className="text-[11px] text-a-muted uppercase tracking-[0.06em]">Provider</div><div className="font-mono text-sm mt-0.5">{status?.proxy?.provider || '—'}</div></div>
        <div><div className="text-[11px] text-a-muted uppercase tracking-[0.06em]">Config Path</div><div className="font-mono text-sm mt-0.5">{status?.proxy?.config_path || '—'}</div></div>
        <div><div className="text-[11px] text-a-muted uppercase tracking-[0.06em]">Schema</div><div className="font-mono text-sm mt-0.5">{status?.store?.schema_version || '—'}</div></div>
        <div><div className="text-[11px] text-a-muted uppercase tracking-[0.06em]">Routes</div><div className="font-mono text-sm mt-0.5">{status?.counts?.routes || '—'}</div></div>
      </div>
      <TabBar
        tabs={[
          { key: 'current', label: 'Current Config' },
          { key: 'preview', label: 'Preview' },
        ]}
        active={tab}
        onChange={setTab}
      />
      <Card>
        <div className="p-[18px]">
          <pre className="bg-a-bg border border-a-border rounded-a-sm p-3 font-mono text-xs text-a-muted overflow-x-auto max-h-[600px] whitespace-pre-wrap">
            {typeof data === 'object' && data !== null
              ? JSON.stringify(data, null, 2)
              : String(data || 'No config data')}
          </pre>
        </div>
      </Card>
    </div>
  );
}

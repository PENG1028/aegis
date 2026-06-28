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
      <PageHeader title="推送配置" helpKey="apply" sub="配置部署与渲染" actions={
        <Btn primary onClick={doApply} disabled={!pending}>Apply</Btn>
      } />

      {pending && <Alert type="warn">有待处理变更。执行 Apply 以应用。</Alert>}

      {status && (
        <div className="grid grid-cols-4 gap-3 mb-4">
          <StatCard label="服务" value={status.counts.services} accent />
          <StatCard label="路由" value={status.counts.routes} success />
          <StatCard label="管理域名" value={status.counts.managed_domains} />
          <StatCard label="待处理" value={pending ? '是' : '否'} warn={!!pending} />
        </div>
      )}

      <TabBar
        tabs={[
          { key: 'status', label: '状态' },
          { key: 'preview', label: '预览' },
          { key: 'history', label: '历史' },
        ]}
        active={tab}
        onChange={setTab}
      />

      {tab === 'status' && status && (
        <Card title="系统状态">
          <div className="p-[18px] grid grid-cols-2 gap-3">
            <MetaRow label="名称" value={status.name} mono />
            <MetaRow label="版本" value={status.version} mono />
            <MetaRow label="提供商" value={status.proxy?.provider || '—'} />
            <MetaRow label="配置路径" value={status.proxy?.config_path || '—'} mono />
            <MetaRow label="验证可用" value={status.proxy?.validate_available ? '✓' : '✗'} />
            <MetaRow label="重载命令" value={status.proxy?.reload_command_configured ? '✓' : '✗'} />
            {status.last_apply && (
              <MetaRow label="上次推送" value={`${status.last_apply.status} @ ${status.last_apply.version}`} />
            )}
          </div>
        </Card>
      )}

      {tab === 'preview' && preview && (
        <Card title="渲染配置预览">
          <div className="p-[18px]">
            <div className="text-xs text-a-muted mb-2">{preview.route_count} routes, {preview.managed_domain_count} managed domains</div>
            <pre className="bg-a-bg border border-a-border rounded-a-sm p-3 text-xs font-mono text-a-muted overflow-x-auto max-h-[500px] whitespace-pre-wrap">
              {preview.rendered_config || 'No config rendered'}
            </pre>
          </div>
        </Card>
      )}

      {tab === 'history' && (
        <Card title="推送历史">
          <table className="w-full text-sm border-collapse">
            <thead>
              <tr>
                {['版本', '状态', '创建时间'].map((h) => (
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
      <PageHeader title="配置预览 / 对比" helpKey="config" sub="配置证据页面，非编辑器" />
      <div className="grid grid-cols-4 gap-3 mb-4">
        <div><div className="text-[11px] text-a-muted uppercase tracking-[0.06em]">提供者</div><div className="font-mono text-sm mt-0.5">{status?.proxy?.provider || '—'}</div></div>
        <div><div className="text-[11px] text-a-muted uppercase tracking-[0.06em]">配置路径</div><div className="font-mono text-sm mt-0.5">{status?.proxy?.config_path || '—'}</div></div>
        <div><div className="text-[11px] text-a-muted uppercase tracking-[0.06em]">模式版本</div><div className="font-mono text-sm mt-0.5">{status?.store?.schema_version || '—'}</div></div>
        <div><div className="text-[11px] text-a-muted uppercase tracking-[0.06em]">路由数</div><div className="font-mono text-sm mt-0.5">{status?.counts?.routes || '—'}</div></div>
      </div>
      <TabBar
        tabs={[
          { key: 'current', label: '当前配置' },
          { key: 'preview', label: '预览' },
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

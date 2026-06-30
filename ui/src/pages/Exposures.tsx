/**
 * Exposures — 端口暴露管理 (v1.8K)
 *
 * 管理 TCP/UDP 端口暴露:
 *   创建 → 激活 (启动端口转发) → 禁用 → 重新激活
 */

import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { exposureApi } from '@/lib/api-bridge';
import { PageHeader, Card, Btn } from '@/components/shared';
import { useToast } from '@/components/shared/Toast';

// ─── Type helpers ───

const TYPE_LABELS: Record<string, string> = { tcp: 'TCP', udp: 'UDP', http: 'HTTP', tunnel: 'Tunnel', internal: '内部' };
const TYPE_COLORS: Record<string, string> = {
  tcp: 'text-[#4a9eff]', udp: 'text-[#a78bfa]', http: 'text-[#4cd964]',
  tunnel: 'text-[#e8b830]', internal: 'text-a-muted',
};

function statusBadge(status: string) {
  const map: Record<string, { cls: string; label: string }> = {
    active: { cls: 'bg-[#4cd964]/10 text-[#4cd964] border-[#4cd964]/20', label: '运行中' },
    active_recorded: { cls: 'bg-[#4a9eff]/10 text-[#4a9eff] border-[#4a9eff]/20', label: '已记录' },
    pending: { cls: 'bg-[#e8b830]/10 text-[#e8b830] border-[#e8b830]/20', label: '待激活' },
    pending_provider: { cls: 'bg-[#ff5c72]/10 text-[#ff5c72] border-[#ff5c72]/20', label: 'Provider 缺失' },
    disabled: { cls: 'bg-a-muted/10 text-a-muted border-a-muted/20', label: '已禁用' },
    failed: { cls: 'bg-[#ff5c72]/10 text-[#ff5c72] border-[#ff5c72]/20', label: '失败' },
  };
  const s = map[status] || { cls: 'bg-a-muted/10 text-a-muted border-a-muted/20', label: status };
  return (
    <span className={`text-[10px] px-1.5 py-0.5 rounded-a-sm border font-medium ${s.cls}`}>
      {s.label}
    </span>
  );
}

// ─── Page ───

export default function ExposuresPage() {
  const toast = useToast();
  const queryClient = useQueryClient();
  const [showCreate, setShowCreate] = useState(false);
  const [filter, setFilter] = useState('all');

  // Create form state
  const [formType, setFormType] = useState('tcp');
  const [formHost, setFormHost] = useState('127.0.0.1');
  const [formPort, setFormPort] = useState('');
  const [formTargetHost, setFormTargetHost] = useState('127.0.0.1');
  const [formTargetPort, setFormTargetPort] = useState('');
  const [formServiceID, setFormServiceID] = useState('');

  const { data, isLoading, error } = useQuery({
    queryKey: ['exposures'],
    queryFn: () => exposureApi.list(),
    refetchInterval: 30_000,
  });

  const createMu = useMutation({
    mutationFn: (input: any) => exposureApi.create(input),
    onSuccess: () => {
      toast('暴露规则已创建');
      setShowCreate(false);
      resetForm();
      queryClient.invalidateQueries({ queryKey: ['exposures'] });
    },
    onError: (e: any) => toast(e.message || '创建失败', 'error'),
  });

  const activateMu = useMutation({
    mutationFn: (id: string) => exposureApi.activate(id),
    onSuccess: () => {
      toast('已激活');
      queryClient.invalidateQueries({ queryKey: ['exposures'] });
    },
    onError: (e: any) => toast(e.message || '激活失败', 'error'),
  });

  const disableMu = useMutation({
    mutationFn: (id: string) => exposureApi.disable(id),
    onSuccess: () => {
      toast('已禁用');
      queryClient.invalidateQueries({ queryKey: ['exposures'] });
    },
    onError: (e: any) => toast(e.message || '禁用失败', 'error'),
  });

  function resetForm() {
    setFormType('tcp');
    setFormHost('127.0.0.1');
    setFormPort('');
    setFormTargetHost('127.0.0.1');
    setFormTargetPort('');
    setFormServiceID('');
  }

  function doCreate() {
    if (!formPort || !formTargetPort) {
      toast('端口号不能为空', 'error');
      return;
    }
    createMu.mutate({
      type: formType,
      host: formHost,
      port: parseInt(formPort),
      target_host: formTargetHost,
      target_port: parseInt(formTargetPort),
      service_id: formServiceID || undefined,
    });
  }

  const exposures: any[] = data?.exposures || [];
  const filtered = filter === 'all'
    ? exposures
    : exposures.filter((e: any) => e.type === filter || e.status === filter);

  return (
    <div>
      <PageHeader
        title="端口暴露"
        helpKey="exposures"
        sub={`${exposures.length} 条规则`}
        actions={
          <Btn primary onClick={() => setShowCreate(!showCreate)}>
            {showCreate ? '取消' : '创建暴露'}
          </Btn>
        }
      />

      {/* ─── Create form ─── */}
      {showCreate && (
        <Card title="创建端口暴露" className="mb-4">
          <div className="p-4 space-y-3">
            <div className="grid grid-cols-4 gap-3">
              <div>
                <label className="block text-[10px] text-a-muted mb-1">类型</label>
                <select
                  className="w-full text-xs px-2.5 py-1.5 rounded-a-sm border border-a-border bg-a-bg text-a-fg"
                  value={formType}
                  onChange={(e) => setFormType(e.target.value)}
                >
                  <option value="tcp">TCP</option>
                  <option value="udp">UDP</option>
                  <option value="tunnel">Tunnel</option>
                </select>
              </div>
              <div>
                <label className="block text-[10px] text-a-muted mb-1">监听地址</label>
                <input className="w-full font-mono text-xs px-2.5 py-1.5 rounded-a-sm border border-a-border bg-a-bg text-a-fg outline-none focus:border-a-accent"
                  value={formHost} onChange={(e) => setFormHost(e.target.value)} placeholder="127.0.0.1" />
              </div>
              <div>
                <label className="block text-[10px] text-a-muted mb-1">监听端口</label>
                <input className="w-full font-mono text-xs px-2.5 py-1.5 rounded-a-sm border border-a-border bg-a-bg text-a-fg outline-none focus:border-a-accent"
                  value={formPort} onChange={(e) => setFormPort(e.target.value)} placeholder="8080" />
              </div>
              <div>
                <label className="block text-[10px] text-a-muted mb-1">Service ID (可选)</label>
                <input className="w-full font-mono text-xs px-2.5 py-1.5 rounded-a-sm border border-a-border bg-a-bg text-a-fg outline-none focus:border-a-accent"
                  value={formServiceID} onChange={(e) => setFormServiceID(e.target.value)} placeholder="svc_xxx" />
              </div>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="block text-[10px] text-a-muted mb-1">目标地址</label>
                <input className="w-full font-mono text-xs px-2.5 py-1.5 rounded-a-sm border border-a-border bg-a-bg text-a-fg outline-none focus:border-a-accent"
                  value={formTargetHost} onChange={(e) => setFormTargetHost(e.target.value)} placeholder="127.0.0.1" />
              </div>
              <div>
                <label className="block text-[10px] text-a-muted mb-1">目标端口</label>
                <input className="w-full font-mono text-xs px-2.5 py-1.5 rounded-a-sm border border-a-border bg-a-bg text-a-fg outline-none focus:border-a-accent"
                  value={formTargetPort} onChange={(e) => setFormTargetPort(e.target.value)} placeholder="3000" />
              </div>
            </div>
            <div className="flex gap-2">
              <Btn primary onClick={doCreate} disabled={createMu.isPending}>
                {createMu.isPending ? '创建中…' : '创建'}
              </Btn>
              <Btn onClick={() => { setShowCreate(false); resetForm(); }}>取消</Btn>
            </div>
            <div className="text-[10px] text-a-muted">
              TCP 暴露创建后需点「激活」才会启动端口转发。监听地址默认 127.0.0.1（仅本机），公开暴露需管理员审核。
            </div>
          </div>
        </Card>
      )}

      {/* ─── Filter bar ─── */}
      <div className="flex gap-1.5 mb-4 flex-wrap">
        {['all', 'tcp', 'udp', 'tunnel', 'active', 'disabled', 'pending'].map(f => (
          <button
            key={f}
            onClick={() => setFilter(f)}
            className={`px-2.5 py-1 text-[10px] rounded-a-sm font-medium transition-colors ${
              filter === f ? 'bg-a-accent text-white' : 'bg-a-border/10 text-a-muted hover:text-a-fg'
            }`}
          >
            {f === 'all' ? '全部' : TYPE_LABELS[f] || f}
          </button>
        ))}
      </div>

      {/* ─── Loading / Empty / Error ─── */}
      {isLoading && <div className="text-center py-10 text-a-muted font-mono text-sm">加载中…</div>}
      {error && (
        <div className="p-3 rounded-a-sm text-xs bg-[#ff5c72]/10 text-[#ff5c72] border border-[#ff5c72]/20 mb-4">
          加载失败: {(error as any).message}
        </div>
      )}

      {/* ─── Exposure list ─── */}
      {!isLoading && filtered.length === 0 && (
        <div className="text-center py-16 text-a-muted text-xs space-y-2">
          <div>暂无暴露规则</div>
          <div className="text-[10px]">点击「创建暴露」添加 TCP/UDP 端口转发</div>
        </div>
      )}

      <div className="space-y-2">
        {filtered.map((e: any) => (
          <Card key={e.id}>
            <div className="p-4 flex items-center gap-4">
              {/* Type badge */}
              <span className={`text-xs font-mono font-bold ${TYPE_COLORS[e.type] || 'text-a-muted'}`}>
                {TYPE_LABELS[e.type] || e.type}
              </span>

              {/* Endpoint info */}
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2">
                  <span className="text-sm font-mono">
                    {e.host || '0.0.0.0'}:{e.port}
                  </span>
                  <span className="text-a-muted text-[10px]">→</span>
                  <span className="text-sm font-mono text-a-muted">
                    {e.target_host || '—'}:{e.target_port || '—'}
                  </span>
                </div>
                <div className="text-[10px] text-a-muted mt-0.5">
                  {e.provider || '无 provider'} · {e.service_id || '无 service'} · {e.id}
                </div>
                {e.message && (
                  <div className="text-[10px] text-a-muted mt-0.5 truncate">{e.message}</div>
                )}
              </div>

              {/* Status */}
              {statusBadge(e.status)}

              {/* Actions */}
              <div className="flex gap-1.5">
                {e.status === 'pending' || e.status === 'disabled' || e.status === 'active_recorded' ? (
                  <Btn onClick={() => activateMu.mutate(e.id)} disabled={activateMu.isPending}>
                    激活
                  </Btn>
                ) : null}
                {e.status === 'active' ? (
                  <Btn onClick={() => disableMu.mutate(e.id)} disabled={disableMu.isPending}>
                    禁用
                  </Btn>
                ) : null}
              </div>
            </div>
          </Card>
        ))}
      </div>
    </div>
  );
}

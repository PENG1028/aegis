// ─── Service Auth (v1.9A) — 服务间认证管理 ───
// List registered services, view APIs, block/unblock, browse call logs.

import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { adminApi } from '@/lib/api-bridge';
import { PageHeader, StatCard, StatusBadge, Drawer, Btn, Card, SearchInput, useToast, Modal, HealthDot, LoadingState, ErrorBanner, EmptyState } from '@/components/shared';
import { cn } from '@/lib/utils';

type ServiceRecord = any;
type CallLogEntry = any;

export default function AuthServices() {
  const toast = useToast();
  const qc = useQueryClient();
  const [search, setSearch] = useState('');
  const [selected, setSelected] = useState<ServiceRecord | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [blockTarget, setBlockTarget] = useState<ServiceRecord | null>(null);
  const [blockReason, setBlockReason] = useState('');
  const [showBlockModal, setShowBlockModal] = useState(false);
  const [tab, setTab] = useState<'services' | 'groups' | 'policies'>('services');

  // ── Data ──

  const { data: servicesData, isLoading, error, refetch } = useQuery({
    queryKey: ['auth-services'],
    queryFn: () => adminApi.listAuthServices(),
    refetchInterval: 15_000,
  });

  const services: ServiceRecord[] = servicesData?.services || [];

  const { data: callLogsData } = useQuery({
    queryKey: ['auth-call-logs'],
    queryFn: () => adminApi.getAuthCallLogs(undefined, 100),
    refetchInterval: 30_000,
  });

  const callLogs: CallLogEntry[] = callLogsData?.call_logs || [];

  const { data: topologyData } = useQuery({
    queryKey: ['auth-topology-preview'],
    queryFn: () => adminApi.getAuthTopology('1h'),
    refetchInterval: 60_000,
  });

  // ── Groups + Policies (v1.9D) ──
  const { data: groupsData } = useQuery({
    queryKey: ['auth-groups'], queryFn: () => adminApi.listAuthGroups(), refetchInterval: 30_000,
  });
  const groups: any[] = groupsData?.groups || [];

  const { data: policiesData } = useQuery({
    queryKey: ['auth-policies'], queryFn: () => adminApi.listAuthPolicies(), refetchInterval: 30_000,
  });
  const policies: any[] = policiesData?.policies || [];

  const [showGroupModal, setShowGroupModal] = useState(false);
  const [showPolicyModal, setShowPolicyModal] = useState(false);
  const [groupForm, setGroupForm] = useState({ name: '', description: '', members: '' });

  const upsertGroup = useMutation({
    mutationFn: (g: any) => adminApi.upsertAuthGroup(g),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['auth-groups'] }); toast('服务组已保存'); setShowGroupModal(false); },
    onError: (e: any) => toast(e.message, 'error'),
  });
  const deleteGroup = useMutation({
    mutationFn: (id: string) => adminApi.deleteAuthGroup(id),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['auth-groups'] }); toast('已删除'); },
    onError: (e: any) => toast(e.message, 'error'),
  });

  const [policyForm, setPolicyForm] = useState({ subject: '', target_service: '', action: '*', effect: 'allow' });
  const upsertPolicy = useMutation({
    mutationFn: (p: any) => adminApi.upsertAuthPolicy(p),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['auth-policies'] }); toast('策略已保存'); setShowPolicyModal(false); },
    onError: (e: any) => toast(e.message, 'error'),
  });
  const deletePolicy = useMutation({
    mutationFn: (id: string) => adminApi.deleteAuthPolicy(id),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['auth-policies'] }); toast('已删除'); },
    onError: (e: any) => toast(e.message, 'error'),
  });

  // ── Mutations ──

  const blockSvc = useMutation({
    mutationFn: ({ id, reason }: { id: string; reason: string }) => adminApi.blockAuthService(id, reason),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['auth-services'] }); toast('服务已封锁'); },
    onError: (e: any) => toast(e.message || '封锁失败', 'error'),
  });

  const unblockSvc = useMutation({
    mutationFn: (id: string) => adminApi.unblockAuthService(id),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['auth-services'] }); toast('已解封'); },
    onError: (e: any) => toast(e.message || '解封失败', 'error'),
  });

  // ── Stats ──

  const activeCount = services.filter((s: ServiceRecord) => s.status === 'active').length;
  const blockedCount = services.filter((s: ServiceRecord) => s.status === 'blocked').length;
  const todayCalls = callLogs.length;
  const topEdge = (topologyData?.edges || []).sort((a: any, b: any) => b.count - a.count)[0];

  // ── Filtered services ──

  const filtered = services.filter((s: ServiceRecord) => {
    if (!search) return true;
    const q = search.toLowerCase();
    return s.name?.toLowerCase().includes(q) || s.host?.includes(q) || `${s.port}`.includes(q);
  });

  // ── Helpers ──

  const openDetail = (svc: ServiceRecord) => {
    setSelected(svc);
    setDrawerOpen(true);
  };

  const confirmBlock = () => {
    if (!blockTarget || !blockReason.trim()) return;
    blockSvc.mutate({ id: blockTarget.id, reason: blockReason.trim() });
    setShowBlockModal(false);
    setBlockReason('');
    setBlockTarget(null);
  };

  return (
    <div className="p-6 space-y-5">
      <PageHeader title="服务认证 · Service Auth" subtitle={`${activeCount} 在线 · ${blockedCount} 已封锁 · ${todayCalls} 调用/24h`} />

      {/* Tabs */}
      <div className="flex gap-1 border-b border-a-border/30 pb-0">
        {(['services', 'groups', 'policies'] as const).map(t => (
          <button key={t} onClick={() => setTab(t)}
            className={cn('px-3 py-1.5 text-xs border-b-2 transition-colors cursor-pointer',
              tab === t ? 'border-a-accent text-a-accent font-medium' : 'border-transparent text-a-muted hover:text-a-fg')}>
            {{services: '服务', groups: '服务组', policies: '策略'}[t]}
          </button>
        ))}
      </div>

      {tab === 'services' && (<>

      {/* Stat cards */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
        <StatCard label="在线服务" value={activeCount} accent />
        <StatCard label="已封锁" value={blockedCount} danger={blockedCount > 0} />
        <StatCard label="24h 调用" value={todayCalls} />
        <StatCard label="最高频" value={topEdge ? `${topEdge.caller}→${topEdge.target}` : '—'} />
      </div>

      {/* Service table */}
      <Card title="注册服务" subtitle={`${filtered.length}/${services.length} 个服务`}>
        <SearchInput value={search} onChange={setSearch} placeholder="搜索服务名、主机、端口..." className="mb-3" />

        {isLoading ? (
          <LoadingState />
        ) : error ? (
          <ErrorBanner message={(error as any)?.message || '加载失败'} onRetry={refetch} />
        ) : filtered.length === 0 ? (
          <EmptyState title={search ? '没有匹配的服务' : '暂无已注册服务'} description={search ? '尝试其他关键词' : '部署 SDK 并注册服务后自动出现'} />
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-xs">
              <thead>
                <tr className="border-b border-a-border text-a-muted text-left">
                  <th className="py-2 px-3 font-medium">名称</th>
                  <th className="py-2 px-3 font-medium">地址</th>
                  <th className="py-2 px-3 font-medium">APIs</th>
                  <th className="py-2 px-3 font-medium">节点</th>
                  <th className="py-2 px-3 font-medium">状态</th>
                  <th className="py-2 px-3 font-medium">操作</th>
                </tr>
              </thead>
              <tbody>
                {filtered.map((s: ServiceRecord) => {
                  const apis = safeParseJSON(s.apis_json);
                  const isBlocked = s.status === 'blocked';
                  return (
                    <tr key={s.id}
                      onClick={() => openDetail(s)}
                      className="border-b border-a-border/50 hover:bg-a-border/10 transition-colors cursor-pointer">
                      <td className="py-2 px-3 font-semibold text-a-fg">{s.name}</td>
                      <td className="py-2 px-3 font-mono text-a-muted">{s.host}:{s.port}</td>
                      <td className="py-2 px-3 font-mono text-a-muted">{apis.length}</td>
                      <td className="py-2 px-3 font-mono text-[11px] text-a-muted">{s.node_host || '—'}</td>
                      <td className="py-2 px-3">
                        <StatusBadge status={isBlocked ? 'disabled' : 'active'} />
                      </td>
                      <td className="py-2 px-3">
                        <div className="flex items-center gap-1.5" onClick={(e) => e.stopPropagation()}>
                          <Btn onClick={() => openDetail(s)} className="text-[10px]">详情</Btn>
                          {isBlocked ? (
                            <Btn onClick={() => unblockSvc.mutate(s.id)} className="text-[10px]">解封</Btn>
                          ) : (
                            <Btn onClick={() => { setBlockTarget(s); setShowBlockModal(true); }} className="text-[10px]" danger>封锁</Btn>
                          )}
                        </div>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </Card>

      {/* Service detail drawer */}
      <Drawer open={drawerOpen} onClose={() => setDrawerOpen(false)}
        title={selected?.name || '服务详情'}
        subtitle={selected ? `${selected.host}:${selected.port}` : ''}
        width="md">
        {selected && <ServiceDetailContent svc={selected} callLogs={callLogs} onBlock={() => {
          setBlockTarget(selected);
          setShowBlockModal(true);
        }} onUnblock={() => unblockSvc.mutate(selected.id)} />}
      </Drawer>

      {/* End services tab */}
      </>)}

      {tab === 'groups' && (
        <Card title="服务组" subtitle="将服务分组以便在策略中引用">
          <div className="mb-3 flex items-center gap-2">
            <Btn primary onClick={() => { setGroupForm({ name: '', description: '', members: '' }); setShowGroupModal(true); }} className="text-xs">新建服务组</Btn>
            <span className="text-[10px] text-a-muted ml-auto">{groups.length} 个组</span>
          </div>
          {groups.length === 0 ? <EmptyState title="暂无服务组" /> : (
            <div className="space-y-2">
              {groups.map((g: any) => (
                <div key={g.id} className="flex items-center gap-3 px-3 py-2 rounded-a-sm border border-a-border/20 bg-a-surface text-xs">
                  <span className="font-medium text-a-fg w-32">{g.name}</span>
                  <span className="text-a-muted flex-1">{g.description || '—'}</span>
                  <span className="text-[10px] text-a-muted">{g.members?.length || 0} 个成员</span>
                  <Btn onClick={() => deleteGroup.mutate(g.id)} className="text-[9px]" danger>删除</Btn>
                </div>
              ))}
            </div>
          )}
        </Card>
      )}

      {tab === 'policies' && (
        <Card title="权限策略" subtitle="基于服务+操作的控制规则，无匹配时默认允许">
          <div className="mb-3 flex items-center gap-2">
            <Btn primary onClick={() => { setPolicyForm({ subject: '', target_service: '', action: '*', effect: 'allow' }); setShowPolicyModal(true); }} className="text-xs">新建策略</Btn>
            <span className="text-[10px] text-a-muted ml-auto">{policies.length} 条</span>
          </div>
          {policies.length === 0 ? <EmptyState title="暂无策略" /> : (
            <div className="overflow-x-auto"><table className="w-full text-xs">
              <thead><tr className="border-b border-a-border/30 text-a-muted text-left"><th className="py-1.5 pr-2">主体</th><th className="py-1.5 px-2">目标</th><th className="py-1.5 px-2">操作</th><th className="py-1.5 px-2">效果</th><th className="py-1.5 pl-2"></th></tr></thead>
              <tbody>{policies.map((p: any) => (
                <tr key={p.id} className="border-b border-a-border/20">
                  <td className="py-1.5 pr-2 font-mono text-[11px] text-a-fg">{p.subject}</td>
                  <td className="py-1.5 px-2 text-a-muted">{p.target_service}</td>
                  <td className="py-1.5 px-2 text-a-muted">{p.action}</td>
                  <td className="py-1.5 px-2"><span className={cn('px-1.5 py-0.5 rounded text-[10px] font-medium', p.effect === 'allow' ? 'bg-[#4cd964]/10 text-[#4cd964]' : 'bg-[#ff5c72]/10 text-[#ff5c72]')}>{p.effect}</span></td>
                  <td className="py-1.5 pl-2"><Btn onClick={() => deletePolicy.mutate(p.id)} className="text-[9px]" danger>删除</Btn></td>
                </tr>
              ))}</tbody>
            </table></div>
          )}
        </Card>
      )}

      {/* Group modal */}
      {showGroupModal && (
        <Modal onClose={() => setShowGroupModal(false)} title="服务组"
          footer={<div className="flex gap-2 justify-end"><Btn onClick={() => setShowGroupModal(false)} className="text-xs">取消</Btn><Btn primary onClick={() => upsertGroup.mutate({...groupForm, members: groupForm.members.split(',').map((s: string) => s.trim()).filter(Boolean)})} className="text-xs">保存</Btn></div>}>
          <div className="space-y-3 text-xs">
            <input value={groupForm.name} onChange={e => setGroupForm({...groupForm, name: e.target.value})} placeholder="组名（如 storage-group）" className="w-full bg-a-bg border border-a-border rounded-a-sm px-2 py-1.5 text-a-fg" />
            <input value={groupForm.description} onChange={e => setGroupForm({...groupForm, description: e.target.value})} placeholder="描述（可选）" className="w-full bg-a-bg border border-a-border rounded-a-sm px-2 py-1.5 text-a-fg" />
            <input value={groupForm.members} onChange={e => setGroupForm({...groupForm, members: e.target.value})} placeholder="成员（逗号分隔，如 B, C, D）" className="w-full bg-a-bg border border-a-border rounded-a-sm px-2 py-1.5 text-a-fg" />
          </div>
        </Modal>
      )}

      {/* Policy modal */}
      {showPolicyModal && (
        <Modal onClose={() => setShowPolicyModal(false)} title="权限策略"
          footer={<div className="flex gap-2 justify-end"><Btn onClick={() => setShowPolicyModal(false)} className="text-xs">取消</Btn><Btn primary onClick={() => upsertPolicy.mutate(policyForm)} className="text-xs">保存</Btn></div>}>
          <div className="space-y-3 text-xs">
            <input value={policyForm.subject} onChange={e => setPolicyForm({...policyForm, subject: e.target.value})} placeholder="主体（服务名 / 组名 / *）" className="w-full bg-a-bg border border-a-border rounded-a-sm px-2 py-1.5 text-a-fg" />
            <input value={policyForm.target_service} onChange={e => setPolicyForm({...policyForm, target_service: e.target.value})} placeholder="目标服务（服务名 / *）" className="w-full bg-a-bg border border-a-border rounded-a-sm px-2 py-1.5 text-a-fg" />
            <select value={policyForm.effect} onChange={e => setPolicyForm({...policyForm, effect: e.target.value})} className="w-full bg-a-bg border border-a-border rounded-a-sm px-2 py-1.5 text-a-fg">
              <option value="allow">allow（允许）</option><option value="deny">deny（拒绝）</option>
            </select>
          </div>
        </Modal>
      )}

      {showBlockModal && (
        <Modal
          onClose={() => { setShowBlockModal(false); setBlockTarget(null); setBlockReason(''); }}
          title="封锁服务"
          footer={
            <div className="flex items-center gap-2 justify-end">
              <Btn onClick={() => { setShowBlockModal(false); setBlockTarget(null); setBlockReason(''); }} className="text-xs">取消</Btn>
              <Btn onClick={confirmBlock} danger className="text-xs" disabled={!blockReason.trim() || blockSvc.isPending}>
                {blockSvc.isPending ? '封锁中...' : '确认封锁'}
              </Btn>
            </div>
          }>
          <div className="space-y-3">
            <p className="text-sm text-a-fg">确定要封锁 <span className="font-semibold">{blockTarget?.name}</span> 吗？</p>
            <p className="text-xs text-a-muted">该服务的所有请求将被拒绝。</p>
            <input
              autoFocus
              type="text"
              placeholder="输入封锁原因"
              value={blockReason}
              onChange={(e) => setBlockReason(e.target.value)}
              className="w-full px-3 py-2 rounded-a-sm bg-a-bg border border-a-border/50 text-sm text-a-fg placeholder:text-a-muted/50 focus:outline-none focus:border-a-accent/50"
            />
          </div>
        </Modal>
      )}
    </div>
  );
}

// ══════════════════════════════════════════════════════════════════
// Service Detail (inside Drawer)
// ══════════════════════════════════════════════════════════════════

function ServiceDetailContent({ svc, callLogs, onBlock, onUnblock }: {
  svc: ServiceRecord;
  callLogs: CallLogEntry[];
  onBlock: () => void;
  onUnblock: () => void;
}) {
  const apis = safeParseJSON(svc.apis_json);
  const isBlocked = svc.status === 'blocked';
  const svcLogs = callLogs.filter((l: CallLogEntry) => l.caller_service === svc.name || l.target_service === svc.name).slice(0, 20);

  return (
    <div className="space-y-4">
      {/* Identity */}
      <Card title="基本信息">
        <div className="grid grid-cols-2 gap-3 text-xs">
          <div>
            <div className="text-a-muted mb-0.5">服务 ID</div>
            <div className="font-mono text-a-fg">{svc.id}</div>
          </div>
          <div>
            <div className="text-a-muted mb-0.5">状态</div>
            <div className="flex items-center gap-1.5">
              <HealthDot status={isBlocked ? 'failed' : 'healthy'} />
              <span className={isBlocked ? 'text-[#ff5c72]' : 'text-[#4cd964]'}>{isBlocked ? '已封锁' : '活跃'}</span>
            </div>
          </div>
          <div>
            <div className="text-a-muted mb-0.5">地址</div>
            <div className="font-mono text-a-fg">{svc.host}:{svc.port}</div>
          </div>
          <div>
            <div className="text-a-muted mb-0.5">节点</div>
            <div className="font-mono text-a-fg">{svc.node_host || '—'}</div>
          </div>
          <div>
            <div className="text-a-muted mb-0.5">最后心跳</div>
            <div className="text-a-fg">{fmtTime(svc.last_seen)}</div>
          </div>
          <div>
            <div className="text-a-muted mb-0.5">注册时间</div>
            <div className="text-a-fg">{fmtTime(svc.created_at)}</div>
          </div>
        </div>
      </Card>

      {/* APIs */}
      <Card title={`APIs (${apis.length})`}>
        {apis.length === 0 ? (
          <div className="text-xs text-a-muted py-2">未注册 API</div>
        ) : (
          <div className="divide-y divide-a-border/20">
            {apis.map((a: any, i: number) => (
              <div key={i} className="py-1.5 flex items-center gap-3 text-xs font-mono">
                <span className={cn(
                  'px-1 py-0.5 rounded text-[10px] font-semibold min-w-[48px] text-center',
                  a.method === 'GET' && 'text-[#4cd964] bg-[#4cd964]/10',
                  a.method === 'POST' && 'text-[#a865ff] bg-[#a865ff]/10',
                  (a.method === 'PUT' || a.method === 'PATCH') && 'text-[#e8b830] bg-[#e8b830]/10',
                  a.method === 'DELETE' && 'text-[#ff5c72] bg-[#ff5c72]/10',
                )}>
                  {a.method || 'ANY'}
                </span>
                <span className="text-a-muted flex-1">{a.path}</span>
                <span className="text-a-fg2">{a.name}</span>
              </div>
            ))}
          </div>
        )}
      </Card>

      {/* Call logs */}
      <Card title={`调用记录 (最近 ${svcLogs.length})`}>
        {svcLogs.length === 0 ? (
          <div className="text-xs text-a-muted py-2">暂无调用记录</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-[11px]">
              <thead>
                <tr className="border-b border-a-border/30 text-a-muted text-left">
                  <th className="py-1.5 pr-2">时间</th>
                  <th className="py-1.5 px-2">方向</th>
                  <th className="py-1.5 px-2">目标</th>
                  <th className="py-1.5 px-2">API</th>
                  <th className="py-1.5 px-2 text-right">延迟</th>
                  <th className="py-1.5 pl-2 text-right">状态</th>
                </tr>
              </thead>
              <tbody>
                {svcLogs.map((l: CallLogEntry, i: number) => (
                  <tr key={i} className="border-b border-a-border/20 hover:bg-a-border/10 transition-colors">
                    <td className="py-1.5 pr-2 font-mono text-a-muted text-[10px] whitespace-nowrap">{fmtTimeShort(l.called_at)}</td>
                    <td className="py-1.5 px-2">
                      {l.caller_service === svc.name ? (
                        <span className="text-[#a865ff]">→ 调用</span>
                      ) : (
                        <span className="text-[#4cd964]">← 被调</span>
                      )}
                    </td>
                    <td className="py-1.5 px-2 font-mono">{l.caller_service === svc.name ? l.target_service : l.caller_service}</td>
                    <td className="py-1.5 px-2 text-a-muted">{l.target_api}</td>
                    <td className="py-1.5 px-2 font-mono text-right text-a-muted">{l.latency_ms}ms</td>
                    <td className="py-1.5 pl-2 text-right">
                      <span className={l.allowed ? 'text-[#4cd964]' : 'text-[#ff5c72]'}>
                        {l.allowed ? '✓' : '✗'}
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Card>

      {/* Actions */}
      <div className="flex items-center gap-2 pt-2 border-t border-a-border/30">
        {isBlocked ? (
          <Btn onClick={onUnblock} className="text-xs">🔓 解封服务</Btn>
        ) : (
          <Btn onClick={onBlock} danger className="text-xs">🔒 封锁服务</Btn>
        )}
      </div>
    </div>
  );
}

// ══════════════════════════════════════════════════════════════════
// Helpers
// ══════════════════════════════════════════════════════════════════

function safeParseJSON(s: string | undefined): any[] {
  if (!s) return [];
  try { return JSON.parse(s); } catch { return []; }
}

function fmtTime(t: string | undefined): string {
  if (!t) return '—';
  try {
    return new Date(t).toLocaleString('zh-CN', {
      month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit', second: '2-digit',
    });
  } catch { return t; }
}

function fmtTimeShort(t: string | undefined): string {
  if (!t) return '—';
  try {
    return new Date(t).toLocaleString('zh-CN', {
      hour: '2-digit', minute: '2-digit', second: '2-digit',
    });
  } catch { return t; }
}

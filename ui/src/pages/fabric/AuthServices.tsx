// ─── Service Auth (v1.9A) — 服务间认证管理 ───
// 视角 A: 全局调度中心。列出所有注册服务及其状态、调用关系。
// 视角 B (服务自检) 见 /gateway/service

import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { adminApi } from '@/lib/api-bridge';
import { PageHeader, StatCard, StatusBadge, Drawer, Btn, Card, SearchInput, useToast, Modal, HealthDot, LoadingState, ErrorBanner, EmptyState } from '@/components/shared';
import { cn } from '@/lib/utils';

type ServiceRecord = any;
type CallLogEntry = any;
type TopoEdge = { caller: string; target: string; api: string; count: number; last_seen: string };
type Tab = 'services' | 'groups';

export default function AuthServices() {
  const toast = useToast();
  const qc = useQueryClient();
  const [search, setSearch] = useState('');
  const [selected, setSelected] = useState<ServiceRecord | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [blockTarget, setBlockTarget] = useState<ServiceRecord | null>(null);
  const [blockReason, setBlockReason] = useState('');
  const [showBlockModal, setShowBlockModal] = useState(false);
  const [tab, setTab] = useState<Tab>('services');

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
  const topoEdges: TopoEdge[] = topologyData?.edges || [];

  // ── Groups ──
  const { data: groupsData } = useQuery({
    queryKey: ['auth-groups'], queryFn: () => adminApi.listAuthGroups(), refetchInterval: 30_000,
  });
  const groups: any[] = groupsData?.groups || [];

  const [showGroupModal, setShowGroupModal] = useState(false);
  const [groupForm, setGroupForm] = useState({ name: '', description: '', members: '' });

  const upsertGroup = useMutation({
    mutationFn: (g: any) => adminApi.upsertAuthGroup(g),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['auth-groups'] }); toast('组已保存'); setShowGroupModal(false); },
    onError: (e: any) => toast(e.message, 'error'),
  });
  const deleteGroup = useMutation({
    mutationFn: (id: string) => adminApi.deleteAuthGroup(id),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['auth-groups'] }); toast('已删除'); },
    onError: (e: any) => toast(e.message, 'error'),
  });

  // ── Mutations ──

  const blockSvc = useMutation({
    mutationFn: ({ id, reason }: { id: string; reason: string }) => adminApi.blockAuthService(id, reason),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['auth-services'] }); toast('已封锁'); },
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
  const topEdge = topoEdges.sort((a, b) => b.count - a.count)[0];

  // ── Filtered services ──

  const filtered = services.filter((s: ServiceRecord) => {
    if (!search) return true;
    return s.name?.toLowerCase().includes(search.toLowerCase());
  });

  return (
    <div className="p-6 space-y-5">
      <PageHeader title="服务认证 · Service Auth" subtitle={`${activeCount} 在线 · ${blockedCount} 已封锁 · ${todayCalls} 调用/24h`} />

      {/* Tabs */}
      <div className="flex gap-1 border-b border-a-border/30 pb-0">
        {(['services', 'groups'] as Tab[]).map(t => (
          <button key={t} onClick={() => setTab(t)}
            className={cn('px-3 py-1.5 text-xs border-b-2 transition-colors cursor-pointer',
              tab === t ? 'border-a-accent text-a-accent font-medium' : 'border-transparent text-a-muted hover:text-a-fg')}>
            {{services: '服务', groups: '服务组'}[t]}
          </button>
        ))}
        <div className="flex-1" />
        <a href="/auth/callgraph" className="px-3 py-1.5 text-xs text-a-accent hover:underline">完整拓扑图 →</a>
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
        <SearchInput value={search} onChange={setSearch} placeholder="搜索服务名..." className="mb-3" />

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
                  <th className="py-2 px-3 font-medium">公钥</th>
                  <th className="py-2 px-3 font-medium">实例</th>
                  <th className="py-2 px-3 font-medium">状态</th>
                  <th className="py-2 px-3 font-medium">最后心跳</th>
                  <th className="py-2 px-3 font-medium">操作</th>
                </tr>
              </thead>
              <tbody>
                {filtered.map((s: ServiceRecord) => {
                  const isBlocked = s.status === 'blocked';
                  return (
                    <tr key={s.id}
                      onClick={() => { setSelected(s); setDrawerOpen(true); }}
                      className="border-b border-a-border/50 hover:bg-a-border/10 transition-colors cursor-pointer">
                      <td className="py-2 px-3 font-semibold text-a-fg">{s.name}</td>
                      <td className="py-2 px-3 font-mono text-[10px] text-a-muted max-w-[120px] truncate">{s.public_key ? s.public_key.slice(0, 20) + '...' : '-'}</td>
                      <td className="py-2 px-3 font-mono text-[10px] text-a-muted">{s.instance_id || '-'}</td>
                      <td className="py-2 px-3">
                        <StatusBadge status={isBlocked ? 'disabled' : 'active'} />
                      </td>
                      <td className="py-2 px-3 text-[10px] text-a-muted whitespace-nowrap">{fmtTimeShort(s.last_seen)}</td>
                      <td className="py-2 px-3">
                        <div className="flex items-center gap-1.5" onClick={(e) => e.stopPropagation()}>
                          <Btn onClick={() => { setSelected(s); setDrawerOpen(true); }} className="text-[10px]">详情</Btn>
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
        subtitle={`ID: ${selected?.id || ''}`}
        width="lg">
        {selected && (
          <ServiceDetailContent
            svc={selected}
            callLogs={callLogs}
            topoEdges={topoEdges}
            onBlock={() => { setBlockTarget(selected); setShowBlockModal(true); }}
            onUnblock={() => unblockSvc.mutate(selected.id)}
          />
        )}
      </Drawer>

      {/* End services tab */}
      </>)}

      {tab === 'groups' && (
        <Card title="服务组" subtitle="将服务分组管理">
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

      {/* Group modal */}
      {showGroupModal && (
        <Modal onClose={() => setShowGroupModal(false)} title="服务组"
          footer={<div className="flex gap-2 justify-end"><Btn onClick={() => setShowGroupModal(false)} className="text-xs">取消</Btn><Btn primary onClick={() => upsertGroup.mutate({...groupForm, members: groupForm.members.split(',').map((s: string) => s.trim()).filter(Boolean)})} className="text-xs">保存</Btn></div>}>
          <div className="space-y-3 text-xs">
            <input value={groupForm.name} onChange={e => setGroupForm({...groupForm, name: e.target.value})} placeholder="组名（如 storage-group）" className="w-full bg-a-bg border border-a-border rounded-a-sm px-2 py-1.5 text-a-fg" />
            <input value={groupForm.description} onChange={e => setGroupForm({...groupForm, description: e.target.value})} placeholder="描述（可选）" className="w-full bg-a-bg border border-a-border rounded-a-sm px-2 py-1.5 text-a-fg" />
            <input value={groupForm.members} onChange={e => setGroupForm({...groupForm, members: e.target.value})} placeholder="成员（逗号分隔）" className="w-full bg-a-bg border border-a-border rounded-a-sm px-2 py-1.5 text-a-fg" />
          </div>
        </Modal>
      )}

      {/* Block modal */}
      {showBlockModal && (
        <Modal
          onClose={() => { setShowBlockModal(false); setBlockTarget(null); setBlockReason(''); }}
          title="封锁服务"
          footer={
            <div className="flex items-center gap-2 justify-end">
              <Btn onClick={() => { setShowBlockModal(false); setBlockTarget(null); setBlockReason(''); }} className="text-xs">取消</Btn>
              <Btn onClick={() => { if (!blockTarget || !blockReason.trim()) return; blockSvc.mutate({ id: blockTarget.id, reason: blockReason.trim() }); setShowBlockModal(false); setBlockReason(''); setBlockTarget(null); }} danger className="text-xs" disabled={!blockReason.trim() || blockSvc.isPending}>
                {blockSvc.isPending ? '封锁中...' : '确认封锁'}
              </Btn>
            </div>
          }>
          <div className="space-y-3">
            <p className="text-sm text-a-fg">确定要封锁 <span className="font-semibold">{blockTarget?.name}</span> 吗？</p>
            <p className="text-xs text-a-muted">该服务的所有请求将被拒绝。</p>
            <input autoFocus type="text" placeholder="输入封锁原因" value={blockReason}
              onChange={(e) => setBlockReason(e.target.value)}
              className="w-full px-3 py-2 rounded-a-sm bg-a-bg border border-a-border/50 text-sm text-a-fg placeholder:text-a-muted/50 focus:outline-none focus:border-a-accent/50" />
          </div>
        </Modal>
      )}
    </div>
  );
}

// ══════════════════════════════════════════════════════════════════
// Service Detail (inside Drawer)
// ══════════════════════════════════════════════════════════════════

function ServiceDetailContent({ svc, callLogs, topoEdges, onBlock, onUnblock }: {
  svc: ServiceRecord;
  callLogs: CallLogEntry[];
  topoEdges: TopoEdge[];
  onBlock: () => void;
  onUnblock: () => void;
}) {
  const isBlocked = svc.status === 'blocked';
  const svcLogs = callLogs.filter((l: CallLogEntry) => l.caller_service === svc.name || l.target_service === svc.name).slice(0, 20);

  // Dependency edges: this service calls others
  const callsOut = topoEdges.filter(e => e.caller === svc.name);
  // Who calls this service
  const callsIn = topoEdges.filter(e => e.target === svc.name);

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
            <div className="text-a-muted mb-0.5">公钥</div>
            <div className="font-mono text-[10px] text-a-fg break-all">{svc.public_key ? svc.public_key.slice(0, 32) + '...' : '—'}</div>
          </div>
          <div>
            <div className="text-a-muted mb-0.5">实例 ID</div>
            <div className="font-mono text-[10px] text-a-fg">{svc.instance_id || '—'}</div>
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

      {/* Dependency: calls out */}
      {callsOut.length > 0 && (
        <Card title={`依赖 · 调用 (${callsOut.length})`}>
          <div className="overflow-x-auto">
            <table className="w-full text-[11px]">
              <thead>
                <tr className="border-b border-a-border/30 text-a-muted text-left">
                  <th className="py-1.5 pr-2 font-medium">目标服务</th>
                  <th className="py-1.5 px-2 font-medium">API</th>
                  <th className="py-1.5 px-2 font-medium text-right">调用次数</th>
                  <th className="py-1.5 pl-2 font-medium text-right">最后调用</th>
                </tr>
              </thead>
              <tbody>
                {callsOut.sort((a, b) => b.count - a.count).map((e, i) => (
                  <tr key={i} className="border-b border-a-border/20">
                    <td className="py-1.5 pr-2 font-semibold text-a-fg">→ {e.target}</td>
                    <td className="py-1.5 px-2 font-mono text-a-muted text-[10px]">{e.api}</td>
                    <td className="py-1.5 px-2 text-right font-mono text-a-fg">{e.count}</td>
                    <td className="py-1.5 pl-2 text-right text-a-muted text-[10px]">{fmtTimeShort(e.last_seen)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Card>
      )}

      {/* Dependency: calls in */}
      {callsIn.length > 0 && (
        <Card title={`被调用 (${callsIn.length})`}>
          <div className="overflow-x-auto">
            <table className="w-full text-[11px]">
              <thead>
                <tr className="border-b border-a-border/30 text-a-muted text-left">
                  <th className="py-1.5 pr-2 font-medium">调用方</th>
                  <th className="py-1.5 px-2 font-medium">API</th>
                  <th className="py-1.5 px-2 font-medium text-right">调用次数</th>
                  <th className="py-1.5 pl-2 font-medium text-right">最后调用</th>
                </tr>
              </thead>
              <tbody>
                {callsIn.sort((a, b) => b.count - a.count).map((e, i) => (
                  <tr key={i} className="border-b border-a-border/20">
                    <td className="py-1.5 pr-2 font-semibold text-a-fg">← {e.caller}</td>
                    <td className="py-1.5 px-2 font-mono text-a-muted text-[10px]">{e.api}</td>
                    <td className="py-1.5 px-2 text-right font-mono text-a-fg">{e.count}</td>
                    <td className="py-1.5 pl-2 text-right text-a-muted text-[10px]">{fmtTimeShort(e.last_seen)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Card>
      )}

      {/* Full public key */}
      {svc.public_key && (
        <Card title="公钥">
          <div className="text-xs font-mono text-a-muted break-all bg-a-bg/50 p-2 rounded-a-sm">{svc.public_key}</div>
        </Card>
      )}

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
                  <tr key={i} className="border-b border-a-border/20">
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
                      <span className={l.allowed ? 'text-[#4cd964]' : 'text-[#ff5c72]'}>{l.allowed ? '✓' : '✗'}</span>
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
          <Btn onClick={onUnblock} className="text-xs">解封服务</Btn>
        ) : (
          <Btn onClick={onBlock} danger className="text-xs">封锁服务</Btn>
        )}
      </div>
    </div>
  );
}

// ══════════════════════════════════════════════════════════════════
// Helpers
// ══════════════════════════════════════════════════════════════════

function fmtTime(t: string | undefined): string {
  if (!t) return '—';
  try { return new Date(t).toLocaleString('zh-CN', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit', second: '2-digit' }); }
  catch { return t; }
}

function fmtTimeShort(t: string | undefined): string {
  if (!t) return '—';
  try { return new Date(t).toLocaleString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' }); }
  catch { return t; }
}

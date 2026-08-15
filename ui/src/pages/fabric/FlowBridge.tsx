// ─── FlowBridge 实例管理 (v1.9C-2) ───
// 实例是 Aegis 管理的 FlowBridge 数据面进程：域名可指向实例的数据面端口，
// Aegis 只做 TLS 终止 + 转发；实例健康由控制面 /health 探活决定。
import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { flowbridgeApi, type FlowBridgeInstance } from '@/lib/api-bridge';
import { Btn, Card, Modal, useToast } from '@/components/shared';
import { cn } from '@/lib/utils';

function healthBadge(inst: FlowBridgeInstance) {
  if (!inst.enabled) return { label: '已禁用', cls: 'bg-a-border/10 text-a-muted border-a-border/20' };
  switch (inst.last_health_status) {
    case 'healthy': return { label: '健康', cls: 'bg-[#4cd964]/10 text-[#4cd964] border-[#4cd964]/20' };
    case 'unhealthy': return { label: '异常', cls: 'bg-[#ff5c72]/10 text-[#ff5c72] border-[#ff5c72]/20' };
    default: return { label: '未知', cls: 'bg-[#e8b830]/10 text-[#e8b830] border-[#e8b830]/20' };
  }
}

export default function FlowBridge() {
  const toast = useToast();
  const qc = useQueryClient();
  const [editing, setEditing] = useState<FlowBridgeInstance | null>(null);
  const [creating, setCreating] = useState(false);
  // Deletion confirmation uses the app Modal (was native confirm()).
  const [deleteTarget, setDeleteTarget] = useState<FlowBridgeInstance | null>(null);
  const [form, setForm] = useState({ name: '', machine_ip: '', data_plane_port: 8080, control_address: '' });

  const { data, isLoading } = useQuery({
    queryKey: ['flowbridge'],
    queryFn: () => flowbridgeApi.list().catch(() => ({ data: [] as FlowBridgeInstance[], meta: { total: 0 } })),
    refetchInterval: 30_000,
  });
  const instances = data?.data || [];

  const invalidate = () => qc.invalidateQueries({ queryKey: ['flowbridge'] });

  const create = useMutation({
    mutationFn: () => flowbridgeApi.create(form),
    onSuccess: () => { toast('实例创建成功，已触发健康检查'); setCreating(false); setForm({ name: '', machine_ip: '', data_plane_port: 8080, control_address: '' }); invalidate(); },
    onError: (e: any) => toast(e.message || '创建失败', 'error'),
  });
  const update = useMutation({
    mutationFn: () => flowbridgeApi.update(editing!.id, { name: editing!.name, machine_ip: editing!.machine_ip, data_plane_port: editing!.data_plane_port, control_address: editing!.control_address }),
    onSuccess: () => { toast('实例已更新'); setEditing(null); invalidate(); },
    onError: (e: any) => toast(e.message || '更新失败', 'error'),
  });
  const toggle = useMutation({
    mutationFn: (inst: FlowBridgeInstance) => flowbridgeApi.update(inst.id, { enabled: !inst.enabled }),
    onSuccess: () => { toast('状态已切换，变更在下次 Apply 生效'); invalidate(); },
    onError: (e: any) => toast(e.message || '操作失败', 'error'),
  });
  const check = useMutation({
    mutationFn: (id: string) => flowbridgeApi.check(id),
    onSuccess: (inst: FlowBridgeInstance) => { toast(`健康检查完成: ${healthBadge(inst).label}`); invalidate(); },
    onError: (e: any) => toast(e.message || '检查失败', 'error'),
  });
  const remove = useMutation({
    mutationFn: (id: string) => flowbridgeApi.del(id),
    onSuccess: () => { toast('实例已删除'); invalidate(); },
    onError: (e: any) => toast(e.message || '删除失败（实例可能仍被路由引用）', 'error'),
  });

  const inputCls = 'w-full px-3 py-2 rounded-a-sm border border-a-border/50 bg-a-bg text-xs outline-none focus:border-a-accent/50 font-mono';

  return (
    <div className="p-6 space-y-5">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-bold text-a-fg">FlowBridge 实例</h2>
          <p className="text-xs text-a-muted mt-1">托管的数据面中间件 — 域名可绑定到实例，Aegis 终止 TLS 后按 Host 转发</p>
        </div>
        <Btn primary onClick={() => setCreating(v => !v)}>{creating ? '收起' : '添加实例'}</Btn>
      </div>

      {creating && (
        <Card>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="text-[10px] text-a-muted block mb-1.5 font-medium">名称</label>
              <input value={form.name} onChange={e => setForm({ ...form, name: e.target.value })} placeholder="edge-prod" className={inputCls} />
            </div>
            <div>
              <label className="text-[10px] text-a-muted block mb-1.5 font-medium">机器 IP</label>
              <input value={form.machine_ip} onChange={e => setForm({ ...form, machine_ip: e.target.value })} placeholder="10.0.0.5" className={inputCls} />
            </div>
            <div>
              <label className="text-[10px] text-a-muted block mb-1.5 font-medium">数据面端口</label>
              <input type="number" value={form.data_plane_port} onChange={e => setForm({ ...form, data_plane_port: Number(e.target.value) })} className={inputCls} />
            </div>
            <div>
              <label className="text-[10px] text-a-muted block mb-1.5 font-medium">控制面地址 (host:port)</label>
              <input value={form.control_address} onChange={e => setForm({ ...form, control_address: e.target.value })} placeholder="10.0.0.5:9090" className={inputCls} />
            </div>
          </div>
          <div className="mt-4 flex justify-end">
            <Btn primary disabled={create.isPending || !form.name || !form.machine_ip || !form.control_address} onClick={() => create.mutate()}>
              {create.isPending ? '创建中...' : '创建并检查健康'}
            </Btn>
          </div>
        </Card>
      )}

      <Card>
        {isLoading ? <div className="text-sm text-a-muted py-8 text-center">加载中...</div>
         : instances.length === 0 ? (
          <div className="text-center py-12 text-a-muted text-sm">
            <p className="mb-2">还没有 FlowBridge 实例</p>
            <p className="text-[11px] text-a-muted/60">创建实例后，在「流量管理 → 添加域名」中可把域名指向它</p>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-xs">
              <thead><tr className="border-b border-a-border text-a-muted text-left">
                <th className="py-2.5 px-3 font-medium">名称</th>
                <th className="py-2.5 px-3 font-medium">数据面</th>
                <th className="py-2.5 px-3 font-medium">控制面</th>
                <th className="py-2.5 px-3 font-medium">健康</th>
                <th className="py-2.5 px-3 font-medium">引用路由</th>
                <th className="py-2.5 px-3 font-medium"></th>
              </tr></thead>
              <tbody>
                {instances.map(inst => {
                  const hb = healthBadge(inst);
                  return (
                    <tr key={inst.id} className="border-b border-a-border/30 hover:bg-a-border/5">
                      <td className="py-2.5 px-3 font-medium text-a-fg">
                        {inst.name}
                        <span className={cn('ml-2 px-1.5 py-0.5 rounded text-[9px] font-medium border', hb.cls)}>{hb.label}</span>
                      </td>
                      <td className="py-2.5 px-3 font-mono text-[11px]">{inst.machine_ip}:{inst.data_plane_port}</td>
                      <td className="py-2.5 px-3 font-mono text-[11px] text-a-muted">{inst.control_address}</td>
                      <td className="py-2.5 px-3 text-[10px] text-a-muted">
                        {inst.enabled
                          ? (inst.last_health_status === 'unknown' ? '尚未检查' : `${inst.last_health_message || inst.last_health_status}${inst.last_health_latency_ms ? ` · ${inst.last_health_latency_ms}ms` : ''}`)
                          : '—'}
                      </td>
                      <td className="py-2.5 px-3 text-[10px] text-a-muted">{inst.route_ref_count ?? 0}</td>
                      <td className="py-2.5 px-3">
                        <div className="flex items-center gap-1">
                          <button onClick={() => check.mutate(inst.id)} disabled={check.isPending} className="text-[10px] px-2 py-0.5 rounded border border-a-border/40 text-a-muted hover:text-a-fg cursor-pointer">检查</button>
                          <button onClick={() => setEditing(inst)} className="text-[10px] px-2 py-0.5 rounded border border-a-border/40 text-a-muted hover:text-a-fg cursor-pointer">编辑</button>
                          <button onClick={() => toggle.mutate(inst)} className={cn('text-[10px] px-2 py-0.5 rounded border cursor-pointer',
                            inst.enabled ? 'border-[#e8b830]/30 text-[#e8b830] hover:bg-[#e8b830]/10' : 'border-[#4cd964]/30 text-[#4cd964] hover:bg-[#4cd964]/10')}>
                            {inst.enabled ? '禁用' : '启用'}
                          </button>
                          <button onClick={() => setDeleteTarget(inst)}
                            className="text-[10px] px-2 py-0.5 rounded border border-[#ff5c72]/30 text-[#ff5c72] hover:bg-[#ff5c72]/10 cursor-pointer">删除</button>
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

      {editing && (
        <Modal title={`编辑实例 — ${editing.name}`} onClose={() => setEditing(null)}
          footer={
            <>
              <Btn onClick={() => setEditing(null)}>取消</Btn>
              <Btn primary onClick={() => update.mutate()} disabled={update.isPending}>{update.isPending ? '保存中...' : '保存'}</Btn>
            </>
          }>
          <div className="space-y-3">
            {(['name', 'machine_ip', 'control_address'] as const).map(k => (
              <div key={k}>
                <label className="text-[10px] text-a-muted block mb-1.5 font-medium">{k === 'name' ? '名称' : k === 'machine_ip' ? '机器 IP' : '控制面地址'}</label>
                <input value={editing[k]} onChange={e => setEditing({ ...editing, [k]: e.target.value })} className={inputCls} />
              </div>
            ))}
            <div>
              <label className="text-[10px] text-a-muted block mb-1.5 font-medium">数据面端口</label>
              <input type="number" value={editing.data_plane_port} onChange={e => setEditing({ ...editing, data_plane_port: Number(e.target.value) })} className={inputCls} />
            </div>
          </div>
        </Modal>
      )}

      {/* Delete confirmation (app Modal, consistent with other pages) */}
      {deleteTarget && (
        <Modal title="删除实例" onClose={() => setDeleteTarget(null)}
          footer={
            <>
              <Btn onClick={() => setDeleteTarget(null)}>取消</Btn>
              <Btn danger onClick={() => { remove.mutate(deleteTarget.id); setDeleteTarget(null); }} disabled={remove.isPending}>
                {remove.isPending ? '删除中...' : '确认删除'}
              </Btn>
            </>
          }>
          <div className="space-y-2">
            <p className="text-sm text-a-fg">确定要删除实例 <span className="font-semibold">{deleteTarget.name}</span> 吗？</p>
            <p className="text-xs text-a-muted">被路由引用的实例无法删除（会返回 409），需先解除绑定。</p>
          </div>
        </Modal>
      )}
    </div>
  );
}

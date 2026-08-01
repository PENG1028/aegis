// ─── New Entry — create domain/port mapping ───
import { useState, useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useNavigate, Link } from 'react-router-dom';
import { runtimeModeApi, exposureApi, nodeApi, certApi, providerApi, flowbridgeApi } from '@/lib/api-bridge';
import type { Composition, CertificateItem, FlowBridgeInstance } from '@/lib/api-bridge';
import { Btn, useToast } from '@/components/shared';
import { routeDisplay } from '@/lib/route-display';
import { certificateCoversDomain, certificateIsCurrentlyValid } from '@/lib/certificate';
import { cn } from '@/lib/utils';
import { capabilityIsReady, type ProviderCapabilityView } from '@/lib/provider-capability';

function entryType(c: Composition) {
  if (c.atoms?.includes('udp')) return 'udp';
  if (c.atoms?.includes('http')) return 'http';
  return 'tcp';
}

interface NodeInfo { id: string; name: string; privateIP: string; publicIP: string; networkID: string; region: string; }

function deriveLabel(n: NodeInfo, all: NodeInfo[]) {
  if (n.networkID) return { label: n.networkID, color: 'bg-blue-500/10 text-blue-400 border-blue-500/20', forced: false };
  const dup = all.filter(o => o.id !== n.id && o.privateIP && o.privateIP === n.privateIP);
  if (dup.length > 0 && n.privateIP) return { label: n.privateIP, color: 'bg-[#e8b830]/10 text-[#e8b830] border-[#e8b830]/20', forced: true };
  if (n.privateIP) { const p = n.privateIP.split('.'); if (p.length===4) return { label: `${p[0]}.${p[1]}.x.x`, color: 'bg-a-border/10 text-a-muted border-a-border/20', forced: false }; }
  return { label: n.region||'', color: 'bg-a-border/10 text-a-muted border-a-border/20', forced: false };
}

export default function NewEntry() {
  const toast = useToast(); const nav = useNavigate();
  const [comp, setComp] = useState('HTTPS Route');
  const [domain, setDomain] = useState('');
  const [internalOnly, setInternalOnly] = useState(false);
  const [nodeId, setNodeId] = useState('');
  const [targetHost, setTargetHost] = useState('127.0.0.1');
  const [targetPort, setTargetPort] = useState(3000);
  const [submitting, setSubmitting] = useState(false);
  const [certMode, setCertMode] = useState<'auto' | 'manual'>('auto');
  const [certId, setCertId] = useState('');
  // v1.9C-2: target kind — "backend" (IP:port) or "flowbridge" (managed instance)
  const [targetKind, setTargetKind] = useState<'backend' | 'flowbridge'>('backend');
  const [flowbridgeId, setFlowbridgeId] = useState('');

  const { data: rm } = useQuery({ queryKey: ['runtime-mode'], queryFn: () => runtimeModeApi.get(), refetchInterval: 60_000 });
  const { data: nd } = useQuery({ queryKey: ['nodes'], queryFn: () => nodeApi.list().catch(() => ({ nodes: [] })), refetchInterval: 120_000 });
  const { data: certData } = useQuery({ queryKey: ['certificates'], queryFn: () => certApi.list(), refetchInterval: 60_000 });
  const { data: providerData } = useQuery({ queryKey: ['providers'], queryFn: () => providerApi.list() as Promise<{ providers: ProviderCapabilityView[] }> });
  const { data: fbData } = useQuery({
    queryKey: ['flowbridge'],
    queryFn: () => flowbridgeApi.list().catch(() => ({ data: [] as FlowBridgeInstance[], meta: { total: 0 } })),
    refetchInterval: 60_000,
  });

  const compositions = rm?.current?.compositions || [];
  const certs: CertificateItem[] = (certData as any)?.certificates || [];
  const flowbridgeInstances: FlowBridgeInstance[] = (fbData as any)?.data || [];
  const enabledInstances = flowbridgeInstances.filter(i => i.enabled);
  const bindableCerts = certs.filter(c =>
    c.record_type !== 'provider_observation'
    && c.source !== 'gateway_auto'
    && certificateIsCurrentlyValid(c)
    && certificateCoversDomain(c, domain),
  );
  // HTTPS composition availability and automatic TLS availability are separate.
  const selectedComp = compositions.find((c: Composition) => c.name === comp);
  const activeExecutorIDs = (rm?.current?.providers || []).map((item: any) => item.provider_id);
  const hasAutoCert = selectedComp?.status === 'available'
    && capabilityIsReady(providerData?.providers || [], 'auto_cert', activeExecutorIDs);

  const nodes: NodeInfo[] = ((nd as any)?.nodes || []).map((n: any) => ({
    id: n.id||n.node_id, name: n.name||n.node_id||n.id, privateIP: n.private_ip||'', publicIP: n.public_ip||'', networkID: n.network_id||'', region: n.region||'',
  }));
  const nodeLabels = useMemo(() => { const m: Record<string, ReturnType<typeof deriveLabel>> = {}; nodes.forEach(n => { m[n.id] = deriveLabel(n, nodes); }); return m; }, [nodes]);

  const selected = compositions.find(c => c.name === comp);
  const isHTTP = selected ? entryType(selected) === 'http' : true;
  const compStatus = selected?.status || 'unsupported';
  const canUse = compStatus === 'available';
  const tlsSelectionValid = certMode === 'auto' ? hasAutoCert : bindableCerts.some(c => c.id === certId);
  const targetValid = targetKind === 'flowbridge'
    ? Boolean(flowbridgeId)
    : Boolean(targetHost && targetPort > 0);
  const canSubmit = canUse && (isHTTP
    ? Boolean(domain && targetValid && (!selectedComp?.atoms?.includes('tls') || tlsSelectionValid))
    : Boolean(targetHost && targetPort > 0));

  const handleSubmit = async () => {
    if (!canSubmit) return;
    setSubmitting(true);
    try {
      if (isHTTP) {
        const body: any = { domain, target_host: targetHost, target_port: targetPort, cert_id: certMode === 'manual' ? certId : '' };
        if (targetKind === 'flowbridge') {
          body.target_host = '';
          body.target_port = 0;
          body.flowbridge_id = flowbridgeId;
        }
        const res = await fetch('/api/v1/actions/bind-http-domain', { method:'POST', headers:{'Content-Type':'application/json'}, credentials:'include',
          body: JSON.stringify(body) });
        if (!res.ok) { const e = await res.json().catch(()=>({})); throw new Error((e as any).error?.message||`HTTP ${res.status}`); }
      } else {
        await exposureApi.create({ type: entryType(selected!), target_host: targetHost, target_port: targetPort });
      }
      toast('入口创建成功，配置已自动 Apply'); nav('/exposure');
    } catch (e: any) { toast(e.message||'创建失败','error'); }
    finally { setSubmitting(false); }
  };

  const selectedNode = nodes.find(n => n.id === nodeId);

  return (
    <div className="p-6 space-y-6">
      <div>
        <h2 className="text-lg font-bold text-a-fg">添加域名</h2>
        <p className="text-xs text-a-muted mt-1">创建域名映射或端口转发，系统自动处理路由和证书</p>
      </div>

      {/* Service type — all options visible, status inline */}
      <div>
        <div className="text-xs font-medium text-a-fg mb-3">服务类型</div>
        <div className="flex gap-2 flex-wrap">
          {compositions.map(c => {
            const ok = c.status === 'available';
            const sel = c.name === comp;
            return (
              <button key={c.name} onClick={() => setComp(c.name)}
                className={cn('px-3 py-2.5 rounded-a-sm text-xs border transition-colors min-w-[130px] text-left',
                  ok ? 'bg-a-surface border-a-border/30 hover:border-[#4cd964]/40 cursor-pointer' : 'bg-a-border/5 border-a-border/20 opacity-50 cursor-not-allowed',
                  sel && 'ring-2 ring-a-accent/50')}>
                <div className="font-medium text-a-fg">{c.name}</div>
                <div className="text-[10px] text-a-muted mt-0.5">{c.chain}</div>
                {!ok && <div className="text-[9px] text-[#ff5c72]/70 mt-1">{c.status==='missing_provider'?'需安装中间件':'模式不支持'}</div>}
              </button>
            );
          })}
        </div>
        {!canUse && selected && (
          <div className="mt-2 text-[10px] text-[#ff5c72]/80">
            {compStatus === 'missing_provider' ? '所选类型缺少可用执行器' : '当前运行模式不支持此入口类型'}
          </div>
        )}
      </div>

      {/* Config */}
      <div className="space-y-4 p-5 rounded-a-md border border-a-border/30 bg-a-surface/50">
        <div className="text-xs font-medium text-a-fg">目标配置</div>

        <div>
          <label className="text-[10px] text-a-muted block mb-1.5 font-medium">目标节点</label>
          <select value={nodeId} onChange={e => { setNodeId(e.target.value); const n = nodes.find(x => x.id===e.target.value); if (n) setTargetHost(n.privateIP||n.publicIP||'127.0.0.1'); }}
            className="w-full px-3 py-2 rounded-a-sm border border-a-border/50 bg-a-bg text-xs outline-none focus:border-a-accent/50">
            <option value="">本节点 (127.0.0.1)</option>
            {nodes.map(n => <option key={n.id} value={n.id}>{n.name} · {n.privateIP||'—'}{n.publicIP?` · ${n.publicIP}`:''}{nodeLabels[n.id]?.label?` · ${nodeLabels[n.id].label}`:''}</option>)}
          </select>
          {/* Always show IP tags for current selection or default */}
          <div className="flex items-center gap-2 mt-1.5">
            <span className="px-1.5 py-0.5 rounded text-[9px] font-mono bg-blue-500/10 text-blue-400 border border-blue-500/20">
              内网 {selectedNode?.privateIP || '—'}
            </span>
            <span className={cn('px-1.5 py-0.5 rounded text-[9px] font-mono border',
              selectedNode?.publicIP ? 'bg-a-border/10 text-a-muted border-a-border/20' : 'bg-a-border/5 text-a-muted/40 border-a-border/20')}>
              公网 {selectedNode?.publicIP || '—'}
            </span>
            {!selectedNode && <span className="text-[9px] text-a-muted/50">（选择节点后显示实际 IP）</span>}
            {selectedNode && nodeLabels[selectedNode.id]?.label && (
              <span className={cn('px-1.5 py-0.5 rounded text-[9px] font-medium border', nodeLabels[selectedNode.id].color)}>
                {nodeLabels[selectedNode.id].forced ? '⚠ 重复内网IP — 需指定网络: ' : ''}{nodeLabels[selectedNode.id].label}
              </span>
            )}
          </div>
          {/* Duplicate IP detected → force network label input */}
          {selectedNode && nodeLabels[selectedNode.id]?.forced && (
            <input placeholder="请输入网络标识（如：阿里云-杭州）"
              className="w-full mt-1.5 px-3 py-1.5 rounded-a-sm border border-[#e8b830]/50 bg-[#e8b830]/5 text-xs outline-none focus:border-[#e8b830]" />
          )}
        </div>

        {isHTTP && (
          <div>
            <label className="text-[10px] text-a-muted block mb-1.5 font-medium">域名</label>
            <input value={domain} onChange={e => setDomain(e.target.value)} placeholder="api.example.com"
              className="w-full px-3 py-2 rounded-a-sm border border-a-border/50 bg-a-bg text-xs outline-none focus:border-a-accent/50" />
            <label className="flex items-center gap-1.5 mt-1.5 text-[10px] text-a-muted cursor-pointer">
              <input type="checkbox" checked={internalOnly} onChange={e => setInternalOnly(e.target.checked)} className="w-3 h-3" />仅内部使用（集群内服务间调用）
            </label>
          </div>
        )}

        {/* TLS Certificate selection */}
        {isHTTP && selectedComp?.atoms?.includes('tls') && (
          <div>
            <label className="text-[10px] text-a-muted block mb-1.5 font-medium">TLS 证书</label>
            <div className="flex gap-2">
              <button
                onClick={() => { setCertMode('auto'); setCertId(''); }}
                className={cn('px-3 py-1.5 rounded-a-sm text-xs border transition-colors cursor-pointer',
                  certMode === 'auto' ? 'bg-a-accent/10 border-a-accent/50 text-a-accent' : 'bg-a-bg border-a-border/30 text-a-muted hover:border-a-border/50',
                  !hasAutoCert && 'opacity-50 cursor-not-allowed')}
                disabled={!hasAutoCert}
                title={hasAutoCert ? '由当前自动 TLS 执行链负责签发和续期' : '当前执行链不能提供自动 TLS，请选择已有证书资产'}
              >
                {hasAutoCert ? '自动 TLS' : '自动 TLS — 不可用'}
              </button>
              <button
                onClick={() => setCertMode('manual')}
                className={cn('px-3 py-1.5 rounded-a-sm text-xs border transition-colors cursor-pointer',
                  certMode === 'manual' ? 'bg-a-accent/10 border-a-accent/50 text-a-accent' : 'bg-a-bg border-a-border/30 text-a-muted hover:border-a-border/50')}
              >
                手动指定
              </button>
            </div>
			{certMode === 'auto' && !hasAutoCert && (
			  <p className="mt-2 text-[10px] text-[#e8b830]">
				当前执行链的自动 TLS 能力未就绪，请恢复执行器或改用证书资产。
			  </p>
			)}
			{certMode === 'auto' && bindableCerts.length > 0 && (
			  <button type="button" onClick={() => setCertMode('manual')}
				className="mt-2 text-left text-[10px] text-a-accent hover:underline">
				发现 {bindableCerts.length} 张覆盖当前域名的证书资产，可手动选择
			  </button>
			)}
            {certMode === 'manual' && (
              <>
                <select value={certId} onChange={e => setCertId(e.target.value)}
                  className="w-full mt-2 px-3 py-2 rounded-a-sm border border-a-border/50 bg-a-bg text-xs outline-none focus:border-a-accent/50">
                  <option value="">选择证书...</option>
                  {bindableCerts.map((c: CertificateItem) => {
                    let domains = c.domains;
                    try { domains = JSON.parse(c.domains).join(', '); } catch {}
                    const expDate = new Date(c.not_after).toLocaleDateString('zh-CN');
                    return (
                      <option key={c.id} value={c.id}>{domains} · 到期 {expDate}</option>
                    );
                  })}
                </select>
                {/* Selected cert preview */}
                {certId && (() => {
                  const selected = bindableCerts.find(c => c.id === certId);
                  if (!selected) return null;
                  let domains = selected.domains; try { domains = JSON.parse(selected.domains).join(', '); } catch {}
                  const es = (notAfter: string) => {
                    const d = new Date(notAfter); const days = Math.floor((d.getTime() - Date.now()) / 86400000);
                    if (days < 0) return { label: '已过期', cls: 'text-[#ff5c72]' };
                    if (days <= 30) return { label: `${days} 天后过期`, cls: 'text-[#e8b830]' };
                    return { label: '有效', cls: 'text-[#4cd964]' };
                  };
                  const s = es(selected.not_after);
                  return (
                    <div className="flex items-center gap-2 mt-1.5 text-[10px] bg-a-border/5 border border-a-border/20 rounded-a-sm px-2 py-1">
                      <span className="text-a-fg font-mono">{domains}</span>
                      <span className="text-a-border">·</span>
                      <span className={s.cls}>{s.label}</span>
                      <span className="text-a-border">·</span>
                      <span className="text-a-muted">{selected.issuer.split(',')[0]?.replace('CN=', '') || selected.issuer}</span>
                    </div>
                  );
                })()}
              </>
            )}
            {certMode === 'manual' && bindableCerts.length === 0 && (
              <div className="mt-1.5 text-[10px] text-a-muted bg-a-border/5 border border-a-border/20 rounded-a-sm px-2 py-1.5">
                暂无可用证书 ·
                <Link to="/access/certificates" className="text-a-accent hover:underline ml-0.5">前往证书管理 →</Link>
              </div>
            )}
          </div>
        )}

        {/* Target kind — only for HTTP entries (v1.9C-2 flowbridge support) */}
        {isHTTP && (
          <div>
            <label className="text-[10px] text-a-muted block mb-1.5 font-medium">后端类型</label>
            <div className="flex gap-2">
              <button
                onClick={() => { setTargetKind('backend'); setFlowbridgeId(''); }}
                className={cn('px-3 py-1.5 rounded-a-sm text-xs border transition-colors cursor-pointer',
                  targetKind === 'backend' ? 'bg-a-accent/10 border-a-accent/50 text-a-accent' : 'bg-a-bg border-a-border/30 text-a-muted hover:border-a-border/50')}>
                旧后端 (IP:端口)
              </button>
              <button
                onClick={() => setTargetKind('flowbridge')}
                className={cn('px-3 py-1.5 rounded-a-sm text-xs border transition-colors cursor-pointer',
                  targetKind === 'flowbridge' ? 'bg-a-accent/10 border-a-accent/50 text-a-accent' : 'bg-a-bg border-a-border/30 text-a-muted hover:border-a-border/50')}>
                FlowBridge 实例
              </button>
            </div>
            {targetKind === 'flowbridge' && (
              <>
                <select value={flowbridgeId} onChange={e => setFlowbridgeId(e.target.value)}
                  className="w-full mt-2 px-3 py-2 rounded-a-sm border border-a-border/50 bg-a-bg text-xs outline-none focus:border-a-accent/50">
                  <option value="">选择 FlowBridge 实例...</option>
                  {enabledInstances.map(i => (
                    <option key={i.id} value={i.id}>{i.name} · {i.machine_ip}:{i.data_plane_port}{i.last_health_status === 'healthy' ? ' · 健康' : ''}</option>
                  ))}
                </select>
                {enabledInstances.length === 0 && (
                  <div className="mt-1.5 text-[10px] text-a-muted bg-a-border/5 border border-a-border/20 rounded-a-sm px-2 py-1.5">
                    暂无可用实例 ·
                    <Link to="/fabric/flowbridge" className="text-a-accent hover:underline ml-0.5">前往实例管理 →</Link>
                  </div>
                )}
              </>
            )}
          </div>
        )}

        <div className="grid grid-cols-2 gap-4">
          {targetKind === 'backend' && (
            <>
              <div>
                <label className="text-[10px] text-a-muted block mb-1.5 font-medium">后端地址</label>
                <input value={targetHost} onChange={e => setTargetHost(e.target.value)} placeholder={selectedNode?.privateIP||'127.0.0.1'}
                  className="w-full px-3 py-2 rounded-a-sm border border-a-border/50 bg-a-bg text-xs outline-none focus:border-a-accent/50 font-mono" />
              </div>
              <div>
                <label className="text-[10px] text-a-muted block mb-1.5 font-medium">后端端口</label>
                <input type="number" value={targetPort} onChange={e => setTargetPort(Number(e.target.value))}
                  className="w-full px-3 py-2 rounded-a-sm border border-a-border/50 bg-a-bg text-xs outline-none focus:border-a-accent/50 font-mono" />
              </div>
            </>
          )}
          {targetKind === 'flowbridge' && (
            <div className="col-span-2">
              <p className="text-[10px] text-a-muted bg-a-border/5 border border-a-border/20 rounded-a-sm px-2 py-1.5">
                域名绑定实例后，流量以实例为准：Aegis 只做 TLS 终止 + 转发（保留 Host 头），
                实例内部的路由/切换/健康由 FlowBridge 自己管理。
              </p>
            </div>
          )}
        </div>

        <Btn primary disabled={!canSubmit||submitting} onClick={handleSubmit}>
          {submitting ? '创建中...' : canUse ? '创建并 Apply' : '不可用 — 需先安装中间件'}
        </Btn>
      </div>
    </div>
  );
}

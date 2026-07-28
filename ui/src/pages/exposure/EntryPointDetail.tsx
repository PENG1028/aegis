// ─── Entry Detail — route info, backend address, certificate, health ───
// Shows everything about a route in one place without forcing the user
// to navigate to service/cert pages for basic information.

import { useState } from 'react';
import { useParams, useNavigate, Link } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { routeApi, runtimeModeApi, providerApi } from '@/lib/api-bridge';
import { viewStore } from '@/lib/view-store';
import { routeDisplay, certSourceLabel } from '@/lib/route-display';
import { certificateCoversDomain, certificateIsCurrentlyValid } from '@/lib/certificate';
import { Card, StatusBadge, Btn, Modal, useToast } from '@/components/shared';
import { cn } from '@/lib/utils';
import { capabilityIsReady, type ProviderCapabilityView } from '@/lib/provider-capability';

function authHeaders(): Record<string, string> {
  const viewAs = viewStore.headerValue;
  return viewAs ? { 'Content-Type': 'application/json', 'X-Aegis-View-As': viewAs } : { 'Content-Type': 'application/json' };
}

function parseDomains(domains: string): string {
  try { const arr = JSON.parse(domains); return Array.isArray(arr) ? arr.join(', ') : domains; }
  catch { return domains; }
}

export default function EntryPointDetail() {
  const { entryId } = useParams<{ entryId: string }>();
  const nav = useNavigate(); const toast = useToast(); const qc = useQueryClient();
	const [showTLSBinding, setShowTLSBinding] = useState(false);
	const [selectedCertID, setSelectedCertID] = useState('');

  // ── Route detail ──
  const { data: route, isLoading: rl } = useQuery({
    queryKey: ['route-detail', entryId],
    queryFn: async () => {
      const res = await fetch(`/api/admin/v1/routes/${entryId}`, { credentials: 'include', headers: authHeaders() });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      return res.json();
    },
    enabled: !!entryId,
  });

  // ── Service info (name) ──
  const { data: svc } = useQuery({
    queryKey: ['service', route?.service_id],
    queryFn: async () => {
      const res = await fetch(`/api/admin/v1/services/${route.service_id}`, { credentials: 'include', headers: authHeaders() });
      if (!res.ok) return null;
      return res.json();
    },
    enabled: !!route?.service_id,
  });

  // ── Endpoints for this service ──
  const { data: eps } = useQuery({
    queryKey: ['endpoints', route?.service_id],
    queryFn: async () => {
      const res = await fetch(`/api/admin/v1/services/${route.service_id}/endpoints`, { credentials: 'include', headers: authHeaders() });
      if (!res.ok) return [];
      const data = await res.json();
      return data.endpoints || data || [];
    },
    enabled: !!route?.service_id,
  });

  // ── Certificate (if bound) ──
  const { data: certData } = useQuery({
    queryKey: ['certificates'],
    queryFn: async () => {
      const res = await fetch('/api/admin/v1/certificates', { credentials: 'include', headers: authHeaders() });
      if (!res.ok) return [];
      return res.json();
    },
  });
  const certs = certData?.certificates || [];
  const cert = route?.cert_id ? certs.find((c: any) => c.id === route.cert_id) : null;
	const assetCerts = certs.filter((c: any) =>
		c.record_type !== 'provider_observation'
		&& c.source !== 'gateway_auto'
		&& certificateIsCurrentlyValid(c));

  // ── Runtime mode ──
  const { data: rm } = useQuery({
    queryKey: ['runtime-mode'], queryFn: () => runtimeModeApi.get(), refetchInterval: 60_000,
  });
  const { data: providerData } = useQuery({
    queryKey: ['providers'],
    queryFn: () => providerApi.list() as Promise<{ providers: ProviderCapabilityView[] }>,
    refetchInterval: 60_000,
  });
  const { data: routeCapability } = useQuery({
    queryKey: ['route-capability', entryId],
    queryFn: async () => {
      const res = await fetch(`/api/admin/v1/routes/${entryId}/capability-status`, { credentials: 'include', headers: authHeaders() });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      return res.json();
    },
    enabled: !!entryId,
  });

  // ── Mutations ──
  const disableMutation = useMutation({
    mutationFn: async () => {
      const res = await fetch(`/api/admin/v1/routes/${entryId}/disable`, { method: 'POST', credentials: 'include', headers: authHeaders() });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
    },
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['route-detail', entryId] }); toast('已禁用'); },
    onError: (e: any) => toast(e.message || '失败', 'error'),
  });
  const enableMutation = useMutation({
    mutationFn: async () => {
      const res = await fetch(`/api/admin/v1/routes/${entryId}/enable`, { method: 'POST', credentials: 'include', headers: authHeaders() });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
    },
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['route-detail', entryId] }); toast('已启用'); },
    onError: (e: any) => toast(e.message || '失败', 'error'),
  });
	const tlsBindingMutation = useMutation({
		mutationFn: () => selectedCertID === '__provider_auto__'
			? routeApi.setTLSBinding(entryId!, {
				mode: 'provider_auto',
			})
			: routeApi.setTLSBinding(entryId!, { mode: 'certificate', cert_id: selectedCertID }),
		onSuccess: (result: any) => {
			qc.invalidateQueries({ queryKey: ['route-detail', entryId] });
			qc.invalidateQueries({ queryKey: ['certificates'] });
			qc.invalidateQueries({ queryKey: ['route-capability', entryId] });
			qc.invalidateQueries({ queryKey: ['providers'] });
			qc.invalidateQueries({ queryKey: ['runtime-mode'] });
			setShowTLSBinding(false);
			toast(result?.status === 'pending_apply' ? 'TLS 绑定已保存，等待配置发布' : 'TLS 绑定已更新');
		},
		onError: (e: any) => toast(e.message || 'TLS 绑定更新失败', 'error'),
	});

  if (rl) return <div className="p-6 text-a-muted text-sm">加载中...</div>;
  if (!route) return <div className="p-6 text-a-muted text-sm">未找到入口 {entryId}</div>;

  const active = route.status === 'active';
  const kind = svc?.kind || '';
  const rd = routeDisplay({ ...route, kind });
  const comp = (rm?.current?.compositions || []).find((c: any) => c.name === rd.typeLabel);
  const providers = providerData?.providers || [];
  const activeExecutorIDs = (rm?.current?.providers || []).map((item: any) => item.provider_id);
  const routeChainReady = rd.isHTTP ? comp?.status === 'available' : true;
  const tlsReady = !rd.tlsActive
    || (route.tls_binding_mode === 'certificate'
      ? capabilityIsReady(providers, 'load_cert', activeExecutorIDs)
      : capabilityIsReady(providers, 'auto_cert', route.tls_provider ? [route.tls_provider] : activeExecutorIDs));
  const entryHealthy = routeChainReady && tlsReady;
  const mode = rm?.current?.label || 'Legacy';

  const serviceName = svc?.name || route.service_id;
  const endpoints: any[] = Array.isArray(eps) ? eps : [];
  const hasEndpoints = endpoints.length > 0;

  // Cert expiry
  const certExpiry = (notAfter: string) => {
    const d = new Date(notAfter);
    const days = Math.floor((d.getTime() - Date.now()) / 86400000);
    if (days < 0) return { label: '已过期', cls: 'text-[#ff5c72]' };
    if (days <= 30) return { label: `${days} 天后过期`, cls: 'text-[#e8b830]' };
    return { label: `${days} 天后`, cls: 'text-[#4cd964]' };
  };

  return (
    <div className="p-6 space-y-5">
      {/* ── Header ── */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-bold text-a-fg">{route.domain}</h2>
          <p className="text-xs text-a-muted mt-1">
            {rd.typeLabel} · <StatusBadge status={active ? 'active' : 'disabled'} /> · {mode} 模式
          </p>
        </div>
        <div className="flex gap-2">
          {active
            ? <Btn onClick={() => disableMutation.mutate()} className="text-xs">禁用</Btn>
            : <Btn onClick={() => enableMutation.mutate()} className="text-xs">启用</Btn>}
          <Link to="/release/apply"><Btn className="text-xs">发布配置</Btn></Link>
          <Btn onClick={() => nav('/exposure')} className="text-xs">返回列表</Btn>
        </div>
      </div>

	  {rd.tlsActive && (
		<div className="flex flex-wrap items-center justify-between gap-3 border-y border-a-border/30 bg-a-surface/40 px-3 py-2.5">
		  <div className="min-w-0">
			<div className="text-xs font-medium text-a-fg">TLS 管理</div>
			<div className="mt-0.5 text-[11px] text-a-muted">
			  {route.tls_binding_mode === 'certificate' && cert
				? `指定证书 · ${parseDomains(cert.domains)}`
				: `自动 TLS · ${route.tls_provider || '当前执行链'}`}
			</div>
		  </div>
		  <Btn onClick={() => {
			setSelectedCertID(route.tls_binding_mode === 'certificate' && route.cert_id ? route.cert_id : '__provider_auto__');
			setShowTLSBinding(true);
		  }}>更改绑定</Btn>
		</div>
	  )}

      {/* ── Health ── */}
      <Card title="运行状态">
        <div className="space-y-2">
          {/* Execution chain health */}
          <div className={cn('flex items-center gap-3 px-3 py-2.5 rounded-a-sm border text-xs',
            entryHealthy ? 'bg-[#4cd964]/5 border-[#4cd964]/15' : 'bg-[#ff5c72]/5 border-[#ff5c72]/15')}>
            <span className={cn('font-mono text-sm shrink-0', entryHealthy ? 'text-[#4cd964]' : 'text-[#ff5c72]')}>
              {entryHealthy ? '✓' : '✗'}
            </span>
            <span className="font-medium w-24 shrink-0">执行链</span>
            <span className={entryHealthy ? 'text-a-muted' : 'text-[#ff5c72]/80'}>
              {entryHealthy ? '路由与 TLS 能力均已就绪' : !routeChainReady ? '路由执行能力不可用' : 'TLS 执行能力不可用，需要恢复原执行器或显式迁移'}
            </span>
          </div>

          {/* Endpoint health */}
          {endpoints.map((ep: any, i: number) => (
            <div key={ep.id || i} className={cn('flex items-center gap-3 px-3 py-2.5 rounded-a-sm border text-xs',
              ep.enabled ? 'bg-[#4cd964]/5 border-[#4cd964]/15' : 'bg-[#ff5c72]/5 border-[#ff5c72]/15')}>
              <span className={cn('font-mono text-sm shrink-0', ep.enabled ? 'text-[#4cd964]' : 'text-[#ff5c72]')}>
                {ep.enabled ? '✓' : '✗'}
              </span>
              <span className="font-medium w-24 shrink-0">后端 {i + 1}</span>
              <span className={ep.enabled ? 'text-a-muted' : 'text-[#ff5c72]/80'}>
                {ep.address} <span className="text-a-muted/50">({ep.type})</span>
                {!ep.enabled && ' — 已禁用'}
              </span>
            </div>
          ))}

          {!hasEndpoints && (
            <div className="flex items-center gap-3 px-3 py-2.5 rounded-a-sm border text-xs bg-a-border/5 border-a-border/20">
              <span className="font-mono text-sm shrink-0 text-a-muted">—</span>
              <span className="font-medium w-24 shrink-0">后端</span>
              <span className="text-a-muted">
                未配置端点 —
                <Link to={`/exposure/service/${route.service_id}`} className="text-a-accent hover:underline ml-1">
                  去服务页添加 →
                </Link>
              </span>
            </div>
          )}
        </div>
      </Card>

      {/* ── Main info grid ── */}
	  <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <Card title="路由信息">
          <div className="space-y-2 text-xs">
            <Row label="域名" value={route.domain} mono />
            <Row label="路径" value={route.path_prefix || '/'} mono />
            <Row label="类型" value={rd.typeLabel} />
            <Row label="状态" badge={<StatusBadge status={active ? 'active' : 'disabled'} />} />
            <Row label="来源" value={route.owner_type === 'system' ? '系统（面板）' : route.owner_type === 'space' ? '服务' : '管理员'} />
			<Row label="执行归属" value={routeCapability?.provider || route.source_provider || '未分配'} mono />
          </div>
        </Card>

        <Card title="后端服务">
          <div className="space-y-2 text-xs">
            <Row label="服务名称" value={serviceName} />
            <Row label="Service ID" value={route.service_id} mono />
            {endpoints.map((ep: any, i: number) => (
              <Row key={i} label={`后端地址 ${i + 1}`} value={`${ep.address} (${ep.type})`} mono />
            ))}
            {!hasEndpoints && <Row label="后端地址" value="未配置" />}
            <div className="pt-1">
              <Link to={`/exposure/service/${route.service_id}`} className="text-[10px] text-a-accent hover:underline">
                查看服务详情 →
              </Link>
            </div>
          </div>
        </Card>

        <Card title="TLS 证书">
          <div className="space-y-2 text-xs">
            {cert ? (
              <>
                <Row label="证书 ID" value={cert.id} mono />
                <Row label="域名" value={parseDomains(cert.domains)} mono />
                <Row label="来源" value={certSourceLabel(cert.source)} />
                <Row label="签发者" value={cert.issuer.split(',')[0]?.replace('CN=', '') || cert.issuer} />
                <Row label="到期" badge={
                  <span className={cn('px-1.5 py-0.5 rounded text-[10px] font-medium',
                    certExpiry(cert.not_after).cls === 'text-[#ff5c72]' ? 'bg-[#ff5c72]/10 text-[#ff5c72]' :
                    certExpiry(cert.not_after).cls === 'text-[#e8b830]' ? 'bg-[#e8b830]/10 text-[#e8b830]' :
                    'bg-[#4cd964]/10 text-[#4cd964]')}>
                    {certExpiry(cert.not_after).label}
                  </span>
                } />
                {cert.auto_renew && (
				  <Row label="续期" value={cert.source === 'gateway_auto' ? '执行器托管' : 'Aegis ACME 自动续期'} />
                )}
                <div className="pt-1">
                  <Link to="/access/certificates" className="text-[10px] text-a-accent hover:underline">
                    证书管理 →
                  </Link>
                </div>
              </>
            ) : (
              <>
                <div className="text-a-muted">
                  {route.cert_id === '' || !route.cert_id
                    ? (rd.isHTTP ? `由自动 TLS 执行链负责签发和续期${route.tls_provider ? `（${route.tls_provider}）` : ''}` : '独立 TLS 加密，不由 Aegis 管理')
                    : '证书信息加载中...'}
                </div>
                <div className="pt-1">
                  <Link to="/access/certificates" className="text-[10px] text-a-accent hover:underline">
                    证书管理 →
                  </Link>
                </div>
              </>
            )}
          </div>
        </Card>

        <Card title="模式信息">
          <div className="space-y-2 text-xs">
            <Row label="当前模式" value={mode} />
            <Row label="组合能力" value={rd.typeLabel} />
            <Row label="执行链" value={entryHealthy ? '已就绪' : '未就绪'} />
            <Row label="TLS" value={rd.tlsActive ? `${rd.tlsLabel}${rd.port > 0 ? ` (:${rd.port})` : ''}` : '关闭'} />
            <Row label="创建时间" value={route.created_at || '—'} mono />
            <div className="pt-1">
              <Link to="/fabric/mode" className="text-[10px] text-a-accent hover:underline">
                模式管理 →
              </Link>
            </div>
          </div>
        </Card>
      </div>

	  {showTLSBinding && (
		<Modal title="TLS 管理方式" onClose={() => setShowTLSBinding(false)}
		  footer={
			<>
			  <Btn onClick={() => setShowTLSBinding(false)}>取消</Btn>
			  <Btn primary onClick={() => tlsBindingMutation.mutate()}
				disabled={tlsBindingMutation.isPending || !selectedCertID}>
				{tlsBindingMutation.isPending ? '应用中...' : '应用绑定'}
			  </Btn>
			</>
		  }>
		  <div className="space-y-3">
			<label className="block text-xs font-medium text-a-muted" htmlFor="tls-binding-select">管理方式</label>
			<select id="tls-binding-select" value={selectedCertID} onChange={e => setSelectedCertID(e.target.value)}
			  className="min-h-11 w-full rounded-a-sm border border-a-border bg-a-bg px-3 text-sm text-a-fg outline-none focus:border-a-accent">
			  <option value="__provider_auto__">自动 TLS</option>
			  {assetCerts.filter((item: any) => certificateCoversDomain(item, route.domain)).map((item: any) => (
				<option key={item.id} value={item.id}>{parseDomains(item.domains)} · {certSourceLabel(item.source)}</option>
			  ))}
			</select>
			<p className="text-xs leading-5 text-a-muted">
			  自动 HTTPS 的签发和续期由中间件负责；指定证书由 Aegis 加载，删除前必须先解除所有域名绑定。
			</p>
			{assetCerts.filter((item: any) => certificateCoversDomain(item, route.domain)).length === 0 && (
			  <p className="text-xs text-[#e8b830]">当前没有覆盖 {route.domain} 的证书资产。</p>
			)}
		  </div>
		</Modal>
	  )}
    </div>
  );
}

function Row({ label, value, badge, mono }: { label: string; value?: string; badge?: any; mono?: boolean }) {
  return (
    <div className="flex justify-between items-center gap-3">
      <span className="text-a-muted shrink-0">{label}</span>
      {badge || <span className={cn('text-right truncate', mono && 'font-mono text-[11px]')}>{value || '—'}</span>}
    </div>
  );
}

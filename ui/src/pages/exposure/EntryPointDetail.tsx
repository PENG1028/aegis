// ─── Entry Detail — route info, backend address, certificate, health ───
// Shows everything about a route in one place without forcing the user
// to navigate to service/cert pages for basic information.

import { useParams, useNavigate, Link } from 'react-router-dom';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { runtimeModeApi } from '@/lib/api-bridge';
import { Card, StatusBadge, Btn, useToast } from '@/components/shared';
import { cn } from '@/lib/utils';

function parseDomains(domains: string): string {
  try { const arr = JSON.parse(domains); return Array.isArray(arr) ? arr.join(', ') : domains; }
  catch { return domains; }
}

export default function EntryPointDetail() {
  const { entryId } = useParams<{ entryId: string }>();
  const nav = useNavigate(); const toast = useToast(); const qc = useQueryClient();

  // ── Route detail ──
  const { data: route, isLoading: rl } = useQuery({
    queryKey: ['route-detail', entryId],
    queryFn: async () => {
      const res = await fetch(`/api/admin/v1/routes/${entryId}`, { credentials: 'include' });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      return res.json();
    },
    enabled: !!entryId,
  });

  // ── Service info (name) ──
  const { data: svc } = useQuery({
    queryKey: ['service', route?.service_id],
    queryFn: async () => {
      const res = await fetch(`/api/admin/v1/services/${route.service_id}`, { credentials: 'include' });
      if (!res.ok) return null;
      return res.json();
    },
    enabled: !!route?.service_id,
  });

  // ── Endpoints for this service ──
  const { data: eps } = useQuery({
    queryKey: ['endpoints', route?.service_id],
    queryFn: async () => {
      const res = await fetch(`/api/admin/v1/services/${route.service_id}/endpoints`, { credentials: 'include' });
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
      const res = await fetch('/api/admin/v1/certificates', { credentials: 'include' });
      if (!res.ok) return [];
      return res.json();
    },
  });
  const certs = certData?.certificates || [];
  const cert = route?.cert_id ? certs.find((c: any) => c.id === route.cert_id) : null;

  // ── Runtime mode ──
  const { data: rm } = useQuery({
    queryKey: ['runtime-mode'], queryFn: () => runtimeModeApi.get(), refetchInterval: 60_000,
  });

  // ── Mutations ──
  const disableMutation = useMutation({
    mutationFn: async () => {
      const res = await fetch(`/api/admin/v1/routes/${entryId}/disable`, { method: 'POST', credentials: 'include' });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
    },
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['route-detail', entryId] }); toast('已禁用'); },
    onError: (e: any) => toast(e.message || '失败', 'error'),
  });
  const enableMutation = useMutation({
    mutationFn: async () => {
      const res = await fetch(`/api/admin/v1/routes/${entryId}/enable`, { method: 'POST', credentials: 'include' });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
    },
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['route-detail', entryId] }); toast('已启用'); },
    onError: (e: any) => toast(e.message || '失败', 'error'),
  });

  if (rl) return <div className="p-6 text-a-muted text-sm">加载中...</div>;
  if (!route) return <div className="p-6 text-a-muted text-sm">未找到入口 {entryId}</div>;

  const active = route.status === 'active';
  const tls = route.tls_enabled;
  const compName = route.composition
    ? (route.composition === 'https_route' ? 'HTTPS Route' :
       route.composition === 'http_route' ? 'HTTP Route' :
       route.composition === 'tls_passthrough' ? 'TLS Passthrough' :
       route.composition === 'http3' ? 'HTTP/3' :
       route.composition === 'raw_tcp' ? 'Raw TCP Forward' :
       route.composition === 'raw_udp' ? 'Raw UDP Forward' : route.composition)
    : (tls ? 'HTTPS Route' : 'HTTP Route');

  const compositions = rm?.current?.compositions || [];
  const comp = compositions.find((c: any) => c.name === compName);
  const entryHealthy = comp?.status === 'available';
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

  const certSourceLabel = (s: string) => {
    switch (s) {
      case 'gateway_auto': return '网关自动签发';
      case 'local_acme': return '本地 ACME 申请';
      case 'manual_upload': return '手动导入';
      case 'external': return '外部渠道';
      default: return s;
    }
  };

  return (
    <div className="p-6 space-y-5">
      {/* ── Header ── */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-bold text-a-fg">{route.domain}</h2>
          <p className="text-xs text-a-muted mt-1">
            {compName} · <StatusBadge status={active ? 'active' : 'disabled'} /> · {mode} 模式
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

      {/* ── Health ── */}
      <Card title="运行状态">
        <div className="space-y-2">
          {/* Provider health */}
          <div className={cn('flex items-center gap-3 px-3 py-2.5 rounded-a-sm border text-xs',
            entryHealthy ? 'bg-[#4cd964]/5 border-[#4cd964]/15' : 'bg-[#ff5c72]/5 border-[#ff5c72]/15')}>
            <span className={cn('font-mono text-sm shrink-0', entryHealthy ? 'text-[#4cd964]' : 'text-[#ff5c72]')}>
              {entryHealthy ? '✓' : '✗'}
            </span>
            <span className="font-medium w-24 shrink-0">Provider</span>
            <span className={entryHealthy ? 'text-a-muted' : 'text-[#ff5c72]/80'}>
              {entryHealthy ? 'Caddy 就绪，可正常提供服务' : 'Provider 未就绪，路由无法生效'}
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
      <div className="grid grid-cols-2 gap-4">
        <Card title="路由信息">
          <div className="space-y-2 text-xs">
            <Row label="域名" value={route.domain} mono />
            <Row label="路径" value={route.path_prefix || '/'} mono />
            <Row label="类型" value={compName} />
            <Row label="状态" badge={<StatusBadge status={active ? 'active' : 'disabled'} />} />
            <Row label="来源" value={route.owner_type === 'system' ? '系统（面板）' : route.owner_type === 'space' ? '服务' : '管理员'} />
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
                  <Row label="续期" value="🔄 Caddy 自动续期，无需干预" />
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
                    ? (tls ? '由 Provider 自动签发 Let\'s Encrypt 证书' : 'HTTP 路由，不使用证书')
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
            <Row label="组合能力" value={compName} />
            <Row label="Provider" value={entryHealthy ? 'Caddy 已就绪' : 'Caddy 未就绪'} />
            <Row label="TLS" value={tls ? '开启（:443）' : '关闭（:80）'} />
            <Row label="创建时间" value={route.created_at || '—'} mono />
            <div className="pt-1">
              <Link to="/fabric/mode" className="text-[10px] text-a-accent hover:underline">
                模式管理 →
              </Link>
            </div>
          </div>
        </Card>
      </div>
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

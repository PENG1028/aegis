// ─── Egress Gateway — 出口网关
// 全局开关 → 出口路径总览 → 各层细节（可折叠）
// 流量路径表统领，DNS/透明代理/出口规则作为可展开的实现细节。

import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { dnsApi, transparentApi, adminApi } from '@/lib/api-bridge';
import { PageHeader, Card, Btn, StatusBadge, useToast, LoadingState, ErrorBanner, Modal } from '@/components/shared';
import { cn } from '@/lib/utils';

// ─── Types ───

interface DnsEntry {
  domain: string; target_ip: string; target_node: string;
  node_ip: string; public_ip: string; is_local: boolean;
  route_id: string; service_id: string;
}

interface DnsStatus {
  running: boolean; listen_addr: string; upstream: string;
  enabled: boolean; local_hits: number; upstream_calls: number;
  managed_count: number; entries?: DnsEntry[];
}

interface EgressRule {
  id: string; type: 'allow' | 'block';
  match_type: 'domain' | 'ip' | 'cidr'; match_value: string;
  priority: number; status: 'active' | 'disabled'; note?: string;
  created_at: string; updated_at: string;
}

interface TransparentRule {
  id: string; original_ip: string; original_port: number;
  local_proxy_port: number; target_service: string; target_node: string;
  description: string; active: boolean; bytes_in: number; bytes_out: number;
}

interface ServiceRecord {
  id: string; name: string; host: string; port: number;
  node_host: string; status: string;
}

type EgressMode = 'sdk_direct' | 'transparent_proxy' | 'dns_upstream' | 'unregistered';

interface EgressRoute {
  target: string;
  dnsLayer: string;      // "内网解析" | "放行→上游" | "上游转发" | "不涉及"
  transportLayer: string; // "SDK直连" | "透明代理" | "直连" | "未注册"
  auth: string;
  node: string;
  mode: EgressMode;
  suggestion?: string;
}

// ─── Collapsible section ───

function Section({ title, subtitle, defaultOpen, children }: {
  title: string; subtitle: string; defaultOpen?: boolean; children: React.ReactNode;
}) {
  const [open, setOpen] = useState(defaultOpen ?? false);
  return (
    <div className="border border-a-border/30 rounded-a-sm overflow-hidden">
      <button
        onClick={() => setOpen(!open)}
        className="w-full flex items-center gap-2 px-3.5 py-2.5 bg-a-border/5 hover:bg-a-border/10 transition-colors text-left cursor-pointer"
      >
        <span className={cn('text-[10px] text-a-muted transition-transform', open && 'rotate-90')}>▶</span>
        <span className="text-xs font-medium text-a-fg">{title}</span>
        <span className="text-[10px] text-a-muted">{subtitle}</span>
      </button>
      {open && <div className="px-3.5 py-3 border-t border-a-border/20">{children}</div>}
    </div>
  );
}

// ─── Badge ───

function ModeBadge({ mode }: { mode: EgressMode }) {
  const s: Record<EgressMode, { bg: string; text: string; label: string }> = {
    sdk_direct:        { bg: 'bg-[#4cd964]/10', text: 'text-[#4cd964]', label: 'SDK 直连' },
    transparent_proxy: { bg: 'bg-[#e8b830]/10', text: 'text-[#e8b830]', label: '透明代理' },
    dns_upstream:      { bg: 'bg-a-border/15',  text: 'text-a-muted',   label: 'DNS 上游' },
    unregistered:      { bg: 'bg-[#ff5c72]/10', text: 'text-[#ff5c72]', label: '未注册' },
  };
  const st = s[mode];
  return <span className={cn('px-1.5 py-0.5 rounded text-[10px] font-medium', st.bg, st.text)}>{st.label}</span>;
}

// ─── Helpers ───

function fmtBytes(b: number | undefined): string {
  if (!b) return '0';
  if (b < 1024) return `${b}B`;
  if (b < 1024 * 1024) return `${(b / 1024).toFixed(1)}KB`;
  return `${(b / (1024 * 1024)).toFixed(1)}MB`;
}

// ─── Data fusion ───

function buildEgressRoutes(
  services: ServiceRecord[],
  tpRules: TransparentRule[],
  dns: DnsStatus | undefined,
): EgressRoute[] {
  const seen = new Set<string>();
  const routes: EgressRoute[] = [];

  // 1. Services (SDK or transparent proxy)
  for (const svc of services) {
    if (seen.has(svc.name)) continue;
    seen.add(svc.name);

    const hasTP = tpRules.some(r =>
      r.target_service?.includes(svc.id) ||
      (r.original_ip === svc.host && r.original_port === svc.port)
    );

    if (svc.status === 'active') {
      routes.push({
        target: svc.name,
        dnsLayer: hasTP ? '不涉及' : (dns?.entries?.some(e => e.service_id === svc.id) ? '内网解析' : '上游转发'),
        transportLayer: hasTP ? '透明代理' : 'SDK 直连',
        auth: hasTP ? '无' : 'Ticket',
        node: svc.node_host || '—',
        mode: hasTP ? 'transparent_proxy' : 'sdk_direct',
        suggestion: hasTP ? '⚠ SDK 已注册但仍被透明代理劫持' : undefined,
      });
    } else {
      routes.push({
        target: svc.name, dnsLayer: '—', transportLayer: '未注册',
        auth: '—', node: svc.node_host || '—', mode: 'unregistered',
        suggestion: '状态异常',
      });
    }
  }

  // 2. Transparent proxy rules without matching SDK service
  for (const r of tpRules) {
    const key = r.target_service?.slice(0, 16) || `${r.original_ip}:${r.original_port}`;
    if (seen.has(key)) continue;
    seen.add(key);
    routes.push({
      target: `${r.original_ip}:${r.original_port}`,
      dnsLayer: '不涉及', transportLayer: '透明代理', auth: '无',
      node: r.target_node || '—', mode: 'transparent_proxy',
    });
  }

  // 3. DNS entries not covered above
  if (dns?.entries) {
    for (const e of dns.entries) {
      if (seen.has(e.domain)) continue;
      seen.add(e.domain);
      const isLocal = e.is_local || !!e.target_node;
      routes.push({
        target: e.domain,
        dnsLayer: isLocal ? '内网解析' : '上游转发',
        transportLayer: isLocal ? 'SDK 直连' : '直连',
        auth: '—', node: e.target_node || '外部',
        mode: isLocal ? 'sdk_direct' : 'dns_upstream',
      });
    }
  }

  // Cross-node suggestions
  for (const r of routes) {
    if (r.mode === 'transparent_proxy' && r.node && r.node !== '—' && r.node !== '外部') {
      r.suggestion = '⚠ 跨机流量 · 建议创建 GatewayLink';
    }
  }

  return routes;
}

// ══════════════════════════════════════════════════════════════════
// PAGE
// ══════════════════════════════════════════════════════════════════

export default function EgressGateway() {
  const toast = useToast();
  const qc = useQueryClient();
  const [deleteTpId, setDeleteTpId] = useState<string | null>(null);
  const [showRuleModal, setShowRuleModal] = useState(false);
  const [newRule, setNewRule] = useState({ type: 'allow', match_type: 'domain', match_value: '', priority: 0, note: '' });

  // ── Data ──

  const { data: egressStatus } = useQuery({
    queryKey: ['egress-status'],
    queryFn: () => adminApi.egressStatus(),
    refetchInterval: 30_000,
  });

  const { data: dnsData, isLoading: dnsLoading, error: dnsError, refetch: refetchDns } = useQuery({
    queryKey: ['egress-dns'],
    queryFn: () => dnsApi.status(true),
    refetchInterval: 15_000,
  });

  const { data: tpStatus } = useQuery({
    queryKey: ['egress-tp-status'],
    queryFn: () => transparentApi.status(),
    refetchInterval: 30_000,
  });

  const { data: tpRules, isLoading: tpRulesLoading, refetch: refetchTpRules } = useQuery({
    queryKey: ['egress-tp-rules'],
    queryFn: () => transparentApi.listRules(),
    refetchInterval: 10_000,
  });

  const { data: egressRulesData, isLoading: egressRulesLoading, error: egressRulesError, refetch: refetchEgressRules } = useQuery({
    queryKey: ['egress-rules'],
    queryFn: () => adminApi.listEgressRules(),
    refetchInterval: 30_000,
  });

  const { data: servicesData } = useQuery({
    queryKey: ['egress-services'],
    queryFn: () => adminApi.listAuthServices(),
    refetchInterval: 30_000,
  });

  // ── Derived ──

  const egressEnabled = egressStatus?.enabled ?? true;
  const dns = dnsData as DnsStatus | undefined;
  const checks: any[] = (tpStatus as any)?.checks || [];
  const fwdTargets: any[] = (tpStatus as any)?.forward_targets || [];
  const rules: TransparentRule[] = (tpRules as any)?.rules || [];
  const egressRules: EgressRule[] = egressRulesData?.rules || [];
  const services: ServiceRecord[] = servicesData?.services || [];
  const egressRoutes = buildEgressRoutes(services, rules, dns);
  const allChecksPassed = checks.length > 0 && checks.every((c: any) => c.passed);
  const readyTargets = fwdTargets.filter((t: any) => t.status === 'available');

  // ── Mutations ──

  const toggleEgress = useMutation({
    mutationFn: (enabled: boolean) => adminApi.toggleEgress(enabled),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['egress-status'] });
      qc.invalidateQueries({ queryKey: ['egress-dns'] });
      toast(egressEnabled ? '出口网关已关闭' : '出口网关已开启');
    },
    onError: (e: any) => toast(e.message || '切换失败', 'error'),
  });

  const enableDns = useMutation({
    mutationFn: () => dnsApi.enable(),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['egress-dns'] }); toast('DNS 已启用'); },
    onError: (e: any) => toast(e.message || '启用失败', 'error'),
  });

  const disableDns = useMutation({
    mutationFn: () => dnsApi.disable(),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['egress-dns'] }); toast('DNS 已停用'); },
    onError: (e: any) => toast(e.message || '停用失败', 'error'),
  });

  const refreshDns = useMutation({
    mutationFn: () => dnsApi.refresh(),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['egress-dns'] }); toast('DNS 已刷新'); },
    onError: (e: any) => toast(e.message || '刷新失败', 'error'),
  });

  const deleteTp = useMutation({
    mutationFn: (id: string) => transparentApi.deleteRule(id),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['egress-tp-rules'] }); toast('规则已删除'); setDeleteTpId(null); },
    onError: (e: any) => toast(e.message || '删除失败', 'error'),
  });

  const deleteEgressRule = useMutation({
    mutationFn: (id: string) => adminApi.deleteEgressRule(id),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['egress-rules'] }); toast('出口规则已删除'); },
    onError: (e: any) => toast(e.message || '删除失败', 'error'),
  });

  const createRule = useMutation({
    mutationFn: (rule: any) => adminApi.createEgressRule(rule),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['egress-rules'] }); toast('规则已创建'); setShowRuleModal(false); setNewRule({ type: 'allow', match_type: 'domain', match_value: '', priority: 0, note: '' }); },
    onError: (e: any) => toast(e.message || '创建失败', 'error'),
  });

  return (
    <div className="p-6 space-y-4">
      {/* ── Header + Global Toggle ── */}
      <div className="flex items-center justify-between">
        <PageHeader
          title="出口网关 · Egress Gateway"
          subtitle="统一管理本机出站流量 — DNS 解析 → 透明代理 → 出口规则"
        />
        <button
          onClick={() => toggleEgress.mutate(!egressEnabled)}
          disabled={toggleEgress.isPending}
          className={cn(
            'shrink-0 px-4 py-2 rounded-a-sm text-xs font-bold border transition-all cursor-pointer',
            egressEnabled
              ? 'bg-[#4cd964]/10 border-[#4cd964]/30 text-[#4cd964] hover:bg-[#4cd964]/20'
              : 'bg-[#ff5c72]/10 border-[#ff5c72]/30 text-[#ff5c72] hover:bg-[#ff5c72]/20'
          )}
        >
          {egressEnabled ? '● 出口已开启' : '○ 出口已关闭'}
        </button>
      </div>

      {/* ── Inline status bar ── */}
      <div className="flex items-center gap-5 text-[11px] text-a-muted bg-a-surface border border-a-border/30 rounded-a-sm px-3.5 py-2 flex-wrap">
        <span className="flex items-center gap-1.5">
          <span className={cn('w-1.5 h-1.5 rounded-full', dns?.running ? 'bg-[#4cd964]' : 'bg-a-border')} />
          DNS {dns?.running ? '运行中' : '已停'} <span className="font-mono text-[10px]">({dns?.listen_addr || '—'} → {dns?.upstream || '—'})</span>
        </span>
        <span className="text-a-border">|</span>
        <span>透明代理 <span className="text-a-fg font-mono">{rules.length}</span> 条规则</span>
        <span className="text-a-border">|</span>
        <span>出口规则 <span className="text-a-fg font-mono">{egressRules.filter(r => r.status === 'active').length}</span> 活跃</span>
        <span className="text-a-border">|</span>
        <span>SDK 直连 <span className="text-a-fg font-mono">{egressRoutes.filter(r => r.mode === 'sdk_direct').length}</span></span>
        {egressRoutes.filter(r => r.suggestion?.includes('跨机')).length > 0 && (
          <>
            <span className="text-a-border">|</span>
            <span className="text-[#e8b830]">⚠ {egressRoutes.filter(r => r.suggestion?.includes('跨机')).length} 条跨机建议</span>
          </>
        )}
      </div>

      {/* ════════════════════════════════════════════════════════════ */}
      {/* MAIN: Egress Route Overview — 出口路径总览 */}
      {/* ════════════════════════════════════════════════════════════ */}

      <Card title="出口路径总览" subtitle="每条目标经过 DNS → 传输 → 节点的完整路径">
        <div className="overflow-x-auto">
          <table className="w-full text-xs">
            <thead>
              <tr className="border-b border-a-border/30 text-a-muted text-left">
                <th className="py-1.5 pr-2 font-medium">目标</th>
                <th className="py-1.5 px-2 font-medium">DNS 层</th>
                <th className="py-1.5 px-2 font-medium">传输层</th>
                <th className="py-1.5 px-2 font-medium">认证</th>
                <th className="py-1.5 px-2 font-medium">节点</th>
                <th className="py-1.5 pl-2 font-medium w-48">提示</th>
              </tr>
            </thead>
            <tbody>
              {egressRoutes.length > 0 ? egressRoutes.map((r, i) => (
                <tr key={i} className="border-b border-a-border/20 hover:bg-a-border/10 transition-colors">
                  <td className="py-1.5 pr-2 font-semibold text-a-fg font-mono">{r.target}</td>
                  <td className="py-1.5 px-2">
                    <span className={cn(
                      'text-[10px]',
                      r.dnsLayer === '内网解析' ? 'text-[#4cd964]' :
                      r.dnsLayer === '放行→上游' ? 'text-[#e8b830]' :
                      r.dnsLayer === '上游转发' ? 'text-a-muted' : 'text-a-muted'
                    )}>{r.dnsLayer}</span>
                  </td>
                  <td className="py-1.5 px-2"><ModeBadge mode={r.mode} /></td>
                  <td className="py-1.5 px-2 text-a-muted text-[11px]">{r.auth}</td>
                  <td className="py-1.5 px-2 font-mono text-[11px] text-a-muted">{r.node}</td>
                  <td className="py-1.5 pl-2">
                    {r.suggestion ? (
                      <span className={cn(
                        'text-[10px]',
                        r.suggestion.includes('跨机') ? 'text-[#e8b830]' :
                        r.suggestion.includes('SDK') ? 'text-[#a865ff]' : 'text-a-muted',
                      )}>{r.suggestion}</span>
                    ) : (
                      <span className="text-[#4cd964] text-[10px]">✓</span>
                    )}
                  </td>
                </tr>
              )) : (
                <tr><td colSpan={6} className="py-8 text-center text-a-muted text-xs">
                  暂无出站流量 · 注册服务或配置路由后自动生成
                </td></tr>
              )}
            </tbody>
          </table>
        </div>
        {/* Legend */}
        <div className="mt-3 flex items-center gap-4 text-[10px] text-a-muted flex-wrap pt-2 border-t border-a-border/20">
          <span className="flex items-center gap-1"><span className="w-2 h-2 rounded-full bg-[#4cd964]" /> 内网 DNS / SDK 直连</span>
          <span className="flex items-center gap-1"><span className="w-2 h-2 rounded-full bg-[#e8b830]" /> 透明代理</span>
          <span className="flex items-center gap-1"><span className="w-2 h-2 rounded-full bg-a-border" /> DNS 上游 / 直出</span>
          <span className="flex items-center gap-1"><span className="w-2 h-2 rounded-full bg-[#ff5c72]" /> 未注册 / 阻断</span>
        </div>
      </Card>

      {/* ════════════════════════════════════════════════════════════ */}
      {/* DETAILS: Collapsible layer sections */}
      {/* ════════════════════════════════════════════════════════════ */}

      {/* ── Section 1: DNS Layer ── */}
      <Section
        title="DNS 解析层"
        subtitle={`${dns?.listen_addr || '—'} → ${dns?.upstream || '—'} · 命中 ${dns?.local_hits || 0} · 上游 ${dns?.upstream_calls || 0}`}
      >
        {dnsLoading ? <LoadingState /> : dnsError ? <ErrorBanner message="DNS 状态加载失败" onRetry={refetchDns} /> : (
          <>
            <div className="flex items-center gap-2 mb-3">
              <StatusBadge status={dns?.running ? 'active' : 'disabled'} />
              <span className="text-[10px] text-a-muted">受管域名 <span className="font-mono text-a-fg">{dns?.managed_count || 0}</span> 条</span>
              <span className="flex-1" />
              {dns?.running
                ? <Btn onClick={() => disableDns.mutate()} disabled={disableDns.isPending} className="text-[10px]" danger>停用 DNS</Btn>
                : <Btn onClick={() => enableDns.mutate()} disabled={enableDns.isPending} className="text-[10px]" primary>启用 DNS</Btn>
              }
              <Btn onClick={() => refreshDns.mutate()} disabled={refreshDns.isPending} className="text-[10px]">刷新</Btn>
            </div>
            {(dns?.entries?.length || 0) > 0 ? (
              <div className="overflow-x-auto">
                <table className="w-full text-xs">
                  <thead>
                    <tr className="border-b border-a-border/30 text-a-muted text-left">
                      <th className="py-1 pr-2 font-medium">域名</th>
                      <th className="py-1 px-2 font-medium">目标 IP</th>
                      <th className="py-1 px-2 font-medium text-center">来源</th>
                      <th className="py-1 pl-2 font-medium">路由</th>
                    </tr>
                  </thead>
                  <tbody>
                    {dns!.entries!.map((e, i) => (
                      <tr key={i} className="border-b border-a-border/20 hover:bg-a-border/10">
                        <td className="py-1 pr-2 font-mono text-a-fg">{e.domain}</td>
                        <td className="py-1 px-2 font-mono text-a-muted">{e.target_ip}</td>
                        <td className="py-1 px-2 text-center text-[10px]">
                          {e.is_local ? <span className="text-[#4cd964]">本机</span>
                            : e.target_node ? <span className="text-[#e8b830]">远端</span>
                            : <span className="text-a-muted">上游</span>}
                        </td>
                        <td className="py-1 pl-2 font-mono text-[10px] text-a-muted">{e.route_id?.slice(0, 12) || '—'}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : (
              <div className="py-4 text-center text-a-muted text-xs">无解析记录 · 无活跃路由时 DNS 无条目</div>
            )}
          </>
        )}
      </Section>

      {/* ── Section 2: Transparent Proxy Layer ── */}
      <Section
        title="透明代理层"
        subtitle={`${checks.filter((c: any) => c.passed).length}/${checks.length} 条件满足 · ${rules.length} 条 iptables 规则`}
      >
        {/* Availability checks */}
        {checks.length > 0 && (
          <div className="mb-3 space-y-1.5">
            {checks.map((c: any, i: number) => (
              <div key={i} className={cn(
                'flex items-center gap-2 px-3 py-1.5 rounded-a-sm border text-[11px]',
                c.passed ? 'bg-[#4cd964]/5 border-[#4cd964]/15' : 'bg-[#ff5c72]/5 border-[#ff5c72]/15',
              )}>
                <span className={c.passed ? 'text-[#4cd964]' : 'text-[#ff5c72]'}>{c.passed ? '✓' : '✗'}</span>
                <span className="text-a-fg font-medium">{c.name}</span>
                <span className={c.passed ? 'text-a-muted' : 'text-[#ff5c72]/80'}>{c.detail}</span>
              </div>
            ))}
          </div>
        )}

        {/* Forward targets */}
        {readyTargets.length > 0 && (
          <div className="mb-3 p-2 rounded-a-sm bg-a-border/5 border border-a-border/20 text-[10px]">
            <span className="text-a-muted">转发入口：</span>
            {readyTargets.map((t: any, i: number) => (
              <span key={i} className="ml-3">
                <span className="text-[#4cd964]">✓</span>
                <span className="text-a-fg ml-0.5">{t.composition}</span>
                <span className="text-a-muted"> → {t.host}:{t.port}（{t.provider_id}）</span>
              </span>
            ))}
          </div>
        )}

        {/* iptables rules */}
        {tpRulesLoading ? <LoadingState /> : rules.length > 0 ? (
          <div className="overflow-x-auto">
            <table className="w-full text-xs">
              <thead>
                <tr className="border-b border-a-border/30 text-a-muted text-left">
                  <th className="py-1 pr-2 font-medium">目标</th>
                  <th className="py-1 px-2 font-medium">代理端口</th>
                  <th className="py-1 px-2 font-medium">服务</th>
                  <th className="py-1 px-2 font-medium">节点</th>
                  <th className="py-1 px-2 font-medium text-right">入</th>
                  <th className="py-1 px-2 font-medium text-right">出</th>
                  <th className="py-1 pl-2 font-medium"></th>
                </tr>
              </thead>
              <tbody>
                {rules.map((r) => (
                  <tr key={r.id} className="border-b border-a-border/20 hover:bg-a-border/10">
                    <td className="py-1 pr-2 font-mono text-a-fg">{r.original_ip}:{r.original_port}</td>
                    <td className="py-1 px-2 font-mono text-a-muted">:{r.local_proxy_port}</td>
                    <td className="py-1 px-2 font-mono text-[10px] text-a-muted">{r.target_service?.slice(0, 12) || '—'}</td>
                    <td className="py-1 px-2 font-mono text-[10px] text-a-muted">{r.target_node?.slice(0, 8) || '—'}</td>
                    <td className="py-1 px-2 font-mono text-right text-a-muted">{fmtBytes(r.bytes_in)}</td>
                    <td className="py-1 px-2 font-mono text-right text-a-muted">{fmtBytes(r.bytes_out)}</td>
                    <td className="py-1 pl-2 text-right">
                      <Btn onClick={() => setDeleteTpId(r.id)} className="text-[9px]" danger>删除</Btn>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <div className="py-4 text-center text-a-muted text-xs">无劫持规则 · 需要 Linux + root 权限</div>
        )}
      </Section>

      {/* ── Section 3: Egress Rules ── */}
      <Section
        title="出口规则"
        subtitle={`${egressRules.filter(r => r.status === 'active').length} 活跃 · allow/block`}
      >
        <div className="mb-3 flex items-center gap-2">
          <Btn onClick={() => setShowRuleModal(true)} className="text-[10px]" primary>新建规则</Btn>
          <Btn onClick={() => refetchEgressRules()} className="text-[10px]">刷新</Btn>
          <span className="text-[10px] text-a-muted ml-auto">{egressRules.length} 条规则</span>
        </div>
        {egressRulesLoading ? <LoadingState /> : egressRulesError ? <ErrorBanner message="加载失败" onRetry={refetchEgressRules} /> : (
          egressRules.length > 0 ? (
            <div className="overflow-x-auto">
              <table className="w-full text-xs">
                <thead>
                  <tr className="border-b border-a-border/30 text-a-muted text-left">
                    <th className="py-1 pr-2 font-medium w-16">类型</th>
                    <th className="py-1 px-2 font-medium">匹配值</th>
                    <th className="py-1 px-2 font-medium w-12 text-center">优先级</th>
                    <th className="py-1 px-2 font-medium">备注</th>
                    <th className="py-1 pl-2 font-medium w-16"></th>
                  </tr>
                </thead>
                <tbody>
                  {egressRules.map((r: EgressRule) => (
                    <tr key={r.id} className="border-b border-a-border/20 hover:bg-a-border/10">
                      <td className="py-1 pr-2">
                        <span className={cn('px-1.5 py-0.5 rounded text-[10px] font-medium',
                          r.type === 'allow' ? 'bg-[#4cd964]/10 text-[#4cd964]' : 'bg-[#ff5c72]/10 text-[#ff5c72]',
                        )}>{r.type === 'allow' ? '放行' : '拦截'}</span>
                      </td>
                      <td className="py-1 px-2 font-mono text-a-fg text-[11px]">
                        <span className="text-a-muted text-[10px] mr-1">[{r.match_type}]</span>
                        {r.match_value}
                      </td>
                      <td className="py-1 px-2 text-center text-a-muted">{r.priority}</td>
                      <td className="py-1 px-2 text-a-muted text-[10px]">{r.note || '—'}</td>
                      <td className="py-1 pl-2 text-right">
                        <Btn onClick={() => deleteEgressRule.mutate(r.id)} className="text-[9px]" danger disabled={deleteEgressRule.isPending}>删除</Btn>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : (
            <div className="py-4 text-center text-a-muted text-xs">无规则 · 所有域名正常解析和转发</div>
          )
        )}
        <p className="text-[10px] text-a-muted mt-2">放行：域名跳过内部 DNS 走上游（重名保护）。拦截：域名拒绝解析。</p>
      </Section>

      {/* ── Delete TP rule modal ── */}
      {deleteTpId && (
        <Modal onClose={() => setDeleteTpId(null)} title="确认删除"
          footer={
            <div className="flex items-center gap-2 justify-end">
              <Btn onClick={() => setDeleteTpId(null)} className="text-xs">取消</Btn>
              <Btn onClick={() => deleteTp.mutate(deleteTpId)} danger className="text-xs" disabled={deleteTp.isPending}>
                {deleteTp.isPending ? '删除中...' : '确认删除'}
              </Btn>
            </div>
          }>
          <p className="text-sm text-a-muted">确定要删除此劫持规则吗？iptables 规则将被移除。</p>
        </Modal>
      )}

      {/* ── New rule modal ── */}
      {showRuleModal && (
        <Modal onClose={() => setShowRuleModal(false)} title="新建出口规则"
          footer={
            <div className="flex items-center gap-2 justify-end">
              <Btn onClick={() => setShowRuleModal(false)} className="text-xs">取消</Btn>
              <Btn onClick={() => createRule.mutate(newRule)} primary className="text-xs" disabled={createRule.isPending}>
                {createRule.isPending ? '创建中...' : '创建规则'}
              </Btn>
            </div>
          }>
          <div className="space-y-3 text-xs">
            <div className="flex items-center gap-3">
              <label className="text-a-muted w-16 shrink-0">类型</label>
              <select value={newRule.type} onChange={e => setNewRule({ ...newRule, type: e.target.value })}
                className="bg-a-bg border border-a-border rounded-a-sm px-2 py-1 text-a-fg text-xs">
                <option value="allow">放行 (allow)</option>
                <option value="block">拦截 (block)</option>
              </select>
            </div>
            <div className="flex items-center gap-3">
              <label className="text-a-muted w-16 shrink-0">匹配方式</label>
              <select value={newRule.match_type} onChange={e => setNewRule({ ...newRule, match_type: e.target.value })}
                className="bg-a-bg border border-a-border rounded-a-sm px-2 py-1 text-a-fg text-xs">
                <option value="domain">域名</option>
                <option value="ip">IP</option>
                <option value="cidr">CIDR</option>
              </select>
            </div>
            <div className="flex items-center gap-3">
              <label className="text-a-muted w-16 shrink-0">匹配值</label>
              <input value={newRule.match_value} onChange={e => setNewRule({ ...newRule, match_value: e.target.value })}
                placeholder="github.com / 10.0.0.0/8"
                className="flex-1 bg-a-bg border border-a-border rounded-a-sm px-2 py-1 text-a-fg text-xs" />
            </div>
            <div className="flex items-center gap-3">
              <label className="text-a-muted w-16 shrink-0">优先级</label>
              <input type="number" value={newRule.priority} onChange={e => setNewRule({ ...newRule, priority: parseInt(e.target.value) || 0 })}
                className="w-20 bg-a-bg border border-a-border rounded-a-sm px-2 py-1 text-a-fg text-xs" />
            </div>
            <div className="flex items-center gap-3">
              <label className="text-a-muted w-16 shrink-0">备注</label>
              <input value={newRule.note} onChange={e => setNewRule({ ...newRule, note: e.target.value })}
                placeholder="可选"
                className="flex-1 bg-a-bg border border-a-border rounded-a-sm px-2 py-1 text-a-fg text-xs" />
            </div>
          </div>
        </Modal>
      )}
    </div>
  );
}

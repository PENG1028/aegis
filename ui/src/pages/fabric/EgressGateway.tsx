// ─── Egress Gateway (v1.9A) — 出口网关 ───
// 统一视图：DNS 解析器 + 透明代理 + 出口路径总览
// 用户从上往下读，理解出站流量走的完整链路。

import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { dnsApi, transparentApi, adminApi } from '@/lib/api-bridge';
import { PageHeader, StatCard, Card, Btn, StatusBadge, useToast, LoadingState, ErrorBanner, EmptyState, Modal } from '@/components/shared';
import { cn } from '@/lib/utils';

// ─── Types for egress rules ───

interface EgressRule {
  id: string;
  type: 'allow' | 'block';
  match_type: 'domain' | 'ip' | 'cidr';
  match_value: string;
  priority: number;
  status: 'active' | 'disabled';
  note?: string;
  created_at: string;
  updated_at: string;
}

interface CheckResult {
  name: string;
  passed: boolean;
  detail: string;
}

// ─── Types ───

interface DnsEntry {
  domain: string;
  target_ip: string;
  target_node: string;
  node_ip: string;
  public_ip: string;
  is_local: boolean;
  route_id: string;
  service_id: string;
}

interface DnsStatus {
  running: boolean;
  listen_addr: string;
  upstream: string;
  enabled: boolean;
  local_hits: number;
  upstream_calls: number;
  managed_count: number;
  entries?: DnsEntry[];
}

interface TransparentCheck {
  name: string;
  passed: boolean;
  detail: string;
}

interface ForwardTarget {
  composition: string;
  status: string;
  provider_id?: string;
  host?: string;
  port?: number;
  provider_ok?: boolean;
  detail: string;
}

interface TransparentRule {
  id: string;
  original_ip: string;
  original_port: number;
  local_proxy_port: number;
  target_service: string;
  target_node: string;
  description: string;
  active: boolean;
  bytes_in: number;
  bytes_out: number;
}

interface ServiceRecord {
  id: string;
  name: string;
  host: string;
  port: number;
  node_host: string;
  status: string;
}

type EgressMode = 'sdk_direct' | 'transparent_proxy' | 'dns_upstream' | 'unregistered';

interface EgressRoute {
  target: string;
  mode: EgressMode;
  auth: string;
  provider: string;
  node: string;
  suggestion?: string;
}

// ══════════════════════════════════════════════════════════════════
// Component
// ══════════════════════════════════════════════════════════════════

export default function EgressGateway() {
  const toast = useToast();
  const qc = useQueryClient();
  const [deleteRuleId, setDeleteRuleId] = useState<string | null>(null);

  // ── Data ──

  const { data: dnsData, isLoading: dnsLoading, error: dnsError, refetch: refetchDns } = useQuery({
    queryKey: ['egress-dns'],
    queryFn: () => dnsApi.status(true),
    refetchInterval: 15_000,
  });

  const { data: tpStatus, isLoading: tpLoading } = useQuery({
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

  // ── Mutations ──

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

  const deleteRule = useMutation({
    mutationFn: (id: string) => transparentApi.deleteRule(id),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['egress-tp-rules'] }); toast('规则已删除'); setDeleteRuleId(null); },
    onError: (e: any) => toast(e.message || '删除失败', 'error'),
  });

  const deleteEgressRule = useMutation({
    mutationFn: (id: string) => adminApi.deleteEgressRule(id),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['egress-rules'] }); toast('出口规则已删除'); },
    onError: (e: any) => toast(e.message || '删除失败', 'error'),
  });

  // ── Health check state ──
  const [checkResults, setCheckResults] = useState<CheckResult[]>([]);
  const [checkRunning, setCheckRunning] = useState(false);
  const [showRuleModal, setShowRuleModal] = useState(false);
  const allHealthy = checkResults.length > 0 && checkResults.every(cr => cr.passed);

  const runCheck = async () => {
    setCheckRunning(true);
    try {
      const res = await adminApi.checkEgress();
      setCheckResults(res.checks);
    } catch (e: any) {
      setCheckResults([{ name: '全链路检测', passed: false, detail: e.message || '检测请求失败' }]);
    } finally {
      setCheckRunning(false);
    }
  };

  // ── Derived Data ──

  const dns = dnsData as DnsStatus | undefined;
  const checks: TransparentCheck[] = (tpStatus as any)?.checks || [];
  const fwdTargets: ForwardTarget[] = (tpStatus as any)?.forward_targets || [];
  const rules: TransparentRule[] = (tpRules as any)?.rules || [];
  const egressRules: EgressRule[] = egressRulesData?.rules || [];
  const services: ServiceRecord[] = servicesData?.services || [];

  const allChecksPassed = checks.length > 0 && checks.every(c => c.passed);
  const readyTargets = fwdTargets.filter(t => t.status === 'available');

  // ── Egress Route Overview ──
  // 融合四路数据: ServiceAuth + 透明代理规则 + DNS 解析记录

  const egressRoutes: EgressRoute[] = buildEgressRoutes(services, rules, dns);

  const crossNodeCount = egressRoutes.filter(r => r.suggestion?.includes('跨机')).length;
  const sdkCount = egressRoutes.filter(r => r.mode === 'sdk_direct').length;
  const tpCount = egressRoutes.filter(r => r.mode === 'transparent_proxy').length;

  return (
    <div className="p-6 space-y-5">
      <PageHeader
        title="出口网关 · Egress Gateway"
        subtitle="DNS 解析 → 透明代理 → 出口路径 — 本机出站流量的完整视图"
      />

      {/* ── Stat Cards ── */}
      <div className="grid grid-cols-5 gap-3">
        <StatCard label="DNS" value={dns?.running ? '● 运行中' : '○ 已停'} accent={!!dns?.running} />
        <StatCard label="解析记录" value={dns?.entries?.length || 0} />
        <StatCard label="劫持规则" value={rules.length} />
        <StatCard label="SDK 直连" value={sdkCount} accent />
        <StatCard label="跨机建议" value={crossNodeCount} warn={crossNodeCount > 0} />
      </div>

      {/* ════════════════════════════════════════════════════════════ */}
      {/* SECTION 1: DNS */}
      {/* ════════════════════════════════════════════════════════════ */}

      <Card title="DNS 解析器" subtitle="内部域名解析 → 本机管理域名 / 上游转发">
        {dnsLoading ? <LoadingState /> : dnsError ? <ErrorBanner message="DNS 状态加载失败" onRetry={refetchDns} /> : (
          <>
            {/* Status bar */}
            <div className="flex items-center gap-3 mb-4 text-xs flex-wrap">
              <StatusBadge status={dns?.running ? 'active' : 'disabled'} />
              <span className="font-mono text-a-muted">{dns?.listen_addr || '—'}</span>
              <span className="text-a-border">→</span>
              <span className="font-mono text-a-muted">{dns?.upstream || '—'}</span>
              <span className="text-a-muted">|</span>
              <span className="text-a-muted">本地命中 <span className="text-a-fg font-mono">{dns?.local_hits || 0}</span></span>
              <span className="text-a-muted">上游 <span className="text-a-fg font-mono">{dns?.upstream_calls || 0}</span></span>
              <span className="flex-1" />
              {dns?.running
                ? <Btn onClick={() => disableDns.mutate()} disabled={disableDns.isPending} className="text-[10px]" danger>停用</Btn>
                : <Btn onClick={() => enableDns.mutate()} disabled={enableDns.isPending} className="text-[10px]" primary>启用</Btn>
              }
              <Btn onClick={() => refreshDns.mutate()} disabled={refreshDns.isPending} className="text-[10px]">刷新</Btn>
            </div>

            {/* Resolution table */}
            <div className="overflow-x-auto">
              <table className="w-full text-xs">
                <thead>
                  <tr className="border-b border-a-border/30 text-a-muted text-left">
                    <th className="py-1.5 pr-3 font-medium">域名</th>
                    <th className="py-1.5 px-2 font-medium">目标 IP</th>
                    <th className="py-1.5 px-2 font-medium">节点</th>
                    <th className="py-1.5 px-2 font-medium text-center">来源</th>
                    <th className="py-1.5 pl-3 font-medium">所属路由</th>
                  </tr>
                </thead>
                <tbody>
                  {(dns?.entries?.length || 0) > 0 ? dns!.entries!.map((e, i) => (
                    <tr key={i} className="border-b border-a-border/20 hover:bg-a-border/10 transition-colors">
                      <td className="py-1.5 pr-3 font-mono text-a-fg">{e.domain}</td>
                      <td className="py-1.5 px-2 font-mono text-a-muted">{e.target_ip}</td>
                      <td className="py-1.5 px-2 font-mono text-a-muted">{e.target_node || '—'}</td>
                      <td className="py-1.5 px-2 text-center">
                        {e.is_local ? (
                          <span className="text-[#4cd964] text-[10px]">本机</span>
                        ) : e.target_node ? (
                          <span className="text-[#e8b830] text-[10px]">远端</span>
                        ) : (
                          <span className="text-a-muted text-[10px]">上游</span>
                        )}
                      </td>
                      <td className="py-1.5 pl-3 font-mono text-[10px] text-a-muted">{e.route_id || '—'}</td>
                    </tr>
                  )) : (
                    <tr><td colSpan={5} className="py-6 text-center text-a-muted text-xs">无解析记录 · 无活跃路由时 DNS 无条目</td></tr>
                  )}
                </tbody>
              </table>
            </div>
          </>
        )}
      </Card>

      {/* ════════════════════════════════════════════════════════════ */}
      {/* SECTION 2: Transparent Proxy */}
      {/* ════════════════════════════════════════════════════════════ */}

      <Card title="透明代理" subtitle="无 SDK 服务的流量劫持 — iptables DNAT → 本地代理转发">
        {tpLoading ? <LoadingState /> : (
          <>
            {/* Diagnosis checks */}
            {checks.length > 0 && (
              <div className="mb-4 space-y-1.5">
                {checks.map((c, i) => (
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
              <div className="mb-4 p-2.5 rounded-a-sm bg-a-border/5 border border-a-border/20 text-[10px]">
                <span className="text-a-muted">转发入口就绪：</span>
                {readyTargets.map((t, i) => (
                  <span key={i} className="ml-3">
                    <span className="text-[#4cd964]">✓</span>
                    <span className="text-a-fg ml-0.5">{t.composition}</span>
                    <span className="text-a-muted"> → {t.host}:{t.port}（{t.provider_id}）</span>
                  </span>
                ))}
              </div>
            )}

            {/* Rules table */}
            <div className="overflow-x-auto">
              <table className="w-full text-xs">
                <thead>
                  <tr className="border-b border-a-border/30 text-a-muted text-left">
                    <th className="py-1.5 pr-2 font-medium">目标 IP</th>
                    <th className="py-1.5 px-2 font-medium">端口</th>
                    <th className="py-1.5 px-2 font-medium">代理端口</th>
                    <th className="py-1.5 px-2 font-medium">所属服务</th>
                    <th className="py-1.5 px-2 font-medium">节点</th>
                    <th className="py-1.5 px-2 font-medium text-right">入流量</th>
                    <th className="py-1.5 px-2 font-medium text-right">出流量</th>
                    <th className="py-1.5 pl-2 font-medium"></th>
                  </tr>
                </thead>
                <tbody>
                  {rules.length > 0 ? rules.map((r, i) => (
                    <tr key={r.id || i} className="border-b border-a-border/20 hover:bg-a-border/10 transition-colors">
                      <td className="py-1.5 pr-2 font-mono text-a-fg">{r.original_ip}</td>
                      <td className="py-1.5 px-2 font-mono text-a-muted">{r.original_port}</td>
                      <td className="py-1.5 px-2 font-mono text-a-muted">:{r.local_proxy_port}</td>
                      <td className="py-1.5 px-2 font-mono text-[11px] text-a-muted">{r.target_service?.slice(0, 12) || '—'}</td>
                      <td className="py-1.5 px-2 font-mono text-[10px] text-a-muted">{r.target_node?.slice(0, 8) || '—'}</td>
                      <td className="py-1.5 px-2 font-mono text-right text-a-muted">{fmtBytes(r.bytes_in)}</td>
                      <td className="py-1.5 px-2 font-mono text-right text-a-muted">{fmtBytes(r.bytes_out)}</td>
                      <td className="py-1.5 pl-2 text-right">
                        <Btn onClick={() => setDeleteRuleId(r.id)} className="text-[9px]" danger>删除</Btn>
                      </td>
                    </tr>
                  )) : (
                    <tr><td colSpan={8} className="py-6 text-center text-a-muted text-xs">无劫持规则</td></tr>
                  )}
                </tbody>
              </table>
            </div>
          </>
        )}
      </Card>

      {/* ════════════════════════════════════════════════════════════ */}
      {/* SECTION 3: Egress Rules — Allow/Block */}
      {/* ════════════════════════════════════════════════════════════ */}

      <Card title="出口规则" subtitle="允许名单（重名保护）和拦截名单 — 域名级别">
        <div className="mb-3 flex items-center gap-2">
          <Btn onClick={() => setShowRuleModal(true)} className="text-[10px]" primary>新建规则</Btn>
          <Btn onClick={() => refetchEgressRules()} className="text-[10px]" disabled={egressRulesLoading}>刷新</Btn>
          <span className="text-[10px] text-a-muted ml-auto">{egressRules.length} 条规则</span>
        </div>
        {egressRulesLoading ? <LoadingState /> : egressRulesError ? <ErrorBanner message="加载失败" onRetry={refetchEgressRules} /> : (
          <div className="overflow-x-auto">
            <table className="w-full text-xs">
              <thead>
                <tr className="border-b border-a-border/30 text-a-muted text-left">
                  <th className="py-1.5 pr-2 font-medium w-16">类型</th>
                  <th className="py-1.5 px-2 font-medium w-20">匹配</th>
                  <th className="py-1.5 px-2 font-medium">值</th>
                  <th className="py-1.5 px-2 font-medium w-12 text-center">优先级</th>
                  <th className="py-1.5 px-2 font-medium w-16">状态</th>
                  <th className="py-1.5 px-2 font-medium">备注</th>
                  <th className="py-1.5 pl-2 font-medium w-16"></th>
                </tr>
              </thead>
              <tbody>
                {egressRules.length > 0 ? egressRules.map((r: EgressRule) => (
                  <tr key={r.id} className="border-b border-a-border/20 hover:bg-a-border/10 transition-colors">
                    <td className="py-1.5 pr-2">
                      <span className={cn('px-1.5 py-0.5 rounded text-[10px] font-medium',
                        r.type === 'allow' ? 'bg-[#4cd964]/10 text-[#4cd964]' : 'bg-[#ff5c72]/10 text-[#ff5c72]',
                      )}>{r.type === 'allow' ? '放行' : '拦截'}</span>
                    </td>
                    <td className="py-1.5 px-2 font-mono text-a-muted">{r.match_type}</td>
                    <td className="py-1.5 px-2 font-mono text-a-fg">{r.match_value}</td>
                    <td className="py-1.5 px-2 text-center text-a-muted">{r.priority}</td>
                    <td className="py-1.5 px-2"><StatusBadge status={r.status === 'active' ? 'active' : 'disabled'} /></td>
                    <td className="py-1.5 px-2 text-a-muted text-[11px]">{r.note || '—'}</td>
                    <td className="py-1.5 pl-2 text-right">
                      <Btn onClick={() => deleteEgressRule.mutate(r.id)} className="text-[9px]" danger disabled={deleteEgressRule.isPending}>删除</Btn>
                    </td>
                  </tr>
                )) : (
                  <tr><td colSpan={7} className="py-6 text-center text-a-muted text-xs">无规则 · 所有域名正常解析和转发</td></tr>
                )}
              </tbody>
            </table>
          </div>
        )}
        <p className="text-[10px] text-a-muted mt-2">放行规则中的域名将跳过内部 DNS 解析，直接走上游 DNS，实现重名保护。</p>
      </Card>

      {/* ════════════════════════════════════════════════════════════ */}
      {/* SECTION 4: Health Check */}
      {/* ════════════════════════════════════════════════════════════ */}

      <Card title="出口检测" subtitle="一键验证出口链路各环节是否正常">
        <Btn onClick={runCheck} disabled={checkRunning} className="text-xs" primary>
          {checkRunning ? '检测中...' : '🔍 运行全链路检测'}
        </Btn>

        {checkResults.length > 0 && (
          <div className="mt-4 space-y-1.5">
            {checkResults.map((cr, i) => (
              <div key={i} className={cn(
                'flex items-center gap-2 px-3 py-2 rounded-a-sm border text-xs',
                cr.passed ? 'bg-[#4cd964]/5 border-[#4cd964]/15' : 'bg-[#ff5c72]/5 border-[#ff5c72]/15',
              )}>
                <span className={cn('font-bold', cr.passed ? 'text-[#4cd964]' : 'text-[#ff5c72]')}>
                  {cr.passed ? '✓' : '✗'}
                </span>
                <span className="font-medium text-a-fg">{cr.name}</span>
                <span className={cr.passed ? 'text-a-muted' : 'text-[#ff5c72]/80'}>{cr.detail}</span>
              </div>
            ))}
            <div className={cn(
              'mt-2 px-3 py-2 rounded-a-sm text-xs font-medium text-center',
              allHealthy ? 'bg-[#4cd964]/10 text-[#4cd964]' : 'bg-[#ff5c72]/10 text-[#ff5c72]',
            )}>
              {allHealthy ? '✓ 出口网关正常' : '✗ 存在异常，请检查上述项目'}
            </div>
          </div>
        )}

        <p className="text-[10px] text-a-muted mt-2">检测 DNS 解析器、出口规则、透明代理、ServiceAuth 等各环节。</p>
      </Card>

      {/* ════════════════════════════════════════════════════════════ */}
      {/* SECTION 5: Egress Route Overview ★ Core */}
      {/* ════════════════════════════════════════════════════════════ */}

      <Card title="出口路径总览" subtitle="融合 DNS + 透明代理 + ServiceAuth — 每条出站流量的完整路径">
        <div className="overflow-x-auto">
          <table className="w-full text-xs">
            <thead>
              <tr className="border-b border-a-border/30 text-a-muted text-left">
                <th className="py-1.5 pr-2 font-medium">目标</th>
                <th className="py-1.5 px-2 font-medium">出口方式</th>
                <th className="py-1.5 px-2 font-medium">认证</th>
                <th className="py-1.5 px-2 font-medium">底层</th>
                <th className="py-1.5 px-2 font-medium">节点</th>
                <th className="py-1.5 pl-2 font-medium">建议</th>
              </tr>
            </thead>
            <tbody>
              {egressRoutes.length > 0 ? egressRoutes.map((r, i) => (
                <tr key={i} className="border-b border-a-border/20 hover:bg-a-border/10 transition-colors">
                  <td className="py-1.5 pr-2 font-semibold text-a-fg">{r.target}</td>
                  <td className="py-1.5 px-2">
                    <ModeBadge mode={r.mode} />
                  </td>
                  <td className="py-1.5 px-2 text-a-muted">{r.auth}</td>
                  <td className="py-1.5 px-2 font-mono text-[11px] text-a-muted">{r.provider}</td>
                  <td className="py-1.5 px-2 font-mono text-[11px] text-a-muted">{r.node || '—'}</td>
                  <td className="py-1.5 pl-2">
                    {r.suggestion ? (
                      <span className={cn(
                        'text-[10px]',
                        r.suggestion.includes('跨机') ? 'text-[#e8b830]' :
                        r.suggestion.includes('SDK') ? 'text-[#a865ff]' : 'text-a-muted',
                      )}>{r.suggestion}</span>
                    ) : (
                      <span className="text-[#4cd964] text-[10px]">✓ 最优</span>
                    )}
                  </td>
                </tr>
              )) : (
                <tr><td colSpan={6} className="py-6 text-center text-a-muted text-xs">
                  暂无数据 · 注册服务或配置路由后自动生成
                </td></tr>
              )}
            </tbody>
          </table>
        </div>

        {/* Legend */}
        <div className="mt-3 flex items-center gap-4 text-[10px] text-a-muted flex-wrap pt-2 border-t border-a-border/20">
          <span className="flex items-center gap-1"><span className="w-2 h-2 rounded-full bg-[#4cd964]" /> SDK 直连</span>
          <span className="flex items-center gap-1"><span className="w-2 h-2 rounded-full bg-[#e8b830]" /> 透明代理</span>
          <span className="flex items-center gap-1"><span className="w-2 h-2 rounded-full bg-a-border" /> DNS 上游</span>
          <span className="flex items-center gap-1"><span className="w-2 h-2 rounded-full bg-[#ff5c72]" /> 未注册</span>
        </div>
      </Card>

      {/* Delete rule confirmation */}
      {deleteRuleId && (
        <Modal onClose={() => setDeleteRuleId(null)} title="确认删除"
          footer={
            <div className="flex items-center gap-2 justify-end">
              <Btn onClick={() => setDeleteRuleId(null)} className="text-xs">取消</Btn>
              <Btn onClick={() => deleteRule.mutate(deleteRuleId)} danger className="text-xs" disabled={deleteRule.isPending}>
                {deleteRule.isPending ? '删除中...' : '确认删除'}
              </Btn>
            </div>
          }>
          <p className="text-sm text-a-muted">确定要删除此劫持规则吗？iptables 规则将被移除。</p>
        </Modal>
      )}
    </div>
  );
}

// ══════════════════════════════════════════════════════════════════
// Sub-components
// ══════════════════════════════════════════════════════════════════

function ModeBadge({ mode }: { mode: EgressMode }) {
  const styles: Record<EgressMode, { bg: string; text: string; label: string }> = {
    sdk_direct:        { bg: 'bg-[#4cd964]/10', text: 'text-[#4cd964]', label: 'SDK 直连' },
    transparent_proxy: { bg: 'bg-[#e8b830]/10', text: 'text-[#e8b830]', label: '透明代理' },
    dns_upstream:      { bg: 'bg-a-border/15',  text: 'text-a-muted',   label: 'DNS 上游' },
    unregistered:      { bg: 'bg-[#ff5c72]/10', text: 'text-[#ff5c72]', label: '未注册' },
  };
  const s = styles[mode];
  return <span className={cn('px-1.5 py-0.5 rounded text-[10px] font-medium', s.bg, s.text)}>{s.label}</span>;
}

// ══════════════════════════════════════════════════════════════════
// Data fusion: build egress routes
// ══════════════════════════════════════════════════════════════════

function buildEgressRoutes(
  services: ServiceRecord[],
  rules: TransparentRule[],
  dns: DnsStatus | undefined,
): EgressRoute[] {
  const seen = new Set<string>();
  const routes: EgressRoute[] = [];

  // 1. SDK Direct — services that registered via ServiceAuth
  for (const svc of services) {
    if (seen.has(svc.name)) continue;
    seen.add(svc.name);

    // Check if this service also has a transparent proxy rule
    const hasTP = rules.some(r =>
      r.target_service?.includes(svc.id) ||
      (r.original_ip === svc.host && r.original_port === svc.port)
    );

    if (svc.status === 'active') {
      routes.push({
        target: svc.name,
        mode: hasTP ? 'transparent_proxy' : 'sdk_direct',
        auth: hasTP ? '无' : 'Ticket',
        provider: hasTP ? 'iptables' : '—',
        node: svc.node_host || '—',
        suggestion: hasTP ? '⚠ SDK 已注册但仍被透明代理劫持' : undefined,
      });
    } else {
      routes.push({
        target: svc.name,
        mode: 'unregistered',
        auth: '—',
        provider: '—',
        node: svc.node_host || '—',
        suggestion: '状态异常',
      });
    }
  }

  // 2. Transparent Proxy only — rules not associated with any SDK service
  for (const r of rules) {
    const matched = routes.some(er => er.target === r.target_service);
    if (matched) continue;
    if (seen.has(r.target_service)) continue;
    seen.add(r.target_service);

    routes.push({
      target: r.target_service?.slice(0, 16) || `${r.original_ip}:${r.original_port}`,
      mode: 'transparent_proxy',
      auth: '无',
      provider: 'iptables',
      node: r.target_node || '—',
    });
  }

  // 3. DNS upstream — domains resolved but not in services
  if (dns?.entries) {
    for (const e of dns.entries) {
      const domainName = e.domain.replace(/\.aegis\.internal$/i, '');
      if (seen.has(domainName)) continue;
      if (seen.has(e.domain)) continue;
      seen.add(e.domain);

      const isLocalManaged = e.is_local || !!e.target_node;
      if (!isLocalManaged) {
        routes.push({
          target: e.domain,
          mode: 'dns_upstream',
          auth: '无',
          provider: dns.upstream || '上游 DNS',
          node: '外部',
        });
      }
    }
  }

  // Add cross-node suggestions
  for (const r of routes) {
    if (r.mode === 'transparent_proxy' && r.node && r.node !== '—' && r.node !== '外部') {
      r.suggestion = '⚠ 跨机流量 · 建议创建 GatewayLink';
    }
    if (r.mode === 'unregistered' && r.node) {
      r.suggestion = r.suggestion || '建议接入 ServiceAuth SDK';
    }
  }

  return routes;
}

// ══════════════════════════════════════════════════════════════════
// Helpers
// ══════════════════════════════════════════════════════════════════

function fmtBytes(b: number | undefined): string {
  if (!b) return '0';
  if (b < 1024) return `${b}B`;
  if (b < 1024 * 1024) return `${(b / 1024).toFixed(1)}KB`;
  return `${(b / (1024 * 1024)).toFixed(1)}MB`;
}

// ─── TLS 证书管理 (v1.9C) ───
// 证书资产与自动 TLS 状态共享观察入口，但不共享 CRUD 生命周期。

import { useState, useMemo, useEffect } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { certApi, acmeApi, type CertificateItem } from '@/lib/api-bridge';
import { PageHeader, Card, Btn, useToast, LoadingState, ErrorBanner, Modal } from '@/components/shared';
import { certSourceMeta } from '@/lib/route-display';
import { cn } from '@/lib/utils';

function sourceBadge(source: string) {
  const m = certSourceMeta(source);
  return (
    <span className={cn('px-1.5 py-0.5 rounded text-[9px] font-medium border whitespace-nowrap', m.color)}
      title={m.desc}>
      {m.label}
    </span>
  );
}

// ─── Helpers ───

function parseDomains(item: CertificateItem): string {
  try {
    const arr = JSON.parse(item.domains);
    return Array.isArray(arr) ? arr.join(', ') : item.domains;
  } catch {
    return item.domains;
  }
}

function expiryStatus(notAfter: string): { label: string; date: string; accent: boolean; warn: boolean; danger: boolean } {
  const d = new Date(notAfter);
  const now = new Date();
  const days = Math.floor((d.getTime() - now.getTime()) / 86400000);
  const dateStr = d.toLocaleDateString('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit' });
  if (days < 0) return { label: '已过期', date: dateStr, accent: false, warn: false, danger: true };
  if (days <= 30) return { label: `${days} 天后`, date: dateStr, accent: false, warn: true, danger: false };
  if (days <= 90) return { label: `${days} 天后`, date: dateStr, accent: true, warn: false, danger: false };
  return { label: '有效', date: dateStr, accent: true, warn: false, danger: false };
}

type Tab = 'assets' | 'automatic_tls';

// ══════════════════════════════════════════════════════════════════

export default function Certificates() {
  const toast = useToast();
  const qc = useQueryClient();
  const [tab, setTab] = useState<Tab>('assets');
  const [showUpload, setShowUpload] = useState(false);
  const [showACME, setShowACME] = useState(false);
  const [acmeDomain, setAcmeDomain] = useState('');
  const [note, setNote] = useState('');
	const [uploadSource, setUploadSource] = useState<'manual_upload' | 'external'>('manual_upload');
  const [certPEM, setCertPEM] = useState('');
  const [keyPEM, setKeyPEM] = useState('');
	const [bindingCertId, setBindingCertId] = useState<string | null>(null);
	const [bindingSelection, setBindingSelection] = useState<string[]>([]);

  const { data: acmeStatus } = useQuery({
    queryKey: ['acme-status'],
    queryFn: () => acmeApi.status(),
    refetchInterval: 60_000,
  });

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['certificates'],
    queryFn: () => certApi.list(),
    refetchInterval: 60_000,
  });
  const assets: CertificateItem[] = data?.assets || data?.certificates || [];
	const automaticTLS: CertificateItem[] = data?.automatic_tls || [];
	const certs = tab === 'assets' ? assets : automaticTLS;

	const filtered = useMemo(() => certs, [certs]);
	const { data: bindingPreview, isFetching: bindingPreviewLoading } = useQuery({
		queryKey: ['certificate-binding-preview', bindingCertId],
		queryFn: () => certApi.bindingPreview(bindingCertId!),
		enabled: !!bindingCertId,
	});
	useEffect(() => {
		setBindingSelection(bindingPreview?.candidates.filter(candidate => !(candidate.already_bound ?? candidate.selected)).map(candidate => candidate.route_id) || []);
	}, [bindingPreview]);
	const bindMut = useMutation({
		mutationFn: () => certApi.bindRoutes(bindingCertId!, bindingSelection),
		onSuccess: (result: any) => {
			qc.invalidateQueries({ queryKey: ['certificates'] });
			qc.invalidateQueries({ queryKey: ['routes'] });
			toast(result?.status === 'pending_apply' ? '绑定已保存，等待配置发布' : '证书已批量绑定');
			setBindingCertId(null);
		},
		onError: (e: any) => toast(e.message || '批量绑定失败', 'error'),
	});

  const uploadMut = useMutation({
    mutationFn: () => certApi.uploadText(certPEM, keyPEM, note, uploadSource),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['certificates'] });
      toast('证书已上传');
	  setShowUpload(false); setNote(''); setUploadSource('manual_upload'); setCertPEM(''); setKeyPEM('');
    },
    onError: (e: any) => toast(e.message || '上传失败', 'error'),
  });

  const deleteMut = useMutation({
    mutationFn: (id: string) => certApi.delete(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['certificates'] });
      toast('证书已删除');
      setDeleteId(null);
    },
    onError: (e: any) => toast(e.message || '删除失败', 'error'),
  });

  const [deleteId, setDeleteId] = useState<string | null>(null);
	const { data: deletePreview, isFetching: deletePreviewLoading, error: deletePreviewError, refetch: retryDeletePreview } = useQuery({
    queryKey: ['certificate-delete-preview', deleteId],
    queryFn: () => certApi.deletePreview(deleteId!),
    enabled: !!deleteId,
  });

  const acmeAvailable = (acmeStatus as any)?.available || false;
  const acmeMsg = (acmeStatus as any)?.message || '';

  // Stats
  const expired = certs.filter(c => expiryStatus(c.not_after).danger).length;
  const expiringSoon = certs.filter(c => expiryStatus(c.not_after).warn).length;
  const valid = certs.filter(c => expiryStatus(c.not_after).accent && !expiryStatus(c.not_after).warn && !expiryStatus(c.not_after).danger).length;

  const acmeMut = useMutation({
    mutationFn: (domain: string) => acmeApi.obtain([domain]),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['certificates'] });
      toast('ACME 证书申请成功');
      setShowACME(false); setAcmeDomain('');
    },
    onError: (e: any) => toast(e.message || 'ACME 申请失败', 'error'),
  });

  return (
    <div className="p-6 space-y-5">
      <PageHeader
        title="TLS 证书"
        subtitle="管理可移植证书资产，并观察入口的自动 TLS 状态"
        actions={tab === 'assets' ? (
          <div className="flex items-center gap-2">
            <span className={cn('flex items-center gap-1 text-[10px]', acmeAvailable ? 'text-[#4cd964]' : 'text-a-muted')}>
              <span className={cn('w-1.5 h-1.5 rounded-full', acmeAvailable ? 'bg-[#4cd964]' : 'bg-a-border')} />
              {acmeAvailable ? 'ACME 就绪' : acmeMsg || 'ACME 不可用'}
            </span>
            <Btn onClick={() => setShowACME(true)} disabled={!acmeAvailable} className="text-xs">申请证书</Btn>
            <Btn primary onClick={() => setShowUpload(true)}>上传证书</Btn>
          </div>
        ) : undefined}
      />

	  {data?.observation_warning && (
		<ErrorBanner
		  message={`自动 TLS 状态读取失败：${data.observation_warning}`}
		  onRetry={() => { void refetch(); }}
		/>
	  )}

      {/* ── Tabs ── */}
      <div className="flex gap-1 bg-a-surface border border-a-border/30 rounded-a-sm p-0.5 w-fit">
        {([
          { key: 'assets' as Tab, label: '证书资产', count: assets.length },
          { key: 'automatic_tls' as Tab, label: '自动 TLS', count: automaticTLS.length },
        ]).map(t => (
          <button key={t.key} onClick={() => setTab(t.key)}
            className={cn('px-3 py-1 rounded-a-sm text-xs font-medium transition-colors',
              tab === t.key ? 'bg-a-bg text-a-fg shadow-sm' : 'text-a-muted hover:text-a-fg')}>
            {t.label} {t.count > 0 && <span className="text-[10px] opacity-60">({t.count})</span>}
          </button>
        ))}
      </div>

      {/* ── Stats ── */}
      {certs.length > 0 && (
		<div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
          <div className="bg-a-surface border border-a-border/30 rounded-a-sm px-3 py-2 text-center">
            <div className="text-lg font-bold text-a-fg">{certs.length}</div>
            <div className="text-[10px] text-a-muted">总数</div>
          </div>
          <div className="bg-a-surface border border-a-border/30 rounded-a-sm px-3 py-2 text-center">
            <div className="text-lg font-bold text-[#4cd964]">{valid}</div>
            <div className="text-[10px] text-a-muted">有效</div>
          </div>
          <div className="bg-a-surface border border-a-border/30 rounded-a-sm px-3 py-2 text-center">
            <div className={cn('text-lg font-bold', expiringSoon > 0 ? 'text-[#e8b830]' : 'text-a-muted')}>{expiringSoon}</div>
            <div className="text-[10px] text-a-muted">即将过期</div>
          </div>
          <div className="bg-a-surface border border-a-border/30 rounded-a-sm px-3 py-2 text-center">
            <div className={cn('text-lg font-bold', expired > 0 ? 'text-[#ff5c72]' : 'text-a-muted')}>{expired}</div>
            <div className="text-[10px] text-a-muted">已过期</div>
          </div>
        </div>
      )}

      {/* ── Certificate list ── */}
      {isLoading ? <LoadingState /> : error ? <ErrorBanner message="加载失败" onRetry={refetch} /> : filtered.length === 0 ? (
        <Card>
          <div className="py-10 text-center text-a-muted">
			<p className="text-sm">{tab === 'automatic_tls' ? '暂无自动 TLS 状态' : '暂无证书资产'}</p>
            <p className="text-xs mt-1 opacity-60 mb-4">
              {tab === 'automatic_tls' ? '入口使用自动 TLS 后，执行器托管状态会显示在这里' :
               '上传 PEM 证书和私钥，或通过 ACME 创建可移植证书资产'}
            </p>
            {tab === 'assets' && (
              <div className="flex items-center justify-center gap-3">
                <Btn primary onClick={() => setShowUpload(true)}>上传证书</Btn>
				{acmeAvailable ? <Btn onClick={() => setShowACME(true)}>ACME 申请</Btn> : (
                  <span className="text-[10px] text-a-muted">{acmeMsg}</span>
                )}
              </div>
            )}
          </div>
        </Card>
      ) : (
        <Card>
          <div className="overflow-x-auto">
            <table className="w-full text-xs">
              <thead>
                <tr className="border-b border-a-border/30 text-a-muted text-left">
                  <th className="py-1.5 pr-2 font-medium">域名</th>
                  <th className="py-1.5 px-2 font-medium">来源</th>
                  <th className="py-1.5 px-2 font-medium">签发者</th>
                  <th className="py-1.5 px-2 font-medium text-center">到期</th>
                  <th className="py-1.5 px-2 font-medium">续期</th>
				  <th className="py-1.5 px-2 font-medium text-center">使用情况</th>
                  <th className="py-1.5 pl-2 font-medium w-16"></th>
                </tr>
              </thead>
              <tbody>
                {filtered.map((c) => {
                  const es = expiryStatus(c.not_after);
                  return (
                    <tr key={c.id || parseDomains(c)} className="border-b border-a-border/20 hover:bg-a-border/10 transition-colors">
                      <td className="py-1.5 pr-2 font-mono text-a-fg text-[11px]">{parseDomains(c)}</td>
                      <td className="py-1.5 px-2">{sourceBadge(c.source)}</td>
                      <td className="py-1.5 px-2 font-mono text-[10px] text-a-muted max-w-[160px] truncate" title={c.issuer}>
                        {c.issuer.split(',')[0]?.replace('CN=', '') || c.issuer}
                      </td>
                      <td className="py-1.5 px-2 text-center">
                        <span className={cn(
                          'px-1.5 py-0.5 rounded text-[10px] font-medium',
                          es.danger ? 'bg-[#ff5c72]/10 text-[#ff5c72]' :
                          es.warn ? 'bg-[#e8b830]/10 text-[#e8b830]' :
                          'bg-[#4cd964]/10 text-[#4cd964]',
                        )} title={`到期: ${es.date}`}>{es.label}</span>
                      </td>
                      <td className="py-1.5 px-2">
						{c.auto_renew ? (
						  <span className="text-[10px] text-purple-400"
							title={c.source === 'gateway_auto' ? '由当前自动 TLS 执行器管理续期' : '由 Aegis ACME 定时续期'}>
							{c.source === 'gateway_auto' ? '执行器托管' : 'Aegis 自动'}
						  </span>
                        ) : (
                          <span className="text-[10px] text-a-muted">手动</span>
                        )}
                      </td>
					  <td className="py-1.5 px-2 text-center">
						<span className="text-[10px] text-a-muted">
						  {c.record_type === 'provider_observation' ? `${c.ref_count || 0} 个域名 · 中间件托管` : `${c.ref_count || 0} 个域名`}
						</span>
					  </td>
                      <td className="py-1.5 pl-2 text-right">
                        {c.source === 'gateway_auto' ? (
                          <span className="text-[9px] text-a-muted/50" title="随入口 TLS 策略管理，不是可删除资产">入口 TLS 策略</span>
                        ) : (
						  <div className="flex items-center justify-end gap-1">
							{parseDomains(c).includes('*.') && (
							  <Btn onClick={() => setBindingCertId(c.id)} className="text-[9px]">批量绑定</Btn>
							)}
							<Btn onClick={() => setDeleteId(c.id)} className="text-[9px]" danger>删除</Btn>
						  </div>
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </Card>
      )}

      {/* ── Upload Modal ── */}
      {showUpload && (
        <Modal onClose={() => setShowUpload(false)} title="导入 TLS 证书"
          footer={
            <div className="flex items-center gap-2 justify-end">
              <Btn onClick={() => setShowUpload(false)} className="text-xs">取消</Btn>
              <Btn onClick={() => uploadMut.mutate()} primary className="text-xs"
                disabled={uploadMut.isPending || !certPEM || !keyPEM}>
                {uploadMut.isPending ? '上传中...' : '导入证书'}
              </Btn>
            </div>
          }>
          <div className="space-y-3 text-xs">
            <div>
              <label className="text-a-muted block mb-1 font-medium">证书 (PEM)</label>
              <textarea value={certPEM} onChange={e => setCertPEM(e.target.value)}
                placeholder="粘贴证书内容，例如：&#10;-----BEGIN CERTIFICATE-----&#10;MIID...&#10;-----END CERTIFICATE-----"
                rows={5}
                className="w-full bg-a-bg border border-a-border rounded-a-sm px-3 py-2 text-a-fg text-xs font-mono resize-none outline-none focus:border-a-accent/50" />
            </div>
            <div>
              <label className="text-a-muted block mb-1 font-medium">私钥 (PEM)</label>
              <textarea value={keyPEM} onChange={e => setKeyPEM(e.target.value)}
                placeholder="粘贴私钥内容，例如：&#10;-----BEGIN PRIVATE KEY-----&#10;MIIE...&#10;-----END PRIVATE KEY-----"
                rows={5}
                className="w-full bg-a-bg border border-a-border rounded-a-sm px-3 py-2 text-a-fg text-xs font-mono resize-none outline-none focus:border-a-accent/50" />
            </div>
            <div>
              <label className="text-a-muted block mb-1 font-medium">渠道（可选）</label>
			  <select value={note.startsWith('CF:') ? 'cloudflare' : note.startsWith('DC:') ? 'digicert' : 'other'}
                onChange={e => {
                  const v = e.target.value;
				  if (v === 'cloudflare') { setNote('CF: Cloudflare Origin CA'); setUploadSource('external'); }
				  else if (v === 'digicert') { setNote('DC: DigiCert'); setUploadSource('external'); }
				  else { setNote(''); setUploadSource('manual_upload'); }
                }}
                className="w-full bg-a-bg border border-a-border rounded-a-sm px-2 py-1 text-a-fg text-xs">
                <option value="other">通用 / 不标注</option>
                <option value="cloudflare">Cloudflare Origin CA</option>
                <option value="digicert">DigiCert</option>
              </select>
            </div>
            <p className="text-[10px] text-a-muted">直接粘贴 PEM 文本内容。导入后可在创建路由时选择此证书替代 Let's Encrypt 自动签发。</p>
          </div>
        </Modal>
      )}

      {/* ── ACME Modal ── */}
      {showACME && (
        <Modal onClose={() => setShowACME(false)} title="ACME 一键申请证书"
          footer={
            <div className="flex items-center gap-2 justify-end">
              <Btn onClick={() => setShowACME(false)} className="text-xs">取消</Btn>
              <Btn onClick={() => acmeMut.mutate(acmeDomain)} primary className="text-xs"
                disabled={acmeMut.isPending || !acmeDomain}>
                {acmeMut.isPending ? '申请中...' : '申请证书'}
              </Btn>
            </div>
          }>
          <div className="space-y-3 text-xs">
            <div>
              <label className="text-a-muted block mb-1">域名</label>
              <input value={acmeDomain} onChange={e => setAcmeDomain(e.target.value)}
                placeholder="api.example.com"
                className="w-full bg-a-bg border border-a-border rounded-a-sm px-2 py-1.5 text-a-fg text-xs" />
            </div>
            <p className="text-[10px] text-a-muted">
              通过 Let's Encrypt 自动签发。需要域名已解析到本机且 80 端口可用。
              {acmeMsg && acmeMsg.includes('未配置') && <span className="text-[#e8b830] block mt-1">⚠ {acmeMsg}</span>}
            </p>
          </div>
        </Modal>
      )}

      {bindingCertId && (
		<Modal onClose={() => setBindingCertId(null)} title="批量绑定匹配域名"
		  footer={
			<div className="flex items-center gap-2 justify-end">
			  <Btn onClick={() => setBindingCertId(null)} className="text-xs">取消</Btn>
			  <Btn primary onClick={() => bindMut.mutate()} className="text-xs"
				disabled={bindingPreviewLoading || bindMut.isPending || bindingSelection.length === 0}>
				{bindMut.isPending ? '绑定中...' : `绑定所选域名 (${bindingSelection.length})`}
			  </Btn>
			</div>
		  }>
		  {bindingPreviewLoading ? <p className="text-sm text-a-muted">正在匹配域名...</p> : (
			<div className="space-y-3">
			  <p className="text-xs text-a-muted">只列出证书 SAN/CN 能覆盖的 TLS 终结入口；通配符只匹配一层子域。</p>
			  {(bindingPreview?.candidates.length || 0) === 0 ? (
				<p className="text-sm text-a-muted">当前没有匹配的域名入口。</p>
			  ) : (
				<div className="divide-y divide-a-border/30 rounded-a-sm border border-a-border/40">
				  {bindingPreview?.candidates.map(candidate => (
					<label key={candidate.route_id} className="flex items-center justify-between gap-3 px-3 py-2 text-xs">
					  <span className="font-mono text-a-fg">{candidate.domain}</span>
					  <span className="flex items-center gap-2 text-a-muted">
						{candidate.replaces_automatic_tls && <span className="text-[#e8b830]">将替换自动 TLS</span>}
						{(candidate.already_bound ?? candidate.selected) && <span>已绑定</span>}
						<input type="checkbox" disabled={candidate.already_bound ?? candidate.selected}
						  checked={(candidate.already_bound ?? candidate.selected) || bindingSelection.includes(candidate.route_id)}
						  onChange={event => setBindingSelection(current => event.target.checked
							? [...current, candidate.route_id]
							: current.filter(id => id !== candidate.route_id))} />
					  </span>
					</label>
				  ))}
				</div>
			  )}
			</div>
		  )}
		</Modal>
	  )}

      {/* ── Delete Confirm ── */}
      {deleteId && (
        <Modal onClose={() => setDeleteId(null)} title="确认删除"
          footer={
            <div className="flex items-center gap-2 justify-end">
              <Btn onClick={() => setDeleteId(null)} className="text-xs">取消</Btn>
			  <Btn onClick={() => deleteMut.mutate(deleteId)} danger className="text-xs"
				disabled={deleteMut.isPending || deletePreviewLoading || !deletePreview?.allowed}>
                {deleteMut.isPending ? '删除中...' : '确认删除'}
              </Btn>
            </div>
          }>
		  {deletePreviewError ? (
			<div role="alert" className="space-y-2 text-sm">
			  <p className="text-[#ff8a9b]">无法检查证书引用，请重试后再删除。</p>
			  <Btn onClick={() => retryDeletePreview()} className="text-xs">重新检查</Btn>
			</div>
		  ) : deletePreviewLoading ? (
			<p className="text-sm text-a-muted">正在检查证书引用...</p>
		  ) : deletePreview?.allowed ? (
			<div className="space-y-2 text-sm">
			  <p className="text-a-fg">该证书当前没有域名引用，可以安全删除。</p>
			  <p className="text-xs text-a-muted">证书记录和 Aegis 管理的 PEM 文件将一并删除。</p>
			</div>
		  ) : (
			<div className="space-y-3 text-sm">
			  <p className="text-[#ff8a9b]">{deletePreview?.reason || '当前不能删除该证书。'}</p>
			  {(deletePreview?.references?.length || 0) > 0 && (
				<div>
				  <div className="mb-1 text-xs font-medium text-a-muted">引用域名</div>
				  <div className="divide-y divide-a-border/30 rounded-a-sm border border-a-border/40">
					{deletePreview?.references.map(ref => (
					  <div key={ref.id} className="flex items-center justify-between gap-3 px-3 py-2">
						<span className="font-mono text-xs text-a-fg">{ref.domain}</span>
						<a href={`/exposure/entry/${ref.id}`} className="text-xs text-a-accent hover:underline">处理绑定</a>
					  </div>
					))}
				  </div>
				</div>
			  )}
			</div>
		  )}
        </Modal>
      )}
    </div>
  );
}

// ─── TLS 证书管理 (v1.9C) ───
// 集中证书中心：上传 PEM 证书、查看列表、过期提醒、删除。

import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { certApi, acmeApi, type CertificateItem } from '@/lib/api-bridge';
import { PageHeader, Card, Btn, useToast, LoadingState, ErrorBanner, Modal } from '@/components/shared';
import { cn } from '@/lib/utils';

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

// ══════════════════════════════════════════════════════════════════

export default function Certificates() {
  const toast = useToast();
  const qc = useQueryClient();
  const [showUpload, setShowUpload] = useState(false);
  const [showACME, setShowACME] = useState(false);
  const [acmeDomain, setAcmeDomain] = useState('');
  const [note, setNote] = useState('');
  const [certPEM, setCertPEM] = useState('');
  const [keyPEM, setKeyPEM] = useState('');

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
  const certs: CertificateItem[] = (data as any)?.certificates || [];

  const uploadMut = useMutation({
    mutationFn: () => certApi.uploadText(certPEM, keyPEM, note),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['certificates'] });
      toast('证书已上传');
      setShowUpload(false); setNote(''); setCertPEM(''); setKeyPEM('');
    },
    onError: (e: any) => toast(e.message || '上传失败', 'error'),
  });

  const deleteMut = useMutation({
    mutationFn: (id: string) => certApi.delete(id),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['certificates'] }); toast('证书已删除'); },
    onError: (e: any) => toast(e.message || '删除失败', 'error'),
  });

  const [deleteId, setDeleteId] = useState<string | null>(null);

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
        subtitle="集中管理 TLS 证书，手动上传或 ACME 自动签发"
        actions={
          <div className="flex items-center gap-2">
            <span className={cn('flex items-center gap-1 text-[10px]', acmeAvailable ? 'text-[#4cd964]' : 'text-a-muted')}>
              <span className={cn('w-1.5 h-1.5 rounded-full', acmeAvailable ? 'bg-[#4cd964]' : 'bg-a-border')} />
              {acmeAvailable ? 'ACME 就绪' : acmeMsg || 'certbot 未安装'}
            </span>
            {acmeAvailable && <Btn onClick={() => setShowACME(true)} className="text-xs">🔑 申请证书</Btn>}
            <Btn primary onClick={() => setShowUpload(true)}>上传证书</Btn>
          </div>
        }
      />

      {certs.length > 0 && (
        <div className="grid grid-cols-4 gap-3">
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

      {isLoading ? <LoadingState /> : error ? <ErrorBanner message="加载失败" onRetry={refetch} /> : certs.length === 0 ? (
        <Card>
          <div className="py-10 text-center text-a-muted">
            <p className="text-lg mb-2">🔐</p>
            <p className="text-sm">暂无证书</p>
            <p className="text-xs mt-1 opacity-60 mb-4">上传 PEM 证书和私钥，或通过 ACME 自动签发</p>
            <div className="flex items-center justify-center gap-3">
              <Btn primary onClick={() => setShowUpload(true)}>上传证书</Btn>
              {acmeAvailable ? <Btn onClick={() => setShowACME(true)}>🔑 ACME 申请</Btn> : (
                <span className="text-[10px] text-a-muted">{acmeMsg}</span>
              )}
            </div>
          </div>
        </Card>
      ) : (
        <Card>
          <div className="overflow-x-auto">
            <table className="w-full text-xs">
              <thead>
                <tr className="border-b border-a-border/30 text-a-muted text-left">
                  <th className="py-1.5 pr-2 font-medium">域名</th>
                  <th className="py-1.5 px-2 font-medium">签发者</th>
                  <th className="py-1.5 px-2 font-medium text-center">到期</th>
                  <th className="py-1.5 px-2 font-medium">备注</th>
                  <th className="py-1.5 pl-2 font-medium w-16"></th>
                </tr>
              </thead>
              <tbody>
                {certs.map((c) => {
                  const es = expiryStatus(c.not_after);
                  return (
                    <tr key={c.id} className="border-b border-a-border/20 hover:bg-a-border/10 transition-colors">
                      <td className="py-1.5 pr-2 font-mono text-a-fg text-[11px]">{parseDomains(c)}</td>
                      <td className="py-1.5 px-2 font-mono text-[10px] text-a-muted max-w-[160px] truncate" title={c.issuer}>{c.issuer.split(',')[0]?.replace('CN=', '') || c.issuer}</td>
                      <td className="py-1.5 px-2 text-center">
                        <span className={cn(
                          'px-1.5 py-0.5 rounded text-[10px] font-medium',
                          es.danger ? 'bg-[#ff5c72]/10 text-[#ff5c72]' :
                          es.warn ? 'bg-[#e8b830]/10 text-[#e8b830]' :
                          'bg-[#4cd964]/10 text-[#4cd964]',
                        )} title={`到期: ${es.date}`}>{es.label}</span>
                      </td>
                      <td className="py-1.5 px-2 text-a-muted text-[11px]">{c.note || '—'}</td>
                      <td className="py-1.5 pl-2 text-right">
                        <Btn onClick={() => setDeleteId(c.id)} className="text-[9px]" danger>删除</Btn>
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
              <label className="text-a-muted block mb-1 font-medium">备注（可选）</label>
              <input value={note} onChange={e => setNote(e.target.value)} placeholder="例如：Cloudflare Origin CA / DigiCert 购买"
                className="w-full bg-a-bg border border-a-border rounded-a-sm px-2 py-1 text-a-fg text-xs" />
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
              通过 Let's Encrypt 自动签发。需要域名已解析到本机且 80 端口临时可用。
              {!acmeAvailable && <span className="text-[#ff5c72] block mt-1">{acmeMsg}</span>}
            </p>
          </div>
        </Modal>
      )}

      {/* ── Delete Confirm ── */}
      {deleteId && (
        <Modal onClose={() => setDeleteId(null)} title="确认删除"
          footer={
            <div className="flex items-center gap-2 justify-end">
              <Btn onClick={() => setDeleteId(null)} className="text-xs">取消</Btn>
              <Btn onClick={() => deleteMut.mutate(deleteId)} danger className="text-xs" disabled={deleteMut.isPending}>
                {deleteMut.isPending ? '删除中...' : '确认删除'}
              </Btn>
            </div>
          }>
          <p className="text-sm text-a-muted">删除后使用此证书的路由将回退到 ACME 自动签发或变为无效。</p>
        </Modal>
      )}
    </div>
  );
}

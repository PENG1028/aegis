/**
 * ImportConfig — 扫描并导入已有 Caddyfile 配置。
 */

import { useState } from 'react';
import { adminApi } from '@/lib/api-bridge';
import { PageHeader, Card, Btn, Alert, StatusBadge } from '@/components/shared';
import { useToast } from '@/components/shared/Toast';

export default function ImportConfigPage() {
  const toast = useToast();
  const [routes, setRoutes] = useState<any[] | null>(null);
  const [loading, setLoading] = useState(false);
  const [importing, setImporting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [result, setResult] = useState<any>(null);

  async function doScan() {
    setLoading(true);
    setError(null);
    setRoutes(null);
    setResult(null);
    try {
      const res = await adminApi.importCaddyPreview();
      setRoutes(res.routes || []);
      // Select all by default
      setSelected(new Set((res.routes || []).map((_: any, i: number) => i)));
      if (res.message) toast(res.message);
    } catch (e: any) {
      setError(e.message);
    }
    setLoading(false);
  }

  async function doImport() {
    const toImport = (routes || []).filter((_, i) => selected.has(i));
    if (toImport.length === 0) {
      toast('请选择要导入的路由', 'error');
      return;
    }
    setImporting(true);
    setError(null);
    try {
      const res = await adminApi.importCaddyConfirm(toImport);
      setResult(res);
      toast(`导入了 ${res.count || 0} 条路由`);
    } catch (e: any) {
      setError(e.message);
    }
    setImporting(false);
  }

  function toggle(i: number) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(i)) next.delete(i);
      else next.add(i);
      return next;
    });
  }

  return (
    <div>
      <PageHeader title="导入配置" helpKey="import-config"
        sub="扫描现有 Caddyfile 中的域名配置，导入到 Aegis 管理"
        actions={<Btn primary onClick={doScan} disabled={loading}>{loading ? '扫描中…' : '扫描 Caddyfile'}</Btn>} />

      {error && <Alert type="err">{error}</Alert>}

      {routes !== null && routes.length === 0 && (
        <Alert type="info">未从 Caddyfile 中发现域名配置。</Alert>
      )}

      {routes && routes.length > 0 && !result && (
        <>
          <div className="flex items-center gap-2 mb-3">
            <span className="text-xs text-a-muted">发现 {routes.length} 条路由：</span>
            <div className="flex gap-1">
              <Btn sm onClick={() => setSelected(new Set(routes.map((_, i) => i)))}>全选</Btn>
              <Btn sm onClick={() => setSelected(new Set())}>清空</Btn>
              <Btn primary sm onClick={doImport} disabled={importing}>
                {importing ? '导入中…' : `导入选中 (${selected.size})`}
              </Btn>
            </div>
          </div>

          <Card>
            <table className="w-full text-sm border-collapse">
              <thead>
                <tr>
                  <th className="w-8 px-3 py-2.5 border-b border-a-border"></th>
                  {['域名', '路径', '后端地址', 'TLS'].map((h) => (
                    <th key={h} className="text-left px-3.5 py-2.5 text-[11px] font-semibold uppercase tracking-[0.06em] text-a-muted bg-a-bg border-b border-a-border whitespace-nowrap">{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {routes.map((r: any, i: number) => (
                  <tr key={i} className="hover:bg-white/[0.04] cursor-pointer" onClick={() => toggle(i)}>
                    <td className="px-3 py-2.5 border-b border-a-border-soft">
                      <input type="checkbox" checked={selected.has(i)} onChange={() => toggle(i)}
                        className="accent-a-accent cursor-pointer" />
                    </td>
                    <td className="px-3.5 py-2.5 border-b border-a-border-soft font-mono text-xs">{r.domain}</td>
                    <td className="px-3.5 py-2.5 border-b border-a-border-soft font-mono text-xs">{r.path_prefix || '/'}</td>
                    <td className="px-3.5 py-2.5 border-b border-a-border-soft font-mono text-xs">{r.upstream_url}</td>
                    <td className="px-3.5 py-2.5 border-b border-a-border-soft">{r.tls_enabled ? <StatusBadge status="ok" /> : '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </Card>
        </>
      )}

      {result && (
        <Card title="导入结果" className="mt-4">
          <div className="p-[18px] space-y-2">
            <div className="text-xs text-a-success">✓ 成功导入 {result.count || 0} 条路由</div>
            {(result.errors || []).length > 0 && (
              <div className="text-xs text-a-warn">
                {result.errors.length} 条失败：
                {(result.errors || []).map((e: string, i: number) => (
                  <div key={i} className="text-[11px] font-mono text-a-danger mt-1">{e}</div>
                ))}
              </div>
            )}
            {result.pending_apply && (
              <div className="mt-3 text-xs text-a-warn">⚠ 需要执行 Apply 才能生效。</div>
            )}
            <div className="flex gap-2 mt-3">
              <Btn sm onClick={() => window.location.href = '/apply'}>去 Apply</Btn>
              <Btn sm onClick={() => { setRoutes(null); setResult(null); }}>继续导入</Btn>
            </div>
          </div>
        </Card>
      )}

      {!routes && !loading && (
        <Card>
          <div className="p-[18px] text-xs text-a-muted leading-relaxed">
            <p>此工具扫描服务器上已有的 Caddyfile，提取域名和后端地址映射，导入到 Aegis 管理。</p>
            <p className="mt-2">支持扫描路径：<code className="text-a-accent">/etc/caddy/Caddyfile</code></p>
            <p className="mt-2">导入后可以像管理普通路由一样管理这些域名，但需要 Apply 才能覆盖 Caddy 配置。</p>
          </div>
        </Card>
      )}
    </div>
  );
}

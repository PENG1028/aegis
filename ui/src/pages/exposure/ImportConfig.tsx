// ─── Import Config Drawer ───
// Scan Caddyfile → preview routes → select → import.
// Not a standalone page — opened from EntryPoints action bar.

import { useState, useCallback } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { Drawer, Btn, StatusBadge, HealthDot, useToast } from '@/components/shared';
import { adminApi } from '@/lib/api-bridge';
import { API_CONFIG } from '@/lib/api-config';
import { cn } from '@/lib/utils';

interface ImportedRoute {
  domain: string;
  path_prefix?: string;
  upstream_url: string;
  tls_enabled: boolean;
  strip_prefix?: boolean;
  source_file: string;
  source_line: number;
}

interface ImportDrawerProps {
  open: boolean;
  onClose: () => void;
}

export default function ImportDrawer({ open, onClose }: ImportDrawerProps) {
  const toast = useToast();
  const queryClient = useQueryClient();
  type Step = 'idle' | 'scanning' | 'preview' | 'importing';
  const [step, setStep] = useState<Step>('idle');
  const [routes, setRoutes] = useState<ImportedRoute[]>([]);
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [error, setError] = useState<string | null>(null);
  const [caddyPath, setCaddyPath] = useState<string>('');
  const [importResult, setImportResult] = useState<any>(null);

  const handleScan = useCallback(async () => {
    setStep('scanning');
    setError(null);
    try {
      const res = await adminApi.importCaddyPreview();
      if (res.routes && res.routes.length > 0) {
        setRoutes(res.routes);
        setCaddyPath(res.caddy_path || res.source_path || '/etc/caddy/Caddyfile');
        setSelected(new Set(res.routes.map((_: any, i: number) => i)));
        setStep('preview');
      } else if (res.message) {
        setError(res.message);
        setStep('idle');
      } else {
        setError('未找到可导入的路由');
        setStep('idle');
      }
    } catch (e: any) {
      setError(e.message || '扫描失败');
      setStep('idle');
    }
  }, []);

  const handleImport = useCallback(async () => {
    const toImport = routes.filter((_, i) => selected.has(i));
    if (toImport.length === 0) {
      toast('请至少选择一条路由', 'error');
      return;
    }
    setStep('importing');
    setError(null);
    try {
      const res = await adminApi.importCaddyConfirm(toImport);
      setImportResult(res);
      queryClient.invalidateQueries({ queryKey: ['routes'] });
      queryClient.invalidateQueries({ queryKey: ['entry-points'] });
      toast(`已导入 ${res.imported || res.count || toImport.length} 条路由`);
      // Reset after success
      setTimeout(() => {
        setStep('idle');
        setRoutes([]);
        setSelected(new Set());
        setImportResult(null);
        onClose();
      }, 1500);
    } catch (e: any) {
      setError(e.message || '导入失败');
      setStep('preview');
    }
  }, [routes, selected, toast, queryClient, onClose]);

  const toggleSelect = (i: number) => {
    const next = new Set(selected);
    if (next.has(i)) next.delete(i); else next.add(i);
    setSelected(next);
  };

  const toggleAll = () => {
    if (selected.size === routes.length) {
      setSelected(new Set());
    } else {
      setSelected(new Set(routes.map((_, i) => i)));
    }
  };

  return (
    <Drawer
      open={open}
      onClose={() => {
        if (step !== 'importing') {
          setStep('idle');
          setRoutes([]);
          setSelected(new Set());
          setError(null);
          setImportResult(null);
          onClose();
        }
      }}
      title="导入配置"
      subtitle={step === 'preview' ? `从 ${caddyPath} 扫描到 ${routes.length} 条路由` : '从已部署的 Caddyfile 导入路由到 Aegis'}
      width="lg"
      footer={
        step === 'preview' ? (
          <div className="flex gap-2">
            <Btn onClick={handleScan}>重新扫描</Btn>
            <Btn primary onClick={handleImport} disabled={selected.size === 0}>
              导入选中 ({selected.size}/{routes.length})
            </Btn>
          </div>
        ) : step === 'importing' ? (
          <Btn disabled>导入中...</Btn>
        ) : null
      }
    >
      <div className="space-y-4">
        {/* Explanation */}
        {step === 'idle' && (
          <div className="space-y-4">
            <div className="p-4 rounded-a-sm bg-a-border/10 text-xs text-a-muted space-y-2">
              <p>Aegis 可以扫描服务器上已有的 Caddyfile，提取域名→上游的映射关系，预览后选择性导入到路由表。</p>
              <ul className="list-disc pl-4 space-y-1 text-[11px]">
                <li>扫描标准路径：<code className="text-a-fg2">/etc/caddy/Caddyfile</code> 等</li>
                <li>解析后预览所有发现的 <code className="text-a-fg2">reverse_proxy</code> 指令</li>
                <li>勾选后确认导入 — 自动创建 Service + Endpoint + Route</li>
              </ul>
            </div>
            <Btn primary onClick={handleScan}>
              扫描 Caddyfile
            </Btn>
          </div>
        )}

        {/* Scanning */}
        {step === 'scanning' && (
          <div className="text-center py-12">
            <div className="text-3xl mb-3 animate-pulse opacity-50">🔍</div>
            <p className="text-sm text-a-fg2">正在扫描服务器上的 Caddyfile...</p>
            <p className="text-[11px] text-a-muted mt-1">检查 /etc/caddy/ 等标准路径</p>
          </div>
        )}

        {/* Error */}
        {error && step !== 'importing' && (
          <div className="p-4 rounded-a-sm bg-[#ff5c72]/5 border border-[#ff5c72]/10 text-xs text-[#ff5c72]">
            <p className="font-semibold mb-1">扫描/导入失败</p>
            <p>{error}</p>
            {step === 'idle' && (
              <p className="mt-2 text-a-muted">确认 Caddy 已安装且 Caddyfile 可读。也可以手动粘贴路由到"快速接入"。</p>
            )}
          </div>
        )}

        {/* Route preview list */}
        {step === 'preview' && routes.length > 0 && (
          <div className="space-y-1">
            {/* Select all toggle */}
            <div className="flex items-center gap-2 px-3 py-1.5 text-[10px] text-a-muted border-b border-a-border/30">
              <button onClick={toggleAll} className="text-a-accent hover:underline cursor-pointer">
                {selected.size === routes.length ? '取消全选' : '全选'}
              </button>
              <span className="flex-1" />
              <span>{selected.size}/{routes.length} 条已选</span>
            </div>

            {routes.map((r, i) => (
              <div key={i}
                onClick={() => toggleSelect(i)}
                className={cn(
                  'flex items-center gap-3 px-3 py-2.5 rounded-a-sm cursor-pointer transition-colors text-xs',
                  'border border-a-border/30 hover:bg-a-border/10',
                  selected.has(i) ? 'bg-a-accent/3 border-a-accent/20' : 'opacity-50',
                )}>
                <input type="checkbox" checked={selected.has(i)} onChange={() => toggleSelect(i)}
                  className="w-3.5 h-3.5 rounded accent-a-accent cursor-pointer shrink-0" />
                <span className="font-mono font-semibold text-a-fg w-40 shrink-0 truncate">{r.domain}</span>
                {r.path_prefix && (
                  <span className="font-mono text-[10px] bg-a-bg px-1.5 py-0.5 rounded text-a-muted shrink-0">{r.path_prefix}</span>
                )}
                <svg className="w-3 h-3 text-a-border shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M9 18l6-6-6-6"/></svg>
                <span className="font-mono text-a-fg2 flex-1 truncate">{r.upstream_url}</span>
                <span className="flex items-center gap-1 shrink-0">
                  {r.tls_enabled && <span className="text-[10px] bg-[#4cd964]/10 text-[#4cd964] px-1.5 py-0.5 rounded">TLS</span>}
                  <span className="text-[10px] text-a-muted">L{r.source_line}</span>
                </span>
              </div>
            ))}
          </div>
        )}

        {/* Success message */}
        {importResult && (
          <div className="p-4 rounded-a-sm bg-[#4cd964]/5 border border-[#4cd964]/10 text-xs text-[#4cd964]">
            <p className="font-semibold mb-1">导入成功</p>
            <p>已导入 {importResult.imported || importResult.count || 0} 条路由</p>
          </div>
        )}

        {/* Manual note */}
        {step === 'idle' && !API_CONFIG.useMock && (
          <div className="text-[10px] text-a-muted border-t border-a-border/30 pt-3">
            仅支持运行 Caddy provider 的节点。如需从其他源导入，请使用"快速接入"手动录入路由。
          </div>
        )}
      </div>
    </Drawer>
  );
}

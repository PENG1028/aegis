// ─── Release Changes ───
// Real API: configDiff for pending, applyHistory for recent.
// Falls back to scenario data in mock mode.

import { useQuery } from '@tanstack/react-query';
import { useNavigate } from 'react-router-dom';
import { Card, PageHeader, StatusBadge, Btn } from '@/components/shared';
import { adminApi } from '@/lib/api-bridge';


import { cn } from '@/lib/utils';

export default function Changes() {
  const nav = useNavigate();
  

  // Pending diff — tells us if there are un-applied changes
  const { data: diffData } = useQuery({
    queryKey: ['config-diff'],
    queryFn: () => adminApi.configDiff(),
    refetchInterval: 60_000,
  });

  // Apply history
  const { data: historyData } = useQuery({
    queryKey: ['apply-history'],
    queryFn: () => adminApi.applyHistory(),
    refetchInterval: 60_000,
  });

  // ── Pending changes ──
  const pendingFromDiff = (() => {
    // configDiff returns a diff summary; non-empty = pending changes exist
    if (diffData && typeof diffData === 'object') {
      const d = diffData as any;
      const routes = d.route_diffs || d.routes || d.changes || [];
      if (routes.length > 0) {
        return routes.map((r: any) => ({
          route_id: r.route_id || r.id || '',
          domain: r.domain || r.name || '未知',
          service_name: r.service_name || r.service || '',
          gateway_name: r.gateway_name || r.gateway || '',
          endpoints: [],
          release_state: 'pending' as const,
          health: 'unknown' as const,
          safety: 'unknown' as const,
          listener: null, gateway_id: null, service_id: '',
          protocol: 'http' as const, tls_mode: '',
        }));
      }
    }
    return [];
  })();

  // ── Recent history ──
  const recentHistory = (() => {
    if (Array.isArray(historyData)) {
      return historyData.slice(0, 10).map((h: any) => ({
        version: `v${h.revision || h.version || '?'}`,
        domain: h.domain || h.route || '—',
        action: h.action || h.message || h.summary || '配置变更',
        time: h.applied_at || h.created_at || h.timestamp || '',
        status: h.status || (h.success ? 'success' : 'failed'),
      }));
    }
    return [];
  })();

  const hasPending = pendingFromDiff.length > 0;

  return (
    <div className="p-6 space-y-6">
      <PageHeader
        title="配置变更"
        subtitle={hasPending
          ? `${pendingFromDiff.length} 项待发布 · 审核后进入 Apply 流程`
          : '无待发布变更'}
        actions={
          hasPending
            ? <Btn primary onClick={() => nav('/release/apply')}>进入发布流程</Btn>
            : undefined
        }
      />

      {/* Pending changes */}
      {hasPending && (
        <Card title={`待发布 (${pendingFromDiff.length})`} subtitle="这些变更已创建但尚未推送到节点">
          <div className="space-y-2">
            {pendingFromDiff.map((ep: any) => {
              const chain = null;
              return (
                <div key={ep.route_id}
                  className={cn(
                    'p-4 rounded-a-md border cursor-pointer transition-colors hover:brightness-105',
                    ep.release_state === 'drifted' ? 'bg-[#ff5c72]/3 border-[#ff5c72]/20 border-l-2 border-l-[#ff5c72]' :
                    'bg-[#e8b830]/3 border-[#e8b830]/20 border-l-2 border-l-[#e8b830]',
                  )}
                  onClick={() => nav(`/exposure/entry/${ep.route_id}`)}>
                  <div className="flex items-center gap-3 mb-2">
                    <span className="text-sm font-mono font-semibold text-a-fg">{ep.domain}</span>
                    <StatusBadge status={ep.release_state} />
                  </div>
                  {chain && (
                    <div className="flex items-center gap-1.5 text-[11px] text-a-muted">
                      <span className="text-a-fg2">{ep.domain}</span>
                      <svg className="w-3 h-3 text-a-border" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M9 18l6-6-6-6"/></svg>
                      <span>{ep.gateway_name || '—'}</span>
                      <svg className="w-3 h-3 text-a-border" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M9 18l6-6-6-6"/></svg>
                      <span>{ep.service_name}</span>
                      <svg className="w-3 h-3 text-a-border" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M9 18l6-6-6-6"/></svg>
                      <span>{ep.endpoints.map((e: any) => e.node_name).join(', ')}</span>
                    </div>
                  )}
                  <div className="flex items-center gap-2 mt-2">
                    <Btn onClick={(e) => { e.stopPropagation(); nav('/release/diff'); }} className="text-[10px]">查看 Diff</Btn>
                    <Btn onClick={(e) => { e.stopPropagation(); nav('/release/dry-run'); }} className="text-[10px]">Dry-run</Btn>
                  </div>
                </div>
              );
            })}
          </div>
        </Card>
      )}

      {/* Recent history */}
      <Card title={'发布历史'}>
        {recentHistory.length === 0 ? (
          <div className="text-center py-8 text-xs text-a-muted">暂无发布记录</div>
        ) : (
          <div className="space-y-1">
            {recentHistory.map((c, i) => (
              <div key={i} className="flex items-center gap-3 px-3 py-2 rounded-a-sm hover:bg-a-border/10 text-xs">
                <span className="font-mono font-medium text-a-fg w-10">{c.version}</span>
                <span className="font-mono text-a-fg2 w-40 truncate">{c.domain}</span>
                <span className="text-a-muted flex-1">{c.action}</span>
                <span className="text-[10px] text-a-muted">{c.time ? new Date(c.time).toLocaleDateString('zh-CN') : ''}</span>
                <StatusBadge status={c.status} />
              </div>
            ))}
            <button onClick={() => nav('/release/history')} className="text-[10px] text-a-accent hover:underline px-3 py-1 cursor-pointer">查看全部历史 →</button>
          </div>
        )}
      </Card>
    </div>
  );
}

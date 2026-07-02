// ─── Release Changes ───
// Shows pending and recent config changes with impact preview.

import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Card, PageHeader, StatusBadge, Btn } from '@/components/shared';
import { getScenario } from '@/mocks';
import { API_CONFIG } from '@/lib/api-config';
import { resolveChain } from '@/mocks/generators/chain-factory';
import { cn } from '@/lib/utils';

export default function Changes() {
  const nav = useNavigate();
  const scenario = API_CONFIG.useMock ? getScenario() : null;

  // Find routes with pending/drifted state
  const pendingChanges = API_CONFIG.useMock
    ? (scenario?.entryPoints || []).filter(ep => ep.release_state === 'pending' || ep.release_state === 'drifted')
    : [];

  // Recent applied changes (from history mock)
  const recentChanges = [
    { version: 'v43', domain: 'api.proofnote.dev', action: '修改端点健康检查间隔', time: '2026-07-02T10:30:00Z', status: 'success' },
    { version: 'v42', domain: 'auth.proofnote.dev', action: '添加端点 endpoint-auth-b', time: '2026-07-01T08:15:00Z', status: 'success' },
  ];

  return (
    <div className="p-6 space-y-6">
      <PageHeader
        title="配置变更"
        subtitle={pendingChanges.length > 0
          ? `${pendingChanges.length} 项待发布 · 审核后进入 Apply 流程`
          : '无待发布变更'}
        actions={
          pendingChanges.length > 0
            ? <Btn primary onClick={() => nav('/release/apply')}>进入发布流程</Btn>
            : undefined
        }
      />

      {/* Pending changes */}
      {pendingChanges.length > 0 && (
        <Card title={`待发布 (${pendingChanges.length})`} subtitle="这些变更已创建但尚未推送到节点">
          <div className="space-y-2">
            {pendingChanges.map(ep => {
              const chain = API_CONFIG.useMock ? resolveChain('route', ep.route_id) : null;
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
                  {/* Mini chain */}
                  {chain && (
                    <div className="flex items-center gap-1.5 text-[11px] text-a-muted">
                      <span className="text-a-fg2">{ep.domain}</span>
                      <svg className="w-3 h-3 text-a-border" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M9 18l6-6-6-6"/></svg>
                      <span>{ep.gateway_name || '—'}</span>
                      <svg className="w-3 h-3 text-a-border" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M9 18l6-6-6-6"/></svg>
                      <span>{ep.service_name}</span>
                      <svg className="w-3 h-3 text-a-border" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M9 18l6-6-6-6"/></svg>
                      <span>{ep.endpoints.map(e => e.node_name).join(', ')}</span>
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
      <Card title="最近发布">
        <div className="space-y-1">
          {recentChanges.map((c, i) => (
            <div key={i} className="flex items-center gap-3 px-3 py-2 rounded-a-sm hover:bg-a-border/10 text-xs">
              <span className="font-mono font-medium text-a-fg w-10">{c.version}</span>
              <span className="font-mono text-a-fg2 w-40 truncate">{c.domain}</span>
              <span className="text-a-muted flex-1">{c.action}</span>
              <span className="text-[10px] text-a-muted">{new Date(c.time).toLocaleDateString('zh-CN')}</span>
              <StatusBadge status={c.status} />
            </div>
          ))}
          <button onClick={() => nav('/release/history')} className="text-[10px] text-a-accent hover:underline px-3 py-1 cursor-pointer">查看全部历史 →</button>
        </div>
      </Card>
    </div>
  );
}

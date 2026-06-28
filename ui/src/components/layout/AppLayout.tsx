/**
 * AppLayout — v2 style with TopBar showing state/leader/apply status.
 */

import { useEffect, useState } from 'react';
import { Outlet, useNavigate } from 'react-router-dom';
import { Sidebar } from './Sidebar';
import { Btn } from '@/components/shared';
import { useAuth } from '@/lib/auth-context';
import { system as systemApi } from '@/lib/api-bridge';
import { API_CONFIG } from '@/lib/api-config';

export function AppLayout() {
  const { user, logout } = useAuth();
  const navigate = useNavigate();

  const [overview, setOverview] = useState<{
    stateVersion?: string;
    leaderNode?: string;
    pendingApply?: boolean;
  } | null>(null);

  useEffect(() => {
    systemApi.overview()
      .then((ov) => {
        setOverview({
          stateVersion: ov.last_apply?.version || '—',
          leaderNode: ov.leader_node || '—',
          pendingApply: false,
        });
      })
      .catch(() => {
        // Silent fail — overview is nice-to-have
      });
  }, []);

  return (
    <div className="flex h-screen overflow-hidden bg-a-bg">
      <Sidebar pendingApply={overview?.pendingApply} />

      <div className="flex-1 flex flex-col overflow-hidden">
        {/* TopBar — v2 style */}
        <header className="h-12 bg-a-surface border-b border-a-border flex items-center px-5 gap-4 shrink-0">
          {overview ? (
            <>
              <div className="flex items-center gap-2">
                <span className="text-[11px] text-a-muted uppercase tracking-[0.06em]">状态</span>
                <span className="font-mono text-xs text-a-accent">{overview.stateVersion}</span>
              </div>
              <div className="w-px h-5 bg-a-border" />
              <div className="flex items-center gap-2">
                <span className="text-[11px] text-a-muted uppercase tracking-[0.06em]">主节点</span>
                <span className="font-mono text-xs">{overview.leaderNode}</span>
              </div>
              <div className="w-px h-5 bg-a-border" />
              <div className="flex items-center gap-2">
                <span className="text-[11px] text-a-muted uppercase tracking-[0.06em]">推送</span>
                <span className={`inline-flex items-center gap-1 font-mono text-[11px] px-2.5 py-0.5 rounded-[10px] ${
                  overview.pendingApply
                    ? 'bg-[#e8b830]/20 text-[#e8b830]'
                    : 'bg-[#4cd964]/20 text-[#4cd964]'
                }`}>
                  ● {overview.pendingApply ? '待应用' : '已应用'}
                </span>
              </div>
            </>
          ) : (
            <div className="flex items-center gap-2">
              <span className="text-[11px] text-a-muted uppercase tracking-[0.06em]">控制台</span>
              <span className="w-px h-4 bg-a-border" />
              <span className="font-mono text-xs text-a-muted">v1.8C</span>
            </div>
          )}

          {API_CONFIG.useMock && (
            <>
              <div className="w-px h-5 bg-a-border" />
              <span className="inline-flex items-center gap-1 font-mono text-[11px] px-2 py-0.5 rounded bg-[#e8b830]/20 text-[#e8b830]">
                Mock 数据
              </span>
            </>
          )}

          <div className="ml-auto flex items-center gap-3">
            <span className="text-[11px] text-a-muted">{user?.username}</span>
            <Btn sm onClick={() => { logout(); navigate('/'); }}>
              登出
            </Btn>
          </div>
        </header>

        {/* Main content */}
        <main className="flex-1 overflow-auto p-6">
          <Outlet />
        </main>
      </div>
    </div>
  );
}

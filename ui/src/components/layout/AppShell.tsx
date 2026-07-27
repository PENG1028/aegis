// ─── AppShell ───
// Layout: TopBar + Left Sidebar (8 workspaces) + Content

import { Outlet, useNavigate } from 'react-router-dom';
import { useState, useEffect } from 'react';
import { useAuth } from '@/lib/auth-context';
import { system } from '@/lib/api-bridge';
import { WorkbenchSidebar } from './WorkbenchSidebar';
import { NodeSwitcher } from './NodeSwitcher';

export function AppShell() {
  const { user, logout } = useAuth();
  const navigate = useNavigate();
  const [statusVersion, setStatusVersion] = useState('...');
  const [panelHost, setPanelHost] = useState('');
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);

  useEffect(() => {
    system.status()
      .then((s: any) => {
        setStatusVersion(s.version || 'dev');
        // Use configured domain, or fall back to browser host
        const host = s.panel_domain || window.location.host;
        setPanelHost(host);
      })
      .catch(() => setStatusVersion('dev'));
  }, []);

  return (
    <div className="h-screen flex flex-col bg-a-bg overflow-hidden">
      {/* ── Top Bar ── */}
      <header className="h-10 shrink-0 bg-a-surface border-b border-a-border flex items-center px-4 gap-3 z-30">
        <button onClick={() => navigate('/')} className="flex items-center gap-2 text-a-fg hover:text-a-accent transition-colors cursor-pointer shrink-0">
          <svg className="w-4 h-4" viewBox="0 0 24 24" fill="currentColor"><path d="M12 2L2 7l10 5 10-5-10-5zM2 17l10 5 10-5M2 12l10 5 10-5" /></svg>
          <span className="text-xs font-bold tracking-wide">AEGIS</span>
        </button>
        {panelHost && (
          <span className="text-[10px] text-a-muted font-mono hidden sm:inline">{panelHost}</span>
        )}
        <span className="text-[10px] text-a-muted font-mono">{statusVersion}</span>
        <div className="flex-1" />
        <NodeSwitcher />
        <span className="text-[10px] text-a-muted">{user?.username || 'admin'}</span>
        <button onClick={logout} className="text-[10px] text-a-muted hover:text-a-fg transition-colors cursor-pointer">登出</button>
      </header>

      {/* ── Body ── */}
      <div className="flex-1 flex overflow-hidden">
        <WorkbenchSidebar collapsed={sidebarCollapsed} onToggle={() => setSidebarCollapsed(!sidebarCollapsed)} />
        <main className="flex-1 overflow-y-auto">
          <Outlet />
        </main>
      </div>
    </div>
  );
}

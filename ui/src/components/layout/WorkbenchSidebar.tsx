// ─── WorkbenchSidebar ───
// Left sidebar with 8 workspace groups, expandable sub-navigation.

import { useLocation, useNavigate } from 'react-router-dom';
import { useState } from 'react';
import { cn } from '@/lib/utils';
import { WORKSPACES, WORKSPACE_NAV } from '@/lib/constants';
import type { WorkspaceId } from '@/lib/constants';

interface WorkbenchSidebarProps {
  collapsed: boolean;
  onToggle: () => void;
}

// Inline SVG icon paths for each workspace
const ICONS: Record<string, string> = {
  'command-center': 'M3 13h8V3H3v10zm0 8h8v-6H3v6zm10 0h8V11h-8v10zm0-18v6h8V3h-8z',
  exposure: 'M19 3H5c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h14c1.1 0 2-.9 2-2V5c0-1.1-.9-2-2-2zm-7 3c1.93 0 3.5 1.57 3.5 3.5S13.93 13 12 13s-3.5-1.57-3.5-3.5S10.07 6 12 6zm7 13H5v-.23c0-.62.28-1.2.76-1.58C7.47 15.82 9.64 15 12 15s4.53.82 6.24 2.19c.48.38.76.97.76 1.58V19z',
  fabric: 'M4 8h4V4H4v4zm6 12h4v-4h-4v4zm-6 0h4v-4H4v4zm0-6h4v-4H4v4zm6 0h4v-4h-4v4zm6-10v4h4V4h-4zm-6 4h4V4h-4v4zm6 6h4v-4h-4v4zm0 6h4v-4h-4v4z',
  runtime: 'M20 2H4c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h6v2H8v2h8v-2h-2v-2h6c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zm0 14H4V4h16v12z',
  release: 'M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-1 17.93c-3.95-.49-7-3.85-7-7.93 0-.62.08-1.21.21-1.79L9 15v1c0 1.1.9 2 2 2v1.93zm6.9-2.54c-.26-.81-1-1.39-1.9-1.39h-1v-3c0-.55-.45-1-1-1H8v-2h2c.55 0 1-.45 1-1V7h2c1.1 0 2-.9 2-2v-.41c2.93 1.19 5 4.06 5 7.41 0 2.08-.8 3.97-2.1 5.39z',
  observe: 'M12 4.5C7 4.5 2.73 7.61 1 12c1.73 4.39 6 7.5 11 7.5s9.27-3.11 11-7.5c-1.73-4.39-6-7.5-11-7.5zM12 17c-2.76 0-5-2.24-5-5s2.24-5 5-5 5 2.24 5 5-2.24 5-5 5zm0-8c-1.66 0-3 1.34-3 3s1.34 3 3 3 3-1.34 3-3-1.34-3-3-3z',
  access: 'M12 1L3 5v6c0 5.55 3.84 10.74 9 12 5.16-1.26 9-6.45 9-12V5l-9-4zm0 10.99h7c-.53 4.12-3.28 7.79-7 8.94V12H5V6.3l7-3.11v8.8z',
  settings: 'M19.14 12.94c.04-.3.06-.61.06-.94 0-.32-.02-.64-.07-.94l2.03-1.58c.18-.14.23-.41.12-.61l-1.92-3.32c-.12-.22-.37-.29-.59-.22l-2.39.96c-.5-.38-1.03-.7-1.62-.94L14.4 2.81c-.04-.24-.24-.41-.48-.41h-3.84c-.24 0-.43.17-.47.41L9.25 5.35c-.59.24-1.13.57-1.62.94l-2.39-.96c-.22-.08-.47 0-.59.22L2.74 8.87c-.12.21-.08.47.12.61l2.03 1.58c-.05.3-.07.62-.07.94s.02.64.07.94l-2.03 1.58c-.18.14-.23.41-.12.61l1.92 3.32c.12.22.37.29.59.22l2.39-.96c.5.38 1.03.7 1.62.94l.36 2.54c.05.24.24.41.48.41h3.84c.24 0 .44-.17.47-.41l.36-2.54c.59-.24 1.13-.56 1.62-.94l2.39.96c.22.08.47 0 .59-.22l1.92-3.32c.12-.22.07-.47-.12-.61l-2.01-1.58zM12 15.6c-1.98 0-3.6-1.62-3.6-3.6s1.62-3.6 3.6-3.6 3.6 1.62 3.6 3.6-1.62 3.6-3.6 3.6z',
};

export function WorkbenchSidebar({ collapsed, onToggle }: WorkbenchSidebarProps) {
  const location = useLocation();
  const navigate = useNavigate();

  // Determine active workspace from URL
  const activeWs = WORKSPACES.find(w =>
    w.path === '/' ? location.pathname === '/' : location.pathname.startsWith(w.path)
  ) || WORKSPACES[0];

  if (collapsed) {
    return (
      <aside className="w-10 shrink-0 bg-a-surface border-r border-a-border flex flex-col items-center py-2 gap-1">
        <button onClick={onToggle} className="w-6 h-6 flex items-center justify-center text-a-muted hover:text-a-fg cursor-pointer text-[10px]">▶</button>
        {WORKSPACES.map(ws => (
          <button key={ws.id} onClick={() => navigate(ws.path)}
            className={cn('w-7 h-7 flex items-center justify-center rounded cursor-pointer text-xs',
              ws.id === activeWs.id ? 'bg-a-accent/20 text-a-accent' : 'text-a-muted hover:text-a-fg hover:bg-a-border/20')}
            title={ws.label}>
            <svg className="w-3.5 h-3.5" viewBox="0 0 24 24" fill="currentColor"><path d={ICONS[ws.id]} /></svg>
          </button>
        ))}
      </aside>
    );
  }

  return (
    <aside className="w-52 shrink-0 bg-a-surface border-r border-a-border flex flex-col overflow-y-auto">
      {/* Header */}
      <div className="px-3 py-2.5 border-b border-a-border flex items-center justify-between">
        <span className="text-[11px] font-semibold text-a-fg tracking-wide">WORKBENCH</span>
        <button onClick={onToggle} className="w-5 h-5 flex items-center justify-center text-a-muted hover:text-a-fg cursor-pointer text-[10px]">◀</button>
      </div>

      {/* Workspace groups */}
      <nav className="flex-1 py-1">
        {WORKSPACES.map(ws => {
          const isActive = ws.id === activeWs.id;
          const subItems = WORKSPACE_NAV[ws.id] || [];
          // Command center goes directly to /
          const isCommandCenter = ws.id === 'command-center';

          return (
            <div key={ws.id} className="mb-0.5">
              {/* Workspace header */}
              <button
                onClick={() => navigate(ws.path)}
                className={cn(
                  'w-full flex items-center gap-2 px-3 py-1.5 text-xs transition-colors cursor-pointer',
                  isActive ? 'bg-a-accent/10 text-a-accent font-semibold' : 'text-a-fg2 hover:bg-a-border/20 hover:text-a-fg',
                )}
              >
                <svg className="w-3.5 h-3.5 shrink-0" viewBox="0 0 24 24" fill="currentColor"><path d={ICONS[ws.id]} /></svg>
                <span className="truncate">{ws.label}</span>
                {!isCommandCenter && subItems.length > 0 && (
                  <svg className={cn('w-3 h-3 ml-auto shrink-0 transition-transform', isActive && 'rotate-90')} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M9 18l6-6-6-6" /></svg>
                )}
              </button>

              {/* Sub-items (only for active workspace, except command center) */}
              {isActive && !isCommandCenter && subItems.length > 0 && (
                <div className="ml-7 mr-2 mt-0.5 space-y-0.5 border-l border-a-border/50 pl-2">
                  {subItems.map((item, idx) => {
                    // First item (index/overview): exact match only to avoid false highlight
                    // Other items: exact match OR path starts with item path + "/"
                    const isFirst = idx === 0;
                    const itemActive = isFirst
                      ? location.pathname === item.path
                      : location.pathname === item.path || location.pathname.startsWith(item.path + '/');
                    return (
                      <button
                        key={item.path}
                        onClick={() => navigate(item.path)}
                        className={cn(
                          'w-full text-left px-2 py-1 rounded-a-sm text-[11px] transition-colors cursor-pointer block',
                          itemActive ? 'text-a-accent font-medium' : 'text-a-muted hover:text-a-fg',
                        )}
                      >
                        {item.label}
                      </button>
                    );
                  })}
                </div>
              )}
            </div>
          );
        })}
      </nav>

      {/* Footer */}
      <div className="px-3 py-2 border-t border-a-border text-[10px] text-a-muted">
        Aegis v2
      </div>
    </aside>
  );
}

/**
 * NodeSwitcher — perspective switcher in the top bar.
 *
 * Shows a dropdown of all known nodes (from distnode membership).
 * Selecting one switches the active view to that node's perspective.
 * All subsequent API calls will include X-Aegis-View-As header,
 * causing the backend proxy to forward requests to the target node.
 */

import { useState, useRef, useEffect } from 'react';
import { useView } from '@/lib/view-context';
import { cn } from '@/lib/utils';

export function NodeSwitcher() {
  const { activeNodeId, setActiveNode, localNodeId, peers, isRemoteView } = useView();
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  // Close on click outside
  useEffect(() => {
    if (!open) return;
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, [open]);

  const currentPeer = peers.find(p => p.id === activeNodeId) || null;
  const localPeer = localNodeId ? { id: localNodeId, addr: '', alive: true } : null;
  // Show local node + all peers
  const allNodes = localPeer ? [localPeer, ...peers] : peers;
  const activeLabel = isRemoteView
    ? (currentPeer?.id || activeNodeId)
    : (localNodeId || 'local');

  const handleSelect = (id: string | null) => {
    setActiveNode(id);
    setOpen(false);
  };

  return (
    <div ref={ref} className="relative">
      <button
        onClick={() => setOpen(!open)}
        className={cn(
          'flex items-center gap-1.5 px-2 py-0.5 text-[11px] rounded transition-colors cursor-pointer',
          isRemoteView
            ? 'bg-[#e8b830]/15 text-[#e8b830] border border-[#e8b830]/30'
            : 'text-a-muted hover:text-a-fg hover:bg-a-border/20',
        )}
        title={isRemoteView ? `查看 ${activeNodeId} 的视角` : '本地视角'}
      >
        {/* Eye icon */}
        <svg className="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" />
          <circle cx="12" cy="12" r="3" />
        </svg>
        <span>{activeLabel}</span>
        {isRemoteView && <span className="text-[10px] opacity-60">◂ 视角</span>}
        <svg className="w-2.5 h-2.5 opacity-50" viewBox="0 0 24 24" fill="currentColor">
          <path d="M7 10l5 5 5-5z" />
        </svg>
      </button>

      {open && (
        <div className="absolute right-0 top-full mt-1 w-44 bg-a-surface border border-a-border rounded shadow-lg z-50 py-1 text-xs">
          <div className="px-3 py-1 text-[10px] text-a-muted uppercase tracking-wide">视角切换</div>

          {localPeer && (
            <button
              onClick={() => handleSelect(null)}
              className={cn(
                'w-full flex items-center gap-2 px-3 py-1.5 text-left transition-colors cursor-pointer',
                !isRemoteView ? 'bg-a-accent/10 text-a-accent' : 'text-a-fg2 hover:bg-a-border/20',
              )}
            >
              <span className={cn(
                'w-1.5 h-1.5 rounded-full shrink-0',
                localPeer.alive ? 'bg-[#4cd964]' : 'bg-a-muted',
              )} />
              <span className="font-medium">{localPeer.id}</span>
              <span className="text-a-muted ml-auto">本机</span>
            </button>
          )}

          {peers.length === 0 && (
            <div className="px-3 py-2 text-a-muted text-[10px]">未发现其他节点</div>
          )}

          {peers.map(p => (
            <button
              key={p.id}
              onClick={() => handleSelect(p.id)}
              className={cn(
                'w-full flex items-center gap-2 px-3 py-1.5 text-left transition-colors cursor-pointer',
                activeNodeId === p.id ? 'bg-[#e8b830]/10 text-[#e8b830]' : 'text-a-fg2 hover:bg-a-border/20',
              )}
            >
              <span className={cn(
                'w-1.5 h-1.5 rounded-full shrink-0',
                p.alive ? 'bg-[#4cd964]' : 'bg-a-muted',
              )} />
              <span>{p.id}</span>
              {p.alive && <span className="text-[10px] text-a-muted ml-auto">{p.addr}</span>}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

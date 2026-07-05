/**
 * View Context — React context for active node perspective.
 *
 * Wraps viewStore for component-level reactivity.
 * Add ViewProvider in AppShell, then useView() anywhere.
 */

import React, { createContext, useContext, useState, useEffect, useCallback, type ReactNode } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { viewStore } from './view-store';

export interface PeerInfo {
  id: string;
  addr: string;
  alive: boolean;
  since?: string;
}

interface ViewState {
  /** Current target node ID (null = local node). */
  activeNodeId: string | null;
  /** Set the target node. Pass null to reset to local. */
  setActiveNode: (id: string | null) => void;
  /** This node's own ID. */
  localNodeId: string | null;
  /** Known peers from distnode membership. */
  peers: PeerInfo[];
  /** Refresh peer list from distnode status API. */
  refresh: () => Promise<void>;
  /** True if viewing a remote node's perspective. */
  isRemoteView: boolean;
}

const ViewContext = createContext<ViewState | null>(null);

const fetchStatus = (() => {
  let cache: { data: any; ts: number } | null = null;
  return async (force = false) => {
    if (!force && cache && Date.now() - cache.ts < 10_000) return cache.data;
    try {
      const res = await fetch('/api/admin/v1/distnode/status', { credentials: 'same-origin' });
      if (!res.ok) return null;
      const data = await res.json();
      cache = { data, ts: Date.now() };
      return data;
    } catch {
      return null;
    }
  };
})();

export function ViewProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient();
  const [activeNodeId, setActiveNodeId] = useState<string | null>(viewStore.activeNodeId);
  const [localNodeId, setLocalNodeId] = useState<string | null>(viewStore.localNodeId);
  const [peers, setPeers] = useState<PeerInfo[]>([]);

  // Sync from store to React state
  useEffect(() => {
    const unsub = viewStore.subscribe((id) => {
      setActiveNodeId(id);
    });
    return unsub;
  }, []);

  const refresh = useCallback(async () => {
    const data = await fetchStatus(true);
    if (!data) return;
    viewStore.localNodeId = data.node_id;
    setLocalNodeId(data.node_id);
    setPeers(data.peers || []);
    // Reset active node if current target is no longer a peer
    if (activeNodeId && activeNodeId !== data.node_id) {
      const stillExists = (data.peers || []).some((p: any) => p.id === activeNodeId);
      if (!stillExists) {
        viewStore.activeNodeId = null;
        setActiveNodeId(null);
      }
    }
  }, [activeNodeId]);

  // Initial fetch
  useEffect(() => {
    refresh();
  }, [refresh]);

  const setActiveNode = useCallback((id: string | null) => {
    viewStore.activeNodeId = id;
    setActiveNodeId(id);
    // P0-3: invalidate all React Query caches — data may differ per node
    queryClient.invalidateQueries();
  }, []);

  return (
    <ViewContext.Provider value={{
      activeNodeId,
      setActiveNode,
      localNodeId,
      peers,
      refresh,
      isRemoteView: !!activeNodeId && activeNodeId !== localNodeId,
    }}>
      {children}
    </ViewContext.Provider>
  );
}

export function useView(): ViewState {
  const ctx = useContext(ViewContext);
  if (!ctx) {
    // Allow fallback for pages not wrapped in ViewProvider
    return {
      activeNodeId: null,
      setActiveNode: () => {},
      localNodeId: null,
      peers: [],
      refresh: async () => {},
      isRemoteView: false,
    };
  }
  return ctx;
}

export { ViewContext };

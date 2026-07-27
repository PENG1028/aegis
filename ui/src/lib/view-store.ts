/**
 * View Store — module-level store for active node perspective.
 *
 * Exists outside React so the API client (real-api-client.ts) can read
 * the active node ID synchronously without needing a React context.
 *
 * The React ViewContext wraps this for component-level reactivity.
 */

type Listener = (id: string | null) => void;

let _activeNodeId: string | null = null;
let _localNodeId: string | null = null;
const _listeners: Listener[] = [];

export const viewStore = {
  get activeNodeId(): string | null {
    return _activeNodeId;
  },

  set activeNodeId(id: string | null) {
    _activeNodeId = id || null;
    _listeners.forEach(fn => fn(_activeNodeId));
  },

  get localNodeId(): string | null {
    return _localNodeId;
  },

  set localNodeId(id: string | null) {
    _localNodeId = id;
  },

  /** Returns the value to put in X-Aegis-View-As header, or null for local. */
  get headerValue(): string | null {
    if (!_activeNodeId || !_localNodeId) return null;
    return _activeNodeId === _localNodeId ? null : _activeNodeId;
  },

  subscribe(fn: Listener): () => void {
    _listeners.push(fn);
    return () => {
      const idx = _listeners.indexOf(fn);
      if (idx >= 0) _listeners.splice(idx, 1);
    };
  },

  reset() {
    _activeNodeId = null;
    _localNodeId = null;
  },
};

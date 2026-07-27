/**
 * Aegis API Configuration
 *
 * Architecture:
 *   Dev:  Browser → localhost:3000/api/* → Vite proxy → 127.0.0.1:7380
 *   Prod: Browser → /api/* → Caddy proxy → 127.0.0.1:7380
 */
export const API_CONFIG = {
  /** Base URL for the Go API server. Empty = same-origin. */
  baseUrl: import.meta.env.VITE_API_BASE_URL ?? '',
  /** Include cookies for session auth. */
  credentials: 'include' as RequestCredentials,
  /** Request timeout in milliseconds */
  timeout: 15_000,
} as const;

export function apiUrl(path: string): string {
  const base = API_CONFIG.baseUrl ? API_CONFIG.baseUrl.replace(/\/+$/, '') : '';
  const p = path.startsWith('/') ? path : '/' + path;
  return `${base}${p}`;
}

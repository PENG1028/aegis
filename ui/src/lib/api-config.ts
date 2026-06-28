/**
 * Aegis API Configuration
 *
 * Controls backend connection and mock/real toggle.
 * In production, API_BASE_URL should be empty (same-origin via Caddy proxy).
 *
 * Architecture:
 *   Dev:  Browser → localhost:3000/api/* → Vite proxy → 127.0.0.1:7380 (same-origin, no CORS)
 *   Prod: Browser → cdn.example.com/api/* → Caddy proxy → 127.0.0.1:7380 (same-origin, no CORS)
 */

export const API_CONFIG = {
  /**
   * Base URL for the Go API server.
   * Empty string = same-origin (Vite proxy in dev, Caddy in prod).
   * Override with VITE_API_BASE_URL env var for cross-origin dev.
   */
  baseUrl: import.meta.env.VITE_API_BASE_URL ?? '',

  /** Whether to use mock data instead of real API calls */
  useMock: import.meta.env.VITE_USE_MOCK === 'true',

  /** Include cookies for session auth. Same-origin only works when baseUrl is empty. */
  credentials: 'include' as RequestCredentials,

  /** Request timeout in milliseconds */
  timeout: 15_000,
} as const;

export function apiUrl(path: string): string {
  const base = API_CONFIG.baseUrl ? API_CONFIG.baseUrl.replace(/\/+$/, '') : '';
  const p = path.startsWith('/') ? path : '/' + path;
  return `${base}${p}`;
}

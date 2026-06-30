/**
 * Login Page — v2 style
 *
 * Full-screen centered login with username/password,
 * error display, loading state, and Enter-key support.
 */

import React, { useState, type FormEvent } from 'react';
import { useAuth } from '@/lib/auth-context';

export default function LoginPage() {
  const { login, loading, error } = useAuth();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    await login(username, password);
  }

  return (
    <div className="flex items-center justify-center min-h-screen bg-a-bg">
      <div className="bg-a-surface border border-a-border rounded-a-lg p-8 w-full max-w-sm shadow-2xl shadow-a-accent/5">
        {/* Logo */}
        <div className="text-center mb-6">
          <svg className="w-10 h-10 mx-auto mb-3 text-a-accent" viewBox="0 0 24 24" fill="none"
            stroke="currentColor" strokeWidth="2">
            <rect x="3" y="3" width="18" height="18" rx="3" />
            <path d="M8 12h8M12 8v8" />
          </svg>
          <h1 className="font-mono text-2xl font-bold text-a-accent tracking-tight">Aegis</h1>
          <p className="text-xs text-a-muted mt-1">网关管理控制台</p>
        </div>

        <form onSubmit={handleSubmit}>
          <div className="mb-3">
            <label className="block text-xs font-medium text-a-muted mb-1.5" htmlFor="login-username">
              用户名
            </label>
            <input
              id="login-username"
              className="w-full font-mono text-sm px-3 py-2 rounded-a-sm border border-a-border bg-a-bg
                         text-a-fg outline-none transition-colors focus:border-a-accent focus:shadow-[0_0_0_3px_rgba(168,101,255,0.15)]"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              autoFocus
              disabled={loading}
              autoComplete="username"
            />
          </div>

          <div className="mb-4">
            <label className="block text-xs font-medium text-a-muted mb-1.5" htmlFor="login-password">
              密码
            </label>
            <input
              id="login-password"
              type="password"
              className="w-full font-mono text-sm px-3 py-2 rounded-a-sm border border-a-border bg-a-bg
                         text-a-fg outline-none transition-colors focus:border-a-accent focus:shadow-[0_0_0_3px_rgba(168,101,255,0.15)]"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              disabled={loading}
              autoComplete="current-password"
            />
          </div>

          {error && (
            <div className="px-3 py-2 rounded-a-sm text-xs border bg-[#ff5c72]/10 text-[#ff5c72] border-[#ff5c72]/20 mb-4" role="alert">
              {error}
            </div>
          )}

          <button
            type="submit"
            disabled={loading}
            className="w-full py-2.5 bg-a-accent text-white rounded-a-md font-semibold text-sm
                       transition-all duration-150 hover:opacity-90 disabled:opacity-45
                       cursor-pointer disabled:cursor-not-allowed
                       shadow-lg shadow-a-accent/20"
          >
            {loading ? (
              <span className="flex items-center justify-center gap-2">
                <span className="w-3.5 h-3.5 border-2 border-white/30 border-t-white rounded-full animate-spin" />
                登录中...
              </span>
            ) : (
              '登 录'
            )}
          </button>
        </form>

        <p className="text-[10px] text-a-muted text-center mt-5">
          默认凭据：admin / admin
        </p>
      </div>
    </div>
  );
}

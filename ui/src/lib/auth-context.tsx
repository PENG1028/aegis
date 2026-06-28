/**
 * Auth Context — manages admin session state.
 *
 * Uses cookie-based auth (aegis_admin_session set by the backend on login).
 * The context provides login/logout/me and loading/error states.
 */

import React, { createContext, useContext, useState, useEffect, useCallback, type ReactNode } from 'react';
import { auth as authApi } from './api-bridge';

export interface UserInfo {
  id: string;
  username: string;
  displayName?: string;
}

interface AuthState {
  user: UserInfo | null;
  loading: boolean;
  error: string | null;
  login: (username: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  checkSession: () => Promise<void>;
}

const AuthContext = createContext<AuthState | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<UserInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const checkSession = useCallback(async () => {
    try {
      setLoading(true);
      const res = await authApi.me();
      if (res?.user) {
        setUser({
          id: res.user.id,
          username: res.user.username,
          displayName: res.user.username,
        });
        setError(null);
      } else {
        setUser(null);
      }
    } catch {
      setUser(null);
    } finally {
      setLoading(false);
    }
  }, []);

  const login = useCallback(async (username: string, password: string) => {
    setError(null);
    setLoading(true);
    try {
      const res = await authApi.login(username, password);
      setUser({
        id: res.user.id,
        username: res.user.username,
        displayName: res.user.username,
      });
    } catch (err: any) {
      const msg = err?.message || '登录失败';
      setError(msg);
      throw err;
    } finally {
      setLoading(false);
    }
  }, []);

  const logout = useCallback(async () => {
    try {
      await authApi.logout();
    } catch {
      // Ignore logout errors
    }
    setUser(null);
  }, []);

  // Check session on mount
  useEffect(() => {
    checkSession();
  }, [checkSession]);

  return (
    <AuthContext.Provider value={{ user, loading, error, login, logout, checkSession }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext);
  if (!ctx) {
    throw new Error('useAuth must be used within AuthProvider');
  }
  return ctx;
}

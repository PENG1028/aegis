/**
 * AuthGuard — wraps protected routes, redirects to LoginPage if not authenticated.
 */

import React, { type ReactNode } from 'react';
import { useAuth } from '@/lib/auth-context';
import LoginPage from './LoginPage';

export default function AuthGuard({ children }: { children: ReactNode }) {
  const { user, loading } = useAuth();

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-screen bg-a-bg">
        <div className="flex flex-col items-center gap-3">
          <div className="w-6 h-6 border-2 border-a-accent/30 border-t-a-accent rounded-full animate-spin" />
          <span className="text-xs text-a-muted font-mono">验证会话...</span>
        </div>
      </div>
    );
  }

  if (!user) {
    return <LoginPage />;
  }

  return <>{children}</>;
}

/**
 * Toast notification system — v2 style.
 *
 * Usage:
 *   <ToastProvider>
 *     <App />
 *   </ToastProvider>
 *
 *   const toast = useToast();
 *   toast('Saved!', 'ok');
 *   toast('Failed!', 'error');
 */

import React, { createContext, useContext, useState, useCallback, type ReactNode } from 'react';

interface ToastItem {
  id: number;
  message: string;
  type: 'ok' | 'error';
}

interface ToastContextValue {
  toast: (message: string, type?: 'ok' | 'error') => void;
}

const ToastContext = createContext<ToastContextValue | null>(null);

export function ToastProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<ToastItem[]>([]);

  const toast = useCallback((message: string, type: 'ok' | 'error' = 'ok') => {
    const id = Date.now();
    setItems((prev) => [...prev, { id, message, type }]);
    setTimeout(() => {
      setItems((prev) => prev.filter((t) => t.id !== id));
    }, 3500);
  }, []);

  return (
    <ToastContext.Provider value={{ toast }}>
      {children}
      <div className="fixed top-14 right-5 z-[200] flex flex-col gap-2 pointer-events-none" role="status" aria-live="polite">
        {items.map((t) => (
          <div
            key={t.id}
            role="alert"
            className={`
              pointer-events-auto animate-[toastIn_0.25s_cubic-bezier(0.16,1,0.3,1)]
              px-3 py-2.5 rounded-a-sm text-xs font-medium max-w-[360px] shadow-lg
              border
              ${t.type === 'error'
                ? 'bg-red-500/20 text-red-400 border-red-500/30'
                : 'bg-green-500/20 text-green-400 border-green-500/30'
              }
            `}
          >
            {t.message}
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}

export function useToast(): (message: string, type?: 'ok' | 'error') => void {
  const ctx = useContext(ToastContext);
  if (!ctx) {
    throw new Error('useToast must be used within ToastProvider');
  }
  return ctx.toast;
}

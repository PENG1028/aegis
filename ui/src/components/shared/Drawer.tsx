// ─── Drawer Component ───
// Right-slide drawer for medium-risk operations and Inspector views.

import { useEffect, useCallback } from 'react';
import { cn } from '@/lib/utils';

interface DrawerProps {
  open: boolean;
  onClose: () => void;
  title?: string;
  subtitle?: string;
  children: React.ReactNode;
  footer?: React.ReactNode;
  width?: 'sm' | 'md' | 'lg' | 'xl';
}

const WIDTH_MAP = {
  sm: 'max-w-sm',
  md: 'max-w-md',
  lg: 'max-w-lg',
  xl: 'max-w-xl',
};

export function Drawer({ open, onClose, title, subtitle, children, footer, width = 'md' }: DrawerProps) {
  const handleKeyDown = useCallback((e: KeyboardEvent) => {
    if (e.key === 'Escape') onClose();
  }, [onClose]);

  useEffect(() => {
    if (open) {
      document.addEventListener('keydown', handleKeyDown);
      document.body.style.overflow = 'hidden';
    }
    return () => {
      document.removeEventListener('keydown', handleKeyDown);
      document.body.style.overflow = '';
    };
  }, [open, handleKeyDown]);

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-50 flex justify-end">
      {/* Backdrop */}
      <div
        className="absolute inset-0 bg-black/50 backdrop-blur-sm anim-fade"
        onClick={onClose}
      />
      {/* Drawer panel */}
      <div
        className={cn(
          'relative w-full h-full bg-a-surface border-l border-a-border shadow-2xl anim-slide',
          'flex flex-col overflow-hidden',
          WIDTH_MAP[width],
        )}
      >
        {/* Header */}
        {title && (
          <div className="flex items-center justify-between px-5 py-4 border-b border-a-border shrink-0">
            <div>
              <h2 className="text-sm font-semibold text-a-fg">{title}</h2>
              {subtitle && <p className="text-xs text-a-muted mt-0.5">{subtitle}</p>}
            </div>
            <button
              onClick={onClose}
              className="w-6 h-6 flex items-center justify-center rounded text-a-muted hover:text-a-fg hover:bg-a-border/30 transition-colors cursor-pointer"
            >
              ✕
            </button>
          </div>
        )}
        {/* Body */}
        <div className="flex-1 overflow-y-auto px-5 py-4">
          {children}
        </div>
        {/* Footer */}
        {footer && (
          <div className="px-5 py-3 border-t border-a-border shrink-0 flex items-center gap-3 justify-end">
            {footer}
          </div>
        )}
      </div>
    </div>
  );
}

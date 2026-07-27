/**
 * Modal dialog — v2 style.
 *
 * Usage:
 *   <Modal title="Create Token" onClose={() => setShow(false)}>
 *     <p>Content here</p>
 *   </Modal>
 *
 * With footer:
 *   <Modal title="Confirm" onClose={...}
 *     footer={<><Btn onClick={...}>Cancel</Btn><Btn primary>OK</Btn></>}>
 *     ...
 *   </Modal>
 */

import React, { type ReactNode, useEffect } from 'react';
import { cn } from '@/lib/utils';

interface ModalProps {
  title?: string;
  children: ReactNode;
  footer?: ReactNode;
  wide?: boolean;
  onClose?: () => void;
  className?: string;
}

export function Modal({ title, children, footer, wide, onClose, className }: ModalProps) {
  // Close on Escape key
  useEffect(() => {
    if (!onClose) return;
    function handler(e: KeyboardEvent) {
      if (e.key === 'Escape') onClose?.();
    }
    document.addEventListener('keydown', handler);
    return () => document.removeEventListener('keydown', handler);
  }, [onClose]);

  return (
    <div
      className="fixed inset-0 z-[100] bg-black/60 flex items-center justify-center"
      onClick={(e) => {
        if (e.target === e.currentTarget && onClose) onClose();
      }}
    >
      <div
        className={cn(
          'bg-a-surface border border-a-border rounded-a-lg shadow-2xl',
          'w-[90%] max-h-[85vh] overflow-y-auto',
          wide ? 'max-w-[700px]' : 'max-w-[540px]',
          className,
        )}
      >
        {/* Header */}
        {title && (
          <div className="flex items-center justify-between px-5 py-4 border-b border-a-border">
            <h3 className="text-base font-semibold text-a-fg">{title}</h3>
            {onClose && (
              <button
                className="text-a-muted hover:text-a-fg text-lg leading-none px-1.5 py-0.5 rounded hover:bg-a-border-soft transition-colors cursor-pointer bg-transparent border-none"
                onClick={onClose}
                aria-label="关闭"
              >
                ×
              </button>
            )}
          </div>
        )}

        {/* Body */}
        <div className="p-5">{children}</div>

        {/* Footer */}
        {footer && (
          <div className="flex gap-2 justify-end px-5 py-3 border-t border-a-border">
            {footer}
          </div>
        )}
      </div>
    </div>
  );
}

/**
 * Button component — matches v2 HTML style.
 *
 * Variants: primary (purple), danger (red), default (outline)
 * Sizes: default (sm: false), small (sm: true)
 */

import React, { type ButtonHTMLAttributes, type ReactNode } from 'react';
import { cn } from '@/lib/utils';

interface BtnProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  children: ReactNode;
  primary?: boolean;
  danger?: boolean;
  sm?: boolean;
  className?: string;
}

export function Btn({ children, primary, danger, sm, className, disabled, ...props }: BtnProps) {
  return (
    <button
      className={cn(
        'inline-flex items-center gap-1.5 font-body font-medium rounded-a-md border transition-all duration-150 ease-out',
        'active:scale-[0.97] whitespace-nowrap',
        'focus-visible:outline-none focus-visible:shadow-[0_0_0_3px_rgba(168,101,255,0.35)]',
        sm ? 'text-[11px] px-2.5 py-1' : 'text-xs px-3.5 py-1.5',
        disabled && 'opacity-45 cursor-not-allowed pointer-events-none',
        primary && 'bg-a-accent border-a-accent text-white hover:opacity-90',
        danger && 'bg-red-500/20 border-red-500 text-red-400 hover:bg-red-500/30',
        !primary && !danger && 'bg-a-surface border-a-border text-a-fg hover:bg-a-border-soft',
        className,
      )}
      disabled={disabled}
      {...props}
    >
      {children}
    </button>
  );
}

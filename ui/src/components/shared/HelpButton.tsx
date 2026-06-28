/**
 * HelpButton — tiny "?" circle that opens a help modal for the current page.
 */

import React, { useState, type ReactNode } from 'react';
import { Modal } from './Modal';

interface HelpButtonProps {
  title: string;
  children: ReactNode;
}

export function HelpButton({ title, children }: HelpButtonProps) {
  const [open, setOpen] = useState(false);

  return (
    <>
      <button
        className="inline-flex items-center justify-center w-5 h-5 rounded-full
                   border border-a-border text-a-muted hover:text-a-accent hover:border-a-accent
                   text-[11px] font-bold leading-none transition-colors cursor-pointer
                   bg-transparent shrink-0"
        onClick={() => setOpen(true)}
        title="帮助"
        aria-label="帮助"
      >
        ?
      </button>

      {open && (
        <Modal title={title} onClose={() => setOpen(false)} wide>
          <div className="text-sm leading-relaxed text-a-fg space-y-3">
            {children}
          </div>
        </Modal>
      )}
    </>
  );
}

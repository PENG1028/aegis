/**
 * TabBar component — v2 style.
 *
 * Usage:
 *   <TabBar
 *     tabs={[
 *       { key: 'table', label: 'Table' },
 *       { key: 'preview', label: 'Preview' },
 *     ]}
 *     active={activeTab}
 *     onChange={setActiveTab}
 *   />
 */

import React from 'react';
import { cn } from '@/lib/utils';

export interface Tab {
  key: string;
  label: string;
}

interface TabBarProps {
  tabs: Tab[];
  active: string;
  onChange: (key: string) => void;
  className?: string;
}

export function TabBar({ tabs, active, onChange, className }: TabBarProps) {
  return (
    <div className={cn('flex gap-0 mb-4 border-b border-a-border overflow-x-auto', className)}>
      {tabs.map((t) => (
        <button
          key={t.key}
          className={cn(
            'px-4 py-2 text-xs font-medium border-b-2 transition-colors whitespace-nowrap bg-transparent cursor-pointer',
            active === t.key
              ? 'border-a-accent text-a-accent'
              : 'border-transparent text-a-muted hover:text-a-fg',
          )}
          onClick={() => onChange(t.key)}
        >
          {t.label}
        </button>
      ))}
    </div>
  );
}

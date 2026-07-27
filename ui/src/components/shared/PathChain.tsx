/**
 * PathChain — visual breadcrumb for route/service topology paths.
 *
 * Usage:
 *   <PathChain steps={[
 *     { label: 'api.example.com', tooltip: 'Domain', color: 'accent' },
 *     { label: 'svc-api', tooltip: 'Service' },
 *     { label: 'node-a', tooltip: 'Node', color: 'danger' },
 *   ]} />
 */

import React, { Fragment } from 'react';
import { cn } from '@/lib/utils';

export interface PathStep {
  label: string;
  tooltip?: string;
  color?: 'accent' | 'danger' | 'warn' | 'default';
}

interface PathChainProps {
  steps: PathStep[];
  className?: string;
}

const STEP_COLORS = {
  accent: 'bg-a-accent/20 text-a-accent',
  danger: 'bg-[#ff5c72]/20 text-[#ff5c72]',
  warn: 'bg-[#e8b830]/20 text-[#e8b830]',
  default: 'bg-a-border/40 text-a-fg',
};

export function PathChain({ steps, className }: PathChainProps) {
  return (
    <div className={cn('flex items-center flex-wrap gap-1 font-mono text-xs bg-a-bg border border-a-border rounded-a-sm p-3', className)}>
      {steps.map((s, i) => (
        <Fragment key={i}>
          {i > 0 && <span className="text-a-muted mx-0.5 select-none">→</span>}
          <span
            className={cn('px-1.5 py-0.5 rounded whitespace-nowrap', STEP_COLORS[s.color || 'default'])}
            title={s.tooltip}
          >
            {s.label}
          </span>
        </Fragment>
      ))}
    </div>
  );
}

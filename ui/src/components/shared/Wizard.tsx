// ─── Wizard Component ───
// Multi-step wizard for high-risk operations.
// Steps: Review → Impact → DryRun → Confirm → Execute → Verify

import { type ReactNode } from 'react';
import { cn } from '@/lib/utils';
import type { RiskStep } from '@/types/risk';

interface WizardStep {
  key: RiskStep;
  title: string;
  description: string;
}

interface WizardProps {
  open: boolean;
  steps: WizardStep[];
  currentStep: RiskStep;
  onClose: () => void;
  children: ReactNode;
  footer?: ReactNode;
}

const STEP_ORDER: RiskStep[] = ['review', 'impact', 'dryrun', 'confirm', 'execute', 'verify'];

export function Wizard({ open, steps, currentStep, onClose, children, footer }: WizardProps) {
  if (!open) return null;

  const currentIndex = STEP_ORDER.indexOf(currentStep);

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      {/* Backdrop */}
      <div
        className="absolute inset-0 bg-black/60 backdrop-blur-sm"
        onClick={onClose}
      />
      {/* Wizard panel */}
      <div className="relative w-full max-w-2xl max-h-[85vh] bg-a-surface border border-a-border rounded-a-lg shadow-2xl flex flex-col overflow-hidden anim-fade mx-4">
        {/* Header */}
        <div className="px-6 py-4 border-b border-a-border shrink-0">
          <div className="flex items-center justify-between mb-3">
            <h2 className="text-base font-semibold text-a-fg">高风险操作向导</h2>
            <button
              onClick={onClose}
              className="w-6 h-6 flex items-center justify-center rounded text-a-muted hover:text-a-fg hover:bg-a-border/30 transition-colors cursor-pointer"
            >
              ✕
            </button>
          </div>
          {/* Step indicators */}
          <div className="flex items-center gap-1">
            {steps.map((step, i) => {
              const stepIdx = STEP_ORDER.indexOf(step.key);
              const isActive = stepIdx === currentIndex;
              const isDone = stepIdx < currentIndex;
              const isPending = stepIdx > currentIndex;
              return (
                <div key={step.key} className="flex items-center gap-1 flex-1">
                  {/* Step circle */}
                  <div
                    className={cn(
                      'flex items-center gap-1.5 px-2 py-1 rounded text-[11px] font-medium transition-colors',
                      isActive && 'bg-a-accent/20 text-a-accent ring-1 ring-a-accent/50',
                      isDone && 'bg-[#4cd964]/10 text-[#4cd964]',
                      isPending && 'bg-a-border/30 text-a-muted',
                    )}
                    title={step.description}
                  >
                    <span className={cn(
                      'w-4 h-4 rounded-full flex items-center justify-center text-[10px] font-bold',
                      isActive && 'bg-a-accent text-white',
                      isDone && 'bg-[#4cd964] text-white',
                      isPending && 'bg-a-border text-a-muted',
                    )}>
                      {isDone ? '✓' : i + 1}
                    </span>
                    <span className="hidden sm:inline">{step.title}</span>
                  </div>
                  {/* Connector line */}
                  {i < steps.length - 1 && (
                    <div className={cn(
                      'flex-1 h-px',
                      isDone ? 'bg-[#4cd964]/40' : 'bg-a-border',
                    )} />
                  )}
                </div>
              );
            })}
          </div>
        </div>
        {/* Body */}
        <div className="flex-1 overflow-y-auto px-6 py-4">
          {children}
        </div>
        {/* Footer */}
        {footer && (
          <div className="px-6 py-3 border-t border-a-border shrink-0 flex items-center gap-3 justify-end">
            {footer}
          </div>
        )}
      </div>
    </div>
  );
}

export type { WizardStep };

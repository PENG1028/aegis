// ─── HealthDot Component ───
// Colored indicator dot for health/runtime status.

import { cn } from '@/lib/utils';

type HealthDotStatus = 'healthy' | 'degraded' | 'failed' | 'unreachable' | 'disabled' | 'unknown' | 'active' | 'inactive';

const DOT_COLORS: Record<string, string> = {
  healthy: 'bg-[#4cd964] shadow-[0_0_6px_rgba(76,217,100,0.5)]',
  degraded: 'bg-[#e8b830] shadow-[0_0_6px_rgba(232,184,48,0.5)]',
  failed: 'bg-[#ff5c72] shadow-[0_0_6px_rgba(255,92,114,0.5)]',
  unreachable: 'bg-[#ff5c72] shadow-[0_0_6px_rgba(255,92,114,0.3)]',
  disabled: 'bg-a-border/80',
  unknown: 'bg-a-muted/60',
  active: 'bg-[#4cd964] shadow-[0_0_6px_rgba(76,217,100,0.5)]',
  inactive: 'bg-a-border/80',
};

interface HealthDotProps {
  status: HealthDotStatus;
  size?: 'sm' | 'md';
  pulse?: boolean;
  className?: string;
}

export function HealthDot({ status, size = 'sm', pulse, className }: HealthDotProps) {
  const sizeClass = size === 'sm' ? 'w-2 h-2' : 'w-2.5 h-2.5';
  return (
    <span className={cn('relative flex items-center justify-center', className)}>
      <span
        className={cn(
          'inline-block rounded-full',
          sizeClass,
          DOT_COLORS[status] || DOT_COLORS.unknown,
          pulse && status === 'healthy' && 'animate-pulse-dot',
        )}
      />
    </span>
  );
}

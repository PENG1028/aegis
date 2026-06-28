import { STATUS_LABEL, STATUS_CLASS } from '@/lib/status-labels';
import { cn } from '@/lib/utils';

interface StatusBadgeProps {
  status: string;
  className?: string;
}

export function StatusBadge({ status, className }: StatusBadgeProps) {
  const label = STATUS_LABEL[status] || status;
  const cls = STATUS_CLASS[status] || 'bg-a-border/60 text-a-muted';
  return (
    <span className={cn('inline-flex items-center gap-1 font-mono text-[11px] px-2 py-0.5 rounded font-medium whitespace-nowrap', cls, className)}>
      {label}
    </span>
  );
}

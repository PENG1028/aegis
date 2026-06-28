import { cn } from '@/lib/utils';

interface DeferredBadgeProps {
  label?: string;
  className?: string;
}

export function DeferredBadge({ label = '延期', className }: DeferredBadgeProps) {
  return (
    <span className={cn(
      'inline-flex items-center gap-1 font-mono text-[10px] px-2 py-0.5 rounded font-medium whitespace-nowrap',
      'bg-a-border/60 text-a-muted',
      className
    )}>
      {label}
    </span>
  );
}

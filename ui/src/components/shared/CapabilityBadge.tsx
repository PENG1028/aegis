import { cn } from '@/lib/utils';

interface CapabilityBadgeProps {
  name: string;
  enabled: boolean;
  className?: string;
}

export function CapabilityBadge({ name, enabled, className }: CapabilityBadgeProps) {
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 font-mono text-[10px] px-2 py-0.5 rounded font-medium whitespace-nowrap',
        enabled
          ? 'bg-[#4cd964]/20 text-[#4cd964]'
          : 'bg-a-border/60 text-a-muted',
        className
      )}
    >
      <span className={`w-1.5 h-1.5 rounded-full ${enabled ? 'bg-[#4cd964]' : 'bg-a-muted'}`} />
      {name}
    </span>
  );
}

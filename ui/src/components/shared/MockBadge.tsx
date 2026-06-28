import { cn } from '@/lib/utils';

interface MockBadgeProps {
  className?: string;
}

export function MockBadge({ className }: MockBadgeProps) {
  return (
    <span className={cn(
      'inline-flex items-center gap-1 font-mono text-[10px] px-2 py-0.5 rounded font-medium whitespace-nowrap',
      'bg-[#e8b830]/15 text-[#e8b830] border border-[#e8b830]/20',
      className
    )}>
      Mock
    </span>
  );
}

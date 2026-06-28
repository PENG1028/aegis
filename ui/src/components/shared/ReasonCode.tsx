import { cn } from '@/lib/utils';

interface ReasonCodeProps {
  code: string;
  message?: string;
  variant?: 'error' | 'warning' | 'info';
  className?: string;
}

export function ReasonCode({ code, message, variant = 'error', className }: ReasonCodeProps) {
  const colors = {
    error: 'bg-[#ff5c72]/10 text-[#ff5c72] border-[#ff5c72]/20',
    warning: 'bg-[#e8b830]/10 text-[#e8b830] border-[#e8b830]/20',
    info: 'bg-a-accent/10 text-a-accent border-a-accent/20',
  };
  return (
    <div className={cn('px-3 py-2 rounded-a-sm text-xs border', colors[variant], className)}>
      <span className="font-mono font-semibold">{code}</span>
      {message && <span className="ml-2 text-a-fg2">{message}</span>}
    </div>
  );
}

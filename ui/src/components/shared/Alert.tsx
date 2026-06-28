import { cn } from '@/lib/utils';

interface AlertProps {
  type?: 'info' | 'warn' | 'err' | 'success';
  children: React.ReactNode;
  className?: string;
}

export function Alert({ type = 'info', children, className }: AlertProps) {
  const m = {
    info: 'bg-a-accent/10 text-a-accent border-a-accent/20',
    warn: 'bg-[#e8b830]/10 text-[#e8b830] border-[#e8b830]/20',
    err: 'bg-[#ff5c72]/10 text-[#ff5c72] border-[#ff5c72]/20',
    success: 'bg-[#4cd964]/10 text-[#4cd964] border-[#4cd964]/20',
  };
  return (
    <div className={cn('px-4 py-3 rounded-a-md text-xs border flex items-center gap-2', m[type], className)}>
      {children}
    </div>
  );
}

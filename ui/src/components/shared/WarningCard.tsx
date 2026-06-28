import { cn } from '@/lib/utils';

interface WarningCardProps {
  title: string;
  children: React.ReactNode;
  type?: 'warn' | 'info' | 'err';
  className?: string;
}

export function WarningCard({ title, children, type = 'warn', className }: WarningCardProps) {
  const colors = {
    warn: 'bg-[#e8b830]/10 border-[#e8b830]/20',
    info: 'bg-a-accent/10 border-a-accent/20',
    err: 'bg-[#ff5c72]/10 border-[#ff5c72]/20',
  };
  const icons = {
    warn: '⚠',
    info: 'ℹ',
    err: '✗',
  };
  return (
    <div className={cn('px-4 py-3 rounded-a-md text-xs border', colors[type], className)}>
      <div className="flex items-start gap-2.5">
        <span className="shrink-0 mt-0.5">{icons[type]}</span>
        <div>
          <p className="font-semibold mb-1">{title}</p>
          <div className="text-a-fg2 space-y-1">{children}</div>
        </div>
      </div>
    </div>
  );
}

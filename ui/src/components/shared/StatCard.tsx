import { cn } from '@/lib/utils';

interface StatCardProps {
  label: string;
  value: string | number;
  sub?: string;
  accent?: boolean;
  success?: boolean;
  danger?: boolean;
  warn?: boolean;
  className?: string;
}

export function StatCard({ label, value, sub, accent, success, danger, warn, className }: StatCardProps) {
  let color = '';
  if (accent) color = 'text-a-accent';
  else if (success) color = 'text-a-success';
  else if (danger) color = 'text-a-danger';
  else if (warn) color = 'text-a-warn';
  else color = 'text-a-fg';

  return (
    <div className={cn('bg-a-surface border border-a-border rounded-a-md p-4 anim-fade', className)}>
      <div className="text-[11px] text-a-muted uppercase tracking-[0.06em] mb-1 font-medium">{label}</div>
      <div className={`font-mono text-[28px] font-bold -tracking-[0.02em] tabular-nums leading-none ${color}`}>
        {value}
      </div>
      {sub && <div className="font-mono text-[11px] text-a-muted mt-1.5">{sub}</div>}
    </div>
  );
}

import { cn } from '@/lib/utils';

interface MetaRowProps {
  label: string;
  value: React.ReactNode;
  mono?: boolean;
  color?: string;
  className?: string;
}

export function MetaRow({ label, value, mono, color, className }: MetaRowProps) {
  return (
    <div className={cn('flex justify-between py-1.5 border-b border-a-border-soft last:border-b-0 text-xs', className)}>
      <span className="text-a-muted font-medium shrink-0 mr-4">{label}</span>
      <span className={cn('text-right break-all max-w-[60%]', mono ? 'font-mono' : '', color || 'text-a-fg')}>
        {value ?? '—'}
      </span>
    </div>
  );
}

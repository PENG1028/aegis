import { cn } from '@/lib/utils';

interface CardProps {
  title?: string;
  subtitle?: string;
  actions?: React.ReactNode;
  children: React.ReactNode;
  className?: string;
}

export function Card({ title, subtitle, actions, children, className }: CardProps) {
  return (
    <div className={cn('bg-a-surface border border-a-border rounded-a-lg overflow-hidden anim-fade', className)}>
      {(title || actions) && (
        <div className="flex items-center justify-between px-5 py-3.5 border-b border-a-border">
          <div className="min-w-0">
            <h3 className="text-sm font-semibold text-a-fg truncate">{title}</h3>
            {subtitle && <p className="text-[11px] text-a-muted mt-0.5">{subtitle}</p>}
          </div>
          {actions && <div className="flex gap-2 shrink-0 ml-4">{actions}</div>}
        </div>
      )}
      <div className={cn('p-5', !title && !actions && '')}>{children}</div>
    </div>
  );
}

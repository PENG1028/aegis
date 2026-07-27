import { cn } from '@/lib/utils';
import { HelpButton } from './HelpButton';
import { HELP } from '@/lib/help-content';

interface PageHeaderProps {
  title: string;
  subtitle?: string;
  actions?: React.ReactNode;
  className?: string;
  /** Key into HELP map. Shows a "?" button when set. */
  helpKey?: string;
}

export function PageHeader({ title, subtitle, actions, className, helpKey }: PageHeaderProps) {
  const help = helpKey ? HELP[helpKey] : null;

  return (
    <div className={cn('flex items-center justify-between mb-5', className)}>
      <div className="flex items-center gap-2">
        <h2 className="text-lg font-bold text-a-fg">{title}</h2>
        {help && <HelpButton title={help.title}>{help.content}</HelpButton>}
        {subtitle && <p className="text-xs text-a-muted mt-0.5 hidden sm:block">— {subtitle}</p>}
      </div>
      {actions && <div className="flex gap-2 shrink-0">{actions}</div>}
    </div>
  );
}

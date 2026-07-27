// ─── Timestamp Component ───
// Smart timestamp: relative (e.g. "3 分钟前") + absolute on hover.

import { cn } from '@/lib/utils';

interface TimestampProps {
  iso: string | null;
  className?: string;
  emptyText?: string;
}

function formatRelative(iso: string): string {
  const now = Date.now();
  const then = new Date(iso).getTime();
  const diff = now - then;

  if (diff < 0) return '刚刚';
  const seconds = Math.floor(diff / 1000);
  if (seconds < 60) return `${seconds} 秒前`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes} 分钟前`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours} 小时前`;
  const days = Math.floor(hours / 24);
  if (days < 30) return `${days} 天前`;
  return new Date(iso).toLocaleDateString('zh-CN');
}

function formatAbsolute(iso: string): string {
  return new Date(iso).toLocaleString('zh-CN');
}

export function Timestamp({ iso, className, emptyText = '—' }: TimestampProps) {
  if (!iso) {
    return <span className={cn('text-a-muted text-xs', className)}>{emptyText}</span>;
  }

  return (
    <span
      className={cn('text-xs text-a-muted tabular-nums cursor-default', className)}
      title={formatAbsolute(iso)}
    >
      {formatRelative(iso)}
    </span>
  );
}

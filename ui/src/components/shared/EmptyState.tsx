interface EmptyStateProps {
  title?: string;
  description?: string;
  action?: React.ReactNode;
}

export function EmptyState({ title = '暂无数据', description, action }: EmptyStateProps) {
  return (
    <div className="text-center py-12">
      <div className="w-12 h-12 mx-auto mb-3 rounded-full bg-a-border/40 flex items-center justify-center">
        <svg className="w-6 h-6 text-a-muted" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
          <circle cx="12" cy="12" r="10" />
          <path d="M12 8v4M12 16h.01" />
        </svg>
      </div>
      <h4 className="text-sm font-medium text-a-fg mb-1">{title}</h4>
      {description && <p className="text-xs text-a-muted max-w-sm mx-auto">{description}</p>}
      {action && <div className="mt-4">{action}</div>}
    </div>
  );
}

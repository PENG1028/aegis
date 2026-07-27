/**
 * QueryGuard — structural wrapper that handles all 4 data states:
 *   loading → LoadingState
 *   error   → ErrorBanner with retry
 *   empty   → EmptyState with description
 *   data    → render children
 *
 * Usage:
 *   <QueryGuard items={nodes} isLoading={loading} error={err} refetch={refetch} emptyMessage="暂无节点">
 *     {(nodes) => nodes.map(n => <NodeCard key={n.id} node={n} />)}
 *   </QueryGuard>
 */
import type { ReactNode } from 'react';
import LoadingState from './LoadingState';
import ErrorBanner from './ErrorBanner';
import { EmptyState } from './EmptyState';

interface QueryGuardProps<T> {
  items: T[];
  isLoading: boolean;
  error: Error | null;
  refetch?: () => void;
  emptyMessage?: string;
  children: (items: T[]) => ReactNode;
}

export function QueryGuard<T>({
  items,
  isLoading,
  error,
  refetch,
  emptyMessage,
  children,
}: QueryGuardProps<T>) {
  if (isLoading) {
    return <LoadingState />;
  }

  if (error) {
    return (
      <ErrorBanner
        message={error.message}
        onRetry={refetch}
      />
    );
  }

  if (items.length === 0) {
    return (
      <EmptyState
        title="暂无数据"
        description={emptyMessage}
      />
    );
  }

  return <>{children(items)}</>;
}

/** Minimal wrapper — shows loading spinner only, delegates errors to caller. */
export function QuerySpinner({
  isLoading,
  children,
}: {
  isLoading: boolean;
  children: ReactNode;
}) {
  if (isLoading) return <LoadingState />;
  return <>{children}</>;
}

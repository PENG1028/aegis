/**
 * useApiList / useApiItem — structural hooks wrapping react-query.
 *
 * Every list page uses useApiList<T> instead of raw useQuery.
 * Every detail page uses useApiItem<T> instead of raw useQuery.
 *
 * This ensures loading / error / empty states are handled uniformly
 * and no page accidentally omits them.
 */
import { useQuery, type UseQueryResult } from '@tanstack/react-query';
import { useMemo } from 'react';

export interface ApiListResult<T> {
  /** Extracted items — always an array (empty on error/no-data). */
  items: T[];
  isLoading: boolean;
  error: Error | null;
  refetch: () => void;
}

export interface ApiItemResult<T> {
  item: T | null;
  isLoading: boolean;
  error: Error | null;
  refetch: () => void;
}

/**
 * useApiList — for endpoints that return an array or a wrapper object.
 *
 * @param queryKey   react-query key
 * @param queryFn    async function returning T[] or { [extractKey]: T[] }
 * @param extractKey optional — if set and data is an object, extract data[extractKey] as the list
 */
export function useApiList<T>(
  queryKey: string[],
  queryFn: () => Promise<T[] | Record<string, unknown>>,
  extractKey?: string,
): ApiListResult<T> {
  const { data, isLoading, isError, error, refetch } = useQuery({
    queryKey,
    queryFn,
  }) as UseQueryResult<T[] | Record<string, unknown>, Error>;

  const items: T[] = useMemo(() => {
    if (data == null) return [];
    if (Array.isArray(data)) return data as T[];
    if (extractKey) {
      const inner = (data as Record<string, unknown>)[extractKey];
      if (Array.isArray(inner)) return inner as T[];
    }
    return [];
  }, [data, extractKey]);

  return {
    items,
    isLoading,
    error: isError ? (error ?? new Error('unknown error')) : null,
    refetch,
  };
}

/**
 * useApiItem — for endpoints that return a single object.
 */
export function useApiItem<T>(
  queryKey: string[],
  queryFn: () => Promise<T | null>,
): ApiItemResult<T> {
  const { data, isLoading, isError, error, refetch } = useQuery({
    queryKey,
    queryFn,
  }) as UseQueryResult<T | null, Error>;

  return {
    item: data ?? null,
    isLoading,
    error: isError ? (error ?? new Error('unknown error')) : null,
    refetch,
  };
}

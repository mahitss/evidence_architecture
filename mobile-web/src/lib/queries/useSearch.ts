import { useEffect, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { searchArchive } from '../api';

export function useDebouncedValue<T>(value: T, delayMs: number): T {
  const [debounced, setDebounced] = useState<T>(value);

  useEffect(() => {
    if (delayMs <= 0) {
      setDebounced(value);
      return;
    }
    const timer = window.setTimeout(() => setDebounced(value), delayMs);
    return () => window.clearTimeout(timer);
  }, [value, delayMs]);

  return debounced;
}

export function useSearch(query: string, category?: string, cursor?: string, debounceMs = 300) {
  const debouncedQuery = useDebouncedValue(query.trim(), debounceMs);

  return useQuery({
    queryKey: ['search', debouncedQuery, category ?? 'all', cursor ?? 'first'],
    queryFn: ({ signal }) =>
      searchArchive({
        query: debouncedQuery,
        category,
        cursor,
        signal,
      }),
    enabled: debouncedQuery.length >= 2,
    staleTime: 30_000,
  });
}

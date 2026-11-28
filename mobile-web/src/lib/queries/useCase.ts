import { useQuery } from '@tanstack/react-query';
import { fetchCaseBySlug, GeoblockError, RateLimitError } from '../api';

export function useCase(slug: string | undefined) {
  return useQuery({
    queryKey: ['case', slug ?? ''],
    queryFn: ({ signal }) => {
      if (!slug) return Promise.resolve(null);
      return fetchCaseBySlug(slug, signal);
    },
    enabled: Boolean(slug && slug.trim().length > 0),
    staleTime: 300_000,
    retry: (failureCount, error) => {
      if (error instanceof GeoblockError || error instanceof RateLimitError) {
        return false;
      }
      return failureCount < 1;
    },
  });
}

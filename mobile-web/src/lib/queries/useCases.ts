import { useQuery } from '@tanstack/react-query';
import { fetchCases } from '../api';

export function useCases(category?: string, cursor?: string) {
  return useQuery({
    queryKey: ['cases', category ?? 'all', cursor ?? 'first'],
    queryFn: ({ signal }) => fetchCases({ category, cursor, signal }),
    staleTime: 60_000,
  });
}

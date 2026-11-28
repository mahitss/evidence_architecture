import { useQuery } from '@tanstack/react-query';
import { fetchEvidenceById, GeoblockError, RateLimitError } from '../api';

export function useEvidence(id: string | undefined) {
  return useQuery({
    queryKey: ['evidence', id ?? ''],
    queryFn: ({ signal }) => {
      if (!id) {
        return Promise.reject(new Error('Missing evidence id'));
      }
      return fetchEvidenceById(id, signal);
    },
    enabled: Boolean(id && id.trim().length > 0),
    staleTime: 300_000,
    retry: (failureCount, error) => {
      if (error instanceof GeoblockError || error instanceof RateLimitError) {
        return false;
      }
      return failureCount < 1;
    },
  });
}

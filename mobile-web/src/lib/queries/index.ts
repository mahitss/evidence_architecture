import { MutationCache, QueryCache, QueryClient } from '@tanstack/react-query';
import { GeoblockError, RateLimitError } from '../api';

export interface GlobalApiErrorHandlers {
  onGeoblocked: (error: GeoblockError) => void;
  onRateLimited: (error: RateLimitError) => void;
}

export function createAppQueryClient(handlers?: GlobalApiErrorHandlers): QueryClient {
  const handleError = (error: unknown) => {
    if (error instanceof GeoblockError) {
      handlers?.onGeoblocked(error);
    } else if (error instanceof RateLimitError) {
      handlers?.onRateLimited(error);
    }
  };

  return new QueryClient({
    queryCache: new QueryCache({
      onError: handleError,
    }),
    mutationCache: new MutationCache({
      onError: handleError,
    }),
    defaultOptions: {
      queries: {
        retry: (failureCount, error) => {
          if (error instanceof GeoblockError || error instanceof RateLimitError) {
            return false;
          }
          return failureCount < 1;
        },
        retryDelay: (attemptIndex) => Math.min(150 * 2 ** attemptIndex, 1000),
        refetchOnWindowFocus: false,
      },
    },
  });
}

export { useCases } from './useCases';
export { useCase } from './useCase';
export { useEvidence } from './useEvidence';
export { useSearch, useDebouncedValue } from './useSearch';
export { useSubmitCorrection } from './useSubmitCorrection';

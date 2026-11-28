import type { ReactNode } from 'react';
import { QueryClientProvider } from '@tanstack/react-query';
import { renderHook, waitFor } from '@testing-library/react';
import { http, HttpResponse } from 'msw';
import { describe, expect, it } from 'vitest';
import { createAppQueryClient, useCases, useSearch } from '../index';
import { API_BASE, server } from '../../../test/mswHandlers';

function createWrapper() {
  const client = createAppQueryClient();
  client.setDefaultOptions({ queries: { retry: false } });
  return function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
  };
}

describe('Query hooks', () => {
  it('useCases transitions from loading to success', async () => {
    const { result } = renderHook(() => useCases(), { wrapper: createWrapper() });

    expect(result.current.isLoading).toBe(true);
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(result.current.data?.data).toHaveLength(2);
    expect(result.current.data?.data[0]?.slug).toBe('us-sec-clearinghouse-2024');
  });

  it('useCases surfaces error state on API failure', async () => {
    server.use(
      http.get(`${API_BASE}/api/v1/cases`, () =>
        HttpResponse.json(
          { error: { code: 'SERVICE_UNAVAILABLE', message: 'Database offline' } },
          { status: 503 },
        ),
      ),
    );

    const { result } = renderHook(() => useCases(), { wrapper: createWrapper() });
    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(result.current.error?.message).toMatch(/Database offline/i);
  });

  it('useSearch stays disabled under 2 characters and executes debounced query for >= 2 chars', async () => {
    const { result, rerender } = renderHook(
      ({ q }: { q: string }) => useSearch(q, undefined, undefined, 50),
      {
        initialProps: { q: 'm' },
        wrapper: createWrapper(),
      },
    );

    expect(result.current.fetchStatus).toBe('idle');

    rerender({ q: 'meridian' });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data?.data).toHaveLength(1);
    expect(result.current.data?.data[0]?.highlight).toContain('<mark>Meridian</mark>');
  });
});

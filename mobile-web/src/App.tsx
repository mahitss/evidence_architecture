import { useMemo } from 'react';
import { QueryClientProvider } from '@tanstack/react-query';
import { BrowserRouter, Route, Routes, useNavigate } from 'react-router-dom';
import { AppErrorBoundary } from './components/ErrorBoundary';
import { ToastProvider, useToast } from './components/ui';
import { ReaderLayout } from './layouts/ReaderLayout';
import { createAppQueryClient } from './lib/queries';
import { About } from './routes/About';
import { CaseDetail } from './routes/CaseDetail';
import { CaseList } from './routes/CaseList';
import { CorrectionForm } from './routes/CorrectionForm';
import { EvidenceDetail } from './routes/EvidenceDetail';
import { GeoblockPage } from './routes/GeoblockPage';
import { NotFound } from './routes/NotFound';
import { SearchResults } from './routes/SearchResults';

/**
 * Router-agnostic application shell so both BrowserRouter (prod) and MemoryRouter (tests)
 * share identical global 451 geoblock navigation and 429 rate-limit countdown toasts.
 */
export function AppShell() {
  const navigate = useNavigate();
  const { pushToast } = useToast();

  const queryClient = useMemo(
    () =>
      createAppQueryClient({
        onGeoblocked: () => {
          navigate('/geoblocked', { replace: true });
        },
        onRateLimited: (err) => {
          pushToast({
            message: 'Rate limit reached — please pause before retrying.',
            variant: 'warning',
            countdownSeconds: err.retryAfter,
          });
        },
      }),
    [navigate, pushToast],
  );

  return (
    <QueryClientProvider client={queryClient}>
      <AppErrorBoundary>
        <Routes>
          <Route element={<ReaderLayout />}>
            <Route path="/" element={<CaseList />} />
            <Route path="/case/:slug" element={<CaseDetail />} />
            <Route path="/evidence/:id" element={<EvidenceDetail />} />
            <Route path="/search" element={<SearchResults />} />
            <Route path="/correction" element={<CorrectionForm />} />
            <Route path="/geoblocked" element={<GeoblockPage />} />
            <Route path="/about" element={<About />} />
            <Route path="*" element={<NotFound />} />
          </Route>
        </Routes>
      </AppErrorBoundary>
    </QueryClientProvider>
  );
}

export function App() {
  return (
    <ToastProvider>
      <BrowserRouter future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
        <AppShell />
      </BrowserRouter>
    </ToastProvider>
  );
}

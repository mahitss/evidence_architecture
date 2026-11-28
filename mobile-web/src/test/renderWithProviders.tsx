import { render } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { AppShell } from '../App';
import { ToastProvider } from '../components/ui';

export function renderAppAt(route: string) {
  return render(
    <ToastProvider>
      <MemoryRouter
        initialEntries={[route]}
        future={{ v7_startTransition: true, v7_relativeSplatPath: true }}
      >
        <AppShell />
      </MemoryRouter>
    </ToastProvider>,
  );
}

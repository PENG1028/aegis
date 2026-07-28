import type { ReactElement } from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { render } from '@testing-library/react';
import { ToastProvider } from '@/components/shared';

export function renderPage(ui: ReactElement, initialPath = '/', routePath?: string) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0 },
      mutations: { retry: false },
    },
  });
  return {
    queryClient,
    ...render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter initialEntries={[initialPath]}>
          <ToastProvider>
            {routePath ? <Routes><Route path={routePath} element={ui} /></Routes> : ui}
          </ToastProvider>
        </MemoryRouter>
      </QueryClientProvider>,
    ),
  };
}

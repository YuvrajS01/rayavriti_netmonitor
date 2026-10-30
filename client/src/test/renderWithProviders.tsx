/* eslint-disable react-refresh/only-export-components -- test helper, never hot-reloaded */
import { render } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom';
import type { ReactElement } from 'react';
import { ToastProvider } from '../components/ui/Toast';
import { SocketContext, type SocketContextValue } from '../hooks/socketContext';

const idleSocket: SocketContextValue = { subscribe: () => () => {} };

function LocationProbe() {
  const location = useLocation();
  return <output data-testid="location">{location.pathname + location.search}</output>;
}

/**
 * Render a page with the providers it expects: a fresh QueryClient (no
 * retries), toasts, an idle socket and a MemoryRouter at `url`. `path` is
 * the route pattern the element is mounted on (for useParams).
 */
export function renderPage(ui: ReactElement, { url = '/', path = '*', socket = idleSocket }: { url?: string; path?: string; socket?: SocketContextValue } = {}) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  const result = render(
    <QueryClientProvider client={client}>
      <SocketContext.Provider value={socket}>
        <ToastProvider>
          <MemoryRouter initialEntries={[url]}>
            <Routes>
              <Route path={path} element={<>{ui}<LocationProbe /></>} />
              <Route path="*" element={<LocationProbe />} />
            </Routes>
          </MemoryRouter>
        </ToastProvider>
      </SocketContext.Provider>
    </QueryClientProvider>,
  );
  return { ...result, client };
}

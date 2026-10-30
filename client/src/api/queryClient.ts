import { QueryClient } from '@tanstack/react-query';
import { isAxiosError } from 'axios';

/** Don't retry client errors (auth, validation, not found); retry the rest twice. */
function shouldRetry(failureCount: number, error: unknown) {
  if (isAxiosError(error)) {
    const status = error.response?.status;
    if (status && status >= 400 && status < 500) return false;
  }
  return failureCount < 2;
}

export function createQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        // Live data arrives over the WebSocket; most REST data only needs
        // refetching when something invalidates it.
        staleTime: 30_000,
        retry: shouldRetry,
        refetchOnWindowFocus: false,
      },
      mutations: {
        retry: false,
      },
    },
  });
}

export const queryClient = createQueryClient();

import { v1, wrapJson, type RequestOptions } from './http';
import type { SearchResults } from './types';

/** The backend rejects shorter queries. */
export const SEARCH_MIN_CHARS = 2;

/** Global entity search for the command palette (GET /search). */
export const searchEntities = (q: string, limit = 8, { signal }: RequestOptions = {}): Promise<SearchResults> =>
  v1.get(`/search?${new URLSearchParams({ q, limit: String(limit) })}`, { signal }).then((r) => {
    const { data } = wrapJson<Partial<SearchResults> | null>(r.data);
    return {
      devices: data?.devices ?? [],
      locations: data?.locations ?? [],
      alerts: data?.alerts ?? [],
    };
  });

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { createResource, deleteResource, listResources, updateResource, type ResourceRow } from '../api/resources';
import { queryKeys } from '../api/queryKeys';

/** Cached list of a generic CRUD collection (e.g. '/contacts'). */
export function useResourceList<T extends ResourceRow = ResourceRow>(path: string) {
  return useQuery({
    queryKey: queryKeys.resources.list(path),
    queryFn: ({ signal }) => listResources(path, { signal }),
    select: (res) => (res.data || []) as T[],
  });
}

/** Create/update and delete mutations for a collection; both refresh its list. */
export function useResourceMutations(path: string) {
  const queryClient = useQueryClient();
  const invalidate = () => queryClient.invalidateQueries({ queryKey: queryKeys.resources.list(path) });

  const save = useMutation({
    mutationFn: ({ id, body }: { id?: number | string; body: ResourceRow }) =>
      id != null ? updateResource(path, id, body) : createResource(path, body),
    onSuccess: invalidate,
  });

  const remove = useMutation({
    mutationFn: (id: number | string) => deleteResource(path, id),
    onSuccess: invalidate,
  });

  return { save, remove };
}

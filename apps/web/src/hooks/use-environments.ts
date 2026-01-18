import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { environmentsApi } from '@/lib/api-client';
import type { Environment, EnvironmentCreateRequest } from '@/types/api';

export const environmentKeys = {
  all: ['environments'] as const,
  list: (vaultId: string) => [...environmentKeys.all, 'list', vaultId] as const,
  detail: (id: string) => [...environmentKeys.all, 'detail', id] as const,
};

export function useEnvironments(vaultId: string) {
  return useQuery({
    queryKey: environmentKeys.list(vaultId),
    queryFn: () => environmentsApi.list(vaultId),
    enabled: !!vaultId,
  });
}

export function useCreateEnvironment() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ vaultId, data }: { vaultId: string; data: EnvironmentCreateRequest }) =>
      environmentsApi.create(vaultId, data),
    onSuccess: (_, { vaultId }) => {
      queryClient.invalidateQueries({ queryKey: environmentKeys.list(vaultId) });
    },
  });
}

export function useDeleteEnvironment() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => environmentsApi.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: environmentKeys.all });
    },
  });
}

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { secretsApi } from '@/lib/api-client';
import type { Secret, SecretCreateRequest, SecretUpdateRequest } from '@/types/api';
import { environmentKeys } from './use-environments';

export const secretKeys = {
  all: ['secrets'] as const,
  list: (envId: string) => [...secretKeys.all, 'list', envId] as const,
  detail: (id: string) => [...secretKeys.all, 'detail', id] as const,
};

export function useSecrets(envId: string) {
  return useQuery({
    queryKey: secretKeys.list(envId),
    queryFn: () => secretsApi.list(envId),
    enabled: !!envId,
  });
}

export function useCreateSecret() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ envId, data }: { envId: string; data: SecretCreateRequest }) =>
      secretsApi.create(envId, data),
    onSuccess: (_, { envId }) => {
      queryClient.invalidateQueries({ queryKey: secretKeys.list(envId) });
    },
  });
}

export function useUpdateSecret() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: SecretUpdateRequest }) =>
      secretsApi.update(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: secretKeys.all });
    },
  });
}

export function useDeleteSecret() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => secretsApi.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: secretKeys.all });
    },
  });
}

export function useRevealSecret() {
  return useMutation({
    mutationFn: (id: string) => secretsApi.reveal(id),
  });
}

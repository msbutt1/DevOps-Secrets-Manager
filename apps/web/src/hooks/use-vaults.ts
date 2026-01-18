import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { vaultsApi } from '@/lib/api-client';
import type { Vault, VaultCreateRequest, VaultUpdateRequest } from '@/types/api';

// Query keys
export const vaultKeys = {
  all: ['vaults'] as const,
  list: (orgId?: string) => [...vaultKeys.all, { orgId }] as const,
  detail: (id: string) => [...vaultKeys.all, 'detail', id] as const,
};

// List vaults
export function useVaults(organizationId?: string) {
  return useQuery({
    queryKey: vaultKeys.list(organizationId),
    queryFn: () => vaultsApi.list(organizationId),
  });
}

// Get single vault
export function useVault(id: string) {
  return useQuery({
    queryKey: vaultKeys.detail(id),
    queryFn: () => vaultsApi.get(id),
    enabled: !!id,
  });
}

// Create vault mutation
export function useCreateVault() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: VaultCreateRequest) => vaultsApi.create(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: vaultKeys.all });
    },
  });
}

// Update vault mutation
export function useUpdateVault() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: VaultUpdateRequest }) =>
      vaultsApi.update(id, data),
    onSuccess: (_, { id }) => {
      queryClient.invalidateQueries({ queryKey: vaultKeys.all });
      queryClient.invalidateQueries({ queryKey: vaultKeys.detail(id) });
    },
  });
}

// Delete vault mutation
export function useDeleteVault() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => vaultsApi.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: vaultKeys.all });
    },
  });
}

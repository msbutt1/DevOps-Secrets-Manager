import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { accessApi } from '@/lib/api-client';
import type { VaultMember, AddMemberRequest, UpdateMemberRequest } from '@/types/api';

export const accessKeys = {
  all: ['access'] as const,
  members: (vaultId: string) => [...accessKeys.all, 'members', vaultId] as const,
};

export function useVaultMembers(vaultId: string) {
  return useQuery({
    queryKey: accessKeys.members(vaultId),
    queryFn: () => accessApi.listMembers(vaultId),
    enabled: !!vaultId,
  });
}

export function useAddMember() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ vaultId, data }: { vaultId: string; data: AddMemberRequest }) =>
      accessApi.addMember(vaultId, data),
    onSuccess: (_, { vaultId }) => {
      queryClient.invalidateQueries({ queryKey: accessKeys.members(vaultId) });
    },
  });
}

export function useUpdateMember() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ vaultId, userId, data }: { vaultId: string; userId: string; data: UpdateMemberRequest }) =>
      accessApi.updateMember(vaultId, userId, data),
    onSuccess: (_, { vaultId }) => {
      queryClient.invalidateQueries({ queryKey: accessKeys.members(vaultId) });
    },
  });
}

export function useRemoveMember() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ vaultId, userId }: { vaultId: string; userId: string }) =>
      accessApi.removeMember(vaultId, userId),
    onSuccess: (_, { vaultId }) => {
      queryClient.invalidateQueries({ queryKey: accessKeys.members(vaultId) });
    },
  });
}

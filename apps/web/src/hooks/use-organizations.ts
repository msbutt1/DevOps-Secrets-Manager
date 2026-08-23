import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { orgsApi } from '@/lib/api-client';
import type { CreateInviteRequest, VaultRole } from '@/types/api';

export const orgKeys = {
  all: ['orgs'] as const,
  list: () => [...orgKeys.all, 'list'] as const,
  detail: (id: string) => [...orgKeys.all, 'detail', id] as const,
  members: (id: string) => [...orgKeys.all, 'members', id] as const,
  invites: (id: string) => [...orgKeys.all, 'invites', id] as const,
};

export function useOrganizations() {
  return useQuery({ queryKey: orgKeys.list(), queryFn: orgsApi.list });
}

export function useOrganizationMembers(orgId: string | undefined) {
  return useQuery({
    queryKey: orgKeys.members(orgId ?? ''),
    queryFn: () => orgsApi.listMembers(orgId!),
    enabled: !!orgId,
  });
}

export function useInvites(orgId: string | undefined, enabled = true) {
  return useQuery({
    queryKey: orgKeys.invites(orgId ?? ''),
    queryFn: () => orgsApi.listInvites(orgId!),
    enabled: !!orgId && enabled,
  });
}

export function useRenameOrganization(orgId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (name: string) => orgsApi.update(orgId, { name }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: orgKeys.all }),
  });
}

export function useCreateInvite(orgId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: CreateInviteRequest) => orgsApi.createInvite(orgId, data),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: orgKeys.invites(orgId) }),
  });
}

export function useRevokeInvite(orgId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (inviteId: string) => orgsApi.revokeInvite(orgId, inviteId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: orgKeys.invites(orgId) }),
  });
}

export function useUpdateOrganizationMember(orgId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ userId, role }: { userId: string; role: VaultRole }) =>
      orgsApi.updateMember(orgId, userId, { role }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: orgKeys.all }),
  });
}

export function useRemoveOrganizationMember(orgId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (userId: string) => orgsApi.removeMember(orgId, userId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: orgKeys.all }),
  });
}

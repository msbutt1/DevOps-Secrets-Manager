import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { serviceTokensApi } from '@/lib/api-client';
import type { CreateServiceTokenRequest } from '@/types/api';

const tokenKeys = {
  list: (envId: string) => ['service-tokens', envId] as const,
};

export function useServiceTokens(envId: string | undefined, enabled: boolean) {
  return useQuery({
    queryKey: tokenKeys.list(envId ?? ''),
    queryFn: () => serviceTokensApi.list(envId!),
    enabled: !!envId && enabled,
  });
}

export function useCreateServiceToken(envId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: CreateServiceTokenRequest) => serviceTokensApi.create(envId, data),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: tokenKeys.list(envId) }),
  });
}

export function useRevokeServiceToken(envId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (tokenId: string) => serviceTokensApi.revoke(tokenId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: tokenKeys.list(envId) }),
  });
}

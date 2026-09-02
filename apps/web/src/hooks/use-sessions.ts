import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { sessionsApi } from '@/lib/api-client';

const sessionKeys = { list: ['sessions'] as const };

export function useSessions() {
  return useQuery({ queryKey: sessionKeys.list, queryFn: sessionsApi.list });
}

export function useRevokeSession() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (sessionId: string) => sessionsApi.revoke(sessionId),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: sessionKeys.list }),
  });
}

export function useRevokeOtherSessions() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: sessionsApi.revokeOthers,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: sessionKeys.list }),
  });
}

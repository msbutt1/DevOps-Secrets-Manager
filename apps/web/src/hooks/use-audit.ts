import { useQuery } from '@tanstack/react-query';
import { auditApi } from '@/lib/api-client';
import type { AuditFilters, AuditEvent, PaginatedResponse } from '@/types/api';

export const auditKeys = {
  all: ['audit'] as const,
  list: (filters: AuditFilters) => [...auditKeys.all, 'list', filters] as const,
};

export function useAuditLogs(filters: AuditFilters) {
  return useQuery<PaginatedResponse<AuditEvent>>({
    queryKey: auditKeys.list(filters),
    queryFn: () => auditApi.list(filters),
  });
}

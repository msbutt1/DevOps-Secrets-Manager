import { useQuery } from '@tanstack/react-query';
import { healthApi, statsApi } from '@/lib/api-client';

export const dashboardKeys = {
  stats: (orgId?: string) => ['dashboard', 'stats', { orgId }] as const,
  health: ['dashboard', 'health'] as const,
  alerts: (orgId?: string) => ['dashboard', 'alerts', { orgId }] as const,
};

export function useDashboardAlerts(organizationId?: string) {
  return useQuery({
    queryKey: dashboardKeys.alerts(organizationId),
    queryFn: () => statsApi.alerts(organizationId),
  });
}

export function useDashboardStats(organizationId?: string) {
  return useQuery({
    queryKey: dashboardKeys.stats(organizationId),
    queryFn: () => statsApi.get(organizationId),
  });
}

export function useHealth() {
  return useQuery({
    queryKey: dashboardKeys.health,
    queryFn: healthApi.check,
    refetchInterval: 60_000,
    retry: false,
  });
}

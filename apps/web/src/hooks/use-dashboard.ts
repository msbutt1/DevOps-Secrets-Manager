import { useQuery } from '@tanstack/react-query';
import { healthApi, statsApi } from '@/lib/api-client';

export const dashboardKeys = {
  stats: ['dashboard', 'stats'] as const,
  health: ['dashboard', 'health'] as const,
  alerts: ['dashboard', 'alerts'] as const,
};

export function useDashboardAlerts() {
  return useQuery({
    queryKey: dashboardKeys.alerts,
    queryFn: statsApi.alerts,
  });
}

export function useDashboardStats() {
  return useQuery({
    queryKey: dashboardKeys.stats,
    queryFn: statsApi.get,
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

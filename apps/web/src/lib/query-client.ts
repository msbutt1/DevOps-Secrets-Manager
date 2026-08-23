import { QueryClient } from '@tanstack/react-query';

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      // Vaults, secrets and memberships change under other people's hands (a new vault role, a
      // rotated secret), so cached data is shown immediately but always revalidated when a page
      // mounts or the window regains focus.
      staleTime: 0,
      retry: 1,
      refetchOnWindowFocus: true,
    },
  },
});

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  ReactNode,
} from 'react';
import { useAuth } from '@/contexts/AuthContext';
import { useOrganizations } from '@/hooks/use-organizations';
import { pickDefaultOrganization } from '@/lib/organizations';
import type { Organization } from '@/types/api';

interface OrganizationContextType {
  organizations: Organization[];
  /** The organization org-level pages (dashboard, vaults, audit, members) are scoped to */
  currentOrganization: Organization | null;
  setCurrentOrganizationId: (id: string) => void;
  isLoading: boolean;
}

const OrganizationContext = createContext<OrganizationContextType | null>(null);

const storageKey = (userId: string) => `dsm.currentOrganization.${userId}`;

// Browser storage can be unavailable (private windows, blocked site data); the choice is a
// convenience, so failures are ignored.
const readChoice = (userId: string) => {
  try {
    return window.localStorage.getItem(storageKey(userId));
  } catch {
    return null;
  }
};
const saveChoice = (userId: string, orgId: string) => {
  try {
    window.localStorage.setItem(storageKey(userId), orgId);
  } catch {
    // ignore
  }
};

export const OrganizationProvider = ({ children }: { children: ReactNode }) => {
  const { user, isAuthenticated } = useAuth();
  const { data: organizations = [], isLoading } = useOrganizations(isAuthenticated);
  const [selectedId, setSelectedId] = useState<string | null>(null);

  useEffect(() => {
    setSelectedId(user ? readChoice(user.id) : null);
  }, [user]);

  const currentOrganization = useMemo(
    () => organizations.find((o) => o.id === selectedId) ?? pickDefaultOrganization(organizations),
    [organizations, selectedId],
  );

  const setCurrentOrganizationId = useCallback(
    (id: string) => {
      setSelectedId(id);
      if (user) saveChoice(user.id, id);
    },
    [user],
  );

  return (
    <OrganizationContext.Provider
      value={{ organizations, currentOrganization, setCurrentOrganizationId, isLoading }}
    >
      {children}
    </OrganizationContext.Provider>
  );
};

export const useCurrentOrganization = () => {
  const context = useContext(OrganizationContext);
  if (!context) {
    throw new Error('useCurrentOrganization must be used within OrganizationProvider');
  }
  return context;
};

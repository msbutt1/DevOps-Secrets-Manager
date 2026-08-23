import { createContext, useContext, useState, useEffect, useCallback, ReactNode } from 'react';
import type { User, LoginRequest } from '@/types/api';
import { authApi, onAuthChange, isAuthenticated as checkAuth, clearTokens } from '@/lib/api-client';

interface AuthContextType {
  user: User | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  error: string | null;
  login: (credentials: LoginRequest) => Promise<void>;
  logout: () => Promise<void>;
  /** Reloads the profile, e.g. after joining an organization */
  refreshUser: () => Promise<void>;
  clearError: () => void;
}

const AuthContext = createContext<AuthContextType | null>(null);

export const useAuth = () => {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within AuthProvider');
  }
  return context;
};

interface AuthProviderProps {
  children: ReactNode;
}

export const AuthProvider = ({ children }: AuthProviderProps) => {
  const [user, setUser] = useState<User | null>(null);
  const [isAuthenticated, setIsAuthenticated] = useState(checkAuth());
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Subscribe to auth changes
  useEffect(() => {
    const unsubscribe = onAuthChange((authenticated) => {
      setIsAuthenticated(authenticated);
      if (!authenticated) {
        setUser(null);
      }
    });
    return () => {
      unsubscribe();
    };
  }, []);

  // Load user on mount if authenticated
  useEffect(() => {
    const loadUser = async () => {
      if (!checkAuth()) {
        setIsLoading(false);
        return;
      }

      try {
        const userData = await authApi.me();
        setUser(userData);
      } catch {
        clearTokens();
      } finally {
        setIsLoading(false);
      }
    };

    loadUser();
  }, []);

  const login = useCallback(async (credentials: LoginRequest) => {
    setError(null);
    setIsLoading(true);

    try {
      await authApi.login(credentials);
      const userData = await authApi.me();
      setUser(userData);
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Login failed';
      setError(message);
      throw err;
    } finally {
      setIsLoading(false);
    }
  }, []);

  const logout = useCallback(async () => {
    setIsLoading(true);
    try {
      await authApi.logout();
    } finally {
      setUser(null);
      setIsLoading(false);
    }
  }, []);

  const refreshUser = useCallback(async () => {
    setUser(await authApi.me());
  }, []);

  const clearError = useCallback(() => {
    setError(null);
  }, []);

  return (
    <AuthContext.Provider
      value={{
        user,
        isAuthenticated,
        isLoading,
        error,
        login,
        logout,
        refreshUser,
        clearError,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
};

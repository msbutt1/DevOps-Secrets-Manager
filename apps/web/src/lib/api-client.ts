import type {
  LoginRequest,
  RegisterRequest,
  RegisterResponse,
  VerifyEmailRequest,
  VerifyEmailResponse,
  ChangePasswordRequest,
  ChangePasswordResponse,
  AuthTokens,
  User,
  Vault,
  VaultCreateRequest,
  VaultUpdateRequest,
  Environment,
  EnvironmentCreateRequest,
  EnvironmentUpdateRequest,
  Secret,
  SecretCreateRequest,
  SecretUpdateRequest,
  SecretRevealResponse,
  VaultMember,
  Organization,
  OrganizationMember,
  UpdateOrganizationRequest,
  Invite,
  CreateInviteRequest,
  CreateInviteResponse,
  InviteLookup,
  ServiceToken,
  Session,
  CreatedServiceToken,
  CreateServiceTokenRequest,
  AddMemberRequest,
  UpdateMemberRequest,
  AuditEvent,
  AuditFilters,
  AuditEventPage,
  DashboardStats,
  DashboardAlert,
  HealthStatus,
  ApiError,
} from '@/types/api';
import { toCamelCase, toSnakeCase } from './case-transform';

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || '/api';

/** An error response from the API, keeping its machine-readable code (e.g. `email_not_verified`). */
export class ApiRequestError extends Error {
  constructor(
    message: string,
    readonly code: string,
    readonly status: number,
  ) {
    super(message);
    this.name = 'ApiRequestError';
  }
}

// The access token lives only in memory. The refresh token is an HttpOnly cookie set by the
// API, so scripts on the page (including injected ones) can never read it.
let accessToken: string | null = null;
let refreshPromise: Promise<AuthTokens> | null = null;

// Event emitter for auth state changes
type AuthEventListener = (isAuthenticated: boolean) => void;
const authListeners: Set<AuthEventListener> = new Set();

export const onAuthChange = (listener: AuthEventListener) => {
  authListeners.add(listener);
  return () => authListeners.delete(listener);
};

const notifyAuthChange = (isAuthenticated: boolean) => {
  authListeners.forEach((listener) => listener(isAuthenticated));
};

// Set tokens (called after login/refresh)
export const setTokens = (tokens: AuthTokens) => {
  accessToken = tokens.accessToken;
  notifyAuthChange(true);
};

// Clear tokens (called on logout)
export const clearTokens = () => {
  accessToken = null;
  refreshPromise = null;
  notifyAuthChange(false);
};

// Check if authenticated
export const isAuthenticated = () => !!accessToken;

// Base fetch with auth handling
async function apiFetch<T>(endpoint: string, options: RequestInit = {}, retry = true): Promise<T> {
  const headers: HeadersInit = {
    'Content-Type': 'application/json',
    ...options.headers,
  };

  if (accessToken) {
    (headers as Record<string, string>)['Authorization'] = `Bearer ${accessToken}`;
  }

  // Transform request body to snake_case
  const transformedOptions = { ...options };
  if (options.body && typeof options.body === 'string') {
    try {
      const parsed = JSON.parse(options.body);
      const snakeCaseBody = toSnakeCase(parsed);
      transformedOptions.body = JSON.stringify(snakeCaseBody);
    } catch {
      // If parsing fails, use original body
    }
  }

  const hadSession = !!accessToken;
  const response = await fetch(`${API_BASE_URL}${endpoint}`, {
    ...transformedOptions,
    headers,
    credentials: 'include',
  });

  // Handle 401 on an authenticated request - the access token expired, so refresh and retry
  if (response.status === 401 && retry && hadSession) {
    try {
      await refreshAccessToken();
      return apiFetch<T>(endpoint, options, false);
    } catch {
      clearTokens();
      throw new Error('Session expired. Please login again.');
    }
  }

  if (!response.ok) {
    const error: Partial<ApiError> = await response.json().catch(() => ({}));
    throw new ApiRequestError(
      error.message ?? 'An unexpected error occurred',
      error.error ?? 'unknown_error',
      response.status,
    );
  }

  // Handle empty responses
  const text = await response.text();
  if (!text) return {} as T;

  const jsonData = JSON.parse(text);
  // Transform response to camelCase
  return toCamelCase<T>(jsonData);
}

const postRefresh = () =>
  fetch(`${API_BASE_URL}/auth/refresh`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    body: '{}',
  });

// Exchanges the refresh cookie for a new access token (and a rotated cookie)
async function refreshAccessToken(): Promise<AuthTokens> {
  // Prevent multiple simultaneous refresh attempts
  if (refreshPromise) {
    return refreshPromise;
  }

  refreshPromise = (async () => {
    let response = await postRefresh();
    if (response.status === 401) {
      // Another tab may have rotated the cookie while this request was in flight; the
      // browser now holds the newer cookie, so one more attempt picks it up.
      await new Promise((resolve) => setTimeout(resolve, 200));
      response = await postRefresh();
    }
    if (!response.ok) {
      throw new Error('Refresh failed');
    }
    const tokens = toCamelCase<AuthTokens>(await response.json());
    setTokens(tokens);
    return tokens;
  })().finally(() => {
    refreshPromise = null;
  });

  return refreshPromise;
}

// ============ AUTH API ============
export const authApi = {
  login: async (credentials: LoginRequest): Promise<AuthTokens> => {
    const tokens = await apiFetch<AuthTokens>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ ...credentials, useCookie: true }),
    });
    setTokens(tokens);
    return tokens;
  },

  register: async (data: RegisterRequest): Promise<RegisterResponse> => {
    return apiFetch<RegisterResponse>('/auth/register', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  },

  resendVerification: (email: string): Promise<{ message: string }> =>
    apiFetch<{ message: string }>('/auth/resend-verification', {
      method: 'POST',
      body: JSON.stringify({ email }),
    }),

  verifyEmail: async (data: VerifyEmailRequest): Promise<VerifyEmailResponse> => {
    return apiFetch<VerifyEmailResponse>('/auth/verify-email', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  },

  changePassword: async (data: ChangePasswordRequest): Promise<ChangePasswordResponse> => {
    return apiFetch<ChangePasswordResponse>('/auth/change-password', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  },

  logout: async (): Promise<void> => {
    try {
      await apiFetch('/auth/logout', {
        method: 'POST',
        body: '{}',
      });
    } finally {
      clearTokens();
    }
  },

  me: (): Promise<User> => apiFetch<User>('/auth/me'),

  refresh: refreshAccessToken,

  /** Restores the session after a page load from the refresh cookie; false when there is none. */
  restoreSession: async (): Promise<boolean> => {
    try {
      await refreshAccessToken();
      return true;
    } catch {
      return false;
    }
  },
};

// ============ ORGANIZATIONS API ============
export const orgsApi = {
  list: (): Promise<Organization[]> => apiFetch<Organization[]>('/orgs'),

  get: (id: string): Promise<Organization> => apiFetch<Organization>(`/orgs/${id}`),

  update: (id: string, data: UpdateOrganizationRequest): Promise<Organization> =>
    apiFetch<Organization>(`/orgs/${id}`, { method: 'PATCH', body: JSON.stringify(data) }),

  listMembers: (id: string): Promise<OrganizationMember[]> =>
    apiFetch<OrganizationMember[]>(`/orgs/${id}/members`),

  updateMember: (
    id: string,
    userId: string,
    data: UpdateMemberRequest,
  ): Promise<OrganizationMember> =>
    apiFetch<OrganizationMember>(`/orgs/${id}/members/${userId}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    }),

  removeMember: (id: string, userId: string): Promise<void> =>
    apiFetch<void>(`/orgs/${id}/members/${userId}`, { method: 'DELETE' }),

  listInvites: (id: string): Promise<Invite[]> => apiFetch<Invite[]>(`/orgs/${id}/invites`),

  createInvite: (id: string, data: CreateInviteRequest): Promise<CreateInviteResponse> =>
    apiFetch<CreateInviteResponse>(`/orgs/${id}/invites`, {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  revokeInvite: (id: string, inviteId: string): Promise<void> =>
    apiFetch<void>(`/orgs/${id}/invites/${inviteId}`, { method: 'DELETE' }),
};

// ============ INVITES API ============
export const invitesApi = {
  // Tokens are sent in the body so they stay out of URLs and access logs
  lookup: (token: string): Promise<InviteLookup> =>
    apiFetch<InviteLookup>('/invites/lookup', { method: 'POST', body: JSON.stringify({ token }) }),

  accept: (token: string): Promise<Organization> =>
    apiFetch<Organization>('/invites/accept', { method: 'POST', body: JSON.stringify({ token }) }),
};

// ============ VAULTS API ============
export const vaultsApi = {
  list: (organizationId?: string): Promise<Vault[]> => {
    return apiFetch<Vault[]>(`/vaults${orgQuery(organizationId)}`);
  },

  get: (id: string): Promise<Vault> => apiFetch<Vault>(`/vaults/${id}`),

  create: (data: VaultCreateRequest): Promise<Vault> =>
    apiFetch<Vault>('/vaults', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  update: (id: string, data: VaultUpdateRequest): Promise<Vault> =>
    apiFetch<Vault>(`/vaults/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    }),

  delete: (id: string): Promise<void> => apiFetch<void>(`/vaults/${id}`, { method: 'DELETE' }),
};

// ============ ENVIRONMENTS API ============
export const environmentsApi = {
  list: (vaultId: string): Promise<Environment[]> =>
    apiFetch<Environment[]>(`/vaults/${vaultId}/envs`),

  get: (envId: string): Promise<Environment> => apiFetch<Environment>(`/envs/${envId}`),

  create: (vaultId: string, data: EnvironmentCreateRequest): Promise<Environment> =>
    apiFetch<Environment>(`/vaults/${vaultId}/envs`, {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  update: (envId: string, data: EnvironmentUpdateRequest): Promise<Environment> =>
    apiFetch<Environment>(`/envs/${envId}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    }),

  delete: (envId: string): Promise<void> => apiFetch<void>(`/envs/${envId}`, { method: 'DELETE' }),
};

// ============ SECRETS API ============
export const secretsApi = {
  list: (envId: string): Promise<Secret[]> => apiFetch<Secret[]>(`/envs/${envId}/secrets`),

  create: (envId: string, data: SecretCreateRequest): Promise<Secret> =>
    apiFetch<Secret>(`/envs/${envId}/secrets`, {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  update: (secretId: string, data: SecretUpdateRequest): Promise<Secret> =>
    apiFetch<Secret>(`/secrets/${secretId}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    }),

  delete: (secretId: string): Promise<void> =>
    apiFetch<void>(`/secrets/${secretId}`, { method: 'DELETE' }),

  reveal: (secretId: string): Promise<SecretRevealResponse> =>
    apiFetch<SecretRevealResponse>(`/secrets/${secretId}/reveal`, {
      method: 'POST',
    }),
};

// ============ SERVICE TOKENS API ============
export const serviceTokensApi = {
  list: (envId: string): Promise<ServiceToken[]> =>
    apiFetch<ServiceToken[]>(`/envs/${envId}/tokens`),

  create: (envId: string, data: CreateServiceTokenRequest): Promise<CreatedServiceToken> =>
    apiFetch<CreatedServiceToken>(`/envs/${envId}/tokens`, {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  revoke: (tokenId: string): Promise<void> =>
    apiFetch<void>(`/tokens/${tokenId}`, { method: 'DELETE' }),
};

// ============ SESSIONS API ============
export const sessionsApi = {
  list: (): Promise<Session[]> => apiFetch<Session[]>('/auth/sessions'),

  revoke: (sessionId: string): Promise<void> =>
    apiFetch<void>(`/auth/sessions/${sessionId}`, { method: 'DELETE' }),

  revokeOthers: (): Promise<{ sessionsRevoked: number }> =>
    apiFetch<{ sessionsRevoked: number }>('/auth/sessions/revoke-others', { method: 'POST' }),
};

// ============ ACCESS API ============
export const accessApi = {
  listMembers: (vaultId: string): Promise<VaultMember[]> =>
    apiFetch<VaultMember[]>(`/vaults/${vaultId}/members`),

  addMember: (vaultId: string, data: AddMemberRequest): Promise<VaultMember> =>
    apiFetch<VaultMember>(`/vaults/${vaultId}/members`, {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  updateMember: (
    vaultId: string,
    userId: string,
    data: UpdateMemberRequest,
  ): Promise<VaultMember> =>
    apiFetch<VaultMember>(`/vaults/${vaultId}/members/${userId}`, {
      method: 'PUT',
      body: JSON.stringify(data),
    }),

  removeMember: (vaultId: string, userId: string): Promise<void> =>
    apiFetch<void>(`/vaults/${vaultId}/members/${userId}`, {
      method: 'DELETE',
    }),
};

// ============ AUDIT API ============
export const auditApi = {
  list: (filters: AuditFilters = {}): Promise<AuditEventPage> => {
    const params = new URLSearchParams();
    Object.entries(filters).forEach(([key, value]) => {
      if (Array.isArray(value)) {
        value.forEach((item) => params.append(key, String(item)));
      } else if (value !== undefined && value !== '') {
        params.append(key, String(value));
      }
    });
    return apiFetch<AuditEventPage>(`/audit?${params.toString()}`);
  },
};

const orgQuery = (organizationId?: string) =>
  organizationId ? `?organizationId=${encodeURIComponent(organizationId)}` : '';

// ============ DASHBOARD API ============
export const statsApi = {
  get: (organizationId?: string): Promise<DashboardStats> =>
    apiFetch<DashboardStats>(`/stats${orgQuery(organizationId)}`),
  alerts: (organizationId?: string): Promise<DashboardAlert[]> =>
    apiFetch<DashboardAlert[]>(`/alerts${orgQuery(organizationId)}`),
};

// ============ HEALTH API ============
export const healthApi = {
  check: (): Promise<HealthStatus> => apiFetch<HealthStatus>('/health'),
};

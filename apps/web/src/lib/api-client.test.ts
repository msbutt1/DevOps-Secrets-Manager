import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { authApi, clearTokens, isAuthenticated } from './api-client';

const json = (status: number, body: unknown) =>
  new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });

describe('auth token handling', () => {
  const fetchMock = vi.fn();

  beforeEach(() => {
    fetchMock.mockReset();
    vi.stubGlobal('fetch', fetchMock);
    clearTokens();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('asks for the refresh token as a cookie when logging in', async () => {
    fetchMock.mockResolvedValueOnce(json(200, { access_token: 'access-1', expires_in: 900 }));

    await authApi.login({ email: 'a@example.test', password: 'pw' });

    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe('/api/auth/login');
    expect(JSON.parse(init.body)).toEqual({
      email: 'a@example.test',
      password: 'pw',
      use_cookie: true,
    });
    expect(init.credentials).toBe('include');
    expect(isAuthenticated()).toBe(true);
  });

  it('restores a session from the cookie without sending a token', async () => {
    fetchMock.mockResolvedValueOnce(json(200, { access_token: 'access-2', expires_in: 900 }));

    await expect(authApi.restoreSession()).resolves.toBe(true);

    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe('/api/auth/refresh');
    expect(init.body).toBe('{}');
    expect(init.credentials).toBe('include');
    expect(isAuthenticated()).toBe(true);

    // The restored access token is used for later requests
    fetchMock.mockResolvedValueOnce(json(200, { id: 'u1' }));
    await authApi.me();
    expect(fetchMock.mock.calls[1][1].headers.Authorization).toBe('Bearer access-2');
  });

  it('reports no session when there is no valid cookie', async () => {
    fetchMock.mockResolvedValue(json(401, { error: 'invalid_token', message: 'x' }));

    await expect(authApi.restoreSession()).resolves.toBe(false);
    expect(isAuthenticated()).toBe(false);
  });

  it('refreshes once and retries when the access token has expired', async () => {
    fetchMock.mockResolvedValueOnce(json(200, { access_token: 'old', expires_in: 900 }));
    await authApi.login({ email: 'a@example.test', password: 'pw' });

    fetchMock
      .mockResolvedValueOnce(json(401, { error: 'token_expired', message: 'expired' }))
      .mockResolvedValueOnce(json(200, { access_token: 'new', expires_in: 900 }))
      .mockResolvedValueOnce(json(200, { id: 'u1' }));

    await expect(authApi.me()).resolves.toEqual({ id: 'u1' });
    expect(fetchMock.mock.calls[2][0]).toBe('/api/auth/refresh');
    expect(fetchMock.mock.calls[3][1].headers.Authorization).toBe('Bearer new');
  });

  it('does not try to refresh after a failed login', async () => {
    fetchMock.mockResolvedValueOnce(json(401, { error: 'invalid_credentials', message: 'Nope' }));

    await expect(authApi.login({ email: 'a@example.test', password: 'bad' })).rejects.toThrow(
      'Nope',
    );
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });
});

/**
 * Cloudflare Pages Function: proxies /api/* to the API on Fly so the browser only ever talks to
 * devops.msbutt.com. One origin keeps the refresh cookie (SameSite=Strict, __Host- prefixed)
 * working and avoids CORS entirely.
 *
 * API_ORIGIN is set as a Pages environment variable, e.g. https://msbutt-secrets-api.fly.dev
 */
interface Env {
  API_ORIGIN: string;
}

export const onRequest: PagesFunction<Env> = async ({ request, params, env }) => {
  if (!env.API_ORIGIN) {
    return new Response('API_ORIGIN is not configured', { status: 500 });
  }

  const path = Array.isArray(params.path) ? params.path.join('/') : (params.path ?? '');
  const incoming = new URL(request.url);
  const upstream = new URL(`${env.API_ORIGIN.replace(/\/$/, '')}/${path}${incoming.search}`);

  const headers = new Headers(request.headers);
  // Let the API log the real client, not Cloudflare's edge
  headers.set('CF-Connecting-IP', request.headers.get('CF-Connecting-IP') ?? '');
  headers.delete('Host');

  const response = await fetch(
    new Request(upstream, {
      method: request.method,
      headers,
      body: request.body,
      redirect: 'manual',
    }),
  );

  // Pass the response through untouched: Set-Cookie, Cache-Control: no-store and the security
  // headers all matter to the browser
  return new Response(response.body, {
    status: response.status,
    statusText: response.statusText,
    headers: response.headers,
  });
};

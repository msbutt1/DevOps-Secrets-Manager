/**
 * Cloudflare Pages Function: proxies /api/* to the API so the browser only ever talks to
 * devops.msbutt.com. One origin keeps the refresh cookie (SameSite=Strict, __Host- prefixed)
 * working and avoids CORS entirely.
 *
 * API_ORIGIN is a Pages environment variable, e.g. https://msbutt-secrets-api.onrender.com.
 * EDGE_TOKEN is a Pages *secret* shared with the API's APP_EDGE_TOKEN: the platform's own
 * hostname stays publicly reachable, and without it anyone could address the API directly and
 * skip this proxy, Cloudflare's rate limits and the CF-Connecting-IP header below.
 */
interface Env {
  API_ORIGIN: string;
  EDGE_TOKEN?: string;
}

export const onRequest: PagesFunction<Env> = async ({ request, params, env }) => {
  if (!env.API_ORIGIN) {
    return new Response('API_ORIGIN is not configured', { status: 500 });
  }

  const path = Array.isArray(params.path) ? params.path.join('/') : (params.path ?? '');
  const incoming = new URL(request.url);
  const upstream = new URL(`${env.API_ORIGIN.replace(/\/$/, '')}/${path}${incoming.search}`);

  const headers = new Headers(request.headers);
  headers.delete('Host');
  // Let the API log the real client, not Cloudflare's edge. This cannot reuse CF-Connecting-IP:
  // Cloudflare manages that header on outgoing subrequests and replaces whatever a Worker sets
  // with the Worker's own egress address, so the API would record 162.x for every visitor.
  // Both headers are deleted first, so a client cannot supply either one itself.
  headers.delete('X-Client-IP');
  const clientIP = request.headers.get('CF-Connecting-IP');
  if (clientIP) {
    headers.set('X-Client-IP', clientIP);
  }
  // A client could otherwise send its own X-Edge-Token; only this proxy's value may reach the API
  headers.delete('X-Edge-Token');
  if (env.EDGE_TOKEN) {
    headers.set('X-Edge-Token', env.EDGE_TOKEN);
  }

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

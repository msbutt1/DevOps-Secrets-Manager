# Deploying

This describes a deployment of the API, the web app and PostgreSQL that has not been run against
a paid account: the configuration and commands are written out in full so they can be followed,
but the hosted demo itself is not part of this repository. Everything here was checked against
the code (environment variable names, migrations, health endpoint) rather than copied from a
template.

The shape is the same wherever you host it:

```
browser ──TLS──> web (nginx, static files + /api proxy) ──> API (Go) ──> PostgreSQL
                                                             │
                                                    MASTER_KEK from the platform's secret store
```

## What the API needs

| Variable | Required | Notes |
|----------|----------|-------|
| `MASTER_KEK` | yes | 64 hex characters. Generate with `openssl rand -hex 32`. Losing it means losing every secret |
| `APP_JWT_SECRET` | yes | At least 32 random characters, different from `MASTER_KEK` |
| `APP_DATABASE_HOST`, `_PORT`, `_USER`, `_PASSWORD`, `_NAME` | yes | Or `APP_DATABASE_SSLMODE=require` for a managed database |
| `APP_ENV` | no | Leave at `production`: it keeps the `Secure` cookie flag on and refuses to log email links |
| `APP_PUBLIC_URL` | yes in practice | The web app's address; it goes into verification, invite and reset links |
| `APP_TRUSTED_PROXIES` | yes behind a proxy | CIDRs of your load balancer, so rate limits and the audit log record real client IPs |
| `APP_CLIENT_IP_HEADER` | behind a CDN | A single-address header the edge writes, e.g. `CF-Connecting-IP` or `Fly-Client-IP`. Preferred over `X-Forwarded-For`, which a client can prepend a fake hop to |
| `PORT` | on Render/Koyeb | The platform sets it; the API listens there unless `APP_SERVER_PORT` says otherwise |
| `APP_EDGE_TOKEN` | behind a CDN | Shared secret the edge proxy must send in `X-Edge-Token`. Without it the platform's own hostname (`*.onrender.com`, `*.fly.dev`) is a way around the CDN's rate limits and client IP header. `/health` stays exempt |
| `APP_CORS_ALLOWED_ORIGINS` | only if split | Needed only when the web app is served from a different origin than the API |
| `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASSWORD`, `SMTP_FROM` | yes | Without them, production refuses to send invitations and resets |
| `MASTER_KEK_VERSION`, `MASTER_KEK_PREVIOUS` | during rotation | See [Rotating the master key](../README.md#rotating-the-master-key) |

The API applies migrations at startup and serves `/health`, which reports the database state and
the schema version — use it as the platform's health check.

Our own instance is at `devops.msbutt.com`; its exact steps, DNS records and secrets are in
[hosting-devops-msbutt-com.md](hosting-devops-msbutt-com.md).

## Docker Compose (one host)

The quickest real deployment. On a host with Docker:

```bash
git clone https://github.com/msbutt1/DevOps-Secrets-Manager.git
cd DevOps-Secrets-Manager
cp .env.example .env
# put real values in .env: MASTER_KEK, JWT_SECRET, DATABASE_PASSWORD, APP_PUBLIC_URL, SMTP_*
docker compose up --build -d
docker compose logs -f api        # watch the migrations run
```

`docker compose.yml` builds both images, keeps PostgreSQL data in the `postgres_data` volume and
waits for the database health check before starting the API. Put a TLS terminator (Caddy,
nginx, or your platform's load balancer) in front of the web container on port 3000, and set
`APP_TRUSTED_PROXIES` to its address so client IPs are not recorded as the proxy's.

## Fly.io + Neon + Cloudflare Pages (free tiers)

**1. Database (Neon).** Create a PostgreSQL 17 project and copy the connection details. Use a
role that owns the database; the API runs migrations itself.

**2. API (Fly.io).** From the repository root, with `apps/api/Dockerfile` as the image:

```bash
fly launch --no-deploy --dockerfile apps/api/Dockerfile --name your-secrets-api   # flags vary by flyctl version
fly secrets set MASTER_KEK="$(openssl rand -hex 32)"
fly secrets set APP_JWT_SECRET="$(openssl rand -hex 32)"
fly secrets set APP_DATABASE_HOST=ep-xxx.eu-central-1.aws.neon.tech \
                APP_DATABASE_PORT=5432 \
                APP_DATABASE_USER=your_user \
                APP_DATABASE_PASSWORD=your_password \
                APP_DATABASE_NAME=your_db \
                APP_DATABASE_SSLMODE=require
fly secrets set APP_PUBLIC_URL=https://secrets.example.com \
                APP_CORS_ALLOWED_ORIGINS=https://secrets.example.com \
                APP_TRUSTED_PROXIES=0.0.0.0/0
fly secrets set SMTP_HOST=smtp.example.com SMTP_PORT=587 \
                SMTP_USER=... SMTP_PASSWORD=... SMTP_FROM="Vault Console <noreply@example.com>"
fly deploy
```

`APP_TRUSTED_PROXIES=0.0.0.0/0` trusts Fly's proxy for the whole network, which is correct there
because only Fly can reach the app; on a host where anything else can connect directly, list the
proxy's addresses instead.

In `fly.toml`, point the health check at the API:

```toml
[http_service]
  internal_port = 8080
  force_https = true

  [[http_service.checks]]
    path = "/health"
    interval = "30s"
    timeout = "5s"
```

**3. Web app (Cloudflare Pages).** Build `apps/web` and proxy `/api` to the API so the browser
keeps talking to one origin — the refresh cookie is `SameSite=Strict`, and one origin avoids CORS
entirely.

- Build command: `npm ci && npm run build`
- Build directory: `apps/web`
- Output directory: `dist`
- Add `apps/web/public/_redirects` so client-side routes survive a reload:

```
/*  /index.html  200
```

- Proxy `/api` with a Pages Function, `apps/web/functions/api/[[path]].ts` (a `_redirects` rewrite
  cannot point at another origin):

```ts
export const onRequest: PagesFunction = ({ request, params }) => {
  const upstream = new URL(request.url);
  upstream.protocol = 'https:';
  upstream.host = 'your-secrets-api.fly.dev';
  upstream.pathname = `/${(params.path as string[]).join('/')}`;
  return fetch(new Request(upstream, request));
};
```

The alternative is to point the browser straight at the API: build with
`VITE_API_BASE_URL=https://your-secrets-api.fly.dev` and set `APP_CORS_ALLOWED_ORIGINS` to the
Pages origin. Both work; one origin keeps the `SameSite=Strict` refresh cookie simple.

If you would rather serve the web app from the same container as in Compose, deploy
`apps/web/Dockerfile` instead; its nginx config already proxies `/api`, sets the security
headers and sends HSTS when the request arrives over HTTPS.

## After the first deploy

1. Register the first account through the web app and verify it by email.
2. Check `/health` returns `"status":"ok"` and the expected `migration_version`.
3. Confirm the logs are JSON and contain no secrets (`fly logs`, `docker compose logs api`).
4. Set up [backups and monitoring](operations.md).
5. Keep `MASTER_KEK` in the platform's secret store and nowhere else — not in the repository, not
   in the image, not in the database backups.

## Upgrading

Deploy the new image; migrations run at startup. To roll back, deploy the previous image. The
image also ships the `keys` command, which is how you check master key versions in production:

```bash
docker compose exec api ./keys status          # Compose: master key versions in use
fly ssh console -C "/app/keys status"          # Fly: the image ships the keys command too
```

Rolling a migration back is not something the running image does for you: check out the matching
commit and run `make migrate-down N=1` against the database, or run
`go run ./apps/api/cmd/migrate down 1` with the same `APP_DATABASE_*` variables.

Every migration in `apps/api/migrations` has a matching `.down.sql`, and CI applies them up,
down and up again on every push, so a rollback is not a leap of faith.

# Hosting it at devops.msbutt.com

The concrete setup for this deployment: Cloudflare Pages serves the console at
`devops.msbutt.com` and proxies `/api` to the Go API on Fly.io, which talks to PostgreSQL on Neon
and sends mail through Resend. [deployment.md](deployment.md) covers the general case; this is the
runbook for ours, with the steps that need your accounts marked **you**.

```
browser ──> Cloudflare (DNS + Pages, devops.msbutt.com)
              ├── static console
              └── /api/* ──> Fly.io (Go API) ──> Neon (PostgreSQL)
                                   └─────────> Resend (SMTP, 587)
```

Everything in the repository is ready: `fly.toml`, the Pages Function in
`apps/web/functions/api/[[path]].ts`, `_headers` and `_redirects` in `apps/web/public`, the
deploy workflow in `.github/workflows/deploy.yml`, and `scripts/check-deployment.sh` to check the
result from outside.

## 1. Database — Neon (**you**)

Create a project on the free tier, PostgreSQL 17, region close to the Fly region (`iad` in
`fly.toml`). From the connection details keep: host, database, role, password. The API creates
its own schema on first start, so nothing else is needed.

## 2. API — Fly.io (**you**, then me)

```fish
fly auth login
fly apps create msbutt-secrets-api        # the name in fly.toml
fly secrets set \
  MASTER_KEK=(openssl rand -hex 32) \
  APP_JWT_SECRET=(openssl rand -hex 32) \
  APP_DATABASE_HOST=ep-xxx.eu-central-1.aws.neon.tech \
  APP_DATABASE_PORT=5432 \
  APP_DATABASE_USER=your_role \
  APP_DATABASE_PASSWORD=your_password \
  APP_DATABASE_NAME=neondb \
  APP_PUBLIC_URL=https://devops.msbutt.com \
  --app msbutt-secrets-api
fly deploy --remote-only
fly logs --app msbutt-secrets-api        # watch the migrations run
```

**Write `MASTER_KEK` down somewhere safe before you deploy.** It never leaves Fly's secret store,
and without it every stored secret is unreadable — including from a backup.

`fly.toml` already sets `APP_ENV=production`, `APP_DATABASE_SSLMODE=require`,
`APP_CLIENT_IP_HEADER=CF-Connecting-IP` and the health check on `/health`. Machines suspend when
idle and wake on the next request, which keeps the bill near zero; the first request after a pause
takes a second or two.

## 3. Console — Cloudflare Pages (**you**, then me)

Create a Pages project connected to the GitHub repository, or deploy from CI with the workflow
already in the repository.

| Setting | Value |
|---|---|
| Build command | `npm ci && npm run build` |
| Build output directory | `apps/web/dist` |
| Root directory | `apps/web` |
| Environment variable | `API_ORIGIN = https://msbutt-secrets-api.fly.dev` |

Then add the custom domain `devops.msbutt.com` in the Pages project; Cloudflare creates the CNAME
in the `msbutt.com` zone for you. Keep the record proxied (orange cloud).

The Pages Function forwards `/api/*` to Fly and passes `CF-Connecting-IP` through, so the audit
log records the visitor's address rather than Cloudflare's. Because the console and the API share
one origin, the refresh cookie stays `SameSite=Strict` and there is no CORS to configure —
`APP_CORS_ALLOWED_ORIGINS` stays empty.

## 4. Email — Resend (**you**, then me)

1. Create a Resend account and add the domain `msbutt.com` (or `mail.msbutt.com` if you would
   rather keep the apex clean).
2. Resend shows DNS records to add. In the Cloudflare DNS tab for `msbutt.com`, add them exactly
   as given — typically:
   - a TXT record for SPF (`v=spf1 include:amazonses.com ~all` or as Resend states)
   - one or more CNAME records for DKIM (`resend._domainkey`, …) — **DNS only, grey cloud**
   - optionally a DMARC TXT record at `_dmarc`: `v=DMARC1; p=none; rua=mailto:you@msbutt.com`
3. Wait for Resend to verify the domain, then create an API key.
4. Give it to the API:

```fish
fly secrets set \
  SMTP_HOST=smtp.resend.com \
  SMTP_PORT=587 \
  SMTP_USER=resend \
  SMTP_PASSWORD=re_your_api_key \
  SMTP_FROM="Vault Console <noreply@msbutt.com>" \
  --app msbutt-secrets-api
```

Port 587 matters: the API uses Go's `smtp.SendMail`, which upgrades to TLS with STARTTLS and
refuses to send credentials over an unencrypted connection. Implicit TLS on port 465 would not
work without a code change.

The `SMTP_FROM` address must be on the domain you verified, or Resend will reject the message.

## 5. Check it (me, once the above exists)

```fish
./scripts/check-deployment.sh https://devops.msbutt.com
```

It checks TLS and HSTS, the security headers, `/health` and its schema version, that API
responses are not cached and carry a request ID, that the console asks not to be indexed, that a
wrong login is refused without setting a cookie, and that no verification link ever appears in a
response. Then, by hand: register an account, receive the real email, verify, log in, create a
vault and reveal a secret, and confirm the reveal shows your own IP in the audit log.

## 6. Afterwards

- **Decide what this instance is.** If it is a portfolio demo, seed it, turn on the banner
  (`VITE_DEMO_BANNER` at build time) and reset it nightly, as in
  [operations.md](operations.md#a-demo-deployment). If it holds anything real, take backups:
  Neon keeps its own point-in-time history, and `scripts/backup.sh` can pull dumps somewhere you
  control.
- **Consider Cloudflare Access** in front of `devops.msbutt.com` while you are the only user: it
  puts a login in front of the whole console before any request reaches the app.
- **Keep the free tiers honest.** Fly machines suspending, Neon's storage limit and Resend's
  monthly allowance are the three ceilings; `/health` and the Resend dashboard will tell you
  first.

## What it costs

Fly's suspended machines, Neon's free project and Resend's free allowance cover an instance of
this size, but all three ask for a card and all three change their terms. Check current pricing
before you rely on it; nothing in this repository spends money on its own.

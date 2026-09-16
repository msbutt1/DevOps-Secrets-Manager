# Hosting it at devops.msbutt.com

The concrete setup for this deployment: Cloudflare Pages serves the console at
`devops.msbutt.com` and proxies `/api` to the Go API on Render, which talks to PostgreSQL on Neon
and sends mail through Resend. [deployment.md](deployment.md) covers the general case; this is the
runbook for ours, with the steps that need your accounts marked **you**.

```
browser ──> Cloudflare (DNS + Pages, devops.msbutt.com)
              ├── static console
              └── /api/* ──> Render (Go API) ──> Neon (PostgreSQL)
                                   └─────────> Resend (SMTP, 587)
```

Everything in the repository is ready: `render.yaml`, the Pages Function in
`apps/web/functions/api/[[path]].ts`, `_headers` and `_redirects` in `apps/web/public`, the
deploy workflow in `.github/workflows/deploy.yml`, and `scripts/check-deployment.sh` to check the
result from outside. `fly.toml` is kept as an alternative; the Fly route is in
[deployment.md](deployment.md#flyio--neon--cloudflare-pages-free-tiers).

## Why Render's free plan

The free plan **cannot bill you**. Over its limits Render stops or throttles the service instead
of charging, so a burst of traffic — someone hammering registration, a scraper, a bad day —
costs an outage rather than money. No card needs to be on file, which is the only guarantee that
actually holds; budget alerts elsewhere notify after the fact, they do not stop the meter.

What you pay instead is a **cold start of roughly 50 seconds** after 15 minutes idle. Step 6
keeps the service warm so the first visitor does not meet a blank screen.

## 1. Database — Neon (**you**)

Create a project on the free tier, PostgreSQL 17, in a US East region to match `region: virginia`
in `render.yaml`.

Neon shows the connection string as soon as the project exists: it is the **Connect** button at
the top right of the project dashboard, or **Dashboard → Connection string**. Choose the
**psql** or **Parameters only** tab; either gives you the same URI. Copy the whole thing — it
already contains the user, the password, the host and the database name:

```
postgresql://neondb_owner:npg_xxxxxxxx@ep-cool-name-a1b2c3.us-east-1.aws.neon.tech/neondb?sslmode=require
```

That single string is all the API needs: it reads `DATABASE_URL` and prefers it over the
separate `APP_DATABASE_*` variables. **Treat it as a password** — it contains one.

Two things to get right in Neon's connection widget:

- **Pick the direct endpoint, not the pooled one.** If the host ends in `-pooler`, switch the
  *Connection pooling* toggle off. Migrations take a session-level advisory lock, and Neon's
  pooler runs in transaction mode, where that lock does not hold.
- **Keep `?sslmode=require`.** Neon refuses unencrypted connections, and with `DATABASE_URL`
  set, `APP_DATABASE_SSLMODE` is not consulted — the URL carries it.

The API creates its own schema on first start, so there is nothing to run against the database
by hand.

Neon's free tier suspends an idle database and wakes it in a second or so. It does not expire,
unlike Render's own free PostgreSQL, which is deleted after 30 days — that is why the database
lives here and not on Render.

## 2. API — Render (**you**, then me)

Generate the three secrets first and **write `MASTER_KEK` down somewhere safe before you
deploy**. It lives only in Render's secret store, and without it every stored secret is
unreadable, including from a backup.

```fish
openssl rand -hex 32   # MASTER_KEK
openssl rand -hex 32   # APP_JWT_SECRET      (must differ from MASTER_KEK; the API checks)
openssl rand -hex 32   # APP_EDGE_TOKEN      (also goes into Pages as EDGE_TOKEN, step 3)
```

In the Render dashboard: **New → Blueprint**, point it at the GitHub repository, and it reads
`render.yaml`. Render then asks for every variable marked `sync: false`:

| Variable | Value |
|---|---|
| `MASTER_KEK`, `APP_JWT_SECRET`, `APP_EDGE_TOKEN` | the three above |
| `DATABASE_URL` | the whole Neon connection string from step 1 |
| `SMTP_*` | left blank for now; step 4 fills them in |

`render.yaml` already sets `APP_ENV=production`, `APP_PUBLIC_URL`,
`APP_CLIENT_IP_HEADER=CF-Connecting-IP` and the health check on `/health`.
It deliberately does not set `APP_SERVER_PORT`: Render sets `PORT`, and the API prefers it.

Watch the first deploy's logs. They say whether the migrations ran and whether mail can be sent
at all — `Email delivery configured` with the host and port, or an error naming the missing
variables. Then check the service's own URL answers:

```fish
curl -s https://msbutt-secrets-api.onrender.com/health | jq
```

`/health` is the one route exempt from the edge token, so this works before Pages exists.
Everything else on that hostname answers `404` until step 3, which is the point.

## 3. Console — Cloudflare Pages (**you**, then me)

Create a Pages project connected to the GitHub repository.

| Setting | Value |
|---|---|
| Framework preset | None |
| Root directory | `apps/web` |
| Build command | `npm ci && npm run build` |
| Build output directory | `dist` |
| Environment variable | `API_ORIGIN = https://msbutt-secrets-api.onrender.com` |
| Environment **secret** | `EDGE_TOKEN` = the same value as `APP_EDGE_TOKEN` |

The build output directory is resolved inside the root directory, so it is `dist` and not
`apps/web/dist`. If a build fails with "output directory not found", that pair is why. The
Pages Function is found the same way: `functions/` has to sit under the root directory, which
is why it lives at `apps/web/functions`.

Then add the custom domain `devops.msbutt.com` in the Pages project; Cloudflare creates the CNAME
in the `msbutt.com` zone for you. Keep the record proxied (orange cloud).

The Pages Function forwards `/api/*` to Render, copies the visitor's address into `X-Client-IP`
so the audit log records them rather than Cloudflare, and adds `X-Edge-Token`.

It cannot simply pass `CF-Connecting-IP` through. Cloudflare manages that header on outgoing
subrequests and replaces whatever a Worker sets with the Worker's own egress address, so the
API saw `162.x` for every visitor. The function reads `CF-Connecting-IP` from the incoming
request, which is trustworthy, and writes it to a header Cloudflare leaves alone; Render sets
`APP_CLIENT_IP_HEADER=X-Client-IP` to match, and both headers are stripped from the incoming
request so a client cannot supply either. Because the
console and the API share one origin, the refresh cookie stays `SameSite=Strict` and there is no
CORS to configure — `APP_CORS_ALLOWED_ORIGINS` stays empty.

**Why the edge token matters.** `msbutt-secrets-api.onrender.com` stays publicly reachable no
matter what DNS says about `devops.msbutt.com`. Without a shared secret, anyone could call it
directly and skip Cloudflare's rate limiting and WAF, and forge `CF-Connecting-IP` into the audit
log and the per-IP limits. With it, only this proxy can reach the API.

One consequence: **the CLI must go through the public hostname too.** Use
`--api-url https://devops.msbutt.com/api`, not the Render hostname.

## 4. Email — Resend (**you**, then me)

1. Create a Resend account and add the domain `msbutt.com` (or `mail.msbutt.com` if you would
   rather keep the apex clean).
2. Resend shows DNS records to add. In the Cloudflare DNS tab for `msbutt.com`, add them exactly
   as given — typically:
   - a TXT record for SPF (`v=spf1 include:amazonses.com ~all` or as Resend states)
   - one or more CNAME records for DKIM (`resend._domainkey`, …) — **DNS only, grey cloud**
   - optionally a DMARC TXT record at `_dmarc`: `v=DMARC1; p=none; rua=mailto:you@msbutt.com`
3. Wait for Resend to verify the domain, then create an API key.
4. Fill in the `SMTP_*` variables on the Render service and let it redeploy:

| Variable | Value |
|---|---|
| `SMTP_HOST` | `smtp.resend.com` |
| `SMTP_PORT` | `2587` |
| `SMTP_USER` | `resend` |
| `SMTP_PASSWORD` | `re_your_api_key` |
| `SMTP_FROM` | `Vault Console <noreply@msbutt.com>` |

**Port 2587, not 587.** Render blocks outbound SMTP on the standard ports (25, 465 and 587) to
keep spam off its free instances, and the block is a silent drop: the API waits and then logs
`dial tcp …:587: connect: connection timed out`, long after the browser was told the account
was created. Resend also listens on 2465 and 2587 for exactly this case. Use 2587, which is
STARTTLS like 587 — the API uses Go's `smtp.SendMail`, which upgrades with STARTTLS and refuses
to send credentials in the clear, so implicit TLS on 465 or 2465 would need a code change.

If a platform blocks the alternative ports too, SMTP is the wrong transport there and Resend's
HTTPS API is the way out; that is not implemented here.

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

Also confirm the bypass is closed, which the script cannot do because it only knows the public
hostname:

```fish
curl -s -o /dev/null -w '%{http_code}\n' https://msbutt-secrets-api.onrender.com/vaults   # 404
curl -s -o /dev/null -w '%{http_code}\n' https://msbutt-secrets-api.onrender.com/health   # 200
```

## 6. Keeping it warm (**you**)

A free Render service sleeps after 15 minutes idle and takes roughly 50 seconds to answer the
next request. A ping every 10 minutes keeps it awake, and doubles as the uptime monitor
[operations.md](operations.md#monitoring) asks for.

Use a free external monitor rather than a GitHub Actions cron: scheduled workflows are delayed by
tens of minutes under load and are disabled after 60 days without a push, which is exactly when
you would stop noticing.

| Service | Free interval | Card |
|---|---|---|
| cron-job.org | 1 minute | no |
| UptimeRobot | 5 minutes | no |
| Better Stack | 3 minutes | no |

Point it at `https://devops.msbutt.com/api/health` every **10 minutes**, expect HTTP 200, and
alert on two consecutive failures. Use the public hostname, not the Render one: that way the ping
also proves Cloudflare, the Pages Function and the API are all still working, rather than only
the last of the three.

Two things to know. The ping keeps Render awake but Neon still suspends, so the first query after
a quiet night adds a second — `/health` touches the database, so the ping keeps Neon warm too.
And Render's free plan has a monthly instance-hour allowance; one always-awake service fits
inside it, a second one may not.

## 7. Afterwards

- **Decide what this instance is.** If it is a portfolio demo, seed it, turn on the banner
  (`VITE_DEMO_BANNER` at build time) and reset it nightly, as in
  [operations.md](operations.md#a-demo-deployment). If it holds anything real, take backups:
  Neon keeps its own point-in-time history, and `scripts/backup.sh` can pull dumps somewhere you
  control.
- **Close signups if you are the only user.** Cloudflare Access in front of `devops.msbutt.com`
  puts a login before any request reaches the app. Short of that, Cloudflare rate limiting rules
  on `/api/auth/*` cost nothing and stop the spam that would otherwise burn your Resend
  allowance and your domain's sending reputation.
- **Keep the free tiers honest.** Render's instance hours, Neon's storage limit and Resend's
  monthly allowance are the three ceilings; `/health` and the Resend dashboard will tell you
  first.

## What it costs

Nothing, by design: Render's free plan throttles rather than bills, Neon's free project does not
expire, and Resend's free allowance covers an instance of this size. All three can change their
terms, so check before you rely on it. Nothing in this repository spends money on its own, and
no card needs to be on file for any of it.

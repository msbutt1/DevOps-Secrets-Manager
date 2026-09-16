# Changelog

All notable changes to this project are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project uses
[semantic versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

Hosting the instance at `devops.msbutt.com` on Render, Neon, Resend and Cloudflare Pages.

### Added

- `render.yaml`, a Render blueprint for the API, and a runbook for the deployment in
  `docs/hosting-devops-msbutt-com.md`, including keeping a free instance warm
- The API listens on `PORT` when the platform sets one, after `APP_SERVER_PORT`

### Security

- `APP_EDGE_TOKEN`: behind a CDN, requests must carry the shared secret in `X-Edge-Token` or be
  answered `404`. The hosting platform's own hostname stays publicly reachable, so without it
  the CDN's rate limits and its `CF-Connecting-IP` header could be skipped entirely
- `APP_CLIENT_IP_HEADER`: the audit log and per-IP rate limits can be told to read a
  single-address header written by the edge, instead of `X-Forwarded-For`, which a client can
  prepend a fake hop to

## [1.0.0] - 2026-09-16

First release: the API, web console and CLI are feature complete, tested and documented.

### Added

**Secrets**
- Version history for every secret, with reveal and rollback of earlier values (`secret.restored`)
- Import and export a whole environment as a `.env` file, with a dry-run preview; the format
  round-trips with the CLI
- Copy chosen keys from one environment to another in the same vault
- Search key names across every vault the caller can read
- Expiry and rotation due dates shown as badges in the web app and warnings in the CLI

**Access and accounts**
- Organizations with email invitations, per-vault membership and five roles
- Service tokens scoped to a single environment for CI, stored as hashes and audited as
  `token:<name>`
- Sessions list with sign-out per device and "sign out everywhere"
- Password reset by email, resend verification, and a password policy checked against the
  10,000 most common passwords
- Dashboard built from real counts, with alerts for expired, expiring and overdue secrets

**Keys**
- Master key rotation: vaults record the key version that wrapped their data key, old keys stay
  configured during a rotation, and `make rotate-kek` re-wraps every data key
- Per-vault data key rotation that re-encrypts the vault's values and their history in one
  transaction

**Operations**
- Structured JSON logs with a request ID per request and no credentials in them
- Rate limits on authentication, reveal and token endpoints, plus account lockout with backoff
- Security headers on the API and nginx, configurable CORS allowlist and trusted proxies
- Dependency scanning (govulncheck, npm audit, cargo audit) on every push and weekly, Dependabot
- `scripts/backup.sh` for rotating database dumps, with a tested restore procedure
- Deployment, operations and security documentation: `docs/deployment.md`,
  `docs/operations.md`, `SECURITY.md`

### Fixed

- Viewers could reveal secret values: permission checks used the organization role instead of
  the effective vault role
- Editing a secret's description wiped its value, because the update always re-encrypted the
  submitted (empty) value
- Registration was not atomic: a failure left an account with no organization that could never
  register again
- Email addresses were case-sensitive, so `Ada@x.test` and `ada@x.test` were two accounts
- Registration accepted any password, including a single character
- Changing a password left every other session signed in
- The web app kept its refresh token in memory, so a page reload signed the user out; token
  refresh also failed silently because it read a snake_case response as camelCase
- The CLI wrote its token file with default permissions when no keyring was available, and
  called the base64 file "encrypted"
- `secrets logout` did not revoke the session on the server, and `secrets pull` wrote
  world-readable `.env` files
- The audit log showed empty vault and user columns, and its date filter used UTC instead of the
  viewer's day
- The dashboard showed invented numbers
- The bulk delete button logged to the console instead of deleting; the vault Edit button did
  nothing; environments could not be deleted at all
- Labels on secrets were renamed by the camelCase transform
- Accessibility: unlabelled inputs and icon buttons, text below 4.5:1 contrast, and dialogs that
  could not be closed or escaped from the keyboard
- Every page scrolled sideways below about 730px

### Security

- Refresh tokens for the web app moved into an `HttpOnly`, `SameSite=Strict` cookie
- Access tokens carry and validate issuer, audience, expiry and token type, and pin HS256
- Updated dependencies with known vulnerabilities: pgx (SQL injection through placeholder
  confusion), x/text, chi, x/crypto, the Go standard library, React Router (open redirect),
  Vite, Vitest and reqwest (rustls-webpki, h2)

### Known limitations

- No multi-factor authentication or single sign-on
- Keys come from environment variables; there is no cloud KMS or HSM integration
- Docker images are defined and validated but were never built locally (no Docker daemon
  available on the development machine); CI builds them on every push
- macOS and Windows CLI binaries are built by the release workflow and have not been run by hand

[1.0.0]: https://github.com/msbutt1/DevOps-Secrets-Manager/releases/tag/v1.0.0

# DevOps Secrets Manager

A complete secrets management platform for DevOps teams featuring end-to-end encryption, role-based access control, and seamless CI/CD integration.

## Architecture

```
+------------------+     +------------------+     +------------------+
|                  |     |                  |     |                  |
|   React Web UI   |---->|    Go REST API   |---->|   PostgreSQL DB  |
|   (TypeScript)   |     |   (Chi Router)   |     |   (Encrypted)    |
|                  |     |                  |     |                  |
+------------------+     +------------------+     +------------------+
                               ^
                               |
+------------------+           |
|                  |           |
|    Rust CLI      |-----------+
|   (secrets)      |
|                  |
+------------------+
```

## Features

- **End-to-End Encryption**: AES-256-GCM envelope encryption with master KEK and per-vault DEKs
- **Role-Based Access Control**: 5 roles (Owner, Admin, Developer, Oncall, Viewer) with granular permissions
- **Vault-Level Membership**: Fine-grained access control per vault, not just organization-wide
- **Audit Log**: Every reveal and change is recorded with who, where and when
- **Environment Injection**: Run any command with secrets injected as environment variables
- **JWT Authentication**: HS256-signed access tokens with refresh token rotation; the web app's refresh token lives in an `HttpOnly`, `SameSite=Strict` cookie that page scripts cannot read
- **Multi-Environment Support**: Organize secrets by environment (dev, staging, production)

## Tech Stack

| Component | Technology | Purpose |
|-----------|------------|---------|
| **API** | Go 1.21+, Chi Router | REST API server |
| **Web** | React 18, TypeScript, Vite | User interface |
| **CLI** | Rust 1.70+ | Command-line tool |
| **Database** | PostgreSQL 17 | Persistent storage |
| **Auth** | JWT (HS256) | Authentication |
| **Encryption** | AES-256-GCM | Secret encryption |

## Quick Start

### Local development (no Docker)

Requires Go 1.25, Node.js 22, PostgreSQL 17 client and server binaries (`pg_ctl`, `initdb`,
`psql`) and, for the CLI, Rust.

```bash
git clone https://github.com/msbutt1/DevOps-Secrets-Manager.git
cd DevOps-Secrets-Manager
make db && make migrate-up && make seed && make dev
```

- `make db` initialises and starts PostgreSQL on port 5433 under `~/.local/share/devops-secrets-manager`
- `make migrate-up` writes `apps/api/.env` with generated keys (if missing) and applies migrations
- `make seed` creates demo users, vaults, environments and secrets through the API
- `make dev` runs the API on http://localhost:8080 and the web app on http://localhost:5173

Log in as `salaar@demo.dev` with the password `Demo-Passw0rd!2026`. `make help` lists every
target, including `make test`, `make lint` and `make cli`.

### Docker Compose

```bash
cp .env.example .env
# Replace MASTER_KEK and JWT_SECRET in .env; generate each with: openssl rand -hex 32
docker compose up --build -d
```

The web app is served on http://localhost:3000 and the API on http://localhost:8080. With
`APP_ENV=development` and no SMTP settings, the verification link for a new account is printed
in the API logs (`docker compose logs api`).

### CLI

```bash
make cli                       # builds apps/cli/target/release/secrets
cargo install --path apps/cli  # optional: puts `secrets` on your PATH
```

## User Guide

### Web Interface

1. **Register and verify**: Create an account at `http://localhost:5173` and open the verification link (logged by the API in development)
2. **Organization**: Registration creates your own organization, where you are the owner
3. **Create a vault**: Organize secrets by project or service, then add environments such as `staging` or `production`
4. **Add secrets**: Store key-value pairs with optional description, rotation interval, expiry and labels. Expired and soon-to-expire secrets are flagged in the vault, on reveal, on the dashboard, and by `secrets run`, `pull` and `list`; expired values still work so a deploy is not broken without warning
5. **Manage members**: Give people in your organization a role on the vault

### CLI Commands

The CLI talks to `http://localhost:8080` by default. Point it elsewhere with `--api-url`, the
`SECRETS_API_URL` variable, or `secrets login --api-url https://secrets.example.com`, which
remembers the URL. Sessions are tied to the API that issued them and are never sent to another URL.

```bash
# Authenticate
secrets login

# List your vaults
secrets vault list

# List a vault's environments
secrets env list --vault my-app

# Pull secrets to a .env file
secrets pull --vault my-app --env production --out .env

# Run a command with injected secrets (exits with the command's code; --mask hides values it prints)
secrets run --vault my-app --env production -- npm start

# Create a secret, or update its value if the key exists (--create-only / --update-only to restrict)
secrets set "API_KEY=sk-123" --vault my-app --env production

# View audit log
secrets audit --vault my-app --since 7d
```

### CI/CD

Service tokens give pipelines read-only access to one environment: create one from the vault
page, store it as `SECRETS_TOKEN`, and run `secrets run -- ./deploy.sh`. See
[docs/ci-github-actions.md](docs/ci-github-actions.md) for a GitHub Actions workflow.

### API Endpoints

The API has no path prefix; the web app reaches it through `/api` (proxied by Vite in
development and nginx in Docker). `docs/openapi.yaml` describes every endpoint.

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/auth/register` | Register (sends a verification email) |
| POST | `/auth/verify-email` | Verify an email address |
| POST | `/auth/resend-verification` | Email a new verification link |
| POST | `/auth/forgot-password` | Email a password reset link |
| POST | `/auth/reset-password` | Set a new password with a reset link (signs out all sessions) |
| POST | `/auth/login` | Log in |
| POST | `/auth/refresh` | Exchange a refresh token for new tokens |
| POST | `/auth/logout` | Revoke a refresh token |
| GET | `/auth/me` | Current user and organizations |
| POST | `/auth/change-password` | Change password (signs out other sessions) |
| GET | `/auth/sessions` | List your signed-in sessions |
| DELETE | `/auth/sessions/{id}` | Sign out one session |
| POST | `/auth/sessions/revoke-others` | Sign out every other session |
| GET, POST | `/vaults` | List or create vaults |
| GET, PUT, DELETE | `/vaults/{id}` | Read, rename or delete a vault |
| POST | `/vaults/{id}/rotate-key` | Replace the vault's data key and re-encrypt its secrets |
| GET, POST | `/vaults/{id}/envs` | List or create environments |
| GET, PUT, DELETE | `/envs/{id}` | Read, rename or delete an environment |
| GET, POST | `/envs/{id}/secrets` | List secret metadata or create a secret |
| PUT, DELETE | `/secrets/{id}` | Update or delete a secret |
| POST | `/secrets/{id}/reveal` | Decrypt a secret value (audited) |
| POST | `/envs/{id}/export` | Decrypt a whole environment for download (audited) |
| POST | `/envs/{id}/import` | Create or update secrets from `.env` pairs, with a dry run |
| POST | `/envs/{id}/copy-from` | Copy keys from another environment of the same vault |
| GET | `/secrets/{id}/versions` | List a secret's earlier values (without the values) |
| POST | `/secrets/{id}/versions/{version}/reveal` | Reveal an earlier value (audited) |
| POST | `/secrets/{id}/versions/{version}/restore` | Roll back to an earlier value as a new version |
| GET, POST | `/vaults/{id}/members` | List or add vault members |
| PUT, DELETE | `/vaults/{id}/members/{userId}` | Change a member's role or remove them |
| GET | `/audit` | Paginated, filterable audit log |
| GET | `/search` | Find secrets by key name across accessible vaults |
| GET | `/stats`, `/alerts` | Dashboard counts and alerts |
| GET | `/health` | Database, schema version and uptime |

## Security Model

The threat model, operating advice, key rotation procedures and how to report a vulnerability
are in [SECURITY.md](SECURITY.md).

### Encryption Architecture

```
Master KEK (environment variable)
    |
    v
[Encrypt/Decrypt]
    |
    v
Vault DEK (per vault, stored encrypted)
    |
    v
[Encrypt/Decrypt]
    |
    v
Secret Values (stored encrypted)
```

- **Master KEK**: 32-byte key stored as environment variable
- **Vault DEK**: Generated per-vault, encrypted with Master KEK
- **Secrets**: Encrypted with Vault DEK using AES-256-GCM

### Rotating the Master Key

Each vault records the master key version that wrapped its data key (`kek_version`), so the
master key can be replaced without downtime and without re-encrypting secret values:

1. Generate a new key: `openssl rand -hex 32`.
2. Move the current key to `MASTER_KEK_PREVIOUS` as `1:<old key>`, put the new key in
   `MASTER_KEK`, set `MASTER_KEK_VERSION=2`, and restart the API. New vaults use version 2;
   existing ones are still read with version 1.
3. Run `make rotate-kek` (or `keys rotate-kek` in the API image). It re-wraps every vault's data
   key, including deleted vaults, one vault per transaction; it is safe to run again if interrupted.
4. When `make keys-status` shows no vault on version 1, remove it from `MASTER_KEK_PREVIOUS` and
   restart.

The API refuses to start if any vault uses a version that is not configured, so a key cannot be
dropped too early.

### Rotating a Vault's Data Key

If a vault's data may have been exposed (for example a database backup leaked together with the
master key), an owner or admin can give the vault a new data key with
`POST /vaults/{id}/rotate-key`. Every secret value in the vault, including deleted ones, is
re-encrypted in one transaction and the old key is discarded. Writes that race with the rotation
wait for it and retry under the new key. The rotation is recorded as `vault.key_rotated`.

### Sessions and Tokens

Access tokens are JWTs signed with HS256 and live for 15 minutes. HS256 was chosen over RS256
because only this API issues and checks the tokens: there is no second service that needs a
public key, and a single secret is simpler to manage. The secret must be at least 32 random
characters and differ from `MASTER_KEK`. Each token carries `iss=devops-secrets-manager` and
`aud=devops-secrets-manager-api`, and the API rejects tokens with a different issuer or
audience, a missing expiry, or any algorithm other than HS256. If tokens ever need to be checked
by other services, switching to RS256 or EdDSA with published keys is the upgrade path.

Refresh tokens are random values stored only as hashes and rotated on every use. The CLI keeps
its tokens in the operating system keyring; the web app keeps the access token in memory and the refresh token
in an `HttpOnly`, `SameSite=Strict` cookie. Changing `APP_JWT_SECRET` signs everyone out of
their access tokens; they get new ones from their refresh token.

Passwords must be 12 to 72 bytes and are checked against the 10,000 most common passwords
(also with digits or symbols added and letters swapped for digits), simple sequences, and the
user's own name and email. There are no composition rules, following NIST SP 800-63B; the
register and change-password forms show a strength hint while typing.

### Role Permissions

Roles are set per vault. Organization owners and admins have that role on every vault in the
organization; everyone else needs to be added to a vault.

| Permission | Owner | Admin | Developer | Oncall | Viewer |
|------------|-------|-------|-----------|--------|--------|
| See vaults, environments, secret names and members | Yes | Yes | Yes | Yes | Yes |
| Create, update and delete secrets and environments | Yes | Yes | Yes | No | No |
| Reveal secret values | Yes | Yes | No | Yes | No |
| Manage members and read the vault's audit log | Yes | Yes | No | No | No |
| Delete the vault | Yes | No | No | No | No |

Only owners can grant or remove the owner role, and a vault always keeps at least one owner.

## Configuration

### Environment Variables

The API reads an optional `.env` file in its working directory, an optional `config.yaml`,
and environment variables. Nested settings use the `APP_` prefix (`database.host` becomes
`APP_DATABASE_HOST`). `make env` writes a working `apps/api/.env` for local development.

| Variable | Default | Purpose |
|----------|---------|---------|
| `MASTER_KEK` | (required) | 64 hex characters (32 bytes); wraps every vault's data key |
| `MASTER_KEK_VERSION` | `1` | Version number of `MASTER_KEK`; raise it when rotating the master key |
| `MASTER_KEK_PREVIOUS` | none | During a rotation, earlier keys as `version:hexkey`, comma-separated |
| `APP_JWT_SECRET` | (required) | HS256 signing secret, at least 32 characters, different from `MASTER_KEK` |
| `APP_DATABASE_HOST` | `localhost` | PostgreSQL host |
| `APP_DATABASE_PORT` | `5432` | PostgreSQL port (`make db` uses 5433) |
| `APP_DATABASE_USER` / `APP_DATABASE_PASSWORD` | | Database credentials |
| `APP_DATABASE_NAME` | | Database name |
| `APP_DATABASE_SSLMODE` | `disable` | `require` or stricter for hosted databases |
| `APP_DATABASE_MIGRATIONS_PATH` | `migrations` | Directory of SQL migrations, relative to the working directory |
| `DATABASE_URL` | | Full connection string; overrides the `APP_DATABASE_*` settings |
| `APP_SERVER_PORT` | `8080` | HTTP port |
| `APP_JWT_ACCESS_TOKEN_TTL` | `15m` | Access token lifetime |
| `APP_JWT_REFRESH_TOKEN_TTL` | `168h` | Refresh token lifetime |
| `APP_ENV` | `production` | `development` enables local-only fallbacks (see below) and drops the `Secure` flag from the refresh cookie so it works over plain HTTP |
| `APP_PUBLIC_URL` | `http://localhost:5173` | Web app address used in emailed links |
| `APP_REVEAL_AUTO_HIDE_SECONDS` | `30` | How long the web app shows a revealed value (5-600) |
| `APP_CORS_ALLOWED_ORIGINS` | none | Comma-separated web app origins allowed to call the API from another origin (not needed with the `/api` proxy) |
| `APP_TRUSTED_PROXIES` | loopback and private ranges | Comma-separated CIDRs whose `X-Forwarded-For` is trusted when working out client IPs for rate limits and the audit log |
| `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASSWORD`, `SMTP_FROM` | | Outgoing email |

The API refuses to start when `MASTER_KEK` or `APP_JWT_SECRET` is missing, too short, a
placeholder such as `change-me`, or a value that was ever published as an example.

Docker Compose reads `.env` in the repository root (copy `.env.example`), where the keys are
named `MASTER_KEK` and `JWT_SECRET` and the database settings `DATABASE_USER`,
`DATABASE_PASSWORD` and `DATABASE_NAME`; `docker-compose.yml` maps them to the variables above.

### Logs

The API writes JSON logs to stdout, one object per line. Each request gets an ID, returned in the
`X-Request-ID` response header (a well-formed incoming `X-Request-ID` from a load balancer is
kept), and every line logged while handling the request carries `request_id` and, once
authenticated, `user_id`. Access log lines record the method, path, route, status, size,
duration and client IP. Headers, query strings and bodies are never logged, so tokens,
passwords and secret values stay out of the logs; `TestLogsNeverContainCredentials` checks this.
The one exception is the development-only email fallback below.

### Email in Development

Registration sends a verification link, and login is blocked until the address is verified.
When `SMTP_HOST` is not set and `APP_ENV=development`, the API does not send mail; it logs
the link instead (look for `SMTP is not configured; email not sent` in the API output) so you
can open it locally. `APP_PUBLIC_URL` sets the web app address used in the link. The fallback
only runs when `APP_ENV=development` (set by `make env` and `.env.example`; the server
defaults to `production`). Without SMTP, a production server reports an error instead of
logging the link, because the link is a one-time credential.

### Generate Encryption Key

```bash
# Generate a secure 32-byte key
openssl rand -hex 32
```

## Development

### Running Tests

```bash
make test       # Go, web and CLI tests
make lint       # gofmt, go vet, ESLint, Prettier, TypeScript, rustfmt and clippy
make build      # API, web bundle and CLI release binary
make audit      # govulncheck, npm audit and cargo audit
```

Go integration tests create a throwaway database per test on the server named by
`TEST_DATABASE_URL` (the Makefile points it at the local PostgreSQL from `make db`) and are
skipped when it is not set. CI runs the same targets.

The `Dependency scan` workflow runs the three scanners on every push and weekly, so newly
published advisories fail a run even without code changes. `govulncheck` checks the standard
library of the Go toolchain it runs with, so run it with the version in `go.mod`'s `toolchain`
line (`GOTOOLCHAIN=go1.25.14 make audit-api`). Dependabot opens weekly update pull requests for
Go modules, npm packages, crates, GitHub Actions and Docker base images.

## Deployment

### Fly.io + Neon (Free Tier)

1. **Create Neon Database**
   - Sign up at neon.tech
   - Create PostgreSQL 17 database
   - Copy connection string

2. **Deploy API to Fly.io**
   ```bash
   cd apps/api
   fly launch
   fly secrets set DATABASE_URL="postgresql://..."
   fly secrets set APP_JWT_SECRET="$(openssl rand -hex 32)"
   fly secrets set MASTER_KEK="$(openssl rand -hex 32)"
   fly deploy
   ```

3. **Deploy Web to Cloudflare Pages**
   ```bash
   cd apps/web
   npm run build
   # Connect GitHub repo to Cloudflare Pages
   # Set VITE_API_BASE_URL to your Fly.io API URL
   ```

## Project Structure

```
devops-secrets-manager/
|-- apps/
|   |-- api/                 # Go REST API
|   |   |-- cmd/             # server, migrate and seed commands
|   |   |-- internal/        # Business logic
|   |   |   |-- http/        # HTTP handlers
|   |   |   |-- policy/      # RBAC policies
|   |   |   |-- storage/     # Database layer
|   |   |   |-- crypto/      # Envelope encryption, JWT, key checks
|   |   |   |-- audit/       # Audit log
|   |   |-- migrations/      # SQL migrations
|   |
|   |-- web/                 # React frontend
|   |   |-- src/
|   |   |   |-- components/  # UI components
|   |   |   |-- pages/       # Route pages
|   |   |   |-- hooks/       # Custom hooks
|   |   |   |-- lib/         # API client
|   |   |   |-- contexts/    # React contexts
|   |
|   |-- cli/                 # Rust CLI
|       |-- src/
|           |-- cli/         # Command implementations
|           |-- api/         # API client
|           |-- config/      # Token storage
|
|-- Makefile                 # Local development and checks
|-- docker-compose.yml       # Containerised stack
|-- .env.example             # Environment template
```

## Troubleshooting

### Common Issues

**Port already in use**
Compose publishes PostgreSQL on 5433, the API on 8080 and the web app on 3000; `make db` also
uses 5433, so stop one before starting the other (`make db-stop`).

**Database fails to start after upgrading**
Compose now uses PostgreSQL 17. A `postgres_data` volume created by the old PostgreSQL 15 image cannot be opened by 17: dump it first (`docker compose exec db pg_dumpall -U secrets_user > backup.sql` with the old image), or, for throwaway local data, remove it with `docker compose down -v`.

**Database connection failed**
```bash
# Check if PostgreSQL is running
docker compose ps

# Check logs
docker compose logs db
```

**JWT token expired**
```bash
# CLI will auto-refresh, or re-login
secrets logout
secrets login
```

**Cannot reveal secrets**
- Only owners, admins and on-call members can reveal values; developers can write but not read them
- Check your role on that vault (the Members page shows it)

## Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/my-feature`
3. Commit changes: `git commit -m 'feat: add my feature'`
4. Push to branch: `git push origin feature/my-feature`
5. Open a Pull Request

### Commit Convention

- `feat:` New feature
- `fix:` Bug fix
- `docs:` Documentation
- `refactor:` Code refactoring
- `test:` Adding tests
- `chore:` Maintenance

## License

MIT License - see LICENSE file for details.

---

Built with Go, React, and Rust.

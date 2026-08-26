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
- **JWT Authentication**: HS256-signed access tokens with refresh token rotation
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
4. **Add secrets**: Store key-value pairs with optional description, rotation interval, expiry and labels
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

### API Endpoints

The API has no path prefix; the web app reaches it through `/api` (proxied by Vite in
development and nginx in Docker). `docs/openapi.yaml` describes every endpoint.

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/auth/register` | Register (sends a verification email) |
| POST | `/auth/verify-email` | Verify an email address |
| POST | `/auth/login` | Log in |
| POST | `/auth/refresh` | Exchange a refresh token for new tokens |
| POST | `/auth/logout` | Revoke a refresh token |
| GET | `/auth/me` | Current user and organizations |
| POST | `/auth/change-password` | Change password |
| GET, POST | `/vaults` | List or create vaults |
| GET, PUT, DELETE | `/vaults/{id}` | Read, rename or delete a vault |
| GET, POST | `/vaults/{id}/envs` | List or create environments |
| GET, PUT, DELETE | `/envs/{id}` | Read, rename or delete an environment |
| GET, POST | `/envs/{id}/secrets` | List secret metadata or create a secret |
| PUT, DELETE | `/secrets/{id}` | Update or delete a secret |
| POST | `/secrets/{id}/reveal` | Decrypt a secret value (audited) |
| GET, POST | `/vaults/{id}/members` | List or add vault members |
| PUT, DELETE | `/vaults/{id}/members/{userId}` | Change a member's role or remove them |
| GET | `/audit` | Paginated, filterable audit log |
| GET | `/stats`, `/alerts` | Dashboard counts and alerts |
| GET | `/health` | Database, schema version and uptime |

## Security Model

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
| `APP_ENV` | `production` | `development` enables local-only fallbacks (see below) |
| `APP_PUBLIC_URL` | `http://localhost:5173` | Web app address used in emailed links |
| `APP_REVEAL_AUTO_HIDE_SECONDS` | `30` | How long the web app shows a revealed value (5-600) |
| `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASSWORD`, `SMTP_FROM` | | Outgoing email |

The API refuses to start when `MASTER_KEK` or `APP_JWT_SECRET` is missing, too short, a
placeholder such as `change-me`, or a value that was ever published as an example.

Docker Compose reads `.env` in the repository root (copy `.env.example`), where the keys are
named `MASTER_KEK` and `JWT_SECRET` and the database settings `DATABASE_USER`,
`DATABASE_PASSWORD` and `DATABASE_NAME`; `docker-compose.yml` maps them to the variables above.

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
```

Go integration tests create a throwaway database per test on the server named by
`TEST_DATABASE_URL` (the Makefile points it at the local PostgreSQL from `make db`) and are
skipped when it is not set. CI runs the same targets.

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

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
- **Secret Versioning**: Full history of secret changes with audit trail
- **Environment Injection**: Run any command with secrets injected as environment variables
- **JWT Authentication**: Secure token-based auth with refresh token rotation
- **Multi-Environment Support**: Organize secrets by environment (dev, staging, production)

## Tech Stack

| Component | Technology | Purpose |
|-----------|------------|---------|
| **API** | Go 1.21+, Chi Router | REST API server |
| **Web** | React 18, TypeScript, Vite | User interface |
| **CLI** | Rust 1.70+ | Command-line tool |
| **Database** | PostgreSQL 17 | Persistent storage |
| **Auth** | JWT (RS256) | Authentication |
| **Encryption** | AES-256-GCM | Secret encryption |

## Quick Start

### Using Docker (Recommended)

```bash
# Clone the repository
git clone <repo-url>
cd devops-secrets-manager

# Copy environment file
cp .env.example .env

# Start all services
docker compose up -d

# Access the web UI
open http://localhost:5173

# Register a new account through the web UI
```

### Manual Development Setup

#### Prerequisites

- Go 1.21+
- Node.js 18+
- Rust 1.70+
- PostgreSQL 17 (the version used by Docker Compose and CI)

#### 1. Start the Database

```bash
# Create database
createdb secrets_manager

# The API will run migrations automatically on startup
```

#### 2. Start the API

```bash
cd apps/api

# Copy and configure
cp ../../.env.example .env
# Edit .env with your database credentials

# Run the server
go run cmd/server/main.go
```

#### 3. Start the Web UI

```bash
cd apps/web

# Install dependencies
npm install

# Start development server
npm run dev
```

#### 4. Build the CLI

```bash
cd apps/cli

# Build release binary
cargo build --release

# Install globally (optional)
cargo install --path .
```

## User Guide

### Web Interface

1. **Register/Login**: Create an account at `http://localhost:5173`
2. **Create Organization**: Set up your team's organization
3. **Create Vault**: Organize secrets by project or service
4. **Add Secrets**: Store key-value pairs with optional metadata
5. **Manage Members**: Invite team members with appropriate roles

### CLI Commands

```bash
# Authenticate
secrets login

# List your vaults
secrets vault list

# View secrets in a vault
secrets env list --vault my-app

# Pull secrets to a .env file
secrets pull --vault my-app --env production --out .env

# Run a command with injected secrets
secrets run --vault my-app --env production -- npm start

# Set a secret
secrets set "API_KEY=sk-123" --vault my-app --env production

# View audit log
secrets audit --vault my-app --since 7d
```

### API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/auth/register` | Register new user |
| POST | `/api/auth/login` | Login |
| POST | `/api/auth/refresh` | Refresh token |
| GET | `/api/vaults` | List user's vaults |
| POST | `/api/vaults` | Create vault |
| GET | `/api/vaults/:id` | Get vault details |
| GET | `/api/vaults/:id/secrets` | List secrets |
| POST | `/api/vaults/:id/secrets` | Create secret |
| GET | `/api/vaults/:id/secrets/:secretId/reveal` | Reveal secret value |
| GET | `/api/vaults/:id/members` | List vault members |
| POST | `/api/vaults/:id/members` | Add member |
| DELETE | `/api/vaults/:id/members/:userId` | Remove member |

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

| Permission | Owner | Admin | Developer | Oncall | Viewer |
|------------|-------|-------|-----------|--------|--------|
| View Secrets | Yes | Yes | Yes | Yes | Yes |
| Reveal Values | Yes | Yes | Yes | Yes | No |
| Create Secrets | Yes | Yes | Yes | No | No |
| Update Secrets | Yes | Yes | Yes | No | No |
| Delete Secrets | Yes | Yes | No | No | No |
| Manage Members | Yes | Yes | No | No | No |
| Delete Vault | Yes | No | No | No | No |

## Configuration

### Environment Variables

```bash
# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=secrets_manager
# OR use connection string
DATABASE_URL=postgresql://user:pass@host:5432/dbname

# Security
JWT_SECRET=your-jwt-secret-min-32-chars
APP_ENCRYPTION_MASTER_KEY=32-byte-hex-encoded-key

# Server
SERVER_PORT=8080
```

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
# API tests
cd apps/api
go test ./...

# Web tests
cd apps/web
npm test

# CLI tests
cd apps/cli
cargo test
```

### Code Quality

```bash
# API linting
cd apps/api
go vet ./...
golangci-lint run

# Web linting
cd apps/web
npm run lint

# CLI formatting
cd apps/cli
cargo fmt --check
cargo clippy
```

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
   fly secrets set JWT_SECRET="your-secret"
   fly secrets set APP_ENCRYPTION_MASTER_KEY="your-key"
   fly deploy
   ```

3. **Deploy Web to Cloudflare Pages**
   ```bash
   cd apps/web
   npm run build
   # Connect GitHub repo to Cloudflare Pages
   # Set VITE_API_BASE_URL to your Fly.io API URL
   ```

### Docker Production

```bash
# Build production images
docker compose -f docker-compose.prod.yml build

# Deploy with your orchestrator of choice
```

## Project Structure

```
devops-secrets-manager/
|-- apps/
|   |-- api/                 # Go REST API
|   |   |-- cmd/server/      # Entry point
|   |   |-- internal/        # Business logic
|   |   |   |-- http/        # HTTP handlers
|   |   |   |-- policy/      # RBAC policies
|   |   |   |-- storage/     # Database layer
|   |   |   |-- crypto/      # Encryption
|   |   |   |-- jwt/         # Token handling
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
|-- docker-compose.yml       # Local development
|-- .env.example             # Environment template
```

## Troubleshooting

### Common Issues

**Port already in use**
```bash
# Change PostgreSQL port in docker-compose.yml
ports:
  - "5433:5432"  # Use 5433 on host
```

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
- Check your role has reveal permission (Developer or higher)
- Verify you're a member of the vault

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

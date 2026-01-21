# DevOps Secrets Manager CLI

A powerful command-line interface for managing secrets in the DevOps Secrets Manager.

## Installation

### Build from source

```bash
cargo build --release
```

The binary will be available at `target/release/secrets`.

### Add to PATH (optional)

```bash
# Copy to a directory in your PATH
sudo cp target/release/secrets /usr/local/bin/

# Or add to your shell profile
export PATH="$PATH:/path/to/apps/cli/target/release"
```

## Usage

### Authentication

Login to the secrets manager:

```bash
secrets login
```

This will prompt for your email and password, then securely store authentication tokens.

Logout and clear stored credentials:

```bash
secrets logout
```

### Managing Vaults

List all available vaults:

```bash
secrets vault list
```

### Managing Environments

List environments in a vault:

```bash
secrets env list --vault my-vault
```

### Working with Secrets

#### Pull secrets

Fetch all secrets from an environment and display as KEY=value format:

```bash
secrets pull --vault my-vault --env production
```

Save to a file:

```bash
secrets pull --vault my-vault --env production --out .env
```

#### Run commands with secrets (KILLER FEATURE)

Inject secrets as environment variables and run a command:

```bash
secrets run --vault my-vault --env production -- node app.js
```

This is perfect for development or CI/CD:

```bash
# Run tests with secrets
secrets run --vault api --env testing -- npm test

# Start a development server
secrets run --vault backend --env dev -- python manage.py runserver

# Deploy with secrets
secrets run --vault deploy --env production -- ./deploy.sh
```

#### Set a secret

Create or update a secret:

```bash
secrets set DATABASE_URL=postgres://... --vault my-vault --env production
```

With description:

```bash
secrets set API_KEY=sk_test_123 --vault my-vault --env staging \
  --description "Stripe API key for testing"
```

With rotation interval:

```bash
secrets set JWT_SECRET=my-secret --vault my-vault --env production \
  --rotation-days 90
```

### Audit Logs

View all audit logs:

```bash
secrets audit
```

Filter by vault:

```bash
secrets audit --vault my-vault
```

## Features

- **Secure Token Storage**: Uses system keyring with encrypted file fallback
- **Automatic Token Refresh**: Transparently refreshes expired tokens
- **Name Resolution**: Use vault/environment names instead of UUIDs
- **Environment Injection**: Securely inject secrets without writing to disk
- **Pretty Tables**: Clean, readable output formatting
- **Cross-Platform**: Works on Linux, macOS, and Windows

## API Endpoints Used

The CLI communicates with the backend API at `localhost:8080`:

- `POST /auth/login` - Authentication
- `POST /auth/refresh` - Token refresh
- `POST /auth/logout` - Logout
- `GET /vaults` - List vaults
- `GET /vaults/{id}/envs` - List environments
- `GET /envs/{id}/secrets` - List secrets
- `POST /envs/{id}/secrets` - Create secret
- `POST /secrets/{id}/reveal` - Reveal secret value
- `GET /audit` - Query audit logs

## Security Notes

1. Tokens are stored securely using the system keyring when available
2. If keyring is unavailable, tokens are base64-encoded and stored in `~/.config/devops-secrets-manager/tokens.json`
3. The `run` command never writes secrets to disk - they're injected directly into the process environment
4. All API communication uses HTTPS (in production)

## Examples

### Complete Workflow

```bash
# Login
secrets login

# List available vaults
secrets vault list

# List environments in a vault
secrets env list --vault backend

# Set some secrets
secrets set DB_HOST=localhost --vault backend --env dev
secrets set DB_PASSWORD=secret123 --vault backend --env dev

# Pull and view secrets
secrets pull --vault backend --env dev

# Run application with secrets
secrets run --vault backend --env dev -- npm start

# Check audit logs
secrets audit --vault backend

# Logout
secrets logout
```

### CI/CD Integration

```yaml
# GitHub Actions example
- name: Deploy with secrets
  run: |
    ./secrets login <<EOF
    ${{ secrets.SECRETS_EMAIL }}
    ${{ secrets.SECRETS_PASSWORD }}
    EOF
    ./secrets run --vault app --env production -- ./deploy.sh
```

## Development

### Project Structure

```
src/
├── main.rs              # Entry point with CLI argument parsing
├── api/
│   ├── mod.rs
│   └── client.rs        # API client with authentication
├── cli/
│   ├── mod.rs
│   ├── login.rs         # Login command
│   ├── logout.rs        # Logout command
│   ├── vault.rs         # Vault operations
│   ├── env.rs           # Environment operations
│   ├── pull.rs          # Pull secrets
│   ├── run.rs           # Run with secrets (killer feature)
│   ├── set.rs           # Set secrets
│   └── audit.rs         # Audit logs
├── config/
│   ├── mod.rs
│   └── token_store.rs   # Secure token storage
└── utils/
    ├── mod.rs
    └── table.rs         # Table formatting
```

### Dependencies

- `clap` - Command-line argument parsing
- `reqwest` - HTTP client with rustls
- `tokio` - Async runtime
- `serde` - JSON serialization
- `keyring` - Secure credential storage
- `comfy-table` - Table formatting
- `rpassword` - Secure password input

## License

Part of the DevOps Secrets Manager project.

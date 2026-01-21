# Quick Start Guide

Get up and running with the DevOps Secrets Manager CLI in 5 minutes.

## Prerequisites

- Rust toolchain installed (rustc, cargo)
- DevOps Secrets Manager API server running at localhost:8080
- Valid user account credentials

## Build

```bash
# Build release binary
cargo build --release

# Or use the Makefile
make release
```

The binary will be at `target/release/secrets` (11MB).

## Quick Test

```bash
# Show help
./target/release/secrets --help

# Check individual commands
./target/release/secrets run --help
```

## First Steps

### 1. Login

```bash
./target/release/secrets login
```

Enter your email and password when prompted. Tokens are stored securely in your system keyring.

### 2. List Vaults

```bash
./target/release/secrets vault list
```

You'll see a table with all available vaults.

### 3. Pull Secrets

```bash
./target/release/secrets pull --vault my-vault --env development
```

Secrets are displayed in KEY=value format.

### 4. Run with Secrets (THE KILLER FEATURE)

Instead of managing .env files, inject secrets directly:

```bash
./target/release/secrets run --vault backend --env dev -- node server.js
```

Secrets are injected as environment variables WITHOUT writing to disk!

## Common Use Cases

### Development

```bash
# Start development server with secrets
secrets run --vault app --env dev -- npm start

# Run database migrations
secrets run --vault db --env dev -- python manage.py migrate
```

### Testing

```bash
# Run tests with test environment secrets
secrets run --vault api --env test -- npm test
```

### CI/CD

```bash
# Deploy with production secrets
secrets run --vault deploy --env prod -- ./deploy.sh
```

### Adding Secrets

```bash
# Add a new secret
secrets set DATABASE_URL=postgres://localhost/mydb \
  --vault backend \
  --env dev \
  --description "Local development database"
```

## Installation (Optional)

To use `secrets` from anywhere:

```bash
# Using make
make install

# Or manually
sudo cp target/release/secrets /usr/local/bin/
```

Now you can use `secrets` instead of `./target/release/secrets`.

## What Makes This CLI Special?

1. **Zero-Disk Secrets**: The `run` command never writes secrets to disk
2. **Automatic Token Refresh**: Expired tokens are refreshed transparently
3. **Smart Resolution**: Use vault/environment names, not UUIDs
4. **Secure Storage**: Tokens stored in system keyring with file fallback
5. **Clean Output**: Beautiful table formatting for all list commands

## Examples

### Complete Workflow

```bash
# Login
secrets login

# See what's available
secrets vault list
secrets env list --vault backend

# Add a secret
secrets set API_KEY=test_123 --vault backend --env dev

# Verify it was added
secrets pull --vault backend --env dev

# Use it in your app
secrets run --vault backend --env dev -- python app.py
```

### Export to .env File

```bash
secrets pull --vault app --env staging --out .env
```

### View Activity

```bash
# See all audit logs
secrets audit

# Filter by vault
secrets audit --vault backend
```

## Troubleshooting

### "Not logged in" error

Run `secrets login` to authenticate.

### "Connection refused"

Make sure the API server is running at localhost:8080.

### Keyring errors on Linux

The CLI will automatically fall back to file-based storage if the system keyring is unavailable.

### Command not found

Either use the full path `./target/release/secrets` or install it system-wide with `make install`.

## Next Steps

- Read the full [README.md](README.md) for detailed documentation
- Check [examples/basic-usage.sh](examples/basic-usage.sh) for more examples
- Explore all commands with `secrets <command> --help`

## Support

For issues or questions about the DevOps Secrets Manager project, please refer to the main project documentation.

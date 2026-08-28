# Using secrets in GitHub Actions

A service token lets a workflow read the secrets of **one environment** without anyone's
personal login. The workflow installs the `secrets` CLI and runs commands with the values
injected as environment variables. Nothing is written to disk, and every read appears in the
audit log as `token:<name>`.

## 1. Create a service token

1. Open the vault in the web app and select the environment the workflow needs (for example
   `production`).
2. Click **Service Tokens** (vault owners and admins only), give the token a name such as
   `github-actions-deploy`, pick an expiry, and click **Create Token**.
3. Copy the token (`dsm_st_...`). It is shown once; the server only keeps a hash.

Create one token per repository and environment so each can be revoked on its own.

## 2. Store it in GitHub

In the repository, go to **Settings → Secrets and variables → Actions** and add:

| Name | Value |
|------|-------|
| `SECRETS_TOKEN` | the `dsm_st_...` token |
| `SECRETS_API_URL` | your API address, e.g. `https://secrets.example.com` (a repository variable is fine) |

For deployments, use a GitHub *environment* with required reviewers and store the token there
instead of at repository level.

## 3. Use it in a workflow

```yaml
name: Deploy

on:
  push:
    branches: [main]

permissions:
  contents: read

jobs:
  deploy:
    runs-on: ubuntu-latest
    environment: production
    env:
      SECRETS_API_URL: ${{ vars.SECRETS_API_URL }}
      SECRETS_TOKEN: ${{ secrets.SECRETS_TOKEN }}
    steps:
      - uses: actions/checkout@v7

      - name: Install the secrets CLI
        run: |
          version=v1.0.0
          file="secrets-${version}-x86_64-unknown-linux-gnu.tar.gz"
          base="https://github.com/msbutt1/DevOps-Secrets-Manager/releases/download/${version}"
          # Keep the release file names: the checksum file refers to the archive by name
          curl -fsSLO "${base}/${file}"
          curl -fsSLO "${base}/${file}.sha256"
          sha256sum -c "${file}.sha256"
          tar -xzf "${file}"
          sudo install secrets /usr/local/bin/secrets

      - name: Deploy with secrets injected
        # --mask replaces any secret value that the command prints with [MASKED]
        run: secrets run --mask -- ./scripts/deploy.sh
```

`secrets run` exits with the command's exit code, so a failing deploy fails the job. With
`SECRETS_TOKEN` set, `--vault` and `--env` are optional; if you pass them, they must match the
token's environment, which guards against using the wrong token.

### Writing a `.env` file instead

Some tools only read files. `secrets pull` writes a file readable only by the runner user:

```yaml
      - name: Write .env for the build
        run: secrets pull --out .env
```

Delete the file when the step is done if later steps do not need it, and never upload it as an
artifact.

## How it is protected

- **Scope:** a token reads one environment's values and cannot list other vaults, change
  secrets or act as a user; the API rejects it everywhere except `GET /token/secrets`.
- **Storage:** only a SHA-256 hash and a short prefix are stored. A leaked database does not leak
  usable tokens.
- **Audit:** each read records an `env.exported` event with the token name, the keys read, the
  runner's IP address and the time. Filter the audit log by action to review CI access.
- **Expiry and revocation:** tokens can expire after 30, 90 or 365 days, and revoking one in the
  web app stops it immediately.
- **Logs:** GitHub masks repository secrets in logs, and `--mask` also hides the secret *values*
  the CLI injects if your scripts echo them.

## Rotating a token

1. Create a new token for the same environment.
2. Update the `SECRETS_TOKEN` secret in GitHub.
3. Re-run the workflow, then revoke the old token in the web app. Its **Last used** date tells
   you whether anything still depends on it.

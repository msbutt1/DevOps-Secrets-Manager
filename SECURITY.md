# Security

DevOps Secrets Manager stores credentials for other systems, so a weakness here exposes
everything it holds. This document describes what the design protects against, what it does
not, how to operate it safely, and how to report a problem.

## Reporting a vulnerability

Please report vulnerabilities privately through GitHub: open the repository's **Security** tab
and choose **Report a vulnerability**. If that option is not available, contact the maintainer
through the email address on their GitHub profile. Do not open a public issue for a security
problem.

Include the affected version or commit, the steps to reproduce, and the impact you expect. You
should get an acknowledgement within a week. Fixes are released as soon as they are ready and
credited to the reporter unless you prefer otherwise.

Only the latest release receives security fixes.

## What is protected

| Asset | Where it lives | Protection |
|-------|----------------|------------|
| Secret values | `secrets.encrypted_value` | AES-256-GCM with the vault's data key; decrypted only in the API, on reveal or export |
| Vault data keys | `vaults.encrypted_dek` | AES-256-GCM with the master key (KEK); the version that wrapped each key is recorded |
| Master key | `MASTER_KEK` environment variable | Never stored in the database; the API refuses weak, placeholder or published values |
| Passwords | `users.password_hash` | bcrypt (cost 10); 12–72 bytes, checked against common passwords |
| Refresh tokens | `refresh_tokens.token_hash` | Random, stored as SHA-256 hashes, rotated on every use |
| Service tokens | `service_tokens.token_hash` | Random `dsm_st_` tokens, stored as SHA-256 hashes, shown once |
| Verification and invite tokens | database | Random, stored as SHA-256 hashes, single use, expiring |

## Threat model

### Protected against

- **Stolen database or backups.** Without the master key an attacker gets key names,
  descriptions, labels, audit records and email addresses, but no secret values, passwords or
  usable tokens.
- **Tampering with stored ciphertext.** GCM authenticates every value and wrapped key. Modified
  ciphertext, a changed nonce, or ciphertext copied from another vault fails to decrypt and the
  API returns an error rather than wrong data (`TestTamperedCiphertextIsRejected`).
- **Other users of the service.** Every vault, environment, secret, member, audit and token
  operation checks the caller's role on the owning vault. Resources the caller cannot see return
  `404`, so IDs cannot be probed; visible resources the role may not change return `403`. A
  permission matrix test runs every endpoint as every role.
- **Password guessing.** Login is rate limited per IP (20/min) and per account (10 per 5 min),
  and after 5 consecutive failures the account backs off exponentially from 1 minute up to
  60 minutes. Registration, verification, refresh, reveal, invite lookup and service token reads
  are rate limited too, and responses include `Retry-After`.
- **Token theft from the browser.** Access tokens live only in memory for 15 minutes. The
  refresh token is an `HttpOnly`, `SameSite=Strict` cookie (`Secure` and `__Host-` prefixed
  outside development) that scripts cannot read. The web app ships a strict Content Security
  Policy with no inline scripts.
- **Token forgery and confusion.** JWTs are HS256 with a secret of at least 32 characters that
  must differ from the master key, and must carry this API's issuer and audience, an expiry and
  the access token type. Other algorithms, including `none`, are rejected.
- **Credentials in logs.** Access logs record method, path, status, timing and client IP only;
  headers, query strings and bodies are never logged. A test drives every credential-bearing
  flow and fails if a password, token, cookie or secret value appears in the logs.
- **Leaked CI credentials.** Service tokens read a single environment and nothing else, can
  expire, can be revoked instantly, and every read is audited with the token's name.
- **A leaked password or a lost device.** Settings lists every signed-in session with its
  client, IP address and last activity. Any session can be signed out, and changing the password
  signs out all the others; their refresh tokens are revoked and their access tokens expire within
  15 minutes.
- **Unaudited access.** Reveals, exports, membership and role changes, key rotations, logins,
  lockouts, password changes and session sign-outs are written to the audit log with the actor,
  target, IP address and user agent.

### Not protected against

- **A compromised API host.** The API process holds the master key and decrypts values in
  memory. Anyone who can read its environment, memory or a core dump can decrypt everything. Run
  it on a host you trust, and keep the master key in the platform's secret store rather than in
  files or images.
- **Authorized users.** A user with reveal permission can copy values. Roles limit who can
  reveal, and the audit log shows who did, but nothing stops a trusted person from misusing
  access.
- **Compromised client machines.** Values injected by `secrets run`, written by `secrets pull`
  or shown in a browser are exposed to malware on that machine.
- **Metadata disclosure.** Key names, descriptions, labels, environment and vault names, and the
  audit log are stored in plain text.
- **An attacker with database write access.** They cannot read or forge secret values, but they
  can delete data, change roles or memberships, or edit audit records. The audit log is not
  tamper-evident. Restrict database credentials to the API.
- **Traffic without TLS.** The API and web server do not terminate TLS themselves. Put them
  behind a TLS-terminating proxy; nginx sends HSTS when it is served over HTTPS.

### Known limitations

- There is no multi-factor authentication or single sign-on.
- Keys come from environment variables; there is no integration with a cloud KMS or HSM.

## Operating securely

- **Generate keys randomly** with `openssl rand -hex 32` (`make env` does this locally). Use
  different values for `MASTER_KEK` and `APP_JWT_SECRET`.
- **Keep the master key out of the database and its backups.** Anyone holding both can decrypt
  every value.
- **Terminate TLS** in front of the web app and API, and set `APP_ENV=production` (the default)
  so cookies are `Secure` and the development email fallback is off.
- **Set `APP_TRUSTED_PROXIES`** to your proxy's addresses so rate limits and audit records use
  real client IPs, and leave `APP_CORS_ALLOWED_ORIGINS` empty unless the web app is served from
  another origin.
- **Configure SMTP.** Without it, production refuses to send verification and invite links
  instead of logging them.
- **Watch the audit log** for `secret.revealed`, `env.exported`, `login.locked` and role changes.
- **Run the dependency scan.** CI runs `govulncheck`, `npm audit` and `cargo audit` on every push
  and weekly; run `make audit` before deploying.

## Rotating keys and credentials

| What | When | How |
|------|------|-----|
| Master key (`MASTER_KEK`) | Periodically, or if the key may have been exposed | Follow [Rotating the Master Key](README.md#rotating-the-master-key): add the new key as the next version, restart, run `make rotate-kek`, then remove the old key. Secret values are not re-encrypted; only data keys are re-wrapped. |
| A vault's data key | If the vault's data and the master key may both have been exposed | `POST /vaults/{id}/rotate-key` as an owner or admin. All values in the vault are re-encrypted in one transaction. |
| All vault data keys | After a master key exposure where encrypted data may also have been copied | Rotate the master key first, then rotate each vault's data key. |
| JWT secret (`APP_JWT_SECRET`) | If it may have been exposed | Set a new value and restart. Existing access tokens stop working immediately; clients get new ones with their refresh token. |
| Service tokens | Periodically, or if one may have leaked | Create a new token, update the pipeline, then revoke the old one. Its **Last used** date shows whether anything still depends on it. See [docs/ci-github-actions.md](docs/ci-github-actions.md#rotating-a-token). |
| Secret values | On their rotation interval, or after exposure | Update the value in the web app or with `secrets set`; the dashboard lists secrets that are overdue. |

After any exposure, also review the audit log for the affected period and rotate the upstream
credentials the secrets grant access to; re-encrypting a value does not help if the value itself
was read.

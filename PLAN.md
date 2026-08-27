# DevOps Secrets Manager: plan to finish

The goal is to make this a finished project someone else can clone, run with one command, and use for real. It covers everything that is broken, everything half-built, and the polish that makes it presentable.

Status: `[ ]` to do · `[x]` done · `[~]` in progress · `[-]` dropped (say why)

---

## Commit rules (read before every commit)

> **Commit after every single fix or feature. One change = one commit.**

- **All commits are mine alone.** Author is `msbutt1 <msbutt112004@gmail.com>`.
- **No trailers.** No `Co-authored-by`, no "generated with" lines, and no tool or assistant attribution in commit messages, PR titles, PR descriptions, tags, or release notes.
- **Use conventional commits.** Each item below suggests a message.
  - `fix(api): ...`, `fix(web): ...`, `fix(cli): ...` for bugs
  - `feat(scope): ...` for new behaviour
  - `test`, `docs`, `chore`, `ci`, `refactor`, `build` for everything else
- **Keep each commit working.** Before committing, run the checks for the part you touched:
  - API: `go build ./... && go vet ./... && go test ./...`
  - Web: `npm run lint && npx tsc -p tsconfig.app.json --noEmit && npm test`
  - CLI: `cargo fmt --check && cargo clippy -- -D warnings && cargo test`
- **Keep commits focused.** Only format files you changed. `go fmt ./...` on the whole repo reformats unrelated files, so leave those for one separate `style:` commit.
- **Never commit** `.env`, real keys, database dumps, or `node_modules`/`target`/`dist`.
- **Push when a phase is done**, not halfway through a fix.
- Tick the box in this file as part of the commit that finishes the item.

---

## Already done

- [x] `fix(api): add missing vault_members migration`: vault handlers failed with 500 errors because the table didn't exist
- [x] `fix(docker): add missing config.docker.yaml`: the API image couldn't build
- [x] `fix(api): map nested config keys to APP_ environment variables`: Docker Compose settings were being ignored
- [x] `fix(api): return secret count when listing environments`: every environment tab showed (0)
- [x] `fix(web): show last updated date and secret name on vault page`: the page showed "Invalid Date" and a blank name in the reveal dialog

---

## Phase 0: Foundations

Do this first so every later fix can be checked automatically.

- [x] **Local run without Docker, in one command.** Add a root `Makefile` (or `justfile`):
  - `make db` starts Postgres
  - `make api` / `make web` / `make cli` start or build each app
  - `make dev` starts everything
  - `make test` / `make lint` run all checks
  - Commit: `build: add Makefile for local development`
- [x] **Seed script.** Commit `scripts/seed.sh` (or `go run ./cmd/seed`). It creates a demo user, an organization, vaults, environments, secrets and members through the API, so it works against any deployment. Commit: `chore: add demo data seed script`
  - Seeds users, the owner's organization (teammates join through invitations), vaults with per-vault members, environments, and secrets with rotation schedules and expiry dates.
- [x] **Clean up the example env file.**
  - [x] `.env.example` contains real-looking `MASTER_KEK` and `JWT_SECRET` values. Replace them with `change-me` placeholders.
  - [x] Make the API refuse to start when either key is missing, too short, or still set to a placeholder or the old example value.
  - Commits: `fix: replace example keys with placeholders` + `feat(api): refuse to start with weak or example keys`
- [x] **Fix the Go module path.** It is still `github.com/razlafan/devops-secret-manager`. Rename it to `github.com/msbutt1/DevOps-Secrets-Manager` and update every import. Commit: `refactor(api): rename Go module to match repository`
- [x] **One-time formatting.** Run `go fmt ./...`, `cargo fmt` and Prettier across the repo. Commit: `style: format codebase`
- [x] **Get all checks green.**
  - [x] The web typecheck has 4 existing TypeScript errors. Fix them. Commit: `fix(web): resolve TypeScript errors`
  - [x] Fix any `go vet` or ESLint findings, one commit per area. (`go vet` was already clean; ESLint had 5 errors.)
- [x] **CI.** Add a GitHub Actions workflow that runs, on every push and PR:
  - Go build, vet, test (with a Postgres service container)
  - Web lint, typecheck, test, build
  - CLI fmt, clippy, test
  - `docker compose build`
  - Commit: `ci: add build and test workflow`
- [x] **Licence.** Add a `LICENSE` file (MIT, unless you want something else). Commit: `docs: add MIT license`

---

## Phase 1: Broken behaviour

These are things that exist in the UI or README but don't work.

### Authorization (highest priority: security)
- [x] **Vault roles aren't enforced.**
  - `policy.Can` looks up the user's role in `user_organizations`, so every permission check uses the organization role.
  - Roles set per vault on the Members page (`vault_members`) are only partly used by some handlers.
  - Result: making someone a "viewer" on one vault may not stop them revealing secrets there.
  - Fix: route every vault, environment, secret and member check through a single `policy.CanOnVault(user, vault, action)` that reads `vault_members`, with organization owners and admins inheriting access.
  - Commit: `fix(api): enforce per-vault roles in policy checks`
- [x] **Test the permission matrix.** Add table-driven tests covering 5 roles × every action, run against a real database. They must match `ROLE_PERMISSIONS` in `apps/web/src/types/api.ts`. Commit: `test(api): cover role permission matrix`
- [x] **Stop cross-vault access.** Every `/envs/{id}` and `/secrets/{id}` route must check that the resource belongs to a vault the caller can access, not just that the ID exists. Add tests where user A uses user B's IDs. Commit: `fix(api): scope environment and secret lookups to caller's vaults`
  - The scoping itself landed with the per-vault role fix (every lookup resolves the owning vault and answers 404 without access); this item added the cross-tenant tests.

### Audit log (empty in the UI)
- [x] **Response shape mismatch.** The web app expects `PaginatedResponse<AuditEvent>` with these fields:
  - `data`, `total`, `page`, `limit`, `hasMore`
  - `vaultName`, `environmentName`, `userEmail`, `targetName`, `timestamp`

  The API returns a plain list with `resource_type` and `resource_id`. Fix: return a paginated, joined response (`page` and `limit` query parameters, plus a total count). Commit: `fix(api): return paginated audit events with names`
- [x] **Action names don't match.** The API writes `SECRET_REVEALED`; the web app filters on `secret.revealed`. Pick the dotted lowercase form, add a migration that rewrites existing rows, and share one list. Commit: `fix: align audit action names between API and web`
- [x] **Log the missing actions.** Check that every action the web app lists is actually written:
  - `vault.updated`
  - `env.created` and `env.deleted`
  - `member.added`, `member.removed`, `member.role_changed`
  - `login.failure`

  Commit: `feat(api): audit environment, member and vault update events`
- [x] **Filters.** Implement the `vaultId`, `environmentId`, `userId`, `action` and `startDate`/`endDate` filters on the server. Commit: `feat(api): filter audit log by vault, user, action and date`
- [x] **CLI `--since`.** `apps/cli/src/cli/audit.rs` has a TODO for filtering by `since`. Implement it by passing `startDate` to the API. Commit: `feat(cli): filter audit log with --since`

### Vault page and API contract
- [x] **"Created by on".** The vault header reads "Created by on" because the API never returns `created_by`. Return the creator's name and `created_at`. Commit: `fix(api): include creator in vault response`
- [x] **Editing a secret wiped its value.** (Found while fixing the contract.) The edit form sends no value when the field is left blank, and the API re-encrypted an empty string. Updates without a value now keep the stored value. Commit: `fix(api): keep secret value when an update omits it`
- [x] **Secret fields the web app expects but the API doesn't send:**
  - `lastUpdatedBy`
  - `rotationPolicy { intervalDays, lastRotatedAt, nextRotationAt }` (the API sends a flat `rotation_interval_days`)

  Commit: `fix(api): return rotation policy and last editor for secrets`
- [x] **Reveal response.** The web app expects `expiresIn` (auto-hide seconds). Check the API sends it; don't rely on the client default. Commit: `fix(api): include auto-hide window in reveal response`
- [x] **Member response.** Check `added_by` is populated (not blank) and that adding a member by email returns a clear 404 when that user doesn't exist. Commit: `fix(api): populate added_by and clarify unknown member errors`
- [x] **Environment names.** The web form only allows `dev`/`staging`/`prod`, but the database accepts any name and the API has no check. Choose one:
  - allow custom names everywhere (recommended), or
  - enforce the three names in the API and database.

  Commit: `fix: consistent environment name rules across API and web`

### Dashboard (fake numbers)
- [x] Replace or remove the placeholders in `DashboardPage.tsx`:
  - hard-coded "Last Backup: 2 hours ago" and "Uptime: 99.99%"
  - `activeUsers`, `secretsExpiringSoon` and `secretsNeedingRotation` are always 0
  - Fix: a `/stats` endpoint with real counts, and a real `/health` check (database ping, migration version, server start time).
  - Commits: `feat(api): add dashboard stats endpoint` + `fix(web): show real dashboard stats`
  - [x] API: `GET /stats` and a real `/health`.
  - [x] Web: stat cards from `/stats`; System Status from `/health` (the false "Key Derivation: PBKDF2" line is gone too, as keys are random, not derived).
- [x] **System alerts.** Make the alerts panel list real items: expired secrets, secrets overdue for rotation, members with no recent login. Otherwise remove the panel. Commit: `feat(web): show expiry and rotation alerts on dashboard`
- [x] **Recent activity.** Once the audit endpoint is fixed, check that the panel fills in. Commit (if changes are needed): `fix(web): load recent activity from audit log`

### Docker and deployment
- [-] **Test the Compose stack from a fresh clone.** Run `docker compose up --build`, register, create a vault, add a secret, reveal it. Record every problem found as its own fix commit.
  - Not possible on the development machine (the Docker daemon needs sudo). The Compose file is validated with `docker compose config`, a static review found and fixed the missing root `.dockerignore`, and CI builds the images; running the stack end to end is left for a machine with Docker.
- [x] **Postgres versions.** Compose uses Postgres 15 while local development used 17. Pick one (16 or 17) and document it. Commit: `chore(docker): pin Postgres version`
- [x] **Health checks.** Add a Compose health check for the API; make the web container wait for it. Commit: `chore(docker): add API healthcheck`
- [x] **Base images.** Pin the Alpine runtime image instead of `alpine:latest`, and drop the obsolete `-a -installsuffix cgo` build flags. Commit: `build(api): pin runtime image and simplify build flags`

### Documentation that doesn't match the code
- [x] **JWT algorithm.** The README says tokens use RS256; the code uses HS256. Either correct the README or move to RS256 (see Phase 3). Commit: `docs: correct JWT signing algorithm`
- [x] **Variable names.** Make the README's environment variable names match what the code reads: `APP_DATABASE_*`, `APP_JWT_SECRET`, `MASTER_KEK`. Commit: `docs: fix environment variable names`
- [x] **Commands.** Check every command in the README and `apps/cli/QUICKSTART.md` by running it. Commit: `docs: verify setup and CLI instructions`
  - Ran the Makefile quick start from a wiped database and the whole QUICKSTART workflow against the local API. Docker commands could not be run here. Problems found and queued under CLI: `logout` never revokes the refresh token, `pull --out` writes a world-readable file, and `set` cannot update.
- [x] **OpenAPI spec.** Update `docs/openapi.yaml` to match the real API, including `/auth/me`, `/auth/change-password`, pagination and error formats. Commit: `docs(api): sync OpenAPI spec with handlers`

---

## Phase 2: Incomplete features

These are half-built or missing, but a team would need them.

### Organizations and people
- [x] **Organization API.**
  - Endpoints: `GET /orgs`, `GET /orgs/{id}`, `PATCH /orgs/{id}` (rename), `GET /orgs/{id}/members`
  - Today an organization is created silently at registration and can't be managed.
  - Commit: `feat(api): add organization endpoints`
  - Also `PUT`/`DELETE /orgs/{id}/members/{userId}` for the settings page; removing someone from an organization also removes them from its vaults.
- [x] **Invites.**
  - Admins invite by email; the API creates a single-use, expiring token and sends an email (the SMTP service already exists). The link goes to a register-or-accept page.
  - Without this, nobody new can ever join an organization, so vault membership is limited to people already in it.
  - Commits: `feat(api): organization invites` + `feat(web): invite and accept flow`
  - [x] API: create/list/revoke, lookup and accept, and register with `invite_token`.
  - [x] Web: `/invite` accept page (log in, create account, or accept), invite-aware registration, and an invite dialog from Settings.
- [x] **Organization settings page.** Members, roles, pending invites, remove a member. Commit: `feat(web): organization members page`
- [x] **Organization picker.** Users in more than one organization need a switcher. The audit handler currently picks the first organization. Commit: `feat(web): switch between organizations`

### Account
- [x] **Registration was not atomic.** (Found while reading the code.) The user and verification token were written outside the transaction that creates the organization, so a failure left an account with no organization that could never register again. Commit: `fix(api): create user, organization and token in one transaction`
- [x] **Email addresses were case-sensitive.** (Found while designing invites.) `Ada@x.test` and `ada@x.test` could be two accounts, and logging in with different capitalisation failed. Addresses are now trimmed and lower-cased everywhere, with a case-insensitive unique index (migration 000014). Commit: `fix(api): treat email addresses case-insensitively`
- [ ] **Change password.** The route and page exist. Test the whole flow end to end, and revoke all refresh tokens after a change. Commit: `fix(api): revoke sessions after password change`
- [ ] **Forgot password.** Emailed reset token, then a reset page. Commit: `feat: password reset by email`
- [x] **Email verification in development.** Login is blocked until the email is verified, but local setups have no SMTP. Add a dev mode that logs the verification link (or a `APP_EMAIL_DISABLED=true` auto-verify switch that is off by default). Document it. Commit: `feat(api): log verification links when SMTP is not configured`
- [ ] **Resend verification email.** Commit: `feat: resend verification email`
- [ ] **Sessions.** List active sessions (refresh tokens) in Settings and allow "sign out everywhere". The Settings page's "Current Session" panel is currently hard-coded ("Chrome on Windows", 192.168.1.100) and must show real data or go. Commit: `feat: view and revoke active sessions`

### Secrets lifecycle
- [ ] **Expiry.**
  - `expires_at` is stored but nothing acts on it.
  - Show expired or expiring badges.
  - Decide whether expired secrets can be revealed or pulled; recommended: allowed, with a warning in the CLI output.
  - Commit: `feat: surface secret expiry in web and CLI`
- [ ] **Rotation reminders.**
  - `rotation_interval_days` and `last_rotated_at` are stored, but nothing computes when a secret is due.
  - Compute `next_rotation_at`, set `last_rotated_at` when the value changes, and list overdue secrets.
  - Commit: `feat(api): track rotation due dates`
- [ ] **Version history.** Keep previous encrypted values (a `secret_versions` table) so a bad update can be rolled back. Record the version number in the audit log. Commit: `feat: secret version history and rollback`
- [ ] **Bulk import and export.**
  - Paste or upload a `.env` file into an environment, with a preview of what gets created, updated or skipped.
  - Export an environment (reveal permission required, and audited).
  - Commit: `feat: import and export environments as .env`
- [ ] **Copy between environments.** Copy selected keys from staging to prod, with values re-encrypted. Commit: `feat: copy secrets between environments`
- [ ] **Search.** Search key names across all vaults the user can see. Commit: `feat: search secrets by key name`

### Encryption and key management
- [ ] **Master key rotation.**
  - Add a `kek_version` column and support multiple master keys during a rotation.
  - Add a command (`server rotate-kek` or `make rotate-kek`) that re-encrypts every vault's data key.
  - The README promises this; nothing implements it.
  - Commit: `feat(api): master key rotation`
- [ ] **Vault data key rotation.** Re-encrypt one vault's secrets under a new data key. Commit: `feat(api): rotate vault data key`
- [ ] **Encryption tests.** Tests exist for `envelope.go` only. Add tests for:
  - a tampered ciphertext failing to decrypt
  - the wrong master key failing
  - rotation round trips

  Commit: `test(api): cover tampering and key rotation`

### CLI
- [x] **Build and install.** Rust isn't installed on this machine, so the CLI has never been built here. Install rustup, build, and fix whatever fails. Commit per fix. (It compiled as-is; clippy needed fixes and `Cargo.lock` was gitignored.)
- [x] **Configurable server address.** The API URL is hard-coded to `http://localhost:8080` in `api/client.rs`. Add `--api-url`, the `SECRETS_API_URL` variable, and a saved setting from `secrets login --api-url`. Commit: `feat(cli): configurable API URL`
- [x] **Update existing secrets.** `secrets set` only creates. Make it update an existing key (with `--create-only` / `--update-only` flags). Commit: `feat(cli): update existing secrets with set`
- [x] **Missing commands.**
  - [x] `secrets delete`
  - [x] `secrets env create`
  - [x] `secrets vault create`
  - [x] `secrets members list/add/remove`
  - [x] `secrets import .env`
  - Commit per command, e.g. `feat(cli): add delete command`
- [x] **Logout and file safety.** (Found while checking the docs.) `secrets logout` sends no refresh token, so the session stays valid on the server; `pull --out` creates the `.env` file readable by everyone. Revoke on logout and write files with mode 0600. Commit: `fix(cli): revoke session on logout and write .env files privately` (revocation shipped with the API URL client rewrite; this commit adds private files)
- [x] **`secrets run` safety.** Never print values. Mask secrets that appear in the child process's output (optional flag). Exit with the child's exit code. Commit: `fix(cli): propagate exit code and mask values in run`
- [x] **Scripting output.** `--output json` on list commands. Commit: `feat(cli): JSON output`
- [x] **Shell completions.** Generate them with `clap_complete`. Commit: `feat(cli): shell completions`
- [ ] **CLI tests.** The CLI has no tests. Add tests for argument parsing, `.env` writing and quoting, and the API client against a mock server. Commit: `test(cli): add unit and client tests`
- [ ] **Release binaries.** Build Linux, macOS and Windows binaries on tag push and attach them to a GitHub Release. Commit: `ci(cli): build release binaries`

### Machine access (for CI/CD, the "DevOps" in the name)
- [ ] **Service tokens.**
  - Read-only, environment-scoped tokens for CI pipelines and servers, created and revoked in the web app.
  - Store only a hash of each token; show it once when created.
  - Audit its use as `token:<name>`.
  - The CLI accepts the token through `SECRETS_TOKEN`.
  - Commits: `feat(api): environment service tokens` + `feat(web): manage service tokens` + `feat(cli): authenticate with service token`
- [ ] **GitHub Actions example.** Pull secrets in a workflow using a service token (`docs/ci-github-actions.md`). Commit: `docs: GitHub Actions usage example`

---

## Phase 3: Hardening

- [ ] **Rate limiting.** Limit `/auth/login`, `/auth/register`, `/auth/refresh` and `/secrets/{id}/reveal` per IP and per account (`httprate` or similar). Return 429 with `Retry-After`. Commit: `feat(api): rate limit auth and reveal endpoints`
- [ ] **Account lockout.** Add a backoff after repeated failed logins, and audit it. Commit: `feat(api): back off after repeated failed logins`
- [ ] **Request limits.** Limit request body size, reject unknown JSON fields, enforce maximum lengths for key names, values and descriptions. Commit: `fix(api): validate and limit request bodies`
- [ ] **Security headers.**
  - The API should send `Cache-Control: no-store` on secret responses.
  - nginx should send a Content-Security-Policy and HSTS (when served over TLS), and drop the obsolete `X-XSS-Protection` header.
  - Commit: `feat: security headers for API and web`
- [ ] **CORS.** No CORS handling exists; it only works today because the web app is proxied through `/api`. If the API is ever deployed separately, add an explicit origin allowlist. Commit: `feat(api): configurable CORS allowlist`
- [ ] **Token storage in the browser.** Check where the web app keeps tokens. If refresh tokens are in `localStorage`, move them to an `HttpOnly`, `Secure`, `SameSite=Strict` cookie. Commit: `fix: store refresh token in HttpOnly cookie`
- [ ] **JWT.** Decide between HS256 with a strong secret, or RS256 with key rotation, and document the choice. Add `iss` and `aud` claims and check them. Commit: `feat(api): validate token issuer and audience`
- [ ] **Password policy.** Minimum length, checked against a common-password list; show a strength hint in the register form. Commit: `feat: password strength requirements`
- [ ] **Structured logging.** Use `slog` JSON logs with a request ID. Never log secret values, tokens, or `Authorization` headers; add a test that checks for this. Commit: `feat(api): structured request logging without secrets`
- [ ] **Dependency scanning.** Run `govulncheck`, `npm audit` and `cargo audit` in CI, and enable Dependabot. Commit: `ci: dependency vulnerability scanning`
- [ ] **Threat model.** Write `SECURITY.md`: what the encryption protects against, what it doesn't (a compromised API host), how to report issues, and how to rotate keys. Commit: `docs: security model and reporting`

---

## Phase 4: Tests

- [x] **API integration tests.** Use testcontainers (or the CI Postgres service) to run the real router against a real database. Cover:
  - register, verify, login, refresh, logout
  - creating vaults, environments and secrets; reveal, update, delete
  - adding members and changing roles
  - that the audit log records each step

  Commit: `test(api): end-to-end API integration tests`
  - Uses the CI Postgres service / local `make db` server through `TEST_DATABASE_URL`, with a fresh database per test (no new dependency).
- [x] **Migration tests.** Every migration applies and rolls back cleanly, which would have caught the missing `vault_members` table. Commit: `test(api): migrations apply and roll back`
- [x] **Contract test.** Generate the web app's TypeScript types from `docs/openapi.yaml` (`openapi-typescript`), and fail CI if the handlers and spec differ. This stops the API/web drift that caused most of Phase 1. Commit: `test: enforce OpenAPI contract for web types`
- [x] **Secret label keys were rewritten.** (Found by the generated types.) The web app's snake_case/camelCase transform also renamed user-defined label keys, so `team_name` came back as `teamName` and was saved that way. Keys inside `metadata` are now left alone. Commit: `fix(web): keep secret label keys unchanged`
- [x] **Web tests.**
  - Components: reveal dialog, permission gate, secret form validation.
  - Replace the placeholder `src/test/example.test.ts`.
  - Commit: `test(web): component tests for secrets and permissions`
- [x] **Browser tests.** Playwright against Docker Compose: log in, create a vault, add a secret, reveal it, check the audit entry, and check a viewer can't reveal. Commit: `test: browser end-to-end tests`
  - Owner flow done (register, verify, log in, vault, environment, secret, reveal, audit entry), run with `make test-e2e` against `make dev` since Docker is unavailable here. The viewer case runs through the real invite flow: invite, register from the link, add to the vault as viewer, no reveal or delete controls, and the link cannot be reused.

---

## Phase 5: Polish and presentation

- [ ] **Loading, empty and error states** on every page (vaults, environments, secrets, audit, members). Commit: `feat(web): consistent loading, empty and error states`
- [ ] **Confirmations.** Deleting a vault, environment or secret, or removing a member, requires typing the name. Commit: `feat(web): typed confirmation for destructive actions`
- [ ] **Keyboard and accessibility.** Focus order, visible focus, labelled icon buttons, dialogs that trap and restore focus, checked with axe. Commit: `fix(web): accessibility pass`
- [ ] **Responsive layout.** The Windows 95 look breaks on narrow screens; make tables scroll and dialogs fit. Commit: `fix(web): responsive layout`
- [ ] **Theme.** Keep the retro Windows 95 look (it's distinctive) but tidy it: consistent spacing, one icon set, and a modern theme toggle if wanted. Commit: `style(web): refine UI theme`
- [ ] **Favicon, page titles, metadata.** Commit: `feat(web): favicon and page titles`
- [ ] **README.**
  - One-paragraph pitch, screenshots, a short demo GIF, an architecture diagram (Mermaid)
  - Quick start in 3 commands, feature list, security model
  - A roadmap section linking to this file
  - Commit: `docs: rewrite README`
- [ ] **Screenshots.** Recapture them after the fixes (dashboard, vault, reveal dialog, audit log, members, CLI in a terminal) into `docs/images/`, then update the portfolio screenshot and write-up (the "where it falls short" section changes once Phase 1 is done). Commit: `docs: update screenshots`
- [ ] **Changelog and version.** Add `CHANGELOG.md` and tag `v1.0.0` once Phases 0–2 are done. Commit: `chore: release v1.0.0`

---

## Phase 6: Hosted demo (optional)

- [ ] **Deploy.** API and database on Fly.io + Neon (already in the README) or Railway; web app on Cloudflare Pages with `/api` proxied.
- [ ] **Demo mode.** A read-only demo account with sample data that resets nightly, plus a banner saying "Demo data resets daily; never store real secrets here."
- [ ] **Backups.** Nightly `pg_dump` to storage, with a documented restore procedure. This makes a real "Last backup" figure possible on the dashboard.
- [ ] **Uptime check.** An external monitor on `/health`, with the badge in the README.

---

## Order of work

| # | Phase | Why this order |
|---|-------|----------------|
| 1 | 0: Foundations | CI and a seed script make every later fix quick to check |
| 2 | 1: Authorization | Security bugs come before features |
| 3 | 1: Audit, contract, dashboard, docs | Makes what's already on screen truthful |
| 4 | 4: Contract + integration tests | Stops the same drift coming back |
| 5 | 2: Organizations and invites, CLI basics, service tokens | The features a real team needs |
| 6 | 3: Hardening | Before anyone stores a real secret |
| 7 | 5: Polish, then release v1.0.0 | Presentation |
| 8 | 2 (rest) + 6 | Nice to have |

**Reminder:** commit after every fix and every feature. The commit is authored by me, with a conventional message and no trailers or attribution of any kind.

# Running it day to day

Backups, monitoring and the demo setup. The deployment itself is in
[deployment.md](deployment.md); the threat model is in [SECURITY.md](../SECURITY.md).

## Backups

`scripts/backup.sh` writes a compressed `pg_dump` and keeps the newest 14:

```bash
scripts/backup.sh                      # ~/.local/share/devops-secrets-manager/backups
BACKUP_DIR=/srv/backups KEEP=30 scripts/backup.sh
```

It reads the usual PostgreSQL variables (`PGHOST`, `PGPORT`, `PGUSER`, `DB_NAME`), writes the
file with mode 600 and only replaces the previous file once the dump finishes, so an interrupted
run cannot leave a half-written backup looking complete.

Nightly with cron (03:15, log kept for the last run):

```cron
15 3 * * * /srv/app/scripts/backup.sh /srv/backups >> /var/log/secrets-backup.log 2>&1
```

Or as a systemd timer, if you would rather have status and failure mail:

```ini
# /etc/systemd/system/secrets-backup.service
[Service]
Type=oneshot
Environment=PGHOST=localhost PGPORT=5432 PGUSER=secrets_user DB_NAME=secrets_db BACKUP_DIR=/srv/backups
ExecStart=/srv/app/scripts/backup.sh

# /etc/systemd/system/secrets-backup.timer
[Timer]
OnCalendar=03:15
Persistent=true
[Install]
WantedBy=timers.target
```

With Docker Compose, dump from inside the database container:

```bash
docker compose exec -T db pg_dump --no-owner --clean --if-exists -U secrets_user secrets_db \
  | gzip -9 > backup-$(date -u +%Y%m%dT%H%M%SZ).sql.gz
```

**The dump holds ciphertext only.** Whoever holds both a dump and `MASTER_KEK` can read every
secret, so keep them apart: the key in the platform's secret store, the dumps in storage that the
API host cannot read. A backup without the key is useless to an attacker — and also useless to
you, so write the key down somewhere safe (a password manager, a sealed envelope) before you
need it.

### Restoring

Tested on this project: dump, drop, restore, then read a secret back through the API.

```bash
createdb -h localhost -p 5432 -U postgres -O secrets_user secrets_db_restored
gunzip -c backup-20260916T031500Z.sql.gz | psql -h localhost -p 5432 -U secrets_user secrets_db_restored
```

Restore **as the role the API connects with** (`secrets_user` above), not as a superuser. A dump
restored by `postgres` leaves every table owned by `postgres`, and the API then fails at startup
with `permission denied for table schema_migrations`. If that has already happened:

```sql
REASSIGN OWNED BY postgres TO secrets_user;
```

Then point the API at the restored database (`APP_DATABASE_NAME=secrets_db_restored`), check
`/health` reports the expected `migration_version`, and reveal one secret to prove the values
decrypt with the master key you still hold. Restore the key first if it also needs restoring:
without it the data is unreadable.

Test the restore on a schedule, not on the day you need it. A backup you have never restored is
a hypothesis.

## Monitoring

The API serves `/health` without authentication:

```json
{"status":"ok","database":"ok","migration_version":21,"migration_dirty":false,
 "started_at":"2026-09-15T08:18:32Z","uptime_seconds":842,"version":"v1.0.0","timestamp":"..."}
```

It returns 503 with `"status":"unavailable"` when the database cannot be reached, so it works as
both a platform health check and an external monitor. `migration_dirty: true` means a migration
failed half-way and needs looking at.

Point an uptime service (UptimeRobot, Better Stack, Healthchecks.io — all have free tiers) at
`https://your-host/api/health` every minute and alert on two consecutive failures. Most of them
give you a status badge; add it at the top of the README once a monitor exists, rather than
linking to one that does not:

```markdown
[![Uptime](https://your-monitor.example.com/badge/123)](https://status.example.com)
```

Worth watching beyond uptime:

- `login.locked` and repeated `login.failure` in the audit log: someone is guessing passwords
- `secret.revealed` and `env.exported` volume: a person or token reading far more than usual
- 5xx rate and `duration_ms` in the JSON access logs, keyed by `request_id`
- Certificate expiry, and the `Dependency scan` workflow's weekly run

## A demo deployment

If you put a public demo online, treat it as a throwaway:

- Seed it with `make seed` (or `go run ./apps/api/cmd/seed --api-url https://demo.example.com`),
  which creates four vaults, four people and sixteen fake secrets, none of them real.
- Reset it on a schedule so anything visitors add disappears:

  ```cron
  0 4 * * * cd /srv/app && dropdb -U postgres secrets_db && createdb -U postgres -O secrets_user secrets_db && ./scripts/seed.sh >> /var/log/secrets-demo-reset.log 2>&1
  ```

- Say so in the interface. The web app shows a banner when it is built with
  `VITE_DEMO_BANNER` set:

  ```bash
  VITE_DEMO_BANNER="Demo data resets daily. Never store real secrets here." npm run build
  ```

- Give visitors an account that cannot do damage: create a separate organization, add the demo
  user to a vault as `viewer` (read, no reveal) or `oncall` (reveal, no writes), and keep the
  owner account for yourself. The demo user then cannot delete vaults or change members.
- Keep the rate limits on (they are on by default) and leave `APP_ENV=production` so email links
  are never written to the logs.

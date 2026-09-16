#!/usr/bin/env bash
# Reset a demo deployment: delete everything anyone has added, then seed it again.
#
# This is destructive and irreversible. It is meant for a public demo where the data is
# disposable, and it refuses to run without being told explicitly that is what you mean:
#
#   DEMO_RESET_CONFIRM=yes DATABASE_URL=postgres://... API_URL=https://... scripts/demo-reset.sh
#
# Every table except schema_migrations is truncated, so the API does not try to migrate again
# on its next start, and the demo accounts are recreated from the database rather than through
# registration, so no mail is sent to the demo addresses.
set -euo pipefail

: "${DATABASE_URL:?set DATABASE_URL to the demo database}"
: "${API_URL:?set API_URL to the demo API, e.g. https://devops.example.com/api}"

if [ "${DEMO_RESET_CONFIRM:-}" != "yes" ]; then
    cat >&2 <<'EOF'
Refusing to run: this deletes every account, vault and secret in the target database.
Set DEMO_RESET_CONFIRM=yes if that is what you want.
EOF
    exit 1
fi

# Show which database is about to be emptied, without printing the password in it.
host=$(printf '%s' "$DATABASE_URL" | sed -E 's#^[^@]*@##; s#[/?].*$##')
echo "Resetting the demo at ${host}"

psql "$DATABASE_URL" -v ON_ERROR_STOP=1 <<'SQL'
DO $$
DECLARE
    statement text;
BEGIN
    SELECT 'TRUNCATE TABLE '
           || string_agg(format('%I.%I', schemaname, tablename), ', ')
           || ' RESTART IDENTITY CASCADE'
      INTO statement
      FROM pg_tables
     WHERE schemaname = 'public'
       AND tablename <> 'schema_migrations';

    IF statement IS NULL THEN
        RAISE EXCEPTION 'no tables found: is DATABASE_URL pointing at the right database?';
    END IF;

    EXECUTE statement;
END $$;
SQL

echo "Database emptied; seeding"
go run ./apps/api/cmd/seed --api-url "$API_URL" --database-url "$DATABASE_URL"
echo "Demo reset complete"

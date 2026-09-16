#!/usr/bin/env bash
# Dumps the database to a timestamped, compressed file and keeps the newest KEEP dumps.
#
#   scripts/backup.sh [output-directory]
#
# The dump holds ciphertext only: without MASTER_KEK it cannot be decrypted. Keep the key
# somewhere else, or the backup and the key together are the whole system.
set -euo pipefail

OUT_DIR="${1:-${BACKUP_DIR:-$HOME/.local/share/devops-secrets-manager/backups}}"
KEEP="${KEEP:-14}"
PGHOST="${PGHOST:-localhost}"
PGPORT="${PGPORT:-5433}"
PGUSER="${PGUSER:-postgres}"
DB_NAME="${DB_NAME:-secrets_db}"

mkdir -p "$OUT_DIR"
stamp="$(date -u +%Y%m%dT%H%M%SZ)"
file="$OUT_DIR/${DB_NAME}-${stamp}.sql.gz"

# --no-owner keeps the dump restorable as a different role (managed databases rename it)
pg_dump --no-owner --clean --if-exists -h "$PGHOST" -p "$PGPORT" -U "$PGUSER" "$DB_NAME" \
  | gzip -9 > "$file.tmp"
mv "$file.tmp" "$file"
chmod 600 "$file"
echo "wrote $file ($(du -h "$file" | cut -f1))"

# Keep the newest $KEEP dumps
ls -1t "$OUT_DIR/${DB_NAME}-"*.sql.gz 2>/dev/null | tail -n "+$((KEEP + 1))" | while read -r old; do
  rm -f "$old"
  echo "removed $old"
done

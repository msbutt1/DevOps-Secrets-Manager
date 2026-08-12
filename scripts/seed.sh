#!/usr/bin/env bash
# Seeds demo data through the API. If no API is answering at API_URL, a local API is
# built and started just for the seed (logging to API_LOG) and stopped afterwards.
set -euo pipefail

API_URL="${API_URL:-http://localhost:8080}"
DEV_DIR="${DEV_DIR:-$HOME/.local/share/devops-secrets-manager}"
API_LOG="${API_LOG:-$DEV_DIR/api.log}"
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

mkdir -p "$DEV_DIR/bin"
server_pid=""
cleanup() {
  if [[ -n "$server_pid" ]]; then
    kill "$server_pid" 2>/dev/null || true
    wait "$server_pid" 2>/dev/null || true
  fi
}
trap cleanup EXIT

if ! curl -fsS "$API_URL/health" >/dev/null 2>&1; then
  echo "No API at $API_URL; starting a temporary local API (logs: $API_LOG)"
  (cd "$root" && go build -o "$DEV_DIR/bin/server" ./apps/api/cmd/server)
  (cd "$root/apps/api" && exec "$DEV_DIR/bin/server" >>"$API_LOG" 2>&1) &
  server_pid=$!
  for _ in $(seq 1 60); do
    curl -fsS "$API_URL/health" >/dev/null 2>&1 && break
    if ! kill -0 "$server_pid" 2>/dev/null; then
      echo "The API exited during startup; see $API_LOG" >&2
      exit 1
    fi
    sleep 0.5
  done
fi

cd "$root"
go run ./apps/api/cmd/seed --api-url "$API_URL" --log "$API_LOG" "$@"

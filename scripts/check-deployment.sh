#!/usr/bin/env bash
# Checks a deployed instance from the outside: TLS, health, security headers, cookie flags,
# that the console is not indexable and that no development fallbacks are on.
#
#   scripts/check-deployment.sh https://devops.msbutt.com
set -uo pipefail

BASE="${1:-}"
if [ -z "$BASE" ]; then
  echo "usage: $0 https://your-host" >&2
  exit 2
fi
BASE="${BASE%/}"
fails=0

pass() { printf '  ok    %s\n' "$1"; }
fail() { printf '  FAIL  %s\n' "$1"; fails=$((fails + 1)); }
check() { # check <description> <condition-output>
  if [ -n "$2" ]; then pass "$1"; else fail "$1"; fi
}

echo "Checking $BASE"
echo "(the header checks expect the production web server; against a local Vite dev server they fail)"

health="$(curl -fsS --max-time 15 "$BASE/api/health" 2>/dev/null)"
check "API answers /api/health" "$health"
check "database reachable" "$(printf '%s' "$health" | grep -o '"database":"ok"')"
check "migrations are clean" "$(printf '%s' "$health" | grep -o '"migration_dirty":false')"
printf '        %s\n' "$(printf '%s' "$health" | head -c 200)"

headers="$(curl -fsSI --max-time 15 "$BASE/" 2>/dev/null)"
check "HSTS on the web app" "$(printf '%s' "$headers" | grep -i '^strict-transport-security')"
check "content security policy" "$(printf '%s' "$headers" | grep -i '^content-security-policy')"
check "no MIME sniffing" "$(printf '%s' "$headers" | grep -i '^x-content-type-options: nosniff')"
check "framing denied" "$(printf '%s' "$headers" | grep -i '^x-frame-options: DENY')"

api_headers="$(curl -fsSI --max-time 15 "$BASE/api/health" 2>/dev/null)"
check "API responses are not cached" "$(printf '%s' "$api_headers" | grep -i '^cache-control:.*no-store')"
check "request IDs returned" "$(printf '%s' "$api_headers" | grep -i '^x-request-id')"

robots="$(curl -fsS --max-time 15 "$BASE/robots.txt" 2>/dev/null)"
check "console asks not to be indexed" "$(printf '%s' "$robots" | grep -i 'Disallow: /')"

# A wrong login must not reveal whether the account exists, and must set no cookie
login="$(curl -sS -i --max-time 15 -X POST "$BASE/api/auth/login" \
  -H 'Content-Type: application/json' \
  -d '{"email":"nobody@example.invalid","password":"definitely-not-a-password","use_cookie":true}' 2>/dev/null)"
check "unknown login is refused" "$(printf '%s' "$login" | grep -i 'invalid_credentials')"
check "no cookie set on a failed login" "$([ -z "$(printf '%s' "$login" | grep -i '^set-cookie')" ] && echo yes)"

# Verification links must never appear in a production deployment's responses
register="$(curl -sS --max-time 15 -X POST "$BASE/api/auth/resend-verification" \
  -H 'Content-Type: application/json' -d '{"email":"nobody@example.invalid"}' 2>/dev/null)"
check "resend-verification answers the same for unknown addresses" "$(printf '%s' "$register" | grep -i 'is on its way')"
check "no verification link in the response" "$([ -z "$(printf '%s' "$register" | grep -o 'verify-email?token=')" ] && echo yes)"

echo
if [ "$fails" -eq 0 ]; then
  echo "All checks passed."
else
  echo "$fails check(s) failed."
fi
exit $((fails > 0))

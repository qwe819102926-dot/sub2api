#!/bin/sh
# =============================================================================
# Sub2API smoke test
#
# Build-free runtime smoke test: starts Postgres + Redis + the given Sub2API
# image in Docker, waits for it to become healthy, then exercises the health,
# setup, frontend and admin login/authenticated endpoints.
#
# Usage:
#   deploy/smoke-test.sh [IMAGE] [HOST_PORT]
#
# Examples:
#   deploy/smoke-test.sh sub2api:smoke
#   deploy/smoke-test.sh ghcr.io/qwe819102926-dot/sub2api:dev 8081
#
# Exit code 0 = all checks passed; non-zero = failure.
# =============================================================================

set -eu

IMAGE="${1:-sub2api:smoke}"
HOST_PORT="${2:-8080}"

NET="sub2api-smoke-$$"
PREFIX="sub2api-smoke"
BASE="http://127.0.0.1:${HOST_PORT}"

ADMIN_EMAIL="smoke@sub2api.test"
ADMIN_PASSWORD="SmokeTest#2026pass"

cleanup() {
  docker rm -f "${PREFIX}-sub2api" "${PREFIX}-postgres" "${PREFIX}-redis" >/dev/null 2>&1 || true
  docker network rm "${NET}" >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

say() { printf '\n==> %s\n' "$*"; }

# ---------------------------------------------------------------------------
# Start dependencies
# ---------------------------------------------------------------------------
say "Creating network ${NET}"
docker network create "${NET}" >/dev/null

say "Starting Postgres"
docker run -d --name "${PREFIX}-postgres" --network "${NET}" \
  -e POSTGRES_USER=sub2api \
  -e POSTGRES_PASSWORD=smoke_pass \
  -e POSTGRES_DB=sub2api \
  postgres:18-alpine >/dev/null

say "Starting Redis"
docker run -d --name "${PREFIX}-redis" --network "${NET}" \
  redis:8-alpine >/dev/null

say "Waiting for Postgres to be ready"
i=0
until docker exec "${PREFIX}-postgres" pg_isready -U sub2api -d sub2api >/dev/null 2>&1; do
  i=$((i + 1))
  if [ "$i" -ge 90 ]; then
    echo "Postgres did not become ready in time" >&2
    docker logs "${PREFIX}-postgres" >&2 || true
    exit 1
  fi
  sleep 2
done

say "Waiting for Redis to be ready"
i=0
until [ "$(docker exec "${PREFIX}-redis" redis-cli ping 2>/dev/null)" = "PONG" ]; do
  i=$((i + 1))
  if [ "$i" -ge 30 ]; then
    echo "Redis did not become ready in time" >&2
    docker logs "${PREFIX}-redis" >&2 || true
    exit 1
  fi
  sleep 2
done

# ---------------------------------------------------------------------------
# Start Sub2API
# ---------------------------------------------------------------------------
say "Starting Sub2API (${IMAGE})"
docker run -d --name "${PREFIX}-sub2api" --network "${NET}" \
  -p "${HOST_PORT}:8080" \
  -e AUTO_SETUP=true \
  -e SERVER_HOST=0.0.0.0 \
  -e SERVER_PORT=8080 \
  -e SERVER_MODE=release \
  -e RUN_MODE=standard \
  -e DATABASE_HOST="${PREFIX}-postgres" \
  -e DATABASE_PORT=5432 \
  -e DATABASE_USER=sub2api \
  -e DATABASE_PASSWORD=smoke_pass \
  -e DATABASE_DBNAME=sub2api \
  -e DATABASE_SSLMODE=disable \
  -e REDIS_HOST="${PREFIX}-redis" \
  -e REDIS_PORT=6379 \
  -e REDIS_PASSWORD= \
  -e REDIS_DB=0 \
  -e ADMIN_EMAIL="${ADMIN_EMAIL}" \
  -e ADMIN_PASSWORD="${ADMIN_PASSWORD}" \
  -e JWT_SECRET="smoke-test-jwt-secret-not-for-production" \
  -e TZ=UTC \
  "${IMAGE}" >/dev/null

say "Waiting for /health to return 200"
i=0
until curl -fsS -o /dev/null "${BASE}/health" 2>/dev/null; do
  i=$((i + 1))
  if [ "$i" -ge 90 ]; then
    echo "Sub2API did not become healthy in time" >&2
    echo "--- container logs ---" >&2
    docker logs "${PREFIX}-sub2api" >&2 || true
    exit 1
  fi
  sleep 2
done

# ---------------------------------------------------------------------------
# Smoke checks
# ---------------------------------------------------------------------------
fail() {
  echo "FAILED: $*" >&2
  echo "--- Sub2API logs ---" >&2
  docker logs "${PREFIX}-sub2api" >&2 || true
  exit 1
}

say "Check 1/5: GET /health"
body="$(curl -fsS "${BASE}/health")"
echo "  ${body}"
echo "${body}" | grep -q '"status":"ok"' || fail "/health response: ${body}"

say "Check 2/5: GET /setup/status (auto setup completed)"
body="$(curl -fsS "${BASE}/setup/status")"
echo "  ${body}"
echo "${body}" | grep -q '"needs_setup":false' || fail "/setup/status response: ${body}"

say "Check 3/5: GET / (embedded frontend)"
body="$(curl -fsS "${BASE}/")"
printf '%s' "${body}" | grep -qi '<!doctype html' || fail "/ does not serve frontend HTML"
echo "  frontend HTML served ($(printf '%s' "${body}" | wc -c) bytes)"

say "Check 4/5: POST /api/v1/auth/login with admin"
login="$(curl -fsS -X POST "${BASE}/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"${ADMIN_EMAIL}\",\"password\":\"${ADMIN_PASSWORD}\"}")"
echo "  ${login}"
echo "${login}" | grep -q '"code":0' || fail "login response: ${login}"
token="$(printf '%s' "${login}" | sed -n 's/.*"access_token":"\([^"]*\)".*/\1/p')"
if [ -z "${token}" ]; then
  fail "login response did not contain access_token"
fi

say "Check 5/5: GET /api/v1/user/profile (authenticated)"
profile="$(curl -fsS "${BASE}/api/v1/user/profile" -H "Authorization: Bearer ${token}")"
echo "  ${profile}"
echo "${profile}" | grep -q '"code":0' || fail "authenticated request failed: ${profile}"

say "ALL SMOKE CHECKS PASSED"
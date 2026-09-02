#!/usr/bin/env bash
set -Eeuo pipefail

DEPLOY_DIR="${DEPLOY_DIR:-/opt/sub2api-deploy}"
ENV_FILE="${ENV_FILE:-${DEPLOY_DIR}/.env}"
CADDY_CONFIG="${CADDY_CONFIG:-${DEPLOY_DIR}/Caddyfile}"
PUBLIC_HEALTH_URL="${PUBLIC_HEALTH_URL:-}"
IMAGE="${SUB2API_IMAGE:-}"
BLUE_PORT="${BLUE_PORT:-8080}"
GREEN_PORT="${GREEN_PORT:-18080}"
DRAIN_SECONDS="${DRAIN_SECONDS:-120}"
KEEP_OLD=0

usage() {
  cat <<'EOF'
Usage: blue-green-update.sh --image IMAGE --public-health-url URL [options]
  --image IMAGE             Immutable image tag, preferably sha-<commit>
  --public-health-url URL   Public /health URL checked after the switch
  --deploy-dir DIR          Deployment directory (default: /opt/sub2api-deploy)
  --env-file FILE           Environment file (default: DIR/.env)
  --caddy-config FILE       Caddyfile to switch (default: DIR/Caddyfile)
  --blue-port PORT          Existing application host port (default: 8080)
  --green-port PORT         Temporary green host port (default: 18080)
  --drain-seconds N         Keep blue alive before stopping it (default: 120)
  --keep-old                Leave blue running for manual rollback
EOF
}

die() { echo "ERROR: $*" >&2; exit 1; }

while (($#)); do
  case "$1" in
    --image) IMAGE="${2:?missing value for --image}"; shift 2 ;;
    --public-health-url) PUBLIC_HEALTH_URL="${2:?missing value for --public-health-url}"; shift 2 ;;
    --deploy-dir) DEPLOY_DIR="${2:?missing value for --deploy-dir}"; ENV_FILE="${DEPLOY_DIR}/.env"; CADDY_CONFIG="${DEPLOY_DIR}/Caddyfile"; shift 2 ;;
    --env-file) ENV_FILE="${2:?missing value for --env-file}"; shift 2 ;;
    --caddy-config) CADDY_CONFIG="${2:?missing value for --caddy-config}"; shift 2 ;;
    --blue-port) BLUE_PORT="${2:?missing value for --blue-port}"; shift 2 ;;
    --green-port) GREEN_PORT="${2:?missing value for --green-port}"; shift 2 ;;
    --drain-seconds) DRAIN_SECONDS="${2:?missing value for --drain-seconds}"; shift 2 ;;
    --keep-old) KEEP_OLD=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) die "unknown option: $1" ;;
  esac
done

[[ -n "$IMAGE" ]] || die "--image is required"
[[ -n "$PUBLIC_HEALTH_URL" ]] || die "--public-health-url is required"
[[ "$DRAIN_SECONDS" =~ ^[0-9]+$ ]] || die "--drain-seconds must be a non-negative integer"
[[ -f "$ENV_FILE" ]] || die "environment file not found: $ENV_FILE"
[[ -f "$CADDY_CONFIG" ]] || die "Caddyfile not found: $CADDY_CONFIG"
for command in docker curl python3 caddy flock; do
  command -v "$command" >/dev/null 2>&1 || die "required command not found: $command"
done

mkdir -p "$DEPLOY_DIR/.blue-green"
cd "$DEPLOY_DIR"
exec 9>"$DEPLOY_DIR/.blue-green-update.lock"
flock -n 9 || die "another blue-green deployment is already running"

TS="${TS:-$(date +%Y%m%d_%H%M%S)}"
WORKDIR="$DEPLOY_DIR/.blue-green/$TS"
STATE_FILE="$DEPLOY_DIR/.blue-green-active"
mkdir -p "$WORKDIR"
chmod 700 "$WORKDIR"

ACTIVE_CONTAINER=""
ACTIVE_PORT=""
if [[ -f "$STATE_FILE" ]]; then
  ACTIVE_CONTAINER="$(sed -n 's/^container=//p' "$STATE_FILE" | head -n 1)"
  ACTIVE_PORT="$(sed -n 's/^port=//p' "$STATE_FILE" | head -n 1)"
fi
if [[ -z "$ACTIVE_CONTAINER" ]]; then
  ACTIVE_CONTAINER="sub2api"
  ACTIVE_PORT="$BLUE_PORT"
fi
[[ -n "$ACTIVE_PORT" ]] || die "active port missing from $STATE_FILE"
docker inspect "$ACTIVE_CONTAINER" >/dev/null 2>&1 || die "active container not found: $ACTIVE_CONTAINER"
[[ "$(docker inspect -f '{{.State.Running}}' "$ACTIVE_CONTAINER")" == "true" ]] || die "active container is not running"
[[ "$ACTIVE_PORT" != "$GREEN_PORT" ]] || die "green port must differ from active port"

OLD_STATE=""
if [[ -f "$STATE_FILE" ]]; then
  OLD_STATE="$WORKDIR/active-state.backup"
  cp "$STATE_FILE" "$OLD_STATE"
fi
cp "$CADDY_CONFIG" "$WORKDIR/caddy.backup"
cp "$ENV_FILE" "$WORKDIR/env.backup"
docker pull "$IMAGE"

GREEN_CONTAINER="sub2api-green-$TS"
PROXY_EDITED=0
COMPLETED=0
NETWORK="$(docker inspect -f '{{range $k, $v := .NetworkSettings.Networks}}{{$k}}{{end}}' "$ACTIVE_CONTAINER")"
[[ -n "$NETWORK" ]] || die "active container has no Docker network"
docker inspect -f '{{range .Config.Env}}{{println .}}{{end}}' "$ACTIVE_CONTAINER" > "$WORKDIR/green.env"
chmod 600 "$WORKDIR/green.env"

rollback() {
  status=$?
  trap - EXIT
  set +e
  if [[ "$COMPLETED" == "1" ]]; then
    exit "$status"
  fi
  if [[ "$PROXY_EDITED" == "1" ]]; then
    cp "$WORKDIR/caddy.backup" "$CADDY_CONFIG"
    caddy validate --config "$CADDY_CONFIG" --adapter caddyfile >/dev/null 2>&1 && \
      caddy reload --config "$CADDY_CONFIG" --adapter caddyfile >/dev/null 2>&1 || true
  fi
  cp "$WORKDIR/env.backup" "$ENV_FILE" 2>/dev/null || true
  if [[ -n "$OLD_STATE" && -f "$OLD_STATE" ]]; then cp "$OLD_STATE" "$STATE_FILE"; else rm -f "$STATE_FILE"; fi
  docker rm -f "$GREEN_CONTAINER" >/dev/null 2>&1 || true
  docker start "$ACTIVE_CONTAINER" >/dev/null 2>&1 || true
  echo "BLUE_GREEN_ROLLBACK=completed" >&2
  exit "$status"
}
trap rollback EXIT

docker run -d --name "$GREEN_CONTAINER" --restart unless-stopped \
  --security-opt no-new-privileges:true \
  --publish "127.0.0.1:$GREEN_PORT:8080" \
  --network "$NETWORK" \
  --volumes-from "$ACTIVE_CONTAINER" \
  --env-file "$WORKDIR/green.env" \
  "$IMAGE" >/dev/null

GREEN_HEALTH="$WORKDIR/green-health.txt"
GREEN_OK=0
for _ in $(seq 1 24); do
  if curl -fsS --max-time 5 "http://127.0.0.1:$GREEN_PORT/health" > "$GREEN_HEALTH"; then GREEN_OK=1; break; fi
  sleep 5
done
[[ "$GREEN_OK" == "1" ]] || { docker logs --tail 160 "$GREEN_CONTAINER" >&2 || true; die "green health check failed"; }

python3 - "$CADDY_CONFIG" "$ACTIVE_PORT" "$GREEN_PORT" <<'PY'
from pathlib import Path
import re
import sys
path = Path(sys.argv[1])
old_port, new_port = sys.argv[2:]
text = path.read_text(encoding="utf-8")
pattern = re.compile(r"((?:127\.0\.0\.1|localhost):)" + re.escape(old_port) + r"\b")
updated, count = pattern.subn(r"\g<1>" + new_port, text)
if count != 1:
    raise SystemExit(f"expected exactly one active Caddy upstream on port {old_port}, found {count}")
path.write_text(updated, encoding="utf-8")
PY
PROXY_EDITED=1
caddy validate --config "$CADDY_CONFIG" --adapter caddyfile
caddy reload --config "$CADDY_CONFIG" --adapter caddyfile

PUBLIC_HEALTH="$WORKDIR/public-health.txt"
PUBLIC_OK=0
for _ in $(seq 1 6); do
  if curl -fsS --max-time 10 "$PUBLIC_HEALTH_URL" > "$PUBLIC_HEALTH"; then PUBLIC_OK=1; break; fi
  sleep 3
done
[[ "$PUBLIC_OK" == "1" ]] || die "public health check failed after Caddy switch"

python3 - "$ENV_FILE" "$IMAGE" <<'PY'
from pathlib import Path
import sys
path = Path(sys.argv[1])
image = sys.argv[2]
lines = path.read_text(encoding="utf-8").splitlines()
for index, line in enumerate(lines):
    if line.startswith("SUB2API_IMAGE="):
        lines[index] = "SUB2API_IMAGE=" + image
        break
else:
    lines.append("SUB2API_IMAGE=" + image)
path.write_text("\n".join(lines) + "\n", encoding="utf-8")
PY

printf 'container=%s\nport=%s\nimage=%s\nupdated_at=%s\n' \
  "$GREEN_CONTAINER" "$GREEN_PORT" "$IMAGE" "$TS" > "$WORKDIR/active-state"
mv "$WORKDIR/active-state" "$STATE_FILE"

if [[ "$KEEP_OLD" == "1" ]]; then
  echo "old container kept for manual rollback: $ACTIVE_CONTAINER"
else
  echo "draining old container for $DRAIN_SECONDS""s"
  sleep "$DRAIN_SECONDS"
  docker stop --time 30 "$ACTIVE_CONTAINER" >/dev/null
fi
COMPLETED=1
echo "BLUE_GREEN_SUCCESS=1"
echo "ACTIVE_CONTAINER=$GREEN_CONTAINER"
echo "ACTIVE_PORT=$GREEN_PORT"
echo "ACTIVE_IMAGE=$IMAGE"
echo "CADDY_CONFIG=$CADDY_CONFIG"
echo "WORKDIR=$WORKDIR"
printf 'green_health_response='; cat "$GREEN_HEALTH"
printf '\npublic_health_response='; cat "$PUBLIC_HEALTH"; printf '\n'
exit 0

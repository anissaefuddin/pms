#!/usr/bin/env bash
# Turn on the server side of one-time-code sign-in (AUTH-08) over SSH.
#   ./deploy/enable-otp.sh [user@host] [remote_dir]      defaults: lsuser@100.116.210.127  ~/pms
#   --dry-run   show what would change, touch nothing     --rotate   replace an existing key (codes in flight stop working)
# It creates PMS_OTP_KEY on the server (the key never leaves the server and is never printed), makes sure docker-compose.yml passes it
# to the app, restarts only the app, and checks health. The admin still has to switch the feature on in Settings > Access policy.
set -euo pipefail

HOST="lsuser@100.116.210.127"; DIR="~/pms"; DRY=0; ROTATE=0; pos=0
for a in "$@"; do
  case "$a" in
    --dry-run) DRY=1 ;;
    --rotate) ROTATE=1 ;;
    -h|--help) sed -n '2,7p' "$0"; exit 0 ;;
    *) pos=$((pos+1)); [ "$pos" = 1 ] && HOST="$a" || DIR="$a" ;;
  esac
done

echo "== target: $HOST:$DIR (dry-run=$DRY rotate=$ROTATE)"
ssh -o ConnectTimeout=10 -o BatchMode=yes "$HOST" true || { echo "SSH to $HOST failed (key/agent/VPN?)" >&2; exit 1; }

# The remote part runs under bash on the server; values are passed as arguments, not interpolated into the script.
ssh "$HOST" bash -s -- "$DIR" "$DRY" "$ROTATE" <<'REMOTE'
set -euo pipefail
DIR="${1/#\~/$HOME}"; DRY="$2"; ROTATE="$3"
cd "$DIR"
[ -f .env ] && [ -f docker-compose.yml ] || { echo "no .env or docker-compose.yml in $DIR" >&2; exit 1; }
command -v openssl >/dev/null || { echo "openssl is required on the server" >&2; exit 1; }
run() { if [ "$DRY" = 1 ]; then echo "[dry-run] $*"; else "$@"; fi; }

have_key=0
grep -qE '^PMS_OTP_KEY=.+' .env && have_key=1
if [ "$have_key" = 1 ] && [ "$ROTATE" = 0 ]; then
  echo "== PMS_OTP_KEY already set: keeping it (use --rotate to replace)"
else
  echo "== creating PMS_OTP_KEY (not printed)"
  if [ "$DRY" = 0 ]; then
    cp -p .env ".env.bak-otp-$(date +%Y%m%d%H%M%S)"
    key="$(openssl rand -base64 32)"
    # one clean line: drop any old/empty PMS_OTP_KEY line, then append
    grep -vE '^PMS_OTP_KEY=' .env > .env.new || true
    printf 'PMS_OTP_KEY=%s\n' "$key" >> .env.new
    chmod --reference=.env .env.new 2>/dev/null || chmod 600 .env.new
    mv .env.new .env
    unset key
  else
    echo "[dry-run] back up .env, set PMS_OTP_KEY=<32 random bytes base64>"
  fi
fi

if grep -q 'PMS_OTP_KEY' docker-compose.yml; then
  echo "== compose already passes PMS_OTP_KEY"
else
  echo "== adding PMS_OTP_KEY passthrough to docker-compose.yml"
  if [ "$DRY" = 0 ]; then
    cp -p docker-compose.yml docker-compose.yml.bak-otp
    if grep -q 'PMS_CONNECTOR_KEY_PREVIOUS' docker-compose.yml; then
      sed -i.tmp 's#^\([[:space:]]*\)PMS_CONNECTOR_KEY_PREVIOUS: .*#&\n\1PMS_OTP_KEY: ${PMS_OTP_KEY:-}#' docker-compose.yml && rm -f docker-compose.yml.tmp
    else
      echo "cannot find the PMS_CONNECTOR_KEY_PREVIOUS anchor line; add 'PMS_OTP_KEY: \${PMS_OTP_KEY:-}' under app > environment by hand" >&2; exit 1
    fi
  else
    echo "[dry-run] add PMS_OTP_KEY: \${PMS_OTP_KEY:-} under app > environment"
  fi
fi

run docker compose config --quiet
echo "== restarting app only"
run docker compose up -d app

if [ "$DRY" = 0 ]; then
  for _ in $(seq 1 30); do
    st="$(docker inspect -f '{{.State.Health.Status}}' "$(docker compose ps -q app)" 2>/dev/null || true)"
    [ "$st" = healthy ] && break; sleep 2
  done
  echo "== health: ${st:-unknown}"
  port="$(docker compose port app 8080 2>/dev/null | sed 's/.*://')"; port="${port:-8086}"
  echo "== /healthz: $(curl -s -o /dev/null -w '%{http_code}' "http://localhost:$port/healthz")"
  if docker compose logs --since 2m app 2>&1 | grep -q 'PMS_OTP_KEY is not set'; then
    echo "FAIL: the app still logs 'PMS_OTP_KEY is not set' (key did not reach the container)" >&2; exit 1
  fi
  echo "== log check: no 'PMS_OTP_KEY is not set' warning"
  code="$(curl -s -o /dev/null -w '%{http_code}' -X POST -H 'Content-Type: application/json' -H "Origin: $(grep -E '^PUBLIC_URL=' .env | cut -d= -f2-)" -d '{"email":"nobody@example.invalid"}' "http://localhost:$port/api/v1/auth/otp/request")"
  echo "== POST /auth/otp/request -> $code (404 = key present, feature still off; 503 = key missing)"
  [ "$code" != 503 ] || { echo "FAIL: endpoint still reports the key as missing" >&2; exit 1; }
fi
echo "== done. Next: an admin turns on 'Sign in with a code' in Settings > Access policy."
REMOTE

#!/bin/sh
# Read-only operational check for one PMS stack (PROMPT #033, OPS-04). It reports and never changes anything, and it sends no
# alert by itself: wire the exit code into the approved monitor once a recipient exists.
#   ./ops-check.sh            print one line per check; exit 1 when any check fails
# Checks: readiness, container health and restarts, disk use of the data volume, PostgreSQL, age and checksums of the newest backup
# set, and application ERROR lines in the last 15 minutes.
set -u

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$script_dir"
set -a
[ ! -f .env ] || . ./.env
set +a
backup_dir=${BACKUP_DIR:-./backups}
fail=0
report() { printf '%-22s %s %s\n' "$1" "$2" "$3"; [ "$2" = OK ] || fail=1; }

# 1. readiness through the local port (the public route is checked by the monitor)
code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 5 "http://127.0.0.1:${PMS_CHECK_PORT:-8086}/api/v1/readyz" || echo 000)
[ "$code" = 200 ] && report readiness OK "http=$code" || report readiness FAIL "http=$code"

# 2. container health and restarts
health=$(docker inspect pms-app-1 --format '{{.State.Health.Status}} {{.RestartCount}}' 2>/dev/null || echo "missing 0")
set -- $health
[ "$1" = healthy ] && [ "$2" = 0 ] && report container OK "health=$1 restarts=$2" || report container FAIL "health=$1 restarts=$2"

# 3. disk use of the root volume (warn at 85 percent)
used=$(df -P / | awk 'NR==2 {gsub("%","",$5); print $5}')
[ "${used:-100}" -lt 85 ] && report disk OK "used=${used}%" || report disk FAIL "used=${used:-?}%"

# 4. PostgreSQL availability
docker exec pms-postgres-1 pg_isready -U pms -d pms >/dev/null 2>&1 && report postgres OK "accepting connections" || report postgres FAIL "not ready"

# 5. newest backup set: present, age under 26 hours, checksums valid
stamp=$(ls "$backup_dir" 2>/dev/null | sed -n 's/^\([0-9]\{8\}T[0-9]\{6\}Z\)-manifest\.txt$/\1/p' | sort | tail -n 1)
if [ -z "$stamp" ]; then
  report backup FAIL "no backup set"
else
  epoch=$(date -u -d "$(printf '%s' "$stamp" | sed 's/^\(....\)\(..\)\(..\)T\(..\)\(..\)\(..\)Z$/\1-\2-\3 \4:\5:\6/')" +%s 2>/dev/null || echo 0)
  age_h=$(( ( $(date -u +%s) - epoch ) / 3600 ))
  if (cd "$backup_dir" && sha256sum -c "$stamp-SHA256SUMS" >/dev/null 2>&1); then sums=valid; else sums=INVALID; fi
  if [ "$epoch" -gt 0 ] && [ "$age_h" -lt 26 ] && [ "$sums" = valid ]; then report backup OK "set=$stamp age_h=$age_h checksums=$sums"
  else report backup FAIL "set=$stamp age_h=$age_h checksums=$sums"; fi
fi

# 6. application errors in the last 15 minutes (counts only, never the messages)
errors=$(docker logs --since 15m pms-app-1 2>&1 | grep -c ' ERROR ' || true)
[ "${errors:-0}" -eq 0 ] && report app-errors OK "count=0" || report app-errors WARN "count=$errors"

exit "$fail"

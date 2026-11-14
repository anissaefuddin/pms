#!/bin/sh
# Restore drill that never touches live data: restore the newest (or given) backup set into a scratch database
# of this stack's Postgres, check it, drop it. Run it from cron/systemd (see systemd/) and read the exit code.
#   ./restore-test.sh [BACKUP_TIMESTAMP]
# Checks: dump restores, every table of the live schema exists, the files archive is readable, row counts are
# printed (and compared with STAMP-counts.txt when migrate-pilot.sh wrote one). Writes a Prometheus metric when
# BACKUP_TEXTFILE_DIR is set.
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$script_dir"
. ./lib.sh
set -a
[ ! -f .env ] || . ./.env
set +a
backup_dir=${BACKUP_DIR:-./backups}

if [ "$#" -ge 1 ]; then stamp=$1; else
  stamp=$(ls "$backup_dir" 2>/dev/null | sed -n 's/^\([0-9]\{8\}T[0-9]\{6\}Z\)-manifest\.txt$/\1/p' | sort | tail -n 1)
fi
printf '%s\n' "$stamp" | grep -Eq '^[0-9]{8}T[0-9]{6}Z$' || { echo "No backup set found in $backup_dir." >&2; exit 1; }
db_dump=$backup_dir/$stamp-database.dump
files_dump=$backup_dir/$stamp-files.tar.gz
[ -f "$db_dump" ] && [ -f "$files_dump" ] || { echo "Backup set incomplete: $stamp" >&2; exit 1; }

scratch=pms_restoretest_$$
psql_admin() { docker compose exec -T postgres psql -U pms -d postgres -v ON_ERROR_STOP=1 "$@"; }
alert() { [ -z "${BACKUP_ALERT_URL:-}" ] || curl -fsS --connect-timeout 5 --max-time 10 -X POST -H 'Content-Type: application/json' --data "{\"text\":\"PMS restore test FAILED on $(hostname) for $stamp\"}" "$BACKUP_ALERT_URL" >/dev/null || true; }
cleanup() { rc=$?; psql_admin -c "DROP DATABASE IF EXISTS $scratch" >/dev/null 2>&1 || true; [ "$rc" -eq 0 ] || { echo "Restore test FAILED for $stamp." >&2; alert; }; }
trap cleanup HUP INT TERM EXIT

start=$(date +%s)
psql_admin -c "CREATE DATABASE $scratch" >/dev/null
docker compose exec -T postgres pg_restore -U pms -d "$scratch" --no-owner --no-privileges --exit-on-error < "$db_dump"
tar -tzf "$files_dump" >/dev/null
restored=$(entity_counts "$scratch")
live=$(entity_counts pms)
missing=$(printf '%s\n%s\n' "$restored" "$live" | awk '{print $1}' | sort | uniq -u)
[ -z "$missing" ] || { echo "Tables differ between the restore and the live schema: $missing" >&2; exit 1; }
users=$(printf '%s\n' "$restored" | awk '$1=="users"{print $2}')
[ "${users:-0}" -gt 0 ] || { echo "Restored database has no users." >&2; exit 1; }
if [ -f "$backup_dir/$stamp-counts.txt" ] && [ "$(grep -v '^__' "$backup_dir/$stamp-counts.txt")" != "$restored" ]; then
  echo "Row counts differ from $stamp-counts.txt (taken at export time)." >&2; exit 1
fi
printf '%s\n' "$restored"
if [ -n "${BACKUP_TEXTFILE_DIR:-}" ]; then
  mkdir -p "$BACKUP_TEXTFILE_DIR"
  printf '%s\n' '# TYPE pms_restore_test_last_success_timestamp_seconds gauge' "pms_restore_test_last_success_timestamp_seconds $(date +%s)" > "$BACKUP_TEXTFILE_DIR/pms_restore_test.prom.tmp"
  mv "$BACKUP_TEXTFILE_DIR/pms_restore_test.prom.tmp" "$BACKUP_TEXTFILE_DIR/pms_restore_test.prom"
fi
printf 'Restore test passed for %s in %s s (users=%s).\n' "$stamp" "$(( $(date +%s) - start ))" "$users"

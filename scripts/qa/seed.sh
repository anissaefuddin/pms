#!/bin/sh
# seed.sh empty [email]        : DB kosong lalu `pms bootstrap` (CLI) -> admin + tautan undangan.
# seed.sh restore <dump.pgc>   : pg_restore dump (-Fc) ke DB pms (database dikosongkan dulu, hanya instance lokal ini).
# seed.sh bootstrap [email]    : jalankan bootstrap lagi TANPA mengosongkan DB (uji idempotensi).
# seed.sh sql -c "..."         : psql ke DB instance lokal.
# seed.sh backup <dir> | files-restore <tgz> : drill backup/restore.
# seed.sh migrate              : jalankan app sekali agar migrasi maju terpasang (log migrasi).
set -eu; . "$(dirname "$0")/common.sh"
PSQL="$DC exec -T postgres psql -U pms -d pms -v ON_ERROR_STOP=1"
case "${1:-}" in
  empty)
    $DC exec -T postgres psql -U pms -d postgres -c "DROP DATABASE IF EXISTS pms WITH (FORCE)" -c "CREATE DATABASE pms OWNER pms" >/dev/null
    $DC run --rm --no-deps app bootstrap --account "QA Local" --admin-email "${2:-admin@qa-local.test}" ;;
  restore)
    [ -f "${2:-}" ] || { echo "pakai: seed.sh restore <dump>" >&2; exit 2; }
    $DC exec -T postgres psql -U pms -d postgres -c "DROP DATABASE IF EXISTS pms WITH (FORCE)" -c "CREATE DATABASE pms OWNER pms" >/dev/null
    $DC exec -T postgres pg_restore -U pms -d pms --no-owner --exit-on-error < "$2" ;;
  backup) # seed.sh backup <dir> : pg_dump -Fc + tar volume files, cetak durasi (RPO/RTO drill)
    d="${2:?pakai: seed.sh backup <dir>}"; mkdir -p "$d"
    $DC exec -T postgres pg_dump -U pms -d pms -Fc > "$d/database.dump"
    docker run --rm -v "pms-qa-${QA_INSTANCE}_files:/data:ro" -v "$(cd "$d" && pwd):/out" alpine:3.23 tar -czf /out/files.tar.gz -C /data . ;;
  files-restore) # seed.sh files-restore <files.tar.gz> : kosongkan volume files instance ini lalu ekstrak
    f="${2:?pakai: seed.sh files-restore <tgz>}"
    docker run --rm -v "pms-qa-${QA_INSTANCE}_files:/data" -v "$(cd "$(dirname "$f")" && pwd):/in:ro" alpine:3.23 sh -c "rm -rf /data/* /data/.[!.]* 2>/dev/null; tar -xzf /in/$(basename "$f") -C /data && chown -R 100:101 /data 2>/dev/null; true" ;;
  bootstrap) $DC run --rm --no-deps app bootstrap --account "QA Local" --admin-email "${2:-admin@qa-local.test}" ;;
  sql) shift; $PSQL "$@" ;;
  migrate) $DC run --rm --no-deps app migrate ;;
  *) echo "pakai: seed.sh empty|bootstrap|sql|restore <dump>|migrate" >&2; exit 2 ;;
esac

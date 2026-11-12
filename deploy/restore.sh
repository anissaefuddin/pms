#!/bin/sh
set -eu

if [ "$#" -ne 2 ] || [ "$2" != "--force" ]; then
  echo "Usage: $0 BACKUP_TIMESTAMP --force" >&2
  exit 2
fi

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$script_dir"
. ./lib.sh
set -a
[ ! -f .env ] || . ./.env
set +a
backup_dir=${BACKUP_DIR:-./backups}
stamp=$1
db_dump=$backup_dir/$stamp-database.dump
files_dump=$backup_dir/$stamp-files.tar.gz
[ -f "$db_dump" ] && [ -f "$files_dump" ] || { echo "Backup set not found: $stamp" >&2; exit 1; }

# 1. Prove both archives are readable BEFORE anything is changed.
docker compose exec -T postgres pg_restore -U pms --list < "$db_dump" >/dev/null || { echo "Database dump is unreadable; nothing was changed." >&2; exit 1; }
gzip -t "$files_dump" || { echo "Files archive is corrupt; nothing was changed." >&2; exit 1; }

# 2. Restore the database into a scratch database; the live one is only replaced after this succeeds.
psql_admin() { docker compose exec -T postgres psql -U pms -d postgres -v ON_ERROR_STOP=1 "$@"; }
psql_admin -c 'DROP DATABASE IF EXISTS pms_restore' -c 'CREATE DATABASE pms_restore' >/dev/null
if ! docker compose exec -T postgres pg_restore -U pms -d pms_restore --no-owner --no-privileges < "$db_dump"; then
  psql_admin -c 'DROP DATABASE IF EXISTS pms_restore' >/dev/null
  echo "pg_restore failed; the live database was not touched." >&2
  exit 1
fi

# 3. Swap. The old database stays as pms_old until the app is healthy again.
docker compose stop app
psql_admin -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname IN ('pms','pms_restore') AND pid <> pg_backend_pid()" >/dev/null
psql_admin -c 'DROP DATABASE IF EXISTS pms_old' -c 'ALTER DATABASE pms RENAME TO pms_old' -c 'ALTER DATABASE pms_restore RENAME TO pms' >/dev/null
docker compose --profile ops run --rm restore sh -c 'find /data/files -mindepth 1 -delete && tar -xzf "/backups/'"$stamp"'-files.tar.gz" -C /data/files'
docker compose up -d app
if ! wait_ready; then
  docker compose stop app
  psql_admin -c 'DROP DATABASE pms' -c 'ALTER DATABASE pms_old RENAME TO pms' >/dev/null
  echo "Restore FAILED health check; the previous database was put back (files were already replaced)." >&2
  exit 1
fi
psql_admin -c 'DROP DATABASE IF EXISTS pms_old' >/dev/null
printf 'Restore completed and health check passed: %s\n' "$stamp"

#!/bin/sh
set -eu

if [ "$#" -ne 2 ] || [ "$2" != "--force" ]; then
  echo "Usage: $0 BACKUP_TIMESTAMP --force" >&2
  exit 2
fi
stamp=$1
if ! printf '%s\n' "$stamp" | grep -Eq '^[0-9]{8}T[0-9]{6}Z$'; then
  echo "Invalid backup timestamp." >&2
  exit 2
fi

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$script_dir"
. ./lib.sh
set -a
[ ! -f .env ] || . ./.env
set +a
export_image_identity "${PMS_IMAGE:-}"
backup_dir=${BACKUP_DIR:-./backups}
db_dump=$backup_dir/$stamp-database.dump
files_dump=$backup_dir/$stamp-files.tar.gz
[ -f "$db_dump" ] && [ -f "$files_dump" ] || { echo "Backup set not found: $stamp" >&2; exit 1; }

verify_checksums "$backup_dir" "$stamp" || exit 1
docker compose exec -T postgres pg_restore -U pms --list < "$db_dump" >/dev/null || { echo "Database dump is unreadable." >&2; exit 1; }
tar -tzf "$files_dump" >/dev/null || { echo "Files archive is unreadable." >&2; exit 1; }

scratch=pms_restore_$$
previous=pms_before_restore_$$
psql_admin() { docker compose exec -T postgres psql -U pms -d postgres -v ON_ERROR_STOP=1 "$@"; }
replace_files() {
  docker compose --profile ops run --rm -T restore sh -ec 'find /data/files -mindepth 1 -delete && tar -xzf - -C /data/files' < "$1"
}
disconnect() {
  psql_admin -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname='pms' AND pid <> pg_backend_pid()" >/dev/null
}

psql_admin -c "CREATE DATABASE $scratch" >/dev/null
if ! docker compose exec -T postgres pg_restore -U pms -d "$scratch" --no-owner --no-privileges < "$db_dump"; then
  psql_admin -c "DROP DATABASE $scratch" >/dev/null
  echo "Database restore failed in the scratch database." >&2
  exit 1
fi

# Preserve the live file volume after stopping its writer, before replacing either store.
rollback_files=$(mktemp "$backup_dir/.pms-rollback.XXXXXX")
if ! docker compose stop app; then
  rm -f "$rollback_files"
  exit 1
fi
if ! docker compose --profile ops run --rm -T backup tar -czf - -C /data/files . > "$rollback_files" ||
   ! tar -tzf "$rollback_files" >/dev/null; then
  docker compose up -d app
  psql_admin -c "DROP DATABASE $scratch" >/dev/null
  rm -f "$rollback_files"
  echo "Could not snapshot the live files. The original data is still in place." >&2
  exit 1
fi
printf 'Rollback data before cutover: database %s, files %s\n' "$previous" "$rollback_files"
if ! disconnect || ! psql_admin -1 -c "ALTER DATABASE pms RENAME TO $previous" -c "ALTER DATABASE $scratch RENAME TO pms" >/dev/null; then
  docker compose up -d app
  psql_admin -c "DROP DATABASE IF EXISTS $scratch" >/dev/null
  rm -f "$rollback_files"
  echo "Could not switch the database. The original data is still in place." >&2
  exit 1
fi

if replace_files "$files_dump" && docker compose up -d app && wait_ready; then
  psql_admin -c "DROP DATABASE $previous" >/dev/null
  rm -f "$rollback_files"
  printf 'Restore completed and health check passed: %s\n' "$stamp"
  exit 0
fi

echo "Restore failed. Rolling back the database and file volume." >&2
if docker compose stop app && disconnect &&
   psql_admin -1 -c "ALTER DATABASE pms RENAME TO $scratch" -c "ALTER DATABASE $previous RENAME TO pms" >/dev/null &&
   replace_files "$rollback_files" && docker compose up -d app && wait_ready; then
  psql_admin -c "DROP DATABASE $scratch" >/dev/null
  rm -f "$rollback_files"
  echo "The original database and files are back online." >&2
else
  echo "Rollback could not be verified. Leave the app stopped and inspect $scratch, $previous and $rollback_files." >&2
  docker compose stop app || true
fi
exit 1

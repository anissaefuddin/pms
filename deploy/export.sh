#!/bin/sh
# Hand-over bundle for the client (USR-04): full database dump, all stored files and readable CSVs, with checksums.
# Run it while the app is stopped or idle, then run ./backup.sh for the final backup before ./decommission.sh.
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$script_dir"
set -a
[ ! -f .env ] || . ./.env
set +a

stamp=$(date -u +%Y%m%dT%H%M%SZ)
export_dir=${EXPORT_DIR:-./exports}
name=pms-export-$stamp
work=$export_dir/$name
mkdir -p "$work"
trap 'rm -rf "$work"' HUP INT TERM EXIT

docker compose exec -T postgres pg_dump -U pms -d pms -Fc > "$work/database.dump"
docker compose --profile ops run --rm -T backup tar -czf - -C /data/files . > "$work/files.tar.gz"
csv() { docker compose exec -T postgres psql -U pms -d pms -v ON_ERROR_STOP=1 -c "COPY ($2) TO STDOUT WITH CSV HEADER" > "$work/$1.csv"; }
csv users "SELECT email,first_name,last_name,user_type,status,created_at FROM users WHERE id <> '00000000-0000-4000-8000-0000000000e1' ORDER BY email"
csv spaces "SELECT id,name,kind,created_at FROM spaces WHERE deleted_at IS NULL ORDER BY name"
csv items "SELECT i.seq,i.type,i.title,s.name AS status,i.start_date,i.due_date,i.created_at,i.description_text AS description,
  (SELECT string_agg(DISTINCT sp.name, '; ') FROM item_locations l JOIN spaces sp ON sp.id=l.space_id WHERE l.item_id=i.id) AS spaces
  FROM items i LEFT JOIN statuses s ON s.id=i.status_id WHERE i.deleted_at IS NULL ORDER BY i.seq"
cat > "$work/README.txt" <<TXT
PMS data export, created $stamp (UTC).
database.dump  PostgreSQL custom format. Restore: createdb pms && pg_restore --no-owner -d pms database.dump
files.tar.gz   Every uploaded file as stored. Attachment names and item links are in the database.
users.csv, spaces.csv, items.csv  Readable copies of the main records (no passwords or 2FA secrets).
SHA256SUMS     Checksums of the files above: sha256sum -c SHA256SUMS
TXT
(cd "$work" && { sha256sum database.dump files.tar.gz *.csv README.txt 2>/dev/null || shasum -a 256 database.dump files.tar.gz *.csv README.txt; } > SHA256SUMS)
tar -czf "$export_dir/$name.tar.gz" -C "$export_dir" "$name"
rm -rf "$work"
trap - HUP INT TERM EXIT
printf 'Export completed: %s\n' "$export_dir/$name.tar.gz"

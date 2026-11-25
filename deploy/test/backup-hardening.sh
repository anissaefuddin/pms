#!/bin/sh
# Backup/export outputs are private to the operator, and restore/restore-test verify checksums. Docker is stubbed.
set -eu
umask 022 # the usual cron/shell default, under which dumps must still not be world-readable
deploy=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/deploy" "$tmp/bin" "$tmp/data"
for s in backup export restore restore-test migrate-pilot lib; do cp "$deploy/$s.sh" "$tmp/deploy/"; done
echo hello >"$tmp/data/attachment"
cat >"$tmp/bin/docker" <<'STUB'
#!/bin/sh
echo "$*" >>"$STUB_LOG"
case "$*" in
  *pg_dump*) echo dump ;;
  *pg_restore*) cat >/dev/null ;;
  *"backup tar -czf - "*) tar -czf - -C "$STUB_DATA" . ;;
  *"backup tar -czf /backups/"*) # a container writing into the bind mount, with its own umask
    for a in "$@"; do case "$a" in /backups/*) (umask 022; tar -czf "$BACKUP_DIR/${a#/backups/}" -C "$STUB_DATA" .) ;; esac; done ;;
  *"run --rm -T restore"*) cat >/dev/null ;;
  *query_to_xml*) echo "users 1" ;;
  *"COPY ("*) echo "a,b" ;;
esac
STUB
chmod +x "$tmp/bin/docker"
export PATH="$tmp/bin:$PATH" STUB_LOG="$tmp/docker.log" STUB_DATA="$tmp/data" BACKUP_DIR="$tmp/backups" EXPORT_DIR="$tmp/exports" \
  MIGRATION_DIR="$tmp/migration" BACKUP_TEXTFILE_DIR="$tmp/metrics"
cd "$tmp/deploy"

failed=0
check() { if eval "$2" >/dev/null 2>&1; then echo "ok   $1"; else echo "FAIL $1"; failed=1; fi; }
private() { [ -z "$(find "$@" -type f -perm +077 2>/dev/null || find "$@" -type f -perm /077)" ]; }

out=$(./backup.sh 2>/dev/null)
stamp=${out##*Backup completed: }
set_file=$BACKUP_DIR/$stamp
check "backup files are not group/world accessible" 'private "$BACKUP_DIR"'
check "checksum file written and valid" '[ -f "$set_file-SHA256SUMS" ] && (cd "$BACKUP_DIR" && { sha256sum -c "$stamp-SHA256SUMS" || shasum -a 256 -c "$stamp-SHA256SUMS"; })'
check "metric stays readable by node_exporter" '[ "$(ls -l "$BACKUP_TEXTFILE_DIR/pms_backup.prom" | cut -c1-10)" = -rw-r--r-- ]'

./export.sh >/dev/null
check "export bundle is not group/world accessible" 'private "$EXPORT_DIR"'

check "restore-test accepts an intact set" './restore-test.sh "$stamp"'
check "restore accepts an intact set" './restore.sh "$stamp" --force'

cp "$set_file-database.dump" "$tmp/dump.orig"
echo tampered >>"$set_file-database.dump"
check "restore-test rejects a changed dump" '! ./restore-test.sh "$stamp" >"$tmp/out" 2>&1 && grep -qi checksum "$tmp/out"'
: >"$STUB_LOG"
check "restore rejects a changed dump before touching anything" '! ./restore.sh "$stamp" --force && ! grep -Eq "CREATE DATABASE|stop app" "$STUB_LOG"'
cp "$tmp/dump.orig" "$set_file-database.dump"

[ ! -f "$set_file-SHA256SUMS" ] || mv "$set_file-SHA256SUMS" "$tmp/sums"
check "a set that declares checksums but lost them is rejected" '! ./restore-test.sh "$stamp"'
grep -v '^checksums=' "$set_file-manifest.txt" >"$tmp/manifest" && cp "$tmp/manifest" "$set_file-manifest.txt"
check "old set without checksums: restore-test passes with a warning" './restore-test.sh "$stamp" >"$tmp/out" 2>&1 && grep -q "WARNING.*checksum" "$tmp/out"'
check "old set without checksums: restore passes with a warning" './restore.sh "$stamp" --force >"$tmp/out" 2>&1 && grep -q "WARNING.*checksum" "$tmp/out"'

sleep 1 # a new stamp
./migrate-pilot.sh export >/dev/null 2>&1
bundle=$(ls "$MIGRATION_DIR"/*.tar.gz)
check "migration bundle is private and carries the checksums" 'private "$MIGRATION_DIR" && tar -tzf "$bundle" | grep -q -- -SHA256SUMS'
mv "$BACKUP_DIR" "$tmp/source-backups"
# The stub reports no active admin, so the final verify step fails; the restore before it must succeed.
./migrate-pilot.sh import "$bundle" >"$tmp/out" 2>&1 || true
check "migration import restores the bundle with its checksums" 'grep -q "Restore completed" "$tmp/out" && ! grep -q WARNING.*checksum "$tmp/out"'

[ "$failed" -eq 0 ] && echo "PASS backup-hardening" || { echo "FAIL backup-hardening"; exit 1; }

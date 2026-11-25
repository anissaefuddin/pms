#!/bin/sh
# Move a pilot instance to a client instance, in three steps. Run each from the deploy/ directory of that instance.
#   ./migrate-pilot.sh export            SOURCE: stop the app, back up, record entity counts, write
#                                        migration/pms-migration-STAMP.tar.gz (+ .sha256). The source stays STOPPED
#                                        (KEEP_RUNNING=1 leaves it up) so nobody writes to it after the cutover.
#   ./migrate-pilot.sh import BUNDLE     TARGET (stack already up: docker compose up -d): check checksum, restore
#                                        the set with restore.sh, then run verify.
#   ./migrate-pilot.sh verify [COUNTS]   TARGET: compare live counts with the bundle's counts file and check access.
# Rollback plan: docs/PILOT_RUNBOOK.md, "Migrasi pilot ke instance klien".
set -eu
umask 077 # migration bundles hold the whole database

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$script_dir"
. ./lib.sh
set -a
[ ! -f .env ] || . ./.env
set +a
backup_dir=${BACKUP_DIR:-./backups}
migration_dir=${MIGRATION_DIR:-./migration}

write_counts() { # $1 = output file, $2 = files tar for the expected file count
  { entity_counts pms
    echo "__files $(tar -tzf "$2" | grep -vc '/$' || true)"
    echo "__active_admins $(docker compose exec -T postgres psql -U pms -d pms -At -c "SELECT count(*) FROM users WHERE user_type='admin' AND status='active'")"
  } > "$1"
}

verify() {
  expected=$1
  [ -f "$expected" ] || { echo "Counts file not found: $expected" >&2; return 1; }
  bad=0
  live=$(entity_counts pms)
  if [ "$live" != "$(grep -v '^__' "$expected")" ]; then
    echo "ENTITY COUNTS DIFFER (expected | live):" >&2
    printf '%s\n' "$live" | diff "$expected" - | grep '^[<>] [^_]' >&2 || true
    bad=1
  fi
  files=$(docker compose --profile ops run --rm -T backup sh -c 'find /data/files -type f | wc -l' | tr -d ' \r')
  [ "$files" = "$(sed -n 's/^__files //p' "$expected")" ] || { echo "File count differs: live $files, expected $(sed -n 's/^__files //p' "$expected")" >&2; bad=1; }
  admins=$(docker compose exec -T postgres psql -U pms -d pms -At -c "SELECT count(*) FROM users WHERE user_type='admin' AND status='active'")
  [ "$admins" -ge 1 ] || { echo "No active admin in the migrated data: nobody can log in." >&2; bad=1; }
  docker compose exec -T app wget -qO- http://localhost:8080/api/v1/readyz >/dev/null 2>&1 || { echo "App is not ready." >&2; bad=1; }
  # Secrets in the dump are encrypted; without the same keys 2FA / SSO / connectors break silently.
  for pair in "user_totp:PMS_2FA_KEY" "sso_providers:PMS_SSO_KEY" "connector_installs:PMS_CONNECTOR_KEY"; do
    t=${pair%%:*}; k=${pair##*:}
    n=$(printf '%s\n' "$live" | awk -v t="$t" '$1==t{print $2}')
    eval "v=\${$k:-}"
    if [ "${n:-0}" -gt 0 ] && [ -z "$v" ]; then echo "WARNING: $t has $n rows but $k is empty here; use the SAME key as the source." >&2; bad=1; fi
  done
  [ "$bad" -eq 0 ] || { echo "VERIFY FAILED." >&2; return 1; }
  echo "VERIFY OK: counts equal ($(grep -vc '^__' "$expected") tables), $files files, $admins active admin(s), app ready."
  echo "Manual check still required: log in as an admin, open a project, download one attachment (runbook)."
}

case "${1:-}" in
export)
  [ "${KEEP_RUNNING:-0}" = 1 ] || docker compose stop app
  out=$(./backup.sh); printf '%s\n' "$out"
  stamp=$(printf '%s\n' "$out" | sed -n 's/^Backup completed: //p')
  [ -n "$stamp" ] || { echo "Backup failed." >&2; exit 1; }
  write_counts "$backup_dir/$stamp-counts.txt" "$backup_dir/$stamp-files.tar.gz"
  mkdir -p "$migration_dir"
  bundle=$migration_dir/pms-migration-$stamp.tar.gz
  tar -czf "$bundle" -C "$backup_dir" "$stamp-database.dump" "$stamp-files.tar.gz" "$stamp-manifest.txt" "$stamp-counts.txt" "$stamp-SHA256SUMS"
  (cd "$migration_dir" && sha_cmd "pms-migration-$stamp.tar.gz" > "pms-migration-$stamp.tar.gz.sha256")
  printf 'Bundle: %s\nSource app is %s. Copy the bundle AND .sha256 over a secure channel; carry PMS_2FA_KEY / PMS_SSO_KEY / PMS_CONNECTOR_KEY separately (secret store).\n' "$bundle" "$([ "${KEEP_RUNNING:-0}" = 1 ] && echo running || echo stopped)"
  ;;
import)
  bundle=${2:?Usage: $0 import BUNDLE.tar.gz}
  [ -f "$bundle" ] || { echo "Bundle not found: $bundle" >&2; exit 1; }
  if [ -f "$bundle.sha256" ]; then (cd "$(dirname "$bundle")" && sha_cmd -c "$(basename "$bundle").sha256") || { echo "Checksum mismatch." >&2; exit 1; }
  else echo "WARNING: no $bundle.sha256 next to the bundle; integrity not checked." >&2; fi
  # Allowlist the members: the bundle is data from another server, never extract unexpected paths.
  members=$(tar -tzf "$bundle")
  printf '%s\n' "$members" | grep -Eqx '[0-9]{8}T[0-9]{6}Z-(database\.dump|files\.tar\.gz|manifest\.txt|counts\.txt|SHA256SUMS)' || { echo "Unexpected bundle content." >&2; exit 1; }
  [ "$(printf '%s\n' "$members" | grep -Evc '^[0-9]{8}T[0-9]{6}Z-(database\.dump|files\.tar\.gz|manifest\.txt|counts\.txt|SHA256SUMS)$')" -eq 0 ] || { echo "Unexpected bundle content." >&2; exit 1; }
  stamp=$(printf '%s\n' "$members" | sed -n 's/^\([0-9]\{8\}T[0-9]\{6\}Z\)-manifest\.txt$/\1/p')
  [ -n "$stamp" ] || { echo "Bundle has no manifest." >&2; exit 1; }
  mkdir -p "$backup_dir"
  tar -xzf "$bundle" -C "$backup_dir"
  ./restore.sh "$stamp" --force
  verify "$backup_dir/$stamp-counts.txt"
  ;;
verify)
  verify "${2:-$(ls -t "$backup_dir"/*-counts.txt 2>/dev/null | head -n 1)}"
  ;;
*)
  echo "Usage: $0 export | import BUNDLE.tar.gz | verify [COUNTS_FILE]" >&2; exit 2 ;;
esac

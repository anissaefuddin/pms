#!/bin/sh
set -eu
umask 077 # dumps hold every record and password hash: readable only by the user running the backup

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$script_dir"
. ./lib.sh
set -a
[ ! -f .env ] || . ./.env
set +a

backup_dir=${BACKUP_DIR:-./backups}
retention_days=${BACKUP_RETENTION_DAYS:-14}
stamp=$(date -u +%Y%m%dT%H%M%SZ)
mkdir -p "$backup_dir"

alert_failure() {
  # The manifest is written last: without it the set is partial. A failure after it (off-server copy) keeps the good local set.
  [ -f "$backup_dir/$stamp-manifest.txt" ] || rm -f "$backup_dir/$stamp"-*
  [ -z "${BACKUP_ALERT_URL:-}" ] || curl -fsS --connect-timeout 5 --max-time 10 -X POST -H 'Content-Type: application/json' --data "{\"text\":\"PMS backup failed on $(hostname) at $stamp\"}" "$BACKUP_ALERT_URL" >/dev/null || true
}
trap alert_failure HUP INT TERM EXIT

# Database first: files are immutable, so the following file snapshot is consistent with this dump.
# Keep it so: a purge job (S1.7.1) must never delete stored files, or a restore can reference a file missing from this archive. Purge only after the retention window and outside backups.
docker compose exec -T postgres pg_dump -U pms -d pms -Fc > "$backup_dir/$stamp-database.dump"
# Through stdout, not into a mounted directory: the file is then owned by this user with the umask above, not by root as 0644.
docker compose --profile ops run --rm -T backup tar -czf - -C /data/files . > "$backup_dir/$stamp-files.tar.gz"
(cd "$backup_dir" && sha_cmd "$stamp-database.dump" "$stamp-files.tar.gz" > "$stamp-SHA256SUMS")
printf '%s\n' "image=${PMS_IMAGE:-unknown}" "created_at=$stamp" "checksums=sha256" > "$backup_dir/$stamp-manifest.txt"
find "$backup_dir" -type f -mtime "+$retention_days" -delete
# Off-server copy (PRD OPS-02). For encryption, make BACKUP_REMOTE an rclone "crypt" remote. Use a path used only by this instance:
# the remote cleanup below deletes everything in it older than the retention period.
if [ -n "${BACKUP_REMOTE:-}" ]; then
  command -v rclone >/dev/null || { echo "BACKUP_REMOTE is set but rclone is not installed" >&2; exit 1; }
  rclone copy "$backup_dir" "$BACKUP_REMOTE" --include "$stamp-*"
  rclone delete "$BACKUP_REMOTE" --min-age "${retention_days}d"
fi
# Alternative or addition: plain rsync over SSH (BACKUP_RSYNC_DEST=user@host:/path used only by this instance). It mirrors the local
# directory, so the remote keeps the same retention window; --max-delete stops an emptied local dir from wiping the remote.
# BACKUP_RSYNC_SSH: ssh command with the key, e.g. "ssh -i /root/.ssh/pms_backup -o BatchMode=yes". No secrets are stored here.
if [ -n "${BACKUP_RSYNC_DEST:-}" ]; then
  command -v rsync >/dev/null || { echo "BACKUP_RSYNC_DEST is set but rsync is not installed" >&2; exit 1; }
  rsync -a --delete --max-delete="${BACKUP_RSYNC_MAX_DELETE:-20}" --exclude '.pms-rollback.*' -e "${BACKUP_RSYNC_SSH:-ssh -o BatchMode=yes}" "$backup_dir"/ "$BACKUP_RSYNC_DEST"/
fi
if [ -z "${BACKUP_REMOTE:-}" ] && [ -z "${BACKUP_RSYNC_DEST:-}" ]; then
  echo "WARNING: neither BACKUP_REMOTE nor BACKUP_RSYNC_DEST is set; backups exist only on this server." >&2
fi
# Metric for the "backup too old" alert: node_exporter's textfile collector reads this directory.
if [ -n "${BACKUP_TEXTFILE_DIR:-}" ]; then
  (umask 022 && mkdir -p "$BACKUP_TEXTFILE_DIR") # node_exporter runs as another user
  {
    echo '# HELP pms_backup_last_success_timestamp_seconds Unix time of the last successful backup.'
    echo '# TYPE pms_backup_last_success_timestamp_seconds gauge'
    echo "pms_backup_last_success_timestamp_seconds $(date +%s)"
  } > "$BACKUP_TEXTFILE_DIR/pms_backup.prom.tmp"
  chmod 644 "$BACKUP_TEXTFILE_DIR/pms_backup.prom.tmp"
  mv "$BACKUP_TEXTFILE_DIR/pms_backup.prom.tmp" "$BACKUP_TEXTFILE_DIR/pms_backup.prom"
fi
trap - HUP INT TERM EXIT
printf 'Backup completed: %s\n' "$stamp"

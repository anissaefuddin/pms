# Backup and restore

Scripts live in `deploy/` and run on the server host with Docker Compose. Run them from `deploy/`; they read `deploy/.env`.

## Backup: `./backup.sh`
Creates, in `BACKUP_DIR` (default `./backups`), a set stamped `YYYYMMDDTHHMMSSZ`: `-database.dump` (`pg_dump -Fc`), `-files.tar.gz` (stored files) and `-manifest.txt` (image tag, time). The manifest is written last; a set without it is partial and is removed on failure.

| Variable | Default | Meaning |
|---|---|---|
| `BACKUP_DIR` | `./backups` | Where sets are stored |
| `BACKUP_RETENTION_DAYS` | `14` | Local files older than this are deleted |
| `BACKUP_REMOTE` | unset | An `rclone` destination for an off-server copy. Use an rclone `crypt` remote for encryption. Use a path used only by this instance: files there older than the retention period are deleted |
| `BACKUP_ALERT_URL` | unset | POSTed `{"text": ...}` JSON when a backup fails |
| `BACKUP_TEXTFILE_DIR` | unset | Writes `pms_backup_last_success_timestamp_seconds` for node_exporter (alert "backup too old"; see `deploy/monitoring/`) |

Without `BACKUP_REMOTE` the script warns that backups exist only on this server. Schedule it with cron or a systemd timer. **Unverified:** a scheduler is not provided by the repo.

**The encryption keys are not in the backup.** Store `PMS_2FA_KEY`, `PMS_SSO_KEY` and `PMS_CONNECTOR_KEY` separately in a secret store ([encryption-keys.md](encryption-keys.md)).

## Restore: `./restore.sh TIMESTAMP --force`
1. Validates the timestamp format and that both files exist and are readable.
2. Restores the dump into a scratch database, stops the app, snapshots the live file volume, swaps databases by renaming, replaces the files, starts the app and waits for `/api/v1/readyz` (up to 60 s).
3. On success the previous database and rollback archive are removed. On failure it rolls back automatically. If the rollback also fails, leave the app stopped and keep the printed database name and rollback archive path until both stores are checked.

After a restore: check readiness, sign in, download an attachment, and confirm the log has no `... KEY is not set` warning. Restoring to an instance with different keys leaves 2FA, SSO secrets and connector/webhook secrets unreadable.

## Data export for hand-over: `./export.sh`
Writes `exports/pms-export-<stamp>.tar.gz` (dump, files, CSVs of users/spaces/items, `SHA256SUMS`). Treat it as confidential. See `deploy/decommission.sh` for end-of-engagement removal.

## Troubleshooting
- "Backup set not found": give the timestamp only, e.g. `20261114T020000Z`.
- Remote copy failed with "rclone is not installed": install rclone or unset `BACKUP_REMOTE`. The local set is kept.
- Restore says health check failed: read `docker compose logs app`; the script is already rolling back.

Related: [updating.md](updating.md).

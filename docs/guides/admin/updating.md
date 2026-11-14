# Updating to a new version

Run from `deploy/` on the server: `./update.sh ghcr.io/anissaefuddin/pms:VERSION` (use an immutable tag, as `docker-compose.yml` requires `PMS_IMAGE`).

## What the script does
1. Runs `./backup.sh` (the pre-update backup) and records its timestamp.
2. Pulls the new image and starts the app with it.
3. Waits up to 60 s for `/api/v1/readyz`; database migrations run at start-up.
4. On success it reminds you to persist the new tag as `PMS_IMAGE` in `deploy/.env` after validation (the script does not write it).

## Rollback
Migrations only move forward, so the old image cannot just be started again over the new schema. If the app is not ready, the script prints `./restore.sh <stamp> --force`. Because `deploy/.env` still names the old image, the restore brings back the old image with the pre-update data. Anything created after the update is lost ([backup-restore.md](backup-restore.md)).

## After updating
Check, in order: readiness, sign in, the log for `PMS_2FA_KEY`/`PMS_SSO_KEY`/`PMS_CONNECTOR_KEY is not set` warnings, and one realistic workflow. Version-specific checks are in [PILOT_RUNBOOK.md](../../PILOT_RUNBOOK.md).

## Troubleshooting
- Not ready within 60 s: a slow migration can exceed the window. **Unverified:** how long the largest migrations take; read `docker compose logs app` before restoring.
- New settings variables: add them to `deploy/.env` first; `docker-compose.yml` passes the key variables through with empty defaults.

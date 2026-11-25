#!/bin/sh
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$script_dir"
. ./lib.sh
if [ "$#" -ne 1 ]; then echo "Usage: $0 ghcr.io/anissaefuddin/pms:VERSION" >&2; exit 2; fi
new_image=$1
backup_output=$(./backup.sh)
printf '%s\n' "$backup_output"
stamp=$(printf '%s\n' "$backup_output" | sed -n 's/^Backup completed: //p')
PMS_IMAGE=$new_image docker compose pull app
export_image_identity "$new_image"
PMS_IMAGE=$new_image docker compose up -d app
if ! wait_ready; then
  # Migrations only move forward, so the old image cannot simply be started again: restore the pre-deploy backup.
  # deploy/.env still names the old image, so restore.sh brings back the old image together with the old data.
  echo "Update to $new_image FAILED: the app did not become ready within 60 s." >&2
  echo "Check: docker compose logs app. To go back to the previous version and data: ./restore.sh $stamp --force" >&2
  exit 1
fi
printf 'Updated to %s. Persist this tag in deploy/.env after validation.\n' "$new_image"

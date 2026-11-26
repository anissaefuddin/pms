#!/bin/sh
# Build the image from this checkout, tag it with the commit, back up, switch the app to it, wait for health,
# and restore the pre-update backup automatically on failure. update.sh stays for registry images (it pulls).
# Safe to re-run: if the commit image is already the running one, nothing happens.
#   ./update-local.sh                 build HEAD and update
#   PMS_LOCAL_REPO=pms-local          image name (default)
#   ALLOW_DIRTY=1                     allow uncommitted changes (tag gets -dirty-<hash of the diff>)
#   NO_PERSIST=1                      do not write the new tag into deploy/.env
#   BUILD_FILE / BUILD_CONTEXT        Dockerfile and context (default: repo root); used by tests
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$script_dir"
. ./lib.sh
repo=$(CDPATH= cd -- .. && pwd)
set -a
[ ! -f .env ] || . ./.env
set +a
current=${PMS_IMAGE:-}

full_commit=$(git -C "$repo" rev-parse HEAD)
commit=$(git -C "$repo" rev-parse --short=12 HEAD)
tag=$commit
if [ -n "$(git -C "$repo" status --porcelain)" ]; then
  [ "${ALLOW_DIRTY:-0}" = 1 ] || { echo "Working tree has uncommitted changes: commit them or set ALLOW_DIRTY=1." >&2; exit 1; }
  tag=$commit-dirty-$(git -C "$repo" diff HEAD | { sha256sum 2>/dev/null || shasum -a 256; } | cut -c1-8)
  full_commit=$full_commit-dirty
fi
image=${PMS_LOCAL_REPO:-pms-local}:$tag

if [ "$current" = "$image" ] && wait_ready; then
  echo "Already running $image; nothing to do."
  exit 0
fi

if docker image inspect "$image" >/dev/null 2>&1; then
  echo "Image $image already built."
else
  # Application version comes from web/package.json (as released); the commit is reported separately.
  version=$(sed -n 's/^  "version": "\([^"]*\)".*/\1/p' "$repo/web/package.json" 2>/dev/null || true)
  docker build -f "${BUILD_FILE:-$repo/Dockerfile}" --build-arg "VERSION=${version:-dev}" --build-arg "COMMIT=$full_commit" -t "$image" "${BUILD_CONTEXT:-$repo}"
fi

backup_output=$(./backup.sh)
printf '%s\n' "$backup_output"
stamp=$(printf '%s\n' "$backup_output" | sed -n 's/^Backup completed: //p')
[ -n "$stamp" ] || { echo "Backup did not report a timestamp; not updating." >&2; exit 1; }

# --pull never: the image only exists locally, a pull would fail.
export_image_identity "$image"
PMS_IMAGE_DIGEST= # built here: it has no registry manifest, even when Docker records a RepoDigest for it
if PMS_IMAGE=$image docker compose up -d --pull never app && wait_ready; then
  if [ "${NO_PERSIST:-0}" != 1 ] && [ -f .env ]; then
    tmp=$(mktemp)
    if grep -q '^PMS_IMAGE=' .env; then sed "s|^PMS_IMAGE=.*|PMS_IMAGE=$image|" .env > "$tmp"; else { cat .env; echo "PMS_IMAGE=$image"; } > "$tmp"; fi
    cat "$tmp" > .env && rm -f "$tmp"
  fi
  printf 'Updated to %s (previous: %s, pre-update backup: %s).\n' "$image" "${current:-none}" "$stamp"
  exit 0
fi

# Migrations only move forward: go back by restoring the backup together with the old image (still named in .env).
echo "Update to $image FAILED. Rolling back to ${current:-the previous image} with backup $stamp." >&2
[ -z "$current" ] || export PMS_IMAGE=$current
if ./restore.sh "$stamp" --force; then
  echo "Rolled back to ${current:-previous image}. See: docker compose logs app" >&2
else
  echo "AUTOMATIC ROLLBACK FAILED. Backup set $stamp is intact; follow restore.sh output and docs/operations/PILOT_RUNBOOK.md." >&2
fi
exit 1

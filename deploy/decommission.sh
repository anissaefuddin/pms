#!/bin/sh
# Removes the instance for good (USR-04): containers, the database volume and the files volume.
# Refuses to run without a readable export bundle and without PMS_HOST typed back. There is no undo.
# Every attempt, refused or not, is appended as JSON lines to $DECOMMISSION_AUDIT_DIR/decommission.jsonl (default ~/pms-audit),
# a host directory that nothing here removes. If that record cannot be written, nothing is removed.
set -eu

if [ "$#" -ne 2 ]; then
  echo "Usage: $0 EXPORT_BUNDLE.tar.gz PMS_HOST" >&2
  exit 2
fi
bundle=$1
typed=$2
script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$script_dir"
. ./lib.sh
set -a
[ ! -f .env ] || . ./.env
set +a

audit_dir=${DECOMMISSION_AUDIT_DIR:-${HOME:-/root}/pms-audit}
audit_file=$audit_dir/decommission.jsonl
operation_id=$(date -u +%Y%m%dT%H%M%SZ)-$$
bundle_id=$(basename -- "$bundle" .tar.gz)
bundle_sha=unavailable
[ ! -f "$bundle" ] || bundle_sha=$(sha_cmd < "$bundle" | cut -d' ' -f1) || bundle_sha=unavailable
app_version=$(docker compose exec -T app pms version </dev/null 2>/dev/null | head -n 1) || app_version=
preflight=not_run
confirmation=not_run
execution=not_run
reason=

# The typed confirmation is deliberately not recorded: a mistyped argument could be a pasted secret.
js() { printf '%s' "$1" | tr -d '\000-\037' | sed 's/\\/\\\\/g; s/"/\\"/g'; }
write_audit() { # $1 = event
  (
    umask 077
    mkdir -p "$audit_dir" && [ ! -L "$audit_file" ] &&
      printf '{"operation_id":"%s","event":"%s","timestamp":"%s","operator":"%s","operator_uid":"%s","sudo_user":"%s","machine":"%s","instance":"%s","target_host":"%s","app_version":"%s","image":"%s","bundle_id":"%s","bundle_sha256":"%s","preflight":"%s","confirmation":"%s","execution":"%s","failure_reason":"%s"}\n' \
        "$operation_id" "$1" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$(js "$(id -un)")" "$(id -u)" "$(js "${SUDO_USER:-}")" "$(js "$(hostname)")" \
        "$(js "${COMPOSE_PROJECT_NAME:-$(basename "$script_dir")}")" "$(js "${PMS_HOST:-}")" "$(js "${app_version:-unavailable}")" "$(js "${PMS_IMAGE:-unavailable}")" \
        "$(js "$bundle_id")" "$bundle_sha" "$preflight" "$confirmation" "$execution" "$(js "$reason")" >> "$audit_file" &&
      chmod 600 "$audit_file"
  ) && sync
}
finish() {
  rc=$?
  trap - EXIT
  write_audit result || echo "WARNING: could not write the audit record to $audit_file (operation $operation_id, execution=$execution)." >&2
  exit "$rc"
}
trap finish EXIT
trap 'reason=interrupted; [ "$execution" != started ] || execution=failed; exit 1' HUP INT TERM
refuse() { # $1 = exit status, $2 = reason
  reason=$2
  echo "$2" >&2
  exit "$1"
}
bad_bundle() { preflight=failed; refuse 1 "$1 Nothing was removed."; }

if [ -z "$typed" ] || [ "$typed" != "${PMS_HOST:-}" ]; then
  confirmation=failed
  refuse 2 "Second argument must be the PMS_HOST value from .env. Nothing was removed."
fi
confirmation=passed
[ -f "$bundle" ] || bad_bundle "Export bundle not found. Run ./export.sh first."
listing=$(tar -tzf "$bundle") || bad_bundle "Export bundle is unreadable."
for part in database.dump files.tar.gz SHA256SUMS; do
  printf '%s\n' "$listing" | grep -q "/$part\$" || bad_bundle "Export bundle lacks $part."
done
bundle_id=$(printf '%s\n' "$listing" | sed -n '1s|/.*||p')
preflight=passed

# The record of the decision must be on disk before anything is destroyed.
execution=started
if ! write_audit started; then
  execution=not_run
  refuse 1 "Cannot write the audit record to $audit_file. Nothing was removed."
fi
if docker compose --profile ops down --volumes --remove-orphans; then
  execution=succeeded
else
  rc=$?
  execution=failed
  refuse "$rc" "docker compose down exited $rc; some resources may already be removed."
fi
echo "Instance removed. Keep the export bundle and the last backup set until the client confirms."
echo "Audit record: $audit_file (operation $operation_id)."

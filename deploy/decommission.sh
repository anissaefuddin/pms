#!/bin/sh
# Removes the instance for good (USR-04): containers, the database volume and the files volume.
# Refuses to run without a readable export bundle and without PMS_HOST typed back. There is no undo.
set -eu

if [ "$#" -ne 2 ]; then
  echo "Usage: $0 EXPORT_BUNDLE.tar.gz PMS_HOST" >&2
  exit 2
fi
bundle=$1
typed=$2
script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$script_dir"
set -a
[ ! -f .env ] || . ./.env
set +a
[ "$typed" = "${PMS_HOST:-}" ] || { echo "Second argument must be the PMS_HOST value from .env. Nothing was removed." >&2; exit 2; }
[ -f "$bundle" ] || { echo "Export bundle not found. Run ./export.sh first." >&2; exit 1; }
listing=$(tar -tzf "$bundle") || { echo "Export bundle is unreadable. Nothing was removed." >&2; exit 1; }
for part in database.dump files.tar.gz SHA256SUMS; do
  printf '%s\n' "$listing" | grep -q "/$part\$" || { echo "Export bundle lacks $part. Nothing was removed." >&2; exit 1; }
done
docker compose --profile ops down --volumes --remove-orphans
echo "Instance removed. Keep the export bundle and the last backup set until the client confirms."

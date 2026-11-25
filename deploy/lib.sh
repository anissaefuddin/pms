# Shared helpers for update.sh and restore.sh. Source it from the deploy directory.

# wait_ready: poll /readyz for up to 60 s; `docker compose up -d` returns before migrations finish.
wait_ready() {
  i=0
  while [ "$i" -lt 30 ]; do
    if docker compose exec -T app wget -qO- http://localhost:8080/api/v1/readyz >/dev/null 2>&1; then return 0; fi
    i=$((i + 1))
    sleep 2
  done
  return 1
}

# entity_counts DB: "table rowcount" per public table, sorted. river_* (job queue) is excluded: it changes while the app runs.
entity_counts() {
  docker compose exec -T postgres psql -U pms -d "$1" -At -F ' ' -v ON_ERROR_STOP=1 -c "SELECT table_name, (xpath('/row/c/text()', query_to_xml(format('select count(*) as c from %I.%I', table_schema, table_name), false, true, '')))[1]::text FROM information_schema.tables WHERE table_schema='public' AND table_type='BASE TABLE' AND table_name NOT LIKE 'river\_%' ORDER BY 1"
}

# sha_cmd: portable SHA-256 (Linux sha256sum, macOS shasum).
sha_cmd() { if command -v sha256sum >/dev/null; then sha256sum "$@"; else shasum -a 256 "$@"; fi; }

# export_image_identity REF: PMS_IMAGE_ID (local image ID) and PMS_IMAGE_DIGEST (registry manifest digest) for the app container.
# shortcut: with Docker's containerd image store a local build also has a RepoDigest, so an image built by hand under a registry
# name and never pushed would be believed; update-local.sh clears it for its own builds.
# The digest is set only when REF names a registry host and Docker records exactly one digest for that repository; a tag or a
# locally built image gets none. Always recomputed, never stored, so it cannot go stale.
export_image_identity() {
  PMS_IMAGE_ID= PMS_IMAGE_DIGEST=
  if [ -n "$1" ]; then
    PMS_IMAGE_ID=$(docker image inspect --format '{{.Id}}' "$1" 2>/dev/null) || PMS_IMAGE_ID=
    _name=${1%@*}
    case "${_name##*/}" in *:*) _name=${_name%:*} ;; esac
    case "${_name%%/*}" in
    *.* | *:* | localhost)
      _digests=$(docker image inspect --format '{{range .RepoDigests}}{{println .}}{{end}}' "$1" 2>/dev/null | sed -n "s|^$_name@\(sha256:[0-9a-f]\{64\}\)\$|\1|p") || _digests=
      [ "$(printf '%s\n' "$_digests" | grep -c sha256)" != 1 ] || PMS_IMAGE_DIGEST=$_digests ;;
    esac
  fi
  export PMS_IMAGE_ID PMS_IMAGE_DIGEST
}

# verify_checksums DIR STAMP: check a backup set against STAMP-SHA256SUMS. Sets made before checksums existed pass with a warning;
# a set whose manifest says it has checksums must have them.
verify_checksums() {
  if [ ! -f "$1/$2-SHA256SUMS" ]; then
    if grep -q '^checksums=' "$1/$2-manifest.txt" 2>/dev/null; then echo "Checksum file missing: $2-SHA256SUMS" >&2; return 1; fi
    echo "WARNING: backup set $2 has no checksum file; integrity is not verified." >&2
    return 0
  fi
  for _file in "$2-database.dump" "$2-files.tar.gz"; do
    grep -q "  $_file\$" "$1/$2-SHA256SUMS" || { echo "Checksum file does not cover $_file." >&2; return 1; }
  done
  (cd "$1" && sha_cmd -c "$2-SHA256SUMS" >/dev/null) || { echo "Checksum mismatch in backup set $2." >&2; return 1; }
}

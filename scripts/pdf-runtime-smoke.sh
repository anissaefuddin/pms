#!/bin/sh
# Smoke test PDF di runtime Docker: pdftoppm harus ada dan merender PDF 1 halaman menjadi PNG.
# IMAGE=pms:<tag>   memeriksa image yang sudah ada (tanpa build).
# (kosong)          membangun image runtime-only: baris `apk add` diambil dari Dockerfile stage runtime (cepat, tanpa npm/go build).
# FULL=1            membangun image penuh dari Dockerfile (lambat) lalu memeriksa.
set -eu
cd "$(dirname "$0")/.."
IMAGE="${IMAGE:-}"
if [ -z "$IMAGE" ]; then
  IMAGE=pms-pdf-smoke:local
  if [ "${FULL:-0}" = 1 ]; then
    docker build -t "$IMAGE" .
  else
    PKGS=$(awk '/^FROM alpine.* AS runtime/{r=1} r&&/apk add/{sub(/.*apk add --no-cache /,"");sub(/ *\\$/,"");print;exit}' Dockerfile)
    BASE=$(awk '/^FROM alpine.* AS runtime/{print $2}' Dockerfile)
    echo "runtime-only build: base=$BASE packages=$PKGS"
    printf 'FROM %s\nRUN apk add --no-cache %s\n' "$BASE" "$PKGS" | docker build -t "$IMAGE" -
  fi
fi
PDF='%PDF-1.4
1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj
2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj
3 0 obj<</Type/Page/Parent 2 0 R/MediaBox[0 0 200 100]>>endobj
trailer<</Root 1 0 R>>
%%EOF'
docker run --rm --entrypoint sh "$IMAGE" -c '
  command -v pdftoppm >/dev/null || { echo "FAIL: pdftoppm tidak ada"; exit 1; }
  pdftoppm -v 2>&1 | head -1
  printf "%s\n" "$0" > /tmp/t.pdf
  pdftoppm -png -r 50 -f 1 -l 1 /tmp/t.pdf /tmp/out 2>/tmp/err || { echo "FAIL: render"; cat /tmp/err; exit 1; }
  f=$(ls /tmp/out*.png 2>/dev/null | head -1); [ -n "$f" ] || { echo "FAIL: tidak ada PNG"; exit 1; }
  head -c 8 "$f" | od -An -c | grep -q "P   N   G" || { echo "FAIL: bukan PNG"; exit 1; }
  echo "OK: $f $(wc -c < "$f") bytes"
' "$PDF"

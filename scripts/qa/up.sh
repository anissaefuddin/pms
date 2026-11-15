#!/bin/sh
# Naikkan instance QA lokal. Bangun image dulu: docker build -t pms:qa-b3d-86c33d1 .   (atau set QA_IMAGE)
# Hanya Postgres+app proyek pms-qa-$QA_INSTANCE; tidak pernah menyentuh dev-lens.
set -eu; . "$(dirname "$0")/common.sh"
if [ "${1:-}" = "db-only" ]; then $DC up -d --wait postgres; else $DC up -d --wait; fi
echo "instance=$QA_INSTANCE app=$BASEURL pg=localhost:$QA_PG_PORT image=$QA_IMAGE"

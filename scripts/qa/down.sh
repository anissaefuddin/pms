#!/bin/sh
# Matikan instance QA lokal. `down.sh purge` juga menghapus volume milik proyek pms-qa-$QA_INSTANCE (bukan DB lain).
set -eu; . "$(dirname "$0")/common.sh"
if [ "${1:-}" = "purge" ]; then $DC down -v; else $DC down; fi

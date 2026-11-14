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

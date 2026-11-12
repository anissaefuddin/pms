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

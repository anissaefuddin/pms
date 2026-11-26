#!/bin/sh
# Starts a built image against a throwaway Postgres and checks that it migrates, is ready, serves the web app and reports the
# expected architecture. Usage: scripts/image-smoke.sh IMAGE [amd64|arm64]
set -eu
image=$1
arch=${2:-amd64}
name=pms-smoke-$$
cleanup() { docker rm -f "$name-app" "$name-db" >/dev/null 2>&1 || true; docker network rm "$name" >/dev/null 2>&1 || true; }
trap cleanup EXIT INT TERM

got=$(docker image inspect --format '{{.Architecture}}' "$image")
[ "$got" = "$arch" ] || { echo "image is $got, wanted $arch" >&2; exit 1; }
docker network create "$name" >/dev/null
password=smoke-$$-$(date +%s)
docker run -d --name "$name-db" --network "$name" -e POSTGRES_DB=pms -e POSTGRES_USER=pms -e POSTGRES_PASSWORD="$password" postgres:16.10-alpine >/dev/null
i=0
until docker exec "$name-db" pg_isready -h 127.0.0.1 -U pms -d pms >/dev/null 2>&1; do i=$((i + 1)); [ "$i" -lt 60 ] || { echo "postgres not ready" >&2; exit 1; }; sleep 1; done
docker run -d --platform "linux/$arch" --name "$name-app" --network "$name" -e DB_HOST="$name-db" -e DB_PASSWORD="$password" -e PUBLIC_URL=http://localhost:8080 -e COOKIE_SECURE=false "$image" >/dev/null
i=0
until docker exec "$name-app" wget -qO- http://localhost:8080/api/v1/readyz >/dev/null 2>&1; do
  i=$((i + 1)); [ "$i" -lt 120 ] || { docker logs "$name-app" >&2; echo "app not ready" >&2; exit 1; }; sleep 1
done
docker exec "$name-app" wget -qO- http://localhost:8080/ | grep -q '<div id="root"' || { echo "web app not served" >&2; exit 1; }
schema=$(docker exec "$name-db" psql -U pms -d pms -Atc "SELECT max(version_id) FROM schema_migrations WHERE is_applied")
echo "smoke ok: image=$(docker image inspect --format '{{.Id}}' "$image") arch=linux/$arch schema=$schema version=$(docker exec "$name-app" pms version | head -n 1)"

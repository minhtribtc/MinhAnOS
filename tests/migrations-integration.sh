#!/bin/sh
# Dedicated disposable database, no published port or production credentials.
set -eu
label="minhanos-migrations-$$"
network="$label-net"
database="$label-db"
cleanup(){ docker rm -fv "$database" >/dev/null 2>&1 || true; docker network rm "$network" >/dev/null 2>&1 || true; }
trap cleanup EXIT INT TERM
docker network create "$network" >/dev/null
docker run -d --name "$database" --network "$network" -e POSTGRES_HOST_AUTH_METHOD=trust postgres:18-alpine@sha256:10a5c69a83035ab5239f5620d708520e8b28f5aa1f2b20143e6024532edadfb5 >/dev/null
ready=0
for attempt in 1 2 3 4 5 6 7 8 9 10; do
 if docker exec "$database" pg_isready -U postgres >/dev/null 2>&1; then ready=1;break;fi
 sleep 1
done
test "$ready" = 1
docker run --rm --network "$network" -e "MIGRATION_TEST_DATABASE_URL=postgres://postgres@$database:5432/postgres?sslmode=disable" -v "$PWD:/src" -w /src golang:1.26-alpine@sha256:a2012b4a1d1f9f7e9f44e0d39b805a901e856b1dd80b4e6ee4338db547050308 go test -count=1 -v ./internal/migrate ./internal/coreprofile ./internal/identity ./internal/platform ./internal/storagecore ./internal/notifycore ./internal/logcore ./internal/drive

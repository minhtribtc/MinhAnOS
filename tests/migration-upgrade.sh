#!/bin/sh
# Restore a private DB backup into an isolated disposable container, then adopt profile.
set -eu
: "${MIGRATION_UPGRADE_DUMP:?Set path to the private pre-upgrade SQL dump}"
test -f "$MIGRATION_UPGRADE_DUMP"
label="minhanos-migration-upgrade-$$"
network="$label-net"
database="$label-db"
cleanup(){ docker rm -fv "$database" >/dev/null 2>&1 || true; docker network rm "$network" >/dev/null 2>&1 || true; }
trap cleanup EXIT INT TERM
docker network create "$network" >/dev/null
docker run -d --name "$database" --network "$network" -e POSTGRES_HOST_AUTH_METHOD=trust postgres:18-alpine@sha256:10a5c69a83035ab5239f5620d708520e8b28f5aa1f2b20143e6024532edadfb5 >/dev/null
ready=0
for attempt in 1 2 3 4 5 6 7 8 9 10; do
 if docker exec "$database" pg_isready -U postgres >/dev/null 2>&1;then ready=1;break;fi
 sleep 1
done
test "$ready" = 1
# Original dump objects belong to the app role. Create it only in the clone.
docker exec "$database" psql -U postgres -v ON_ERROR_STOP=1 -c 'CREATE ROLE minhanos;' >/dev/null
docker exec -i "$database" psql -U postgres -v ON_ERROR_STOP=1 < "$MIGRATION_UPGRADE_DUMP" >/dev/null
docker run --rm --network "$network" -e "MIGRATION_UPGRADE_TEST_DATABASE_URL=postgres://postgres@$database:5432/postgres?sslmode=disable" -v "$PWD:/src" -w /src golang:1.26-alpine@sha256:a2012b4a1d1f9f7e9f44e0d39b805a901e856b1dd80b4e6ee4338db547050308 go test -count=1 -run TestUpgradeRestoredDatabase -v ./internal/coreprofile

docker run --rm --network "$network" -e "MIGRATION_UPGRADE_TEST_DATABASE_URL=postgres://postgres@$database:5432/postgres?sslmode=disable" -v "$PWD:/src" -w /src golang:1.26-alpine@sha256:a2012b4a1d1f9f7e9f44e0d39b805a901e856b1dd80b4e6ee4338db547050308 go test -count=1 -run TestRestoreUpgradeIdentity -v ./internal/identity

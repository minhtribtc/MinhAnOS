#!/bin/sh
# Backup and restore in isolation (app standard row 19). Takes the backup the operator would take of a running
# install (pg_dump of the database, a tar of the Files volume), restores both into disposable places with no
# network and checks them against contracts/data/*.data.json: every listed table exists with the row count of
# the dump, the migration ledger is intact, and every restored file has the checksum of the original.
# Reads the live install only (pg_dump, read-only volume mount); run from the compose directory.
set -eu
label="minhanos-restore-$$"
work=$(mktemp -d)
cleanup(){ docker rm -fv "$label-db" >/dev/null 2>&1 || true; rm -rf "$work"; }
trap cleanup EXIT INT TERM
postgres=postgres:18-alpine@sha256:10a5c69a83035ab5239f5620d708520e8b28f5aa1f2b20143e6024532edadfb5
alpine=alpine:3.22@sha256:2e0b81d60597f449baa56dc25d355367869d2dca4cd57e866f1e2b52af3eb55f
db=$(docker compose ps -q db)
drive=$(docker compose ps -q drive)
test -n "$db" && test -n "$drive"

# 1. Backup
docker exec "$db" sh -c 'pg_dump --no-owner --no-privileges -U "$POSTGRES_USER" "$POSTGRES_DB"' > "$work/database.sql"
docker run --rm --network none --volumes-from "$drive:ro" "$alpine" tar cf - -C /data . > "$work/files.tar"
docker run --rm --network none --volumes-from "$drive:ro" "$alpine" sh -c 'cd /data && find . -type f -exec sha256sum {} + | sort -k2' > "$work/files.sum"

# 2. Restore the database into a disposable server with no network.
docker run -d --name "$label-db" --network none -e POSTGRES_HOST_AUTH_METHOD=trust "$postgres" >/dev/null
ready=0
for attempt in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15; do
 if docker exec "$label-db" pg_isready -U postgres >/dev/null 2>&1; then ready=1; break; fi
 sleep 1
done
test "$ready" = 1
sleep 2 # the entrypoint restarts the server once after initdb
docker exec -i "$label-db" psql -q -v ON_ERROR_STOP=1 -U postgres -d postgres < "$work/database.sql" >/dev/null
query(){ docker exec "$label-db" psql -tA -U postgres -d postgres -c "$1"; }

# 3. Every table in the data contracts is restored with the rows of the dump.
tables=$(cat contracts/data/*.data.json | grep -o '"name": "[a-z_]*"' | sed 's/.*: "\(.*\)"/\1/' | sort -u)
checked=0
for table in $tables platform_migrations; do
 expected=$(awk -v t="COPY public.$table " 'index($0,t)==1{on=1;next} on&&$0=="\\."{on=0} on{n++} END{print n+0}' "$work/database.sql")
 actual=$(query "SELECT count(*) FROM public.$table")
 if [ "$expected" != "$actual" ]; then echo "FAIL restore $table: dump $expected rows, restored $actual"; exit 1; fi
 checked=$((checked+1))
done
modules=$(query "SELECT count(DISTINCT module) FROM platform_migrations")
test "$modules" -ge 5

# 4. Files: extract into an empty directory and compare every checksum.
mkdir "$work/files"
docker run --rm --network none -v "$work:/w" "$alpine" sh -c 'tar xf /w/files.tar -C /w/files && cd /w/files && find . -type f -exec sha256sum {} + | sort -k2 > /w/restored.sum'
cmp "$work/files.sum" "$work/restored.sum"
count=$(wc -l < "$work/files.sum")
echo "PASS backup/restore in isolation: $checked tables (row counts = dump), $modules migration modules, $count files (sha256 equal)"

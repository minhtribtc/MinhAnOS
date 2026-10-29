#!/bin/sh
# Run only in the test checkout after drive-smoke.mjs. Removes the exact two fixtures.
set -eu
docker run --rm -v "$PWD":/src -w /src node:22-alpine node --input-type=module -e '
 import {readFileSync} from "node:fs";
 const rows=JSON.parse(readFileSync("artifacts/drive-test-users.json","utf8"));
 if(!Array.isArray(rows)||rows.length>2)throw new Error("unexpected fixture list");
 for(const r of rows){if(!/^[a-f0-9]{64}$/.test(r.id)||!/^drive_e2e_[a-f0-9]{8}$/.test(r.username))throw new Error("invalid fixture");console.log(r.id+" "+r.username)}
' > artifacts/drive-cleanup-targets.txt
while read -r test_id test_name; do
  case "$test_name" in drive_e2e_*) ;; *) exit 1;; esac
  docker compose exec -T drive rm -rf -- "/data/users/$test_id" </dev/null
  docker compose exec -T db psql -U minhanos -d minhanos -v test_id="$test_id" -v test_name="$test_name" <<'SQL'
DELETE FROM users WHERE id = :'test_id' AND username = :'test_name';
SQL
done < artifacts/drive-cleanup-targets.txt

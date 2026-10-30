#!/bin/sh
# Exact disposable rows from identity-phase3-smoke only; that test writes no files.
set -eu
docker run --rm -v "$PWD:/src" -w /src node:22-alpine node --input-type=module -e '
 import {readFileSync} from "node:fs";
 const rows=JSON.parse(readFileSync("artifacts/identity-phase3-users.json","utf8"));
 if(!Array.isArray(rows)||rows.length>3)throw new Error("unexpected fixture count");
 for(const r of rows){if(!/^[0-9a-f-]{36}$/.test(r.id)||!/^identity_e2e_[0-9a-f]{8}$/.test(r.username))throw new Error("invalid fixture");console.log(r.id+" "+r.username)}
' > artifacts/identity-phase3-cleanup-targets.txt
while read -r test_id test_name;do
 docker compose exec -T db psql -U minhanos -d minhanos -v test_id="$test_id" -v test_name="$test_name" <<'SQL'
DELETE FROM users WHERE public_id=:'test_id'::uuid AND username=:'test_name' AND display_name='Identity disposable test';
SQL
done < artifacts/identity-phase3-cleanup-targets.txt

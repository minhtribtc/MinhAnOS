#!/bin/sh
set -eu
mkdir -p artifacts
# Controlled breaking mutation in ignored test fixtures, never in the canonical document.
docker run --rm -v "$PWD:/src" -w /src node:22-alpine node --input-type=module -e '
import {readFileSync,writeFileSync} from "node:fs";
const api=JSON.parse(readFileSync("contracts/api/openapi.yaml","utf8"));delete api.paths["/api/profile"].get;
writeFileSync("artifacts/openapi-breaking-negative.yaml",JSON.stringify(api));'
image=$(cat contracts/tooling/oasdiff-image.txt)
set +e
docker run --rm --network none --read-only --cap-drop ALL --security-opt no-new-privileges:true -v "$PWD:/specs:ro" "$image" breaking /specs/contracts/api/openapi.yaml /specs/artifacts/openapi-breaking-negative.yaml --allow-external-refs=false --fail-on WARN > artifacts/openapi-breaking-negative.log 2>&1
result=$?
set -e
if test "$result" -ne 1; then cat artifacts/openapi-breaking-negative.log;echo 'Expected oasdiff to reject removed profile endpoint' >&2;exit 1;fi
grep -q 'api-removed' artifacts/openapi-breaking-negative.log
echo 'PASS breaking-change guard rejects removal of GET /api/profile'

#!/bin/sh
set -eu
mkdir -p artifacts
base_ref=${CONTRACT_BASE_REF:-$(git rev-parse HEAD)}
case "$base_ref" in ''|*[!0-9a-fA-F]*) echo 'Expected a commit SHA for CONTRACT_BASE_REF' >&2;exit 1;; esac
if git cat-file -e "$base_ref:contracts/api/openapi.yaml" 2>/dev/null; then
 git show "$base_ref:contracts/api/openapi.yaml" > artifacts/openapi-base.yaml
else
 echo 'Initial OpenAPI contract: no historical spec at the base SHA; comparing to itself.'
 cp contracts/api/openapi.yaml artifacts/openapi-base.yaml
fi
image=$(cat contracts/tooling/oasdiff-image.txt)
docker run --rm --network none --read-only --cap-drop ALL --security-opt no-new-privileges:true -v "$PWD:/specs:ro" "$image" breaking /specs/artifacts/openapi-base.yaml /specs/contracts/api/openapi.yaml --allow-external-refs=false --fail-on WARN --deprecation-days-stable 180

if git cat-file -e "$base_ref:contracts/apps/catalog.v1.json" 2>/dev/null;then
 git show "$base_ref:contracts/apps/catalog.v1.json" > artifacts/app-catalog-base.json
 docker run --rm --network none -v "$PWD:/src:ro" -w /src node:22-alpine node tests/app-catalog-compatibility.mjs artifacts/app-catalog-base.json contracts/apps/catalog.v1.json
else
 echo 'Initial first-party app catalog: no predecessor catalog to compare.'
fi

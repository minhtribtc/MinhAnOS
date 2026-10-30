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

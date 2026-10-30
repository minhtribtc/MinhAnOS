# MinhAnOS Platform Contracts v1

Contract-first preview; current implementation status is explicit in each document.
`manifestVersion: "1"` / `contractVersion: "1"` describe schema protocols, not SDK package version.
The SDK package is currently 0.1.0; manifests requiring ^1.0.0 are rejected until an actual SDK 1 release.

| Contract | Canonical source | Runtime status |
|---|---|---|
| App Manifest | manifests/app-manifest.v1.schema.json | Schema + sample + CI checks; registry/entrypoint integration pending |
| Authorization | authz/permissions.yaml, check.v1.schema.json | Planned user AND app grant enforcement; no new authz endpoint |
| API | api/openapi.yaml | Only /api/profile is represented as implemented; other legacy routes remain documented in docs/api.md |
| Migration | ../docs/migration-policy.md | Transactional runner + profile adoption; full legacy/module adoption pending |
| Jobs | jobs/jobs.v1.yaml, job.v1.schema.json | Planned; no worker/job API |
| Upload | uploads/uploads.v1.yaml, upload.v1.schema.json | Planned; existing uploads remain browser-owned |
| File Resource | files/file-resource.v1.schema.json | Planned persistent IDs/version/ACL; current path-based API unchanged |
| Events | events/events.v1.yaml, event.v1.schema.json | Planned outbox/dedupe; no event transport |

All `/api/v1/*` routes are planned, not live endpoints. Files manifest entrypoint is a logical future host route;
it does not imply `/apps/files` is deployed. Existing registry ID `files` remains a legacy alias of `com.minhanos.files`.
Do not rename IDs in persisted layouts until an explicit compatible migration exists.
v1 OpenAPI requires both the verified user session and a scoped app-delegation bearer credential.
A client context.appId must match that verified app principal. Requested capabilities are never grants. No app installation or authorization enforcement is introduced by this schema commit.

JSON Schema uses draft 2020-12. The `.yaml` documents use JSON syntax, a valid YAML subset, so the OpenAPI document
can be parsed by JSON tooling as well as YAML tooling. OpenAPI is self-contained to make historical diffs reliable.

Run at repository root:

```sh
npm ci
npm run contracts:generate
npm run contracts:check
npm run build
sh tests/openapi-breaking.sh
sh tests/openapi-breaking-negative.sh
```

Generated `packages/shared-types/src/index.d.ts` is checked against the schemas; never edit it by hand.
`@minhanos/api-client` has no Vue/Desktop dependency and accepts both legacy and v1 error bodies.
App SDK re-exports it for compatibility. Backend still emits legacy `{code,error:string}`.
The nested v1 envelope and requestId are required for future versioned APIs; do not silently change `/api`.

CI validates schemas/negative fixtures, OpenAPI with Redocly, generated types and SDK semver/capabilities.
oasdiff is pinned by image digest; compare against PR base / push predecessor. The first contract commit has no
historical OpenAPI and therefore bootstraps with a self-compare; a controlled removed-endpoint fixture verifies rejection.
No unknown API is exposed to make the specification appear implemented.

References: [OpenAPI 3.1](https://spec.openapis.org/oas/v3.1.0),
[Redocly lint](https://redocly.com/docs/cli/commands/lint),
[oasdiff Docker](https://github.com/oasdiff/oasdiff/blob/main/docs/DOCKER.md).

Current approval/progress: người dùng đã ủy quyền chọn phương án phù hợp và ghi quyết định. Phase 1 và Phase 2 đã triển khai; xem [Phase 2 report](PHASE2_REPORT.md). Các đoạn approval checkpoint bên dưới là lịch sử Phase 0, không yêu cầu hỏi lại cho công việc đã được ủy quyền.

# IMPLEMENTATION PLAN — Account Center & Identity

Status: Phase 0 hoàn tất; người dùng đã duyệt Phase 1 và đã triển khai foundation. Xem [PHASE1_REPORT](PHASE1_REPORT.md) cho kết quả thực tế; các phase sau vẫn là kế hoạch.
Bốn deliverables: CURRENT_ARCHITECTURE, GAP_ANALYSIS, IDENTITY_ARCHITECTURE và tài liệu này.

## 1. Thứ tự và điều kiện nghiệm thu

| Phase | Phạm vi | Nghiệm thu/automated tests dự kiến | Migration impact + security review |
|---|---|---|---|
| 0 | Audit, architecture, gaps, task plan | Bốn báo cáo + evidence/source links; phân biệt implemented/planned | Docs-only; không chạy test nghiệp vụ hoặc khai thác |
| 1 | Identity contracts, manifest extensions, schema adoption/UUID expand | Schema/semver/generated types/OpenAPI compatibility; fresh/upgrade/concurrent migrations; old APIs and Files data unchanged | Additive only, backup/restore isolated; review ID mapping, secret DTO và lock ordering |
| 2 | Shared auth/session service, credential/recovery primitives | Legacy hashes verify, fixation, account state, both realms revoke, password/reset replay, CSRF/Origin, bounded brute-force tests | Session metadata/state; không credential copy; recovery requires local operator |
| 3 | Identity business + RBAC/app access + audit | BOLA/self-escalation/permission ceiling, last-admin race, stale If-Match, idempotency, audit transaction tests | Protected seeds/bindings; legacy route adapters share policy; host privilege split before new SystemAdmin grants |
| 4 | Account Center native + standalone | vi/en parity, theme/portals, keyboard, own/admin navigation, real API, forms retained across Home/minimize | No business state in Desktop; SDK lifecycle bridge tested against Files |
| 5 | Files authorization + file-resource integration | User permission AND app scope AND resource/owner/storage; cross-user tests; protected host paths denied | Preserve data paths/ownership; ACL/fileId has separate expand/backfill plan |
| 6 | Debian provisioning jobs/adapters | Agent restart, stale lease/fencing, UID collision/reservation, idempotent reconciliation, restore/link existing identity | No default SSH/SMB, no arbitrary shell, web user survives provisioning failure |
| 7 | Production readiness | Security regression suite, reauth, audit retention/redaction, backup restore, HTTPS/secrets, measurements | Release gate, no claim production-ready before passing |
| 8 | OIDC/SSO design after approval | Protocol/library review, conformance/redirect/PKCE/key lifecycle tests before enablement | Separate OAuth client registry, stable issuer/sub; separate approval required |

Mỗi phase tạo file-change inventory, implementation report, security review và migration impact thực tế trong docs.
Chỉ mở feature đã có backend, policy và test; schema/manifest draft không làm app thành ready.

## 2. Tasks Phase 1 được giao Codex sau duyệt

Không cần hai agent sửa cùng file; danh sách này là work breakdown, không tự khởi chạy agent/Claude.
Claude Code chưa được xác nhận khả dụng trong phiên này; không tự coi việc review đã được một công cụ khác làm.

| Task | Input → output | File scope được phép | Acceptance + security/test |
|---|---|---|---|
| P1.1 Contracts | Approved models → versioned Identity JSON schemas/OpenAPI planned operations | contracts/identity/*, contracts/api/openapi.yaml, contracts/authz/permissions.yaml | 10 models: User/Group/Role/Permission/RoleBinding/Session/CredentialRef/IdentityLink/LinuxIdentity/AuditEvent; no hash/token in DTO; negative schema tests |
| P1.2 Manifest | App ID/capability/nav design → backward-compatible optional properties + Account draft | contracts/manifests/*, contracts/apps/catalog.v1.json | Existing Files manifest still valid; SDK stays ^0.1.0; requested capabilities never become grants; status stays partial/planned |
| P1.3 Migration adoption | Actual legacy users/session schemas → immutable Identity-owned adopt/expand steps | internal/identity/schema/*, internal/migrate tests, startup initialization call sites after review | Verify legacy schema instead of assuming; keep profile 001 checksum; no rekey/chown; reject incompatible DB/history |
| P1.4 UUID expansion | Existing hex IDs → one public UUID per same users row | New Identity migration steps + explicit backfill tool/test fixtures | Add nullable field without volatile-default table rewrite, set default for future inserts; backfill bounded/checkpointed outside startup; unique/NOT NULL only after validation; v1 gate until complete |
| P1.5 Types/client | Schemas → generated DTO and typed Identity client wrappers | packages/shared-types generated output, scripts/generate-contract-types.mjs, packages/api-client typed additions | strict TS no any, generator deterministic, legacy/nested errors both supported, no Vue/Desktop dependency in transport |
| P1.6 Compatibility verification | P1.1–P1.5 → report and rehearsal | tests/identity-*.mjs/go, isolated DB test scripts, docs/account-center/*, docs/validation.md, CI | Old login/create/list/profile + both Drive realms + Files read/trash/restore still work; restore pre-change DB; users/profile/data mapping unchanged |

P1.3 initializer placement is a mandatory design review item: current platform and Drive bootstrap transactions
must commit before the shared runner uses another connection. Do not introduce nested cross-connection DDL waits.
During adoption retain profile's documented external legacy-users precondition; only declare a formal Identity
module dependency when every startup/standalone/test registry includes that module. Do not fabricate ledger history.

P1.4 is staged: schema expansion may complete before backfill/contract. Large backfill does not run in the
120-second startup transaction. If a resumable backfill tool needs implementation, review its checkpoint/idempotency
and scope separately; no hidden unbounded UPDATE during startup. Existing internal IDs and API response shapes remain stable.

## 3. Schema migration/recovery sequence (proposed)

1. Read-only preflight: table/index/FK checks, legacy ID format, username normalization collisions,
   user/profile/session count and source/schema version. Report incompatibility; do not auto-normalize rows.
2. Take private pg_dump + source revision snapshot; record required app role/PG version; restore into isolated DB.
3. Adopt legacy Identity schema with immutable checksum. Add UUID/state/version fields and empty new relation tables
   in small additive steps; no deletion/rekey and no production role changes in Phase 1.
4. Set defaults for future legacy writers; resumably backfill missing UUIDs from trusted code/PG UUID generator,
   one stable UUID per row, never regenerate assigned IDs. Audit/compare row/file storage mapping.
5. Validate unique/non-null/references; only then allow new v1 responses. Do not delete old columns/endpoints.
6. Switch legacy route implementations to shared IdentityService before enabling disable/RBAC/grants.
   Last-admin guard, both-realm revocation and Files host-role split are required for relevant mutations.
7. Contract/retirement is a separate release after all writers/readers migrated and restore tested. No automatic down.

A restored DB alone does not verify Files volume recovery; back up and verify actual data separately before a phase
that touches storage/ownership. Rollback to pre-runner legacy binaries is not automatically safe.

## 4. V0.1 slice and deferred work

First functional slice after contracts: Users + My Account + Sessions + password security + audited role/app access.
Groups/reserved roles follow through the same backend policy; no placeholder/mock response counts as a feature.
Email reset only after verified delivery; local/admin reset is distinct and reauthenticated. Hard delete is deferred.
MFA/WebAuthn show unsupported until real integration; Linux mapping can show not-provisioned without modifying host.
No Redis, separate IAM DB, custom crypto, OIDC provider, Flow Designer, LDAP/SAML/SCIM or implicit SSH/SMB in V0.1.

UI uses real capabilities and permission-filtered DTOs. Preserve legacy Settings users page until Account Center has
feature parity and fallback; make it a thin compatibility wrapper, then remove in a separately reviewed change.
Avoid hidden dual writes of user/password/authorization state. Every path calls the same service before rollout.

## 5. Benchmarks (not measured in Phase 0)

Goal idle Identity components below 150 MB is a target, not a result. In modular monolith measure whole backend RSS
and baseline/delta, not only Go heap. Record hardware/CPU/RAM, cold/warm startup, login p50/p95/CPU, hash throughput,
bounded parallel logins, memory peak, DB connections/query count, list pagination and role-decision latency.
Benchmark before/after under the same dataset and current password work factor. Set budgets after baseline;
do not reduce hashing cost or session checks to meet an arbitrary memory number. No performance certification yet.

## 6. Phê duyệt cần thiết trước khi sửa code

Đề nghị duyệt:

1. Modular monolith/shared library + reuse users và hai realms session hiện có.
2. Public UUIDv4 làm User.id của contract v1, giữ legacy users.id làm internal/storage compatibility key.
3. System Admin web không tự có host-root/private-file authority; existing privileged operator binding phải chuyển
   qua policy riêng có review, trước khi cấp role mới. Root/DBA thực vẫn là trust boundary riêng.
4. Phase 1 chỉ contracts + adoption/expand + isolated tests; chưa bật app UI/OIDC/provisioning hoặc sửa quyền dữ liệu.

Nguồn checkpoint là master prompt người dùng: **“Chỉ thực hiện Phase 0 và chuẩn bị phương án Phase 1 trong lượt đầu tiên.
Chờ phê duyệt trước khi sửa code hoặc migration.”**
Sau khi duyệt mới sửa runtime/contracts/migration; Phase 8 OIDC còn cần phê duyệt riêng.

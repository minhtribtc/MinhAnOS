# Account Center — Phase 1 implementation report

Ngày 2026-10-30. Người dùng đã duyệt triển khai sau Phase 0. Nhánh: `account-center-phase1`.

## Kết quả thực tế

- Contracts: schema Identity, manifest Account Center, permission names và OpenAPI 3.1; generated shared types và API client độc lập framework. Các operation Identity đều đánh dấu planned; chưa có HTTP handlers hoặc giao diện Account Center.
- Adoption: module `internal/identity` dùng runner hiện hữu, chạy sau legacy bootstrap commit trong cả Desktop và Drive. Ba migration kiểm tra schema cũ, mở rộng UUID và tạo relations rỗng. Không tạo users/session/password store thứ ba; không seed quyền.
- UUID: thêm public_id và password_credential_id trên cùng users row. Giữ nguyên hex64 users.id, đường dẫn Files, foreign keys và hai session realms. UUID đã gán không được thay đổi/xóa. Không xuất storage key hoặc credential secret trong public DTO.
- Backfill: CLI identity-admin có batch 1–1000, tối đa 1–1000 batches; idempotent, concurrent-safe bằng SKIP LOCKED, không chạy backfill vô hạn khi startup. Mặc định một batch 250 hàng. Legacy rows vẫn nullable trong expand phase; NOT NULL cần migration riêng sau khi toàn bộ dữ liệu đạt điều kiện.
- Linux reservations chỉ là metadata; không tạo tài khoản Debian, SSH, SMB hoặc quyền root.

## Quyết định và security review

Reuse modular Go library và PostgreSQL hiện hữu; không thêm IAM daemon, Redis hoặc database. PublicUser/LegacyID là mapper nội bộ, không phải authorization check. Capabilities trong manifest là yêu cầu, không phải quyền đã cấp. Role/app grant tables rỗng và không được tham gia guard hiện tại.

Scope JSON rỗng/chưa biết phải bị policy tương lai từ chối; không suy ra system scope. Backend Phase 2/3 phải xác minh session, account state, user permission AND application grant; không tin subject/appId từ client. Phase 1 chưa thực thi disable/revoke, last-admin protection, audit service hoặc tách SystemAdmin khỏi host-root. Các rủi ro host files-agent đã ghi trong Phase 0 vẫn còn; không gọi bản này production-ready.

Không sửa password algorithm: authn đang dùng Go crypto/pbkdf2 SHA256 600000. Không copy hash; credential reference chỉ là metadata. Session schema mới là hợp đồng public, không phải kho session thứ ba. DTO/audit schemas loại secret và role mass assignment.

## Migration và vận hành

Migration identity 001–003 đã áp dụng trên VM, checksum đã đóng băng. Không sửa SQL đã áp dụng; thêm migration kế tiếp. Bootstrap mới và upgrade cùng đi qua adoption validation; schema không tương thích bị từ chối thay vì tự sửa dữ liệu. Các thay đổi legacy user fields và session/FK được kiểm tra bằng fingerprint/counts trong integration tests.

Backup trước nâng cấp ở `/opt/minhanos-backups/before-identity-phase1.sql` và `.tgz`, quyền 0600. Đã restore bản DB thật vào PostgreSQL cô lập và chạy adoption/backfill. Đây không phải kiểm thử phục hồi đầy đủ dữ liệu Files volume. Không hạ binary/schema hoặc xóa volume để rollback; cần quy trình restore phù hợp với phiên bản và dữ liệu ghi sau backup.

Lệnh trong thư mục deployment (DATABASE_URL lấy từ service, không in ra):

```sh
docker compose exec -T drive /app/identity-admin -mode status
docker compose exec -T drive /app/identity-admin -mode backfill -batch-size 250 -max-batches 1
```

Lặp có giới hạn theo kế hoạch vận hành đến pending=0. `complete=true` chỉ có nghĩa UUID/credential references đầy đủ; không có nghĩa API, RBAC hoặc sản phẩm đã sẵn sàng. VM hiện users=1, pending=0 sau khi dọn đúng các fixture test.

## Kiểm chứng đã chạy

- Root contracts:generate + npm run check: PASS; generated drift, strict TypeScript, schema, OpenAPI parity và mock client. Redocly còn một warning IdentityLink chưa dùng vì OIDC/linking chưa bật.
- OpenAPI breaking comparison với bản trước: PASS (bổ sung tương thích).
- Docker Desktop/Drive build, go test và go vet: PASS. DB tests chạy riêng với PostgreSQL thật, không dựa vào các test bị skip khi thiếu biến môi trường.
- Dedicated migration integration: PASS; concurrent adoption, schema rejection, immutable IDs, bounded/concurrent backfill 41 users và dữ liệu cũ không đổi.
- Restore DB pre-upgrade trong container riêng: PASS profile + Identity adoption/backfill.
- VM deployed: cả Desktop, Drive và DB healthy; ledger identity 1/2/3 + profile 1 applied. Backfill một user, chạy lại updated=0.
- Profile smoke, Files-in-Desktop, Drive smoke và API smoke: PASS; exact fixture cleanup. Drive vẫn login/list/logout khi Desktop dừng, sau đó Desktop khởi động lại.
- Cài mới standalone Drive trong project cô lập `minhanos-identity-fresh`, port loopback 8083, volumes mới: PASS bootstrap/schema/backfill và frontend/login/personal-files/logout. Đã dọn đúng project và hai volumes thử nghiệm; không đụng production volumes.

Lỗi build lần đầu do tên import identity trùng helper platform.identity đã sửa bằng alias identityschema. Không có UI mới nên không tuyên bố browser UI Account Center đã được kiểm thử. Không đổi crypto nên chưa có benchmark crypto mới.

## Tiếp theo

Phase 2: shared AuthCore/session adapters, account state/auth_epoch và revoke cả hai realms, password/recovery với reauthentication, rate limit và replay tests. Phase 3 mới bật business APIs/RBAC/audit/idempotency/If-Match, kèm last-admin concurrency và host privilege split. Phase 4 mới xây app native + standalone dùng chung SDK/i18n/theme. OIDC Phase 8 vẫn cần phê duyệt riêng.

## File inventory

- `Drive.Dockerfile`
- `cmd/identity-admin/main.go`
- `contracts/api/openapi.yaml`
- `contracts/authz/check.v1.schema.json`
- `contracts/authz/permissions.yaml`
- `contracts/identity/application-page.v1.schema.json`
- `contracts/identity/application.v1.schema.json`
- `contracts/identity/audit-event.v1.schema.json`
- `contracts/identity/audit-page.v1.schema.json`
- `contracts/identity/change-password.v1.schema.json`
- `contracts/identity/create-group.v1.schema.json`
- `contracts/identity/create-role-binding.v1.schema.json`
- `contracts/identity/create-role.v1.schema.json`
- `contracts/identity/create-user.v1.schema.json`
- `contracts/identity/credential-page.v1.schema.json`
- `contracts/identity/credential-ref.v1.schema.json`
- `contracts/identity/group-member.v1.schema.json`
- `contracts/identity/group-page.v1.schema.json`
- `contracts/identity/group.v1.schema.json`
- `contracts/identity/identity-link.v1.schema.json`
- `contracts/identity/index.v1.json`
- `contracts/identity/linux-identity.v1.schema.json`
- `contracts/identity/mutation-result.v1.schema.json`
- `contracts/identity/patch-group.v1.schema.json`
- `contracts/identity/patch-user.v1.schema.json`
- `contracts/identity/permission-page.v1.schema.json`
- `contracts/identity/permission.v1.schema.json`
- `contracts/identity/role-binding.v1.schema.json`
- `contracts/identity/role-page.v1.schema.json`
- `contracts/identity/role.v1.schema.json`
- `contracts/identity/session-page.v1.schema.json`
- `contracts/identity/session.v1.schema.json`
- `contracts/identity/status.v1.json`
- `contracts/identity/user-page.v1.schema.json`
- `contracts/identity/user.v1.schema.json`
- `contracts/manifests/accounts.app.json`
- `contracts/manifests/app-manifest.v1.schema.json`
- `internal/drive/auth.go`
- `internal/identity/migrations/001_adopt.sql`
- `internal/identity/migrations/002_expand.sql`
- `internal/identity/migrations/003_relations.sql`
- `internal/identity/repository.go`
- `internal/identity/schema.go`
- `internal/identity/schema_test.go`
- `internal/platform/server.go`
- `package-lock.json`
- `package.json`
- `packages/api-client/package.json`
- `packages/api-client/src/identity.generated.d.ts`
- `packages/api-client/src/identity.generated.js`
- `packages/shared-types/src/index.d.ts`
- `scripts/generate-contract-types.mjs`
- `scripts/generate-identity-client.mjs`
- `tests/identity-contracts.mjs`
- `tests/migration-upgrade.sh`
- `tests/migrations-integration.sh`
- `tests/shared-types.ts`

Tài liệu liên quan được cập nhật cùng commit: AGENTS, README, CHANGELOG, docs index/handoff/user-guide/architecture/api/roadmap/validation, báo cáo Phase 1 và contracts README.

## Storage API v1 (2026-10-31)

Moved from /api/storage to /api/v1/storage with contracts/storage schemas, OpenAPI operations storageListVolumes, storageListShares, storageCreateShare, storageSetShareQuota, storageListHomes, storageSetHomeQuota, storageMeasureHome, storageGetSettings, storageSetSettings; generated client @minhanos/api-client/storage. Writes: POST Idempotency-Key, PUT If-Match; v1 error envelope.

## Storage API (Desktop host, 2026-10-31)

/api/storage: GET volumes, shares, homes, settings; POST shares {key,name,volume,limitBytes}; PUT shares/{id}/quota {version,limitBytes}; PUT homes/{userId}/quota {limitBytes}; POST homes/{userId}/measure; PUT settings {defaultHomeLimitBytes}. storage.manage required; writes need reauthentication within 5 minutes. Drive /api/roots returns quota {limitBytes,usedBytes,reservedBytes,measured}; personal writes over quota return 507 QUOTA_EXCEEDED.

## Identity Phase 5 — 40 active operations

GET /api/v1/identity/applications (identityListApplications), GET|POST /api/v1/identity/applications/{appId}/access, DELETE /api/v1/identity/applications/{appId}/access/{ruleId}. Files (/api/roots, /api/files/*) returns 403 APP_ACCESS_DENIED / APP_PERMISSION_DENIED / IDENTITY_PERMISSION_DENIED from the identity policy before resource checks.

## Identity — 34 active operations

Added POST /api/v1/identity/users/{userId}/password (identityResetUserPassword; body {newPassword}; If-Match, Idempotency-Key, reauthentication; 409 IDENTITY_USE_OWN_PASSWORD, 403 IDENTITY_HOST_OPERATOR_PROTECTED). GET groups accepts member=<userId>; GET audit-events accepts target=<userId> (actor or target) and is ordered newest first. Audit outcome adds failure; details may include realm and count.

## Identity Phase 4 — 33 active operations

Added GET /api/v1/identity/groups/{groupId} (identityGetGroup, permission identity.groups.read, ETag = group version) for Account Center detail/edit. Static GET /accounts/ is served by both hosts from ACCOUNTS_STATIC_DIR (default /app/accounts); it is a frontend only and uses the host's existing cookie realm.

## Identity Phase 3B — 32 active operations

Runtime inventory: contracts/identity/status.v1.json. Added GET me/sessions, DELETE me/sessions/{handle}, POST me/sessions/revoke-others, POST me/password, GET me/credentials; groups GET/POST/PATCH/DELETE and members GET/POST/DELETE; roles POST; role-bindings GET/POST/DELETE; GET me/access and users/{uuid}/access. All under /api/v1/identity, in both cookie hosts.
Pages use the existing signed owner/filter/collection-bound cursors. Member cursors also bind groupId. Policy writes require recent reauthentication; POST/DELETE policy operations use required idempotency keys and applicable If-Match versions. Password change proves currentPassword, validates newPassword, revokes both realms and returns accepted; then re-login is required. Revoke-others keeps the current session. Credential response is metadata only; lastSeen/device hints are unknown/null, not simulated.
Generic bindings allow user-admin or custom role with exact {kind:system,id:null}; no System Admin assignment through groups. Custom roles reject governance-write permissions and unknown registry actions. Effective-access DTO is separate from User, does not expose physical IDs and returns no active permissions for inactive targets. App-access/grant mutation remains planned. [ADR 0006](adr/0006-identity-security-and-delegation-apis.md).

## Identity Phase 3A — active subset

Implemented at /api/v1/identity in both Desktop and standalone Drive: me GET/PATCH; reauthenticate POST; users GET/POST; users/{uuid} GET/PATCH/DELETE; disable/enable POST; role PUT; roles/permissions/audit-events GET. See contracts/identity/status.v1.json for exact operation IDs; remaining OpenAPI operations stay planned.
Use verified host cookie + session CSRF, application/json and same Origin. Admin writes require reauthenticate with current password within five minutes. ETag/If-Match is quoted decimal version (e.g. "1"); 428 missing, 412 stale. Required Idempotency-Key is 8–128 chars; replay is actor/route/body/precondition bound and reauthorized. Errors are nested with server requestId; /api legacy keeps its envelope.
Pages use limit 1–100 and opaque signed cursors (15-minute TTL, actor/epoch/collection/search bound), stable ID/key ordering. Users search matches normalized username substrings. Audit ordering is stable ID, not chronological yet. New user creation requires 12 Unicode characters and a 256 UTF-8-byte maximum; additional semantic validation can reject schema-valid input. Legacy/Phase 2 credential policy remains separately documented.
GET me/user, create/update/role/status responses return User DTO + ETag. Role input is system-admin/user-admin/standard-user, never host-root. Soft-delete requires disabled state, retains data and cannot be enabled. 409 LAST_ADMIN_REQUIRED preserves an active System Admin. 503 IDENTITY_NOT_READY blocks v1 while historical UUID/reference backfill is incomplete. [ADR 0005](adr/0005-identity-policy-and-host-operators.md).

## AuthCore Phase 2 runtime

/api/login, /api/session, /api/logout của Desktop/Drive và gateway dùng shared AuthCore; cùng response/cookie legacy. Trạng thái active và auth_epoch được kiểm tại backend. Thêm login codes AUTH_RATE_LIMITED (429) và AUTH_BUSY (503). Identity /api/v1 routes vẫn chỉ là spec; password/session/recovery mutations hiện là internal primitives/OS-root CLI, chưa là HTTP endpoints. Xem [ADR 0004](adr/0004-shared-auth-core-and-recovery.md).

## Identity v1 — contract only

OpenAPI có planned /api/v1/identity operations và generated client; chưa có HTTP handlers. /api legacy giữ error envelope và session behavior hiện hữu. Public UUID không thay storage ID; manifest permissions không tự cấp quyền. Xem [Phase 1](account-center/PHASE1_REPORT.md).

# API desktop 0.1

JSON cùng origin, prefix `/api`. Error: `{ "error": "thông báo" }`.
Không phải proxy cho API 1Panel/kodbox. Frontend dùng types tại `src/api.ts`.

| Method | Path | Quyền | Kết quả |
|---|---|---|---|
| GET | /health | Public | Trạng thái DB và phiên bản ứng dụng |
| POST | /login | Public, Origin check và rate limit | Nhận username/password; tạo cookie, trả user và csrfToken |
| GET | /session | Session | user và csrfToken |
| POST | /logout | Session + CSRF | Thu hồi session hiện tại |
| GET | /apps | Session | Registry 8 ứng dụng |
| GET | /preferences | Session | Preferences chỉ của user hiện tại |
| PUT | /preferences | Session + CSRF | Lưu theme/wallpaper/mode/windows/notes/pinned |
| GET | /system | Session | Metrics tiến trình/container và trạng thái tích hợp |
| GET | /audit | Session | 60 event gần nhất; admin thấy tất cả, user thấy event của mình |
| GET | /admin/users | Admin | id, username, displayName, role; không có hash/password |
| POST | /admin/users | Admin + CSRF | Tạo user từ username, displayName, role, password |

Các write request ngoài login cần header `X-CSRF-Token` từ session. Backend
chấp nhận mode `focus` hoặc `windows`; preferences cũ thiếu mode được đọc như `focus`.
Backend
không nhận user_id đích từ request preferences. Tên đăng nhập 3–32 ký tự ASCII
lowercase/number/_/-, bắt đầu bằng chữ; password 12–256 byte; role admin hoặc user.
Các request body giới hạn 128 KiB. Login tối đa 20 lần theo IP trong cửa sổ 10 phút.

Chưa có endpoint xóa/sửa user, OIDC, files, package install hay cấu hình API key.

## Drive API 0.1 (separate origin/service on port 8082)

Drive authentication is separate from desktop cookies. POST /api/login accepts username/password;
GET /api/session returns user + csrfToken; POST /api/logout revokes the Drive session.
GET /api/roots returns personal and, for admin with agent configured, system.
All file paths below are relative to the selected root. Query: root=personal|system, path=<relative path>.
Only server-side identity determines the user namespace. Mutations require same origin and X-CSRF-Token.

| Method | Route | Request / response |
|---|---|---|
| GET | /api/files/list | {path, entries:[name,path,kind,size,modified,mode,uid,gid,etag]} |
| POST | /api/files/mkdir | {name} |
| POST | /api/files/upload | Raw bytes; query name; max 128 MiB; no overwrite |
| GET | /api/files/download | Attachment, never executable inline content |
| GET | /api/files/text | {content,etag}; UTF-8 <=256 KiB |
| POST | /api/files/save-text | {content,etag}; returns backup filename; rejects stale ETag |
| POST | /api/files/rename | {name}; rejects existing destination |
| POST | /api/files/move | {destination}; same root/filesystem; no overwrite |
| GET | /api/files/trash | Private trash items {id,path,name,deleted} |
| POST | /api/files/trash | Move private path into trash |
| POST | /api/files/restore | {id}; same user only; rejects target collision |
| POST | /api/files/remove | System only; {confirm:true}; regular file or empty directory |

401 unauthenticated; 403 CSRF/origin/role/permission; 404 missing; 409 collision/stale state;
413 size limit; 415 unsupported text; 422 oversized directory; 503 agent/identity/audit unavailable; 507 disk full.
The host agent accepts this narrow file API only on its protected Unix socket. It has no login or shell endpoint.
See drive.md for permissions, backups, data layouts and unsupported operations.

## Embedded Files gateway and copy

Desktop GET /api/drive/session and /api/drive/roots use the existing desktop cookie.
/api/drive/files/* mirrors the Drive file API; all writes require desktop X-CSRF-Token and valid Origin.
Login/logout through the gateway are forbidden. Backend role is resolved from the desktop session, even if another Drive cookie is present.
Integrated Drive requires ALLOW_PLATFORM_SESSIONS=true; desktop requires DRIVE_INTERNAL_URL=http://drive:8082.
Standalone users continue to use /api/* on 8082 and independent Drive cookies.
POST /api/files/copy (or embedded /api/drive/files/copy): query root/path, JSON {destination}.
Only regular files <=128 MiB, same root; 409 if destination exists; folders/special files not supported.

## Planned error-code migration

The accepted future error-code/params contract is in platform-foundations.md, section 4.
Existing {error:string} endpoints still work unchanged. Add stable codes compatibly before removing legacy messages;
frontend translations must use code mappings and a safe unknown-code fallback, not parsing localized error strings.
No locale/profile endpoint or schema migration was implemented by the documentation update.


## Đã triển khai: profile và mã lỗi nền tảng 0.1

GET/PUT `/api/profile` trên Desktop và Drive; native gateway là `/api/drive/profile`.
Xác thực bằng cookie của host, write yêu cầu CSRF/Origin hiện có. Không nhận user ID đích.
Body PUT đầy đủ: `{"locale":"vi-VN","theme":"system","timeZone":"Asia/Bangkok"}`.
Locale chỉ vi-VN/en-US; theme light/dark/system; timezone phải hợp lệ, không nhận Local/rỗng.
GET trả profile mặc định nếu chưa lưu, ưu tiên legacy preferences.theme hợp lệ nếu có.
Cùng DB sẽ có cùng hồ sơ dù cookie Desktop/Drive độc lập; Drive standalone chỉ cần DB của chính nó.
Chưa có tự push cập nhật tới các tab đã đăng nhập, revision hoặc UI timezone.

Lỗi hiện trả `{code:string,error:string}`; `error` tạm giữ tương thích.
Code chung: INVALID_REQUEST, AUTH_REQUIRED, ACCESS_DENIED, NOT_FOUND, METHOD_NOT_ALLOWED,
CONFLICT, PAYLOAD_TOO_LARGE, UNSUPPORTED_MEDIA_TYPE, RESOURCE_LIMIT, STORAGE_FULL,
RATE_LIMITED, SERVICE_UNAVAILABLE, INTERNAL_ERROR, REQUEST_FAILED.
Login sai credential dùng INVALID_CREDENTIALS; profile không hợp lệ dùng PROFILE_INVALID.
Các code chung được chọn từ HTTP status, không suy từ chuỗi tiếng Việt; chưa phân loại mọi lỗi nghiệp vụ chi tiết.
Runtime frontend mới dịch code, code lạ dùng lỗi chung. `params` và `requestId` là phần hợp đồng tiếp theo,
chưa được server phát ra. Phần Planned error-code migration bên trên là lịch sử trước đợt triển khai này.


## Platform Contracts v1 — đặc tả, chưa bật route v1

[OpenAPI 3.1](../contracts/api/openapi.yaml) mô tả profile đang chạy và các API v1 planned.
Các route jobs/uploads/authz/files v1 chưa tồn tại. Các route legacy khác vẫn theo tài liệu hiện tại.
Giữ legacy `{code,error:string}`; client mới đọc thêm `{error:{code,message,requestId,details}}`.
Không đổi sang error object trên /api hiện tại. RequestId v1 phải được server sinh/ghi log, không do frontend tự giả lập.
Cursor/idempotency/deprecation policy nằm trong OpenAPI extensions và [ADR](adr/0001-platform-contracts-v1.md).
Body authz không nhận subject; appId chỉ là ngữ cảnh yêu cầu, backend cần xác minh app principal/delegation riêng.


## Migration startup

Không thêm endpoint hoặc đổi response trong đợt migration runner. /api/profile vẫn giữ hợp đồng hiện có.
Startup kiểm tra profile schema/history trước khi phục vụ; schema không tương thích làm tiến trình dừng thay vì tự sửa dữ liệu.


## Naming compatibility

Chuẩn hóa tên/App ID không đổi endpoint, cookie hoặc registry IDs hiện có. /api/drive và Drive /api vẫn hoạt động.
Catalog contracts/apps là metadata first-party, chưa có endpoint catalog/resolver mới. Chưa dùng alias từ client làm bằng chứng app identity.

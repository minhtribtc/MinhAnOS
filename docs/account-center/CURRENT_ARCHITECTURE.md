# CURRENT ARCHITECTURE — Account Center Phase 0

Ngày khảo sát: 2026-10-30. Source baseline: 026a24f7f9335bb0e42fbdee93aa08bc7e46066c.
Phạm vi: đọc source/contracts/tests và tài liệu chính thức. Không truy vấn dữ liệu riêng, không chạy khai thác,
không đổi code, migration, runtime hoặc database. Các nhận xét dưới đây là kết quả khảo sát mã, không phải pentest.

## Frontend và Desktop integration

| Thành phần | Thực tế | Bằng chứng |
|---|---|---|
| Framework | Vue 3, TypeScript, Vite; npm workspaces, lockfile root | apps/desktop/package.json, package.json |
| State | Pinia dành riêng Desktop | apps/desktop/src/store.ts |
| Router | Không có Vue Router; app ID/registry + window store; query ?app=... | store.ts, App.vue |
| Native host | DesktopWindow → AppContent; keyed v-for/v-show giữ instance/draft | App.vue, components/DesktopWindow.vue |
| Lifecycle | foreground prop; close guards boolean; async lifecycle chưa triển khai | DesktopWindow.vue, store.ts |
| Shared Files | packages/files-app; Desktop /api/drive, standalone /api | AppContent.vue, apps/drive |
| Accounts hiện có | Trang My Account/Users trong AppContent Settings; list/create users | AppContent.vue |
| UI nền | lucide-vue-next; semantic tokens light/dark/system; chưa có component kit đầy đủ | packages/ui |
| i18n | ICU/Intl vi/en, namespace catalog; text cũ chưa chuyển hết | packages/i18n, files-app/locales |
| SDK | provider/profile/API adapter, protocol 1, package 0.1.0 | packages/app-sdk |

Account Center chưa là app native độc lập. Không có packages/accounts-app hoặc apps/accounts.
Catalog đã dành com.minhanos.account-center / minhanos-accounts, trạng thái partial chỉ phản ánh trang Users cũ.

## Backend và database

Go 1.26, net/http + pgxpool, PostgreSQL 18. Desktop và Drive là hai binary hiện có;
không có Identity daemon, Redis hoặc IAM service riêng. Cả hai có thể dùng cùng users table.

| Table | Owner/usage hiện tại | Thiếu so với prompt |
|---|---|---|
| users | id text PK, username unique, display_name, role admin/user, password_hash, created_at | UUID công khai, state/version, groups/RBAC |
| sessions | Desktop token_hash PK, user_id FK, csrf_token, expires_at | session public ID, revoke metadata, activity/device |
| drive_sessions | Phiên Drive độc lập, cùng users; platform adapter đọc sessions | Cùng thiếu metadata/revoke-all |
| preferences | Layout/wallpaper/notes/theme legacy, FK users.id | Không chuyển sang Account Center |
| user_profiles | locale/theme/timezone, FK users.id | Reuse nguyên dữ liệu |
| audit | Desktop auth success/logout/create-user và hoạt động tài khoản | Login failure/revoke/role/disable events chưa đủ |
| drive_audit | File mutation request, không phải auth audit tập trung | Outcome/correlation/unified reader |
| schema_version | Schema bootstrap legacy v1 | Identity chưa được runner tiếp nhận |
| platform_migrations | Module runner, profile v1 đã triển khai | Identity module chưa có |

Bootstrap users/sessions còn ở internal/platform/server.go và internal/drive/auth.go.
Runner internal/migrate có checksum/DAG/transaction/advisory lock; profile SQL 001 bất biến.
Không sửa profile migration 001 để thêm Identity. Không gọi runner qua connection khác trong transaction bootstrap chưa commit.

## Authentication và session

[Shared primitives](../../internal/authn/password.go) dùng Go standard crypto/pbkdf2, HMAC-SHA256 600000 iterations,
16-byte salt, 32-byte output; format saltHex:hashHex; crypto/rand tạo token 32 bytes, SHA-256 chỉ lưu digest.
Đây đã là thư viện chuẩn, không phải PBKDF2 tự viết. Format chưa lưu algorithm/work-factor nên cần metadata/version khi mở rộng.

Desktop login xoay token và xóa cookie-session cũ trong transaction, ghi audit success.
Drive login tạo token mới nhưng chưa xóa phiên Drive cũ khi đăng nhập lại. Hai session realms hiện có là chủ ý tương thích;
không tạo bảng users/session thứ ba cho Account Center.

Cookie HttpOnly, SameSite=Strict, TTL 24h; Secure theo cấu hình. LAN HTTP preview chưa là cấu hình production HTTPS.
Cookie không được phân tách theo port: backend phải chọn đúng cookie/realm, không coi 8081/8082 là hai cookie jars.
Write requests kiểm tra CSRF/Origin; limiter in-memory 20 attempts/IP/10 phút ở từng process.
Chưa có account status gate, lockout theo tài khoản, password change/reset, reauthentication, MFA/WebAuthn hoặc recovery CLI.
Bootstrap secret chỉ tạo admin khi users rỗng, không phải cơ chế recovery cho DB đang có user.

## Authorization, registry và API

Role được backend lấy từ DB/session join, admin middleware kiểm tra role=admin; không dựa vào role frontend để cấp quyền.
Hiện có /api/login, /session, /logout, /apps, /profile, /preferences, /admin/users GET/POST, /audit.
Không có Identity API v1 hoặc session-management endpoints. Legacy errors {code,error:string}; nested v1 là contract tương lai.

Manifest schema có id/version/sdk/entrypoint/display/fileHandlers/capabilities/lifecycle declaration.
Chưa có navigation/settings integration schema hoặc runtime manifest loader/delegated grants.
Catalog apps và aliases là metadata first-party, chưa phải runtime resolver. Requested capabilities không phải grants.

## Files ownership và agent

Personal files nằm dưới /data/users/<64-hex-users.id>/files; validID hiện chỉ nhận hex 64 ký tự.
Shared web service chạy uid/gid 10001; không ánh xạ mỗi web user sang POSIX uid/gid riêng.
Phân tách personal API dựa vào identity server và rooted namespace, không phải POSIX ACL theo user web.
ETag dựa inode/size/mtime; chưa có persistent fileId, share ACL hoặc storage policy service.

Host file agent mở root /, chạy root và nghe Unix socket 0660 root:10001; caller được kiểm soát qua filesystem socket.
Agent không có shell endpoint nhưng có quyền đọc/ghi file host rộng; blocked roots là proc/sys/dev/run.
Actor header dùng để log, không phải per-user authorization proof; chưa có peer-credential/job-grant verification độc lập.
Web admin được vào system root khi agent cấu hình. Quyền này có thể đi vòng logical personal namespace qua host storage;
chưa đủ cơ sở khẳng định System Admin web luôn bị cấm đọc file cá nhân. Không thử đọc các tệp đó trong audit này.

## Tái sử dụng trực tiếp

Giữ users, cả hai realms session, authn primitives, coreprofile, migrate runner, apicontract/API client,
i18n/ui/SDK, keyed native host và Files rooted engine. Refactor ownership/repository, không dựng IAM DB/store mới.
Tests hiện có: password verification, CSRF/Origin, platform/Drive sessions, gateway isolation, Files operations,
profile isolation và migration rehearsal. Chưa có test last-admin, disable, groups/RBAC, recovery hoặc delegated grants.

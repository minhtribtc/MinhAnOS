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

# Desktop 0.1 — kiến trúc đã triển khai

- Frontend Vue 3/TypeScript/Pinia, Vite. Không phụ thuộc kodbox hoặc 1Panel.
- Go net/http phục vụ cùng origin cho API và static assets; không cần reverse
  proxy trong LAN test. PostgreSQL lưu users, sessions, preferences và audit.
- Session token ngẫu nhiên 256 bit; DB chỉ lưu SHA256 token; cookie HttpOnly,
  SameSite Strict, TTL tuyệt đối 24 giờ. LAN HTTP test chưa dùng Secure cookie.
- PBKDF2-SHA256 600.000 vòng, salt riêng cho password; giới hạn login theo IP.
- Write API kiểm tra Origin/fetch metadata và CSRF session token. Backend kiểm
  tra role admin tại các route user management; frontend chỉ điều chỉnh UI.
- Preferences gồm theme, wallpaper, notes, pin list và window geometry; khóa user
  lấy từ session, không nhận user ID đích từ request. Unknown app IDs bị từ chối.
- API chạy uid 10001, read-only rootfs, drop ALL capabilities, không có Docker socket.
- Admin bootstrap từ secret file khi users table rỗng. Không có password mặc định.
- Registry gồm 8 module: overview, files, apps, connections, settings, activity,
  notes, about. Files và integrations ghi rõ planned/unconfigured.
- Metrics: heap Go, goroutines, process uptime và filesystem /data. Chưa đo host
  toàn diện, không có Linux agent, không dùng số liệu hoặc phần trăm CPU giả.
- Migration schema v1 tạo transaction tại startup. Chưa có migration runner đa
  phiên bản hoặc chính sách downgrade; phải bổ sung trước các thay đổi schema tiếp.

## Những giới hạn chủ động của bản đầu

Chưa có OIDC, ACL file, NAS, jobs, package install, SSO, file upload/download,
change/reset password, disable/delete user hay nhiều node. App Center là registry
nội bộ, không phải marketplace. Nhấn vào một module chưa hoàn thành mở trang mô
tả trạng thái, không thực hiện thao tác dịch vụ.

Go API/desktop/database chạy Docker Compose trên VM test. Ứng dụng quản trị
thực tế cần HTTPS, IdP/permission model chi tiết, migration/backup restore đã thử
và hardening riêng trước khi đưa ra ngoài LAN.

## Các quyết định tiếp theo

1. Authentik OIDC có issuer/client/callback thật; phân tách external identity và local user.
2. Adapter đọc trạng thái NetBird/1Panel sau khi kiểm kê version/capability.
3. Permission registry chi tiết theo module/action; audit và thu hồi phiên.
4. File service có data volume, ACL và journal/reconciliation, không nối trực tiếp
   vào data của kodbox đang chạy.

## Interface V2 (0.2.0)

Shell components: App.vue orchestrates Home, search, switcher and Control Center.
HomeScreen renders widgets from existing store/API data; AppSwitcher lists real open instances.
ipados.css loads last, replacing the outer Fluent shell while keeping shared app interiors.
Home visibility is local UI state; it does not minimize/remove application instances.
Preference IDs and backend API/schema remain compatible with desktop-v1.
Do not confuse this interface iteration with completion of the full V2 multitasking roadmap.

## Drive 0.1 — independently runnable application

Read [drive.md](drive.md). apps/drive is a separate Vue build; cmd/drive/internal/drive serve its API/static files on 8082.
The Drive image does not copy desktop/platform sources. compose.drive.yaml deploys only Drive/PostgreSQL.
Integrated mode shares users but has a separate Drive session table/cookie. IdentityStore decouples identity integration;
authn holds shared password/token primitives. Both schema initializers serialize startup via the same advisory transaction lock.
No desktop HTTP call, UI state, iframe or parent-window messaging is required.
Host files are a separate privileged Unix-socket service; the web container remains uid 10001 without a host root mount.
Private files and trash are filesystem-backed under immutable user IDs. Request audit uses PostgreSQL; host outcomes use journald.
This release does not add durable transfers/jobs or claim power-loss transactionality; see drive.md for limits.

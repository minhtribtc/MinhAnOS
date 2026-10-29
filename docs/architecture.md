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

# MinhAnOS Desktop 0.1

Nhánh `desktop-v1` chứa nền tảng Go/Vue được xây mới. Nhánh `main` của repo
`minhtribtc/MinhAnOS` tiếp tục giữ bản kodbox tham chiếu.

**Trải nghiệm mặc định:** launcher toàn màn hình và mỗi app một vùng làm việc,
với taskbar kiểu Fluent. Chuyển ứng dụng không làm unmount component; biểu mẫu,
tab và tìm kiếm trong app đang mở được giữ. Chế độ cửa sổ nâng cao vẫn có tại
Cài đặt → Giao diện → Cách làm việc.

Xem [roadmap V1/V2/V3 và hợp đồng giữ trạng thái](docs/roadmap.md),
[bản cập nhật giao diện](docs/fluent-update.md).

Desktop web độc lập: Vue 3 + TypeScript, Go, PostgreSQL. Bản đầu có đăng nhập
cục bộ, desktop/window manager, launcher, theme/hình nền, ghi chú riêng, quản lý
tạo tài khoản, nhật ký xác thực/quản trị và số liệu tiến trình/filesystem thật.

NetBird, Authentik, 1Panel, File Station, App Center packages và Linux host agent
**chưa được tích hợp**. Các màn hình tương ứng ghi rõ trạng thái hiện tại.
Ứng dụng vẫn chạy độc lập với bản kodbox tham chiếu trên cổng 8080.

## Triển khai thử

VM: `10.1.1.20`, nguồn: `/opt/minhanos-next`, URL: `http://10.1.1.20:8081`.
Tên đăng nhập bootstrap là `admin`. Mật khẩu được tạo ngẫu nhiên và đọc từ
`secrets/bootstrap_password` khi database chưa có người dùng; không ghi vào git
hoặc log. Thay file bootstrap sau khi tạo user không đổi mật khẩu đã lưu.

```sh
# Trước lần chạy đầu, tạo .env chứa POSTGRES_PASSWORD ngẫu nhiên,
# và secrets/bootstrap_password dài tối thiểu 12 ký tự.
# Trên VM test, bootstrap file thuộc uid 10001, mode 0400.
docker compose up -d --build
docker compose ps
docker compose logs --tail=50 desktop
```

App chạy non-root, root filesystem read-only, không gắn Docker socket. Database
không mở cổng ra host. Cấu hình hiện tại dành riêng LAN HTTP: COOKIE_SECURE=false.
Khi dùng qua HTTPS phải đặt COOKIE_SECURE=true; cài OIDC và đánh giá trước khi
phát hành ra Internet. Cookie có HttpOnly/SameSite=Strict, session lưu server-side,
write API kiểm tra CSRF và Origin, password hash PBKDF2-SHA256 với salt riêng.

## Sử dụng

- Dùng Start/launcher hoặc icon taskbar để mở/chuyển ứng dụng. Chế độ cửa sổ có thêm icon desktop.
- Ctrl+Space mở tìm ứng dụng; Escape đóng menu; Alt+Tab chuyển cửa sổ.
- Ở chế độ cửa sổ, kéo titlebar để di chuyển; kéo góc dưới phải để resize. Có thể dùng phím mũi tên
  khi focus nút resize. Double click titlebar để phóng to/khôi phục.
- Theme, wallpaper, ghi chú, pinned apps và các cửa sổ lưu riêng theo user trong
  PostgreSQL. Chờ trạng thái tự lưu xong trước khi đóng tab.
- Tài khoản admin có thể tạo user tại Cài đặt → Người dùng. User thường không có
  quyền gọi API quản trị. Chưa có sửa/xóa/reset mật khẩu trong UI bản 0.1.
- `?app=notes` hoặc `?app=settings` mở trực tiếp một module sau đăng nhập.

## Build và test

```sh
# Có Node 22.12+ và Go 1.26+
cd apps/desktop && npm ci && npm run build
# Tại root repository
go test ./...
go vet ./...
docker compose build
# API integration test, sau khi ứng dụng chạy:
TEST_URL=http://10.1.1.20:8081 TEST_PASSWORD_FILE=secrets/bootstrap_password node tests/api-smoke.mjs
```

Test tích hợp tạo một user dùng một lần, ghi username vào artifacts/test-user.txt.
Dọn đúng user đó khỏi database test sau khi chạy; không chạy trên database người dùng
thật. Test khôi phục preferences admin về giá trị ban đầu trong finally.

## Dữ liệu và vận hành

Named volumes `database` và `desktop-data` tồn tại độc lập với image/container.
Restart không xóa user hoặc preferences. Không dùng `docker compose down -v`
nếu cần giữ dữ liệu. Backup cả PostgreSQL, cấu hình deployment và các volume dữ liệu.

Metrics hiện là Go heap, goroutines, uptime API và filesystem `/data`; không phải
CPU/RAM toàn VM. Đăng nhập cục bộ là cơ chế thật, không giả lập Authentik SSO.
Chưa có tenant/ACL file và các chức năng NAS, chưa kết luận sẵn sàng production.

Nguồn mới không chứa lõi PHP làm rối của bản tham chiếu. Chưa công bố giấy phép
phân phối riêng cho mã ứng dụng; cần chốt trước khi cấp phép phân phối lại.

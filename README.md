# MinhAnOS Desktop + Drive

[Hướng dẫn sử dụng](docs/user-guide.md) · [Danh mục tài liệu](docs/README.md) · [Kiến trúc nền tảng, i18n và theme](docs/platform-foundations.md)

**Drive 0.1 is available at http://10.1.1.20:8082** on branch `file-station-desktop`.
It is an independent Vue/Go application with its own login, API, build and deployment.
File Station opens natively inside desktop 8081; both hosts share packages/files-app. See [Drive setup, capabilities and limitations](docs/drive.md).
Standalone: `docker compose -f compose.drive.yaml up -d --build` (with runtime secrets configured).

The sections below retain desktop setup history; Files is now implemented in Drive.
# MinhAnOS Desktop 0.2

Nhánh `desktop-v2-ipados` chứa giao diện V2 lấy cảm hứng iPadOS. `desktop-v1` giữ bản trước, `main` giữ kodbox tham chiếu.

Home có widget, lưới ứng dụng, Dock nổi, tìm kiếm và bộ chuyển ứng dụng. Trung tâm điều khiển quản lý giao diện sáng/tối và tài khoản. Mặc định mở từng app trong vùng làm việc lớn; chế độ cửa sổ nâng cao vẫn có tại Cài đặt → Giao diện.

Chuyển app, về Home hoặc thu nhỏ giữ component và biểu mẫu trong phiên. Đóng app, reload hoặc đăng xuất không giữ biểu mẫu chưa lưu. Terminal/upload/tác vụ nền bền vững chưa có.

Agent tiếp theo bắt đầu tại [AGENTS.md](AGENTS.md), [bàn giao](docs/HANDOFF.md), [kiến trúc](docs/architecture.md), [roadmap](docs/roadmap.md) và [kiểm chứng](docs/validation.md). Tài liệu phải cập nhật cùng commit triển khai.
Desktop web độc lập: Vue 3 + TypeScript, Go, PostgreSQL. Bản đầu có đăng nhập
cục bộ, desktop/window manager, launcher, theme/hình nền, ghi chú riêng, quản lý
tạo tài khoản, nhật ký xác thực/quản trị và số liệu tiến trình/filesystem thật.

NetBird, Authentik, 1Panel và App Center packages
**chưa được tích hợp**. File Station đã chuyển thành Drive độc lập; host agent hiện chỉ phục vụ thao tác tệp.
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

- Chạm icon Home hoặc Dock để mở app; dùng nút các lớp trong Dock để chuyển ứng dụng đang mở.
- Ctrl+Space mở tìm ứng dụng; Escape đóng menu; Alt+Tab chuyển cửa sổ.
- Ở chế độ cửa sổ, kéo titlebar để di chuyển; kéo góc dưới phải để resize. Có thể dùng phím mũi tên
  khi focus nút resize. Double click titlebar để phóng to/khôi phục.
- Theme, wallpaper, ghi chú, pinned apps và các cửa sổ lưu riêng theo user trong
  PostgreSQL. Chờ trạng thái tự lưu xong trước khi đóng tab.
- Tài khoản admin có thể tạo user tại Cài đặt → Người dùng. User thường không có
  quyền gọi API quản trị. Chưa có sửa/xóa/reset mật khẩu trong UI bản 0.2.
- `?app=notes` hoặc `?app=settings` mở trực tiếp một module sau đăng nhập.

## Build và test

```sh
# Có Node 22.12+ và Go 1.26+
npm ci
npm run build --workspace=minhanos-desktop
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

Bản thiết kế ứng dụng quản lý tệp: [File Station](docs/file-station-design.md) — đang đề xuất, chưa kết nối dữ liệu thật.


Foundation preview: npm workspace/lockfile chung, profile locale/theme và shared i18n/ui/SDK adapters.
Đây là chuyển đổi từng phần. Xem [mức độ đáp ứng và quy ước mở rộng](docs/extension-contracts.md).


[Platform Contracts v1](contracts/README.md): machine-readable schemas, generated shared-types và OpenAPI CI.
Chạy npm run contracts:generate sau khi đổi schema, npm run contracts:check trước khi phát hành.
Các route v1 là planned, chưa phải chức năng có trên VM.


Migration profile hiện dùng runner transactional; xem [ADR 0002](docs/adr/0002-transactional-migration-runner.md).
Test riêng: sh tests/migrations-integration.sh (Docker; tạo DB disposable, không dùng DB ứng dụng).


Naming: [ADR 0003](docs/adr/0003-app-naming-and-repository-policy.md) defines eight app identities in the existing monorepo.
Standalone Files frontend: npm run build --workspace=minhanos-files (source remains apps/drive).

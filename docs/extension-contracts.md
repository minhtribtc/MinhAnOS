# Hợp đồng mở rộng và kết quả rà soát nền tảng

Ngày: 2026-10-30. Nhánh: `platform-foundations-v1`.

## Kết quả rà soát

Mã cũ chỉ đáp ứng một phần kiến trúc đã chốt. Files/Drive độc lập với Shell và API tệp/agent đã tách;
nhưng dependency graph, i18n, theme, hồ sơ tài khoản và lỗi chưa thống nhất.

| Yêu cầu | Hiện tại sau đợt tối ưu này | Phần còn thiếu |
|---|---|---|
| Core độc lập Shell | `internal/coreprofile`, `internal/apicontract`, auth primitives dùng chung | Core identity/capability API chính thức, OIDC |
| Shared Design System | `@minhanos/ui`: semantic tokens, light/dark/system, focus/motion tokens | Component library, chuyển hết palette và kiểm tra contrast/accessibility |
| App SDK | `@minhanos/app-sdk` 0.1: host provider v1, profile, API client/ApiError, theme resolver | Manifest schema, capability negotiation, lifecycle async, SDK compatibility tests |
| i18n | `@minhanos/i18n` 0.1, FormatJS ICU, Intl, vi/en, namespace ownership/unregister | Chuyển toàn bộ nội dung Desktop/Files và quản lý plugin ngôn ngữ |
| Locale tài khoản | `/api/profile`; cùng DB/profile cho Desktop và Drive; guest fallback | Push cập nhật giữa các tab đang mở; UI múi giờ; optimistic concurrency |
| Catalog theo app | Files sở hữu `files.*`; SDK chỉ giữ lỗi chung `common.errors.*` | Catalog `desktop.*` đầy đủ |
| Lỗi ổn định | Thêm `code` bên cạnh `error`; auth phân biệt INVALID_CREDENTIALS/AUTH_REQUIRED | Code chi tiết theo nghiệp vụ, params, requestId và chuyển mọi màn hình cũ |
| Kiểm tra build | Một npm workspace/lock; catalog parity/ICU/parameter và ranh giới phụ thuộc | Phát hiện mọi hardcoded text/token, contract/API schema generation |

**Chưa đạt đầy đủ V0.1**: menu thao tác Files, các nhãn hồ sơ, lỗi chung và đồng hồ dùng nền mới;
nhiều nội dung cũ vẫn là tiếng Việt. Chọn English sẽ có giao diện pha trộn trong giai đoạn chuyển đổi.
Không coi package đầu tiên là SDK hoàn chỉnh hay Design System hoàn chỉnh.

## Cách dùng các package

Chạy `npm ci` tại root, `npm run build` để kiểm tra/build cả hai host.
`npm run build --workspace=minhanos-drive` chỉ build Drive; Docker image không cần mã Desktop frontend/backend.
Docker builder có đọc manifest Desktop để tái tạo lockfile chung; đó không phải phụ thuộc runtime.
Các lockfile riêng từng app đã được thay bằng `package-lock.json` tại root.

Entry point tạo `createHost()` và cài vào Vue app. Mỗi host sở hữu provider, profile và client riêng.
Files lấy host qua `useHost()`, đăng ký catalog Files của mình và hủy đăng ký khi unmount.
Không đưa Pinia/Desktop vào SDK hoặc Files. Provider hiện quản lý theme/lang ở document root của mỗi host;
chưa hỗ trợ hai root provider có theme khác nhau trong cùng document. Portal dùng tokens từ document root.

`createApiClient(base)` có CSRF theo instance; chỉ gửi cùng origin. `ApiError` giữ status/code và legacy message.
Host `errorMessage` dịch code đã biết; code lạ trả thông báo chung, không hiển thị legacy payload.
Không dùng nội dung `error.message` để quyết định đăng nhập/quyền. Các catch cũ ngoài phần đã chuyển còn nợ xử lý.

Runtime i18n không phụ thuộc Vue hoặc Desktop. ICU dùng `intl-messageformat`, không tự viết parser.
API: `createI18n`, `registerCatalog`, `setLocale`, `setTimeZone`, `t`, `formatDate/Number/Currency`.
Catalog phải đủ vi/en, cùng key/tham số/loại ICU; đăng ký namespace trùng bị từ chối.
Chuỗi là plain text, không dùng `v-html`. Tên ngôn ngữ tự gọi (Tiếng Việt/English) là autonym cố định.
Bản dịch plugin phải được host cấp namespace; chưa có cơ chế tải plugin không tin cậy.

## Những điều nên thống nhất trước khi thêm app

1. **Danh tính và quyền**: ID opaque ổn định, quyền dạng `files.read`, `files.system.write` do backend kiểm tra;
   role chỉ là nhóm quyền. App không tự suy quyền từ tên tài khoản, UI hoặc tham số client.
2. **Manifest và SDK**: app ID không đổi, phiên bản manifest/SDK, capabilities, entry points native/standalone,
   giới hạn nhiều instance, lifecycle foreground/background/close. Viết schema và compatibility policy trước plugin đầu tiên.
3. **API**: JSON schema/OpenAPI, code lỗi không phụ thuộc ngôn ngữ, pagination/cursor, timestamp UTC,
   ETag/If-Match cho cập nhật xung đột, requestId cho chẩn đoán. Thêm trường tương thích trước khi loại bỏ trường cũ.
4. **Công việc dài hạn**: job ID thuộc server, trạng thái queued/running/succeeded/failed/cancelled,
   retry/idempotency/cancel và sự kiện tiến độ. Terminal/transfer thuộc service; Shell chỉ hiển thị.
   Upload trình duyệt hiện tại chưa có tính bền vững đó.
5. **Dữ liệu**: migration có số phiên bản và lock; backup/restore được thử; quyền sở hữu bảng/schema theo service.
   `CREATE TABLE IF NOT EXISTS` hiện có không thay thế migration runner cho mọi thay đổi tương lai.
6. **Hồ sơ, theme và i18n**: Core profile là nguồn locale/theme/timezone; Shell chỉ giữ layout/wallpaper.
   Không thêm theme thứ hai trong app. Catalog theo namespace, semantic tokens, keyboard/focus,
   reduced motion, quy tắc portal và accessibility là tiêu chí chung.
7. **SDK và dependency**: một lockfile, graph không chu trình, không import implementation của app khác.
   Không chia repo trước khi có versioning và phát hành package độc lập ổn định.
8. **Vận hành**: audit cho thao tác đặc quyền, logs có correlation ID, health/readiness và configuration schema;
   không lưu secret vào registry, manifest, catalog hoặc log. CI bắt buộc trước khi phát hành.

Các mục chưa hiện thực ở trên là quyết định/việc tiếp theo, không phải chức năng đã có.
Ưu tiên tiếp: hoàn tất catalog Desktop/Files → Settings hồ sơ chung và bỏ legacy theme khỏi preferences
bằng migration tương thích → manifest/lifecycle → migration runner + API schema → jobs/capabilities.

Tham khảo thư viện ICU chính thức: [FormatJS Intl MessageFormat](https://formatjs.github.io/docs/intl-messageformat/).


Cập nhật migration: shared runner transactional đã có, áp dụng profile-first; identity adoption còn pending.
Xem [ADR 0002](adr/0002-transactional-migration-runner.md). Generated shared types đã được kiểm tra tên export
và SDK Profile không suy thành any bằng một TypeScript check riêng không bỏ qua declaration errors.

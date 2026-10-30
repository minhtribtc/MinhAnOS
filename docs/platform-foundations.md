# Nền tảng MinhAnOS: kiến trúc, i18n và Theme

Ngày chốt định hướng: 2026-10-30. Trạng thái: **quyết định kiến trúc và đặc tả để triển khai**.
Tài liệu này không xác nhận các package/API mô tả bên dưới đã tồn tại.

## 1. Kiến trúc đích

**Platform Core + Shared Design System + App SDK + ứng dụng ngang hàng + nhiều Shell có thể thay thế.**

| Thành phần | Trách nhiệm | Không được phụ thuộc vào |
|---|---|---|
| Platform Core | Danh tính, quyền, hồ sơ/locale, registry, audit; nền tảng jobs/notifications theo lộ trình | UI hoặc store riêng của một Shell |
| Shared Design System | Design Tokens, component dùng chung, accessibility và quy ước tương tác | Desktop, Drive, một ứng dụng nghiệp vụ cụ thể |
| App SDK | Manifest, capability contracts, lifecycle/foreground/close, adapter API/identity/i18n/theme | Window manager cụ thể hoặc import store của Shell |
| Ứng dụng ngang hàng | Nghiệp vụ và API riêng: Files, webserver, NAS… | Mã nội bộ Desktop hoặc ứng dụng ngang hàng khác |
| Shell | Home/Dock, điều hướng, bố cục, chuyển app và presentation | Quyết định quyền truy cập thay backend |

Shell Windows, iPad, mobile hoặc Drive là các host có thể thay thế. Một ứng dụng native có thể gắn
trực tiếp vào Shell hoặc chạy độc lập với provider tương đương. Không dùng iframe để thay cho hợp đồng ứng dụng.
Settings quản lý hồ sơ và các trang cấu hình do app đăng ký; không trở thành nơi chứa toàn bộ nghiệp vụ các app.

Monorepo là cách tổ chức hiện tại. Tách package/service và khả năng phát hành trước khi cân nhắc tách repo.
SDK/shared packages cần version và compatibility policy rõ; không tạo chu trình Core → Shell → App → Core implementation.

### Phần đã có và khoảng cách

Đã có: Vue desktop host, Drive độc lập, package `files-app` dùng lại, API tệp độc lập,
adapter phiên tích hợp và agent file đặc quyền tách khỏi web app.
Chưa có package App SDK, Shared Design System, i18n hay hệ Theme Tokens hoàn chỉnh.
Auth/session còn hai chế độ có adapter; jobs/notifications bền vững chưa được coi là hoàn tất.

## 2. Sáu nguyên tắc i18n bắt buộc

1. Văn bản hiển thị dùng translation key, không hardcode trong component, action registry, dialog hoặc toast.
2. Key là định danh có cấu trúc, không dùng nguyên câu tiếng Việt: `files.actions.delete`.
3. Catalog chia theo ứng dụng; Shell không sở hữu từ vựng của NAS, Odoo, Docker hoặc Files.
4. Locale lưu theo tài khoản trong Core và đồng bộ khi đăng nhập ở thiết bị khác; có fallback trước đăng nhập.
5. Dùng ICU messages và Intl cho định dạng ngày/giờ, số, tiền tệ, số nhiều và danh sách.
6. Backend trả error code ổn định; frontend dịch theo code, không so sánh/phân tích chuỗi lỗi tiếng Việt.

Các chuỗi hardcode hiện tại là nợ chuyển đổi. Cần triển khai foundation trước rồi di chuyển theo ứng dụng;
không tuyên bố đạt i18n chỉ vì thêm một file JSON nhưng component vẫn chứa văn bản trực tiếp.

## 3. Package `@minhanos/i18n` — đặc tả

Locale khởi đầu: `vi-VN`, `en-US`. Đây là mốc i18n V0.1, không phải đổi ngược phiên bản desktop hiện tại.
Package đứng độc lập với Shell. Cần core runtime và Vue adapter; không tạo global mutable singleton
chứa locale của mọi app/phiên. Provider phải được cô lập theo app host/SSR request nếu sau này có SSR.

API định hướng: `t(key, params)`, `setLocale(locale)`, `formatDate`, `formatNumber`, `formatCurrency`,
`registerCatalog(namespace, locale, messages)` và hủy đăng ký khi tháo plugin. Tên API chưa phải hợp đồng đã phát hành.

### Namespace và catalog

```text
packages/i18n/             # Runtime/adapters; từ chung tối thiểu
packages/ui/               # Component/tokens nền tảng
packages/app-sdk/          # Provider và hợp đồng host/app
apps/desktop/locales/      # desktop.*
packages/files-app/locales/# files.* — Drive và desktop cùng dùng
apps/drive/locales/        # drive.* — phần host riêng của Drive
```

Ví dụ key: `desktop.launcher.search`, `files.actions.delete`, `files.selection.count`,
`files.errors.nameConflict`, `settings.language.label`.
Namespace `common.*` chỉ chứa khái niệm thật sự chung; không biến thành catalog tổng hợp mọi ứng dụng.
Không nối các mảnh câu đã dịch, dùng nguyên câu và tham số có tên. Label, aria-label,
placeholder, tooltip, empty state, progress, confirm và thông báo lỗi đều nằm trong phạm vi chuyển đổi.

Ví dụ catalog **định hướng**:

```json
{
  "files.selection.count": "{count, plural, one {# item selected} other {# items selected}}",
  "files.actions.delete": "Move to trash"
}
```

ICU xử lý nội dung/số nhiều; Intl xử lý format. Không tự viết parser ICU tùy tiện.
Chốt thư viện sau khi kiểm tra khả năng build, license và kiểm thử; tài liệu này chưa lựa chọn dependency cụ thể.

### Chọn và đồng bộ locale

Thứ tự: locale tài khoản đã xác thực → lựa chọn khách đã lưu → locale trình duyệt được hỗ trợ → `vi-VN`.
Nếu locale tài khoản không được hỗ trợ, chọn fallback nhưng không tự ghi đè lựa chọn tài khoản phía server.
Khi đăng nhập, tài khoản là nguồn sự thật; lựa chọn khách không âm thầm ghi đè tài khoản.
Đổi locale từ Settings gọi API hồ sơ Core rồi phát sự kiện provider để các app đang mở cập nhật mà không remount.
App độc lập dùng cùng hợp đồng hồ sơ hoặc adapter tương đương; không gọi store Settings/Desktop.
Chỉ lưu mã locale không nhạy cảm ở client; không lưu token/danh tính vào localStorage để phục vụ i18n.

Ngày/giờ API nên dùng dạng máy đọc ổn định như ISO 8601/UTC; số giữ dạng số.
Locale và múi giờ là hai tùy chọn khác nhau; đổi sang tiếng Anh không tự đổi múi giờ.
Tiền tệ cần mã tiền tệ từ dữ liệu/ngữ cảnh nghiệp vụ, không suy ra từ locale một cách máy móc.
Đổi locale không đổi tên tệp, tên người dùng hay nội dung tài liệu do người dùng tạo.

### Fallback và lỗi catalog

Lookup trong namespace của app, locale hiện tại → fallback `vi-VN` → placeholder dễ nhận diện/log thiếu key.
Không để exception dịch thuật làm sập UI. Production cần fallback rõ, development/CI phải báo thiếu key.
Một catalog chỉ được đăng ký trong namespace đã cấp, không ghi đè từ của Core hoặc app khác.
Bản dịch và tham số mặc định là plain text, không render HTML tùy ý; không chạy mã do plugin ngôn ngữ cung cấp.

## 4. Hợp đồng lỗi backend — cần triển khai

Các endpoint hiện còn trả `{error: "..."}`. Hợp đồng đích minh họa:

```json
{
  "code": "FILES_NAME_CONFLICT",
  "params": {"name": "report.txt"},
  "requestId": "opaque-correlation-id"
}
```

Code mô tả ngữ nghĩa và giữ ổn định; HTTP status tiếp tục mô tả nhóm kết quả.
App sở hữu ánh xạ code → translation key. Ví dụ `FILES_NAME_CONFLICT` → `files.errors.nameConflict`.
Không trả translation key của một Shell từ backend; không lấy chuỗi tiếng Việt làm mã lỗi.
Không đổi code chỉ vì đổi lời thông báo hoặc locale.
Params không chứa secret, stack trace hoặc đường dẫn host không được phép tiết lộ.

Di chuyển có tương thích: thêm code/params trước, frontend ưu tiên code; tạm giữ `error` cho client cũ,
sau đó mới bỏ theo phiên bản hợp đồng. Code không biết dùng thông báo lỗi chung kèm requestId nếu có.
Frontend không được render trực tiếp HTML từ error/params hoặc dò substring chuỗi để điều khiển nghiệp vụ.
Kiểm thử phải xác nhận code/status/quyền, không khóa vào câu chữ của một ngôn ngữ.

## 5. Theme / Design Tokens — năng lực nền tảng

Theme provider thuộc nền tảng/Design System, không thuộc Desktop. Shell và app dùng cùng semantic tokens,
nhưng Shell được quyết định bố cục, Dock, titlebar và độ đậm đặc của chính nó.
Ví dụ tên token định hướng: `--mn-color-surface`, `--mn-color-text-primary`, `--mn-color-border`,
`--mn-color-action-primary`, `--mn-color-danger`, `--mn-space-3`, `--mn-radius-control`, `--mn-focus-ring`.

Tách ba lớp: primitive palette/scale → semantic tokens → component tokens. App không import CSS Shell
để nhận màu/font và không chép palette riêng cho mỗi host. Màu chỉ mang ý nghĩa phải đi cùng icon/label.
Token phải bao phủ typography, spacing, radius, elevation, focus, motion và trạng thái tương tác.
Component dùng token; ngoại lệ như màu nội dung người dùng cần lý do và không thay cho semantic UI tokens.

Ban đầu light/dark/system; mode tài khoản và system preference có fallback trước đăng nhập.
Theo dõi thay đổi system theme khi chọn system, hỗ trợ reduced motion, focus hiển thị và contrast.
Theme và locale thay đổi tại runtime, giữ nguyên form, vị trí, lựa chọn và transfer đang chạy.
Portal/modal/menu phải nhận được provider/token tương ứng, không quay về màu hardcode khi ra ngoài cây app.
Giao diện Windows và iPad dùng cùng nền tảng, không buộc các app sao chép cách bố trí của Shell.

## 6. Lộ trình và nghiệm thu

| Mốc | Phạm vi | Điều kiện hoàn thành |
|---|---|---|
| Foundation/i18n V0.1 | `@minhanos/i18n`, vi-VN/en-US, quy ước key, locale tài khoản, error codes; bộ token/theme đầu tiên | Desktop + Files/Drive đổi ngôn ngữ/theme khi đang mở; không mất state; hai host dùng chung catalog; API trả code ổn định |
| Foundation/i18n V0.2 | Kiểm tra catalog/ICU/params khi build, phát hiện thiếu bản dịch, plugin đăng ký catalog | Build thất bại khi key/schema/placeholder không hợp lệ; plugin không vượt namespace; unregister/fallback hoạt động |
| Thương mại hóa | Thêm locale, quy trình đóng góp/review cộng đồng, ownership và compatibility policy | Có hướng dẫn dịch, kiểm chứng format/accessibility, hỗ trợ layout/RTL khi locale yêu cầu |

Thứ tự triển khai: runtime/provider và schema locale → error codes tương thích → chuyển Files/Drive và Desktop
→ Settings ngôn ngữ/theme → design tokens/component chung → SDK chính thức → công cụ/registry plugin.
Không cần đợi thương mại hóa để kiểm tra tối thiểu key parity/ICU syntax; V0.2 hoàn thiện và bắt buộc hóa công cụ.

Kiểm chứng: hai user có locale khác nhau, đăng nhập thiết bị khác, khách chưa đăng nhập, fallback,
chuỗi dài tiếng Anh, ngày/số/số nhiều, lỗi code không biết, portal/theme, isolation provider và đổi locale trong lúc upload.
Đánh dấu từng phần đã triển khai trong `HANDOFF.md`, API/kiến trúc và hướng dẫn sử dụng cùng commit.

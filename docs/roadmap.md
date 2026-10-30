# MinhAnOS: dễ dùng như tablet, đa nhiệm cho quản trị

## Quyết định sản phẩm

Đề xuất launcher → đa nhiệm từng phần → window manager phù hợp với MinhAnOS.
Giữ tên MinhAnOS; tên NextZen trong đề xuất được hiểu là ví dụ tham chiếu.

Trải nghiệm mặc định là **Tập trung**: launcher toàn màn hình, mỗi ứng dụng chiếm
vùng làm việc phía trên taskbar. Chế độ **Cửa sổ** đã có trong prototype được giữ
lại như tùy chọn nâng cao; không bắt người dùng phổ thông quản lý nhiều cửa sổ.
Hai cách hiển thị sử dụng cùng app registry, app instances, session và dữ liệu.

## V1 — App workspace

- Launcher, tìm kiếm, taskbar/Dock, thông tin tài khoản, lịch/thông báo.
- Ứng dụng native chạy toàn màn hình mặc định; chuyển ứng dụng không unmount.
- Preferences riêng từng user: theme, wallpaper, workspace mode, bố cục và notes.
- Module Settings, Overview, Notes, Audit và registry hoạt động với API thật.
- Authentik/NetBird/1Panel tiếp tục theo capability đã chốt; không giả dữ liệu.
- Tách app lifecycle khỏi cách hiển thị ngay từ đầu.

Đã áp dụng: focus/windows preference, app host giữ component khi bị ẩn, launcher
toàn màn hình, giữ biểu mẫu Settings khi chuyển app/thu nhỏ. Chưa có Terminal,
upload hoặc worker bền vững trong bản hiện tại; không đánh dấu các phần đó hoàn tất.

Nghiệm thu: user nhập một form chưa submit, chuyển qua Notes rồi quay lại form,
toàn bộ giá trị/tab/search phải còn. Thu nhỏ/mở lại cũng giữ trạng thái. User có
thể dùng mọi module bằng launcher/taskbar mà không kéo/resize cửa sổ.

## V2 — Đa nhiệm có kiểm soát

- Split View hai ứng dụng, đổi tỷ lệ, keyboard navigation.
- App tabs theo khả năng ứng dụng; không mặc định mọi module có nhiều instance.
- Panel nổi cho jobs/notifications/monitor, lựa chọn app và phím tắt.
- Định nghĩa background policies: chỉ giảm tần suất render/polling phù hợp;
  không ngắt tác vụ hay session chỉ vì panel không hiện.
- App switcher biểu diễn instance và trạng thái rõ ràng.

Nghiệm thu: Split View và tab có cùng app instance với chế độ focus; thay đổi cách
hiển thị không tạo terminal session mới hoặc lặp một lệnh đang chạy.

## V3 — Workspace chuyên nghiệp

- Window manager hoàn chỉnh, snap layouts, đa instance theo app manifest.
- Lưu/khôi phục workspace theo user, màn hình và phiên bản schema.
- Phục hồi bố cục khi đổi độ phân giải; focus/z-order và thao tác keyboard rõ.
- Chỉ gắn nhãn hoàn chỉnh sau khi có kiểm thử draft/task/session lifecycle.

Các thao tác move/resize/minimize/maximize đã có ở mức prototype. Điều đó không
đồng nghĩa V3 đã hoàn tất: snap, nhiều instance, phục hồi nâng cao và task lifecycle
vẫn còn phải phát triển.

## Hợp đồng trạng thái bắt buộc

| Loại trạng thái | Chủ sở hữu | Khi chuyển ứng dụng | Khi reload hoặc mất kết nối |
|---|---|---|---|
| Form/tab/filter/scroll | App instance native | Giữ component và refs/store, không unmount | Draft có schema/version nếu cần; không tự lưu password/API key vào localStorage |
| Notes/preferences | Backend theo user | Giữ trong instance, debounce save | Nạp lại dữ liệu đã xác nhận lưu; hiển thị lỗi lưu |
| Terminal | Backend session + PTY ID | Chỉ tách view, không kill PTY | Reconnect bằng session ID, xác thực quyền, giới hạn buffer và TTL |
| Upload | Backend upload ID + chunks/checksum | View theo dõi tiến độ, không hủy transfer vì navigation | Resume/reconcile chunks; browser không bảo đảm tiếp tục khi tab đóng, phải biểu diễn paused/needs-file chính xác |
| Tác vụ dài | Backend durable job ID | Job tiếp tục độc lập với view | Lease/heartbeat/retry/idempotency; không hứa exactly-once |
| Ứng dụng ngoài | Adapter/remote app session | Theo hợp đồng dịch vụ | Không cam kết giữ form trong iframe nếu không có API/lifecycle contract |

Lifecycle của app: **open → active ↔ background → close**. Navigation chỉ đổi
active/background. Đóng là ý định riêng; sau này phải hỏi khi có draft hoặc tác vụ
chưa xử lý. Đóng terminal/upload view và kết thúc session/job phải là hai hành động
khác nhau, có quyền và lựa chọn phù hợp.

## Nguyên tắc kỹ thuật

1. App registry và app host không thuộc Window Manager. Host giữ instance theo ID;
   layout chỉ quyết định vùng hiển thị. Không remount component chỉ để chuyển app.
2. Không dùng `v-if` hay danh sách chỉ gồm visible windows cho host ứng dụng;
   dùng visibility cho instance đã mở. Dữ liệu sau reload là lớp persistence khác.
3. Backend là nguồn sự thật cho identity, tasks, transfers và terminal sessions.
4. Không tạo platform bằng cách chỉ ẩn/hiện iframe. Module quản trị ưu tiên native
   component + API; app ngoài được đánh dấu rõ và cách ly theo origin.
5. Desktop web vẫn là ứng dụng trong browser: tab bị đóng/OS suspend có giới hạn.
   Cam kết phục hồi phải được kiểm chứng theo từng loại trạng thái.

## Bước tiếp theo sau V1

Hoàn thiện app lifecycle/draft contracts; tích hợp Authentik và adapter đọc trạng
thái NetBird/1Panel; triển khai durable jobs trước các thao tác quản trị dài hạn.
Sau đó xây upload/file và terminal với session/transfer ID riêng, rồi mới đưa
Split View và đa instance vào bản vận hành.

## Interface V2 — 2026-10-30

Delivered scope: iPadOS-inspired Home, Dock, app search, app switcher, Control Center,
responsive layout and existing mounted app lifecycle. Optional window mode remains.
Split View, tabs, floating utility panels and durable jobs remain future milestones.
See HANDOFF.md for implementation boundaries and next steps.

## File Station design preview

Proposed interface and lifecycle boundaries: [file-station-design.md](file-station-design.md).
This is design work only; the deployed Files module remains a placeholder until implementation.

## Delivered — Drive 0.1

File Station is now a standalone application on 8082, launched from the desktop in another tab.
Personal filesystem CRUD, trash/restore, text editor with backups and admin Debian file agent are implemented.
Independent frontend/backend build and compose.drive.yaml are available; see drive.md.
Current priority: durable/resumable transfers, multi-select/copy/quota, shared ACL and previews.
Earlier sections describing Files as a placeholder/design-only are historical.

## Native File Station and context menus

File Station now opens inside the desktop using shared packages/files-app. Drive remains independently runnable.
Implemented contextual actions, overflow controls, basic multi-select, same-root clipboard copy/cut/paste and keyboard actions.
Next: recursive copy via durable jobs, resumable uploads/quota and shared ACL/previews.

## Current foundation priority — i18n and theme

Accepted architecture: Core + Design System + App SDK + peer apps + replaceable Shells.
See [platform-foundations.md](platform-foundations.md) for scope and acceptance criteria.
Foundation/i18n V0.1: independent @minhanos/i18n, vi-VN/en-US, account locale/fallback, ICU/Intl,
stable backend error codes and first semantic Theme Tokens. Then migrate shared Files/Drive and Desktop/Settings.
Foundation V0.2: enforce missing/invalid catalog checks at build, versioned plugin catalog registration and namespace isolation.
Commercial stage: additional locales and reviewed community translation contributions.
These foundation milestones are distinct from existing desktop release numbering and are not implemented by this documentation update.
The user guide and documentation index must accompany every relevant feature change.


## Foundation 0.1 — triển khai bước đầu

Đã có workspace/lockfile chung, i18n ICU/Intl, catalog validation trong build, host SDK adapter,
semantic theme tokens, account profile và lỗi có code. Chưa nghiệm thu toàn bộ Foundation V0.1:
catalog Desktop/Files, Settings chung và chuyển hết text/colors còn dang dở.
Thứ tự mở rộng và bảng khoảng cách: [extension-contracts.md](extension-contracts.md).

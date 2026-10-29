# Kiểm chứng bản desktop 0.1

## Bổ sung: Fluent và focus workspace

- Build Go/tests/vet và Vue/TypeScript thành công sau cập nhật workspace mode.
- Chế độ focus là mặc định cho preferences cũ chưa có mode; window mode vẫn được hỗ trợ.
- Đã nhập form tạo user chưa submit: `draft_state_test` và tên hiển thị thử.
  Chuyển sang Overview rồi trở lại Settings giữ nguyên tab, form và hai giá trị.
- Kiểm tra DOM xác nhận Settings bị ẩn nhưng vẫn có đúng một instance mounted.
- Thu nhỏ/mở lại Settings vẫn giữ tên hiển thị nháp. Không submit form thử này.
- API smoke 10 nhóm kiểm tra chạy lại thành công; có thêm assertion lưu mode và
  default focus riêng của user mới.
- Calendar và launcher được triển khai bằng Vue, không dùng iframe.
- Terminal, upload và durable jobs chưa triển khai; xem docs/roadmap.md cho hợp đồng lifecycle.

Ngày: 29/10/2026. VM: 10.1.1.20; cổng: 8081.

## Build

- `vue-tsc --noEmit` và Vite production build: thành công.
- `go test ./...`: thành công; gồm password/salt, Origin, preferences không tin
  cậy, session bắt buộc và đăng ký router.
- `go vet ./...`: thành công.
- Docker multi-stage build: thành công. Base images được pin digest; npm/go có lockfiles.
- API container chạy UID/GID 10001, rootfs read-only, không có Docker socket.
- Desktop và PostgreSQL báo healthy. Bản tham chiếu cổng 8080 vẫn chạy.

## Kiểm thử tích hợp API

`tests/api-smoke.mjs` đã thực thi 10 nhóm kiểm tra:

1. Khách chưa đăng nhập không đọc được preferences hoặc quản trị user.
2. Password sai không đăng nhập được.
3. Admin đăng nhập bằng cookie HttpOnly/SameSite Strict.
4. Thiếu CSRF hoặc Origin khác bị từ chối khi ghi dữ liệu.
5. App ID không tồn tại không được lưu trong window state.
6. Trạng thái PostgreSQL/heap/filesystem lấy từ runtime; integrations unconfigured.
7. User thường không được GET/POST API quản trị user.
8. Preferences/notes tách biệt giữa admin và user, giữ qua session mới.
9. User thường chỉ thấy audit của tài khoản mình.
10. Logout vô hiệu session trên server.

Script khôi phục preferences admin; tài khoản e2e tạo riêng được dọn sau test.
Không ghi password/session token vào báo cáo JSON.

## Kiểm thử trong trình duyệt

- Đăng nhập admin và render overview có dữ liệu thật.
- Tìm `ghi chú` trong launcher và mở đúng module.
- Nhập ghi chú tiếng Việt, tự lưu; reload vẫn còn nội dung.
- Thu nhỏ ghi chú, mở lại từ taskbar.
- Resize bằng bàn phím: kích thước đổi từ 680×560 sang 660×540 trong lần thử.
- Kéo ghi chú: x đổi từ 303 sang 413; reload giữ x=413 và width=660.
- Phóng to, khôi phục, đóng ghi chú.
- Đổi theme tối và wallpaper midnight; reload xác nhận `platform midnight dark-mode`.
- Trả về theme sáng/mountain cho bàn giao.
- Viewport 390×844: cửa sổ hiển thị x=12, y=54, width=366, height=674, nằm hoàn
  toàn trong viewport; nội dung cuộn. Đã trả viewport về mặc định sau thử.
- API và PostgreSQL được recreate khi chuyển image digest; session/preferences
  vẫn có sau khi reload bản mới.
- Đã kiểm tra ảnh login, desktop sáng, settings tối và màn hình nhỏ.

Ảnh bàn giao: `artifacts/desktop-0.1.jpg` (local, được gitignore).
Kết quả API: `artifacts/api-smoke.json` (local, được gitignore).

## Giới hạn

Đây không phải pentest, kiểm thử tải, backup/restore drill hay xác nhận production.
Chưa kiểm tra SSO/NetBird/1Panel vì chưa cấu hình; File Station chưa triển khai.
Chưa kiểm thử nhiều thiết bị thực, mọi trình duyệt, screen reader đầy đủ hoặc
khôi phục sự cố disk/database. HTTP LAN dùng cookie không Secure theo cấu hình
test; phải đổi cùng HTTPS trước khi công bố ra ngoài.

## V2 iPadOS-inspired interface — 2026-10-30

- Docker frontend build passed vue-tsc and Vite. Backend go test ./... and go vet ./... passed during image build.
- Both Compose services healthy; desktop serves version 0.2.0 on port 8081.
- API smoke: all 10 checks passed (guest rejection, wrong password, cookies, CSRF/Origin, app validation,
  real metrics, admin restrictions, isolated preferences/audit, revoked logout session).
- Exact disposable e2e user removed after test (DELETE 1); no browser form submitted.
- Browser: Home, Dock app launch, Spotlight query/open, switcher resume, minimize/reopen passed.
- Draft display name and username remained after Home → switcher → app and minimize → Dock reopen.
- Control Center light/dark toggles worked; dark-mode persisted through reload; returned to light for handoff.
- Modal background is inert; initial focus moves to dialog and returns to trigger on dismissal.
- 390x844: Home and Settings inspected, document width 390 (no page overflow); Dock scrolls horizontally.
- 1024x768: Home inspected, document width 1024. Home content scrolls vertically when widgets exceed height.
- Browser viewport reset to default. No warn/error console entries during checks.
- Screenshot: artifacts/ipados-v2.jpg (local, ignored).

Remaining: full accessibility audit, real touch devices, Split View, durable jobs and service integrations.
Previous sections describe V1 verification history and should not be interpreted as V2 measurements.

## File Station design verification — 2026-10-30

Interactive prototype only: folder navigation, name filtering, selection/details,
new mock folder, mock trash, layout variants and upload pause checked in a browser.
An initial mock folder submission did not run in the sandbox; changed the action to an explicit local click handler and retested successfully.
No backend/source runtime behavior changed; Go/API tests were not repeated for documentation/mockup work.

## Files: nhật ký hoạt động chi tiết và xem ai đang truy cập — 2026-10-31

- **Bốn mức nhật ký** cho Files, System Admin chọn trong *Cài đặt → MinhAnOS Files → Nhật ký hoạt động*: Tối thiểu (như trước), Thao tác ghi (kèm đường dẫn, tên), Truy cập (thêm mọi lượt mở thư mục, mở/tải tệp, ZIP, tìm kiếm, lượt bị từ chối, khách dùng liên kết công khai), Đầy đủ (thêm IP, trình duyệt, thời gian, số byte).
- **Đang truy cập Files** trong Trung tâm nhật ký (mức Đầy đủ): ai đang xem thư mục nào, mở tệp nào, đang tải lên/xuống gì và được bao nhiêu.
- Trung tâm nhật ký: sự kiện Files hiện đúng kết quả (thành công/bị từ chối/thất bại) và đường dẫn; lọc mới theo **người** và theo **đường dẫn / nội dung** cho mọi nguồn.
- Người dùng được báo trong Files khi hoạt động của họ được ghi (từ mức Truy cập).

## Files: chia sẻ nội bộ và liên kết công khai — 2026-10-31

- **Chia sẻ với người trong tổ chức**: chọn tệp/thư mục ở vùng riêng → *Chia sẻ*, tìm người cùng tổ chức, cho quyền *Chỉ xem* hoặc *Được sửa*. Người nhận thấy ở mục **Chia sẻ với tôi** trên thanh bên; thay đổi lưu thẳng vào kho của người chia sẻ.
- **Liên kết công khai** (không cần tài khoản): xem và tải về, cho tải lên + tạo thư mục, hoặc *chỉ nhận tệp*; có mật khẩu và ngày hết hạn tuỳ chọn, xem số lượt dùng, xoá là thu hồi ngay. Trang liên kết cho tải từng tệp hoặc cả thư mục (.zip).
- Mục **Đã chia sẻ** trên thanh bên liệt kê mọi mục bạn đã chia sẻ; mục đã chia sẻ có biểu tượng riêng.
- Sửa lỗi: tệp trong **Yêu thích** đã bị xoá vẫn còn hiện. Nay mục Yêu thích (và chia sẻ, liên kết) tự đi theo khi đổi tên/di chuyển và tự bỏ khi xoá.

## Files: quản lý tệp đầy đủ hơn — 2026-10-31

- **Sắp xếp** theo tên, kích thước, ngày sửa, loại tệp (tăng/giảm dần, thư mục luôn trước), được nhớ lại.
- **Tải thư mục lên** bằng nút hoặc kéo thả cả thư mục, giữ nguyên cây thư mục.
- **Tải nhiều mục / cả thư mục thành một tệp .zip.**
- **Kéo thả để di chuyển** vào thư mục hoặc lên breadcrumb.
- **Tìm kiếm cả thư mục con**: gõ để lọc, nhấn Enter để tìm sâu trên máy chủ.
- Thanh bên có **Gần đây** (50 tệp sửa gần nhất) và **Yêu thích** (đánh dấu tệp/thư mục ở mọi không gian).
- **Thùng rác**: xóa vĩnh viễn từng mục hoặc dọn sạch, dung lượng được cập nhật ngay; mỗi mục hiện vị trí gốc và ngày xóa.
- Xem trước và sửa các định dạng (ảnh, PDF, Office…) sẽ là các module riêng, làm sau.

## Files: danh sách không còn nhảy, hai cột bên đứng yên — 2026-10-31

- Chọn tệp hoặc thư mục không còn đẩy danh sách xuống: nút Sao chép/Cắt/Dán/Xóa nằm ngay trên hàng đường dẫn; thông báo hiện nổi phía dưới.
- Khi cuộn tìm tệp, chỉ danh sách cuộn; thanh bên trái và cột Thông tin bên phải đứng yên. Trên điện thoại, hàng đường dẫn dính ở trên cùng khi cuộn.

## Nền tảng: tác vụ nền, sự kiện và lịch chạy dùng chung — 2026-10-31

- Mọi ứng dụng giờ dùng chung một **bộ tác vụ nền** của MinhAnOS (tiến độ, hủy, thử lại có giãn cách, không bao giờ chạy trùng). Sao chép thư mục trong Files chuyển sang bộ này; tác vụ đang chạy khi nâng cấp vẫn tiếp tục, bảng *Tải lên và sao chép* không đổi.
- Việc định kỳ (dọn nhật ký Files, xóa dữ liệu tài khoản đã xóa, dọn tác vụ cũ) chạy theo **lịch chung**, chỉ một lần dù có nhiều máy chủ.
- **Sự kiện giữa các ứng dụng**: khi một tài khoản bị vô hiệu hóa, các tác vụ Files chưa xong của tài khoản đó tự hủy. Việc đăng xuất tài khoản vẫn diễn ra ngay lập tức như trước, không phụ thuộc sự kiện.
- Ứng dụng mới (SMB, NFS, Backup…) sẽ dùng sẵn các phần này thay vì tự làm hàng đợi riêng.

## Khởi tạo lần đầu — 2026-10-31

- Máy cài mới không còn tự tạo tài khoản `admin`. Mở Desktop sẽ thấy **Khởi tạo lần đầu** gồm 2 bước: (1) tạo tài khoản **System Admin** với tên đăng nhập và mật khẩu do bạn chọn (cần **mã cài đặt** là nội dung tệp `secrets/bootstrap_password` trên máy chủ), (2) đặt tên **tổ chức đầu tiên**. Xong là tự đăng nhập.
- Chỉ khởi tạo được một lần, khi máy chủ chưa có tài khoản nào; người khác trong mạng không chiếm được máy mới cài nếu không có mã cài đặt. Mã cài đặt không dùng làm mật khẩu đăng nhập.
- Sửa lỗi: tải lên, di chuyển, xóa vào thùng rác và khôi phục trong vùng riêng hoặc Shared Folder đặt trên pool **ZFS** (OpenZFS 2.1, Debian 12) báo lỗi máy chủ. Nay hoạt động bình thường, vẫn không bao giờ ghi đè tệp trùng tên.

## Tổ chức (NAS + Cloud nhiều tổ chức) — 2026-10-31

- **Tổ chức** dạng cây (tối đa 8 cấp). Máy hiện có tự thành một tổ chức mặc định *MinhAnOS*: mọi tài khoản và nhóm người dùng đã có nằm trong đó, không cần làm gì thêm.
- Account Center → *Tổ chức*: xem các tổ chức bạn thuộc về; quản trị viên tổ chức thêm thành viên (theo tên đăng nhập), tạo tài khoản mới trong tổ chức, đổi vai trò, gỡ thành viên, quản lý nhóm của tổ chức và tạo tổ chức con (nếu được phép). Quản trị viên tổ chức cha quản lý cấu trúc và hạn mức của tổ chức con nhưng **không xem dữ liệu** của tổ chức con.
- Storage Manager → *Tổ chức*: System Admin cấp dung lượng cho tổ chức trên từng pool; hạn mức của tổ chức con được trừ khỏi phần còn lại của tổ chức cha, không bao giờ cấp vượt. Khi tạo Shared Folder chọn tổ chức sở hữu; chỉ thành viên tổ chức đó được cấp quyền. Quản trị viên tổ chức tự quản lý Shared Folder, quyền và thư mục quota của tổ chức mình.
- Files nhóm Shared Folder theo tổ chức. Rời tổ chức hoặc tổ chức bị tạm ngưng thì mất quyền ngay.
- Trung tâm nhật ký: quản trị viên tổ chức xem được nhật ký của tổ chức mình, lọc theo tổ chức. Thông báo khi được thêm/gỡ khỏi tổ chức, đổi vai trò, và khi hạn mức tổ chức sắp đầy (gửi cho quản trị viên tổ chức).

## Desktop và Files: hoàn tất các mục quy chuẩn còn thiếu — 2026-10-31

- Files: sao chép **thư mục** (và tệp lớn hơn 128 MB) chạy nền trên máy chủ, có tiến độ và nút hủy trong bảng *Tải lên và sao chép*. Đóng trang không dừng việc sao chép; lỗi tạm thời được thử lại tối đa 3 lần.
- Files: thông báo khi dung lượng dùng đạt 80 %, 95 % và 100 % (vùng riêng báo cho chủ tài khoản, Shared Folder/thư mục quota báo cho quản trị viên).
- Files: nút *Xuất toàn bộ tệp riêng (.tar)* ở chân trang khi đang ở vùng riêng. Desktop: `GET /api/export` tải bố cục, ứng dụng ghim và ghi chú dạng JSON.
- Khi tài khoản bị xóa trong Account Center: dữ liệu Desktop (bố cục, ghi chú) bị xóa trong vòng 1 giờ; tác vụ Files bị xóa ngay; vùng riêng trong Files bị làm trống sau 30 ngày (để kịp cứu dữ liệu từ backup).
- Lưu cùng lúc ở hai tab Desktop không còn ghi đè âm thầm: tab lưu sau tải phiên bản mới rồi lưu lại. Gửi lại một thao tác ghi Files (mạng chập chờn) không làm hai lần.
- Màu chữ phụ, nút chính và màu trạng thái đậm hơn một chút để đạt độ tương phản WCAG AA ở giao diện sáng và tối.
- Bỏ hẳn API cũ `/api/admin/users`; quản lý tài khoản ở Account Center.

- Files: tên hiển thị thống nhất là **MinhAnOS Files** (bỏ "MinhAnOS Drive", "File Station · Preview"). Toàn bộ chữ trong Files có cả tiếng Việt và tiếng Anh; ngày giờ theo ngôn ngữ và múi giờ trong Cài đặt.
- Files: thông báo lỗi rõ nguyên nhân (không tìm thấy, đã tồn tại, hết dung lượng, không có quyền, …) thay cho "yêu cầu thất bại" chung chung.
- Files: thao tác tệp (tải lên, sửa, xóa, đổi tên, …) được ghi vào Trung tâm nhật ký, nguồn *Files*, giữ 365 ngày. Không lưu đường dẫn tệp trong vùng riêng của người dùng.
- Desktop: tên ứng dụng lấy theo ngôn ngữ của bạn; Spotlight có nút ghim/bỏ ghim; Control Center mở *Cài đặt → Giao diện* thay cho các ô chọn giao diện riêng.
- Desktop: bỏ các ứng dụng cũ Tổng quan, App Center, Kết nối, Nhật ký và Giới thiệu (nhật ký xem ở Trung tâm nhật ký). Ghi chú thành ứng dụng **Ghi chú** riêng; ghi chú cũ giữ nguyên. Bố cục đã lưu tự bỏ các ứng dụng không còn.
- Màn hình đăng nhập không còn hiện địa chỉ máy chủ và số phiên bản thử nghiệm. Màu sắc Desktop và Files lấy từ bộ màu chung nên giao diện sáng/tối đồng nhất (một vài sắc độ hơi khác trước).

## Thư mục quota trong Shared Folder — 2026-10-31

- Storage Manager → Shared Folder → *Thư mục quota*: đặt giới hạn riêng cho một thư mục con (ví dụ `du-an/2026`). Files tự tạo thư mục nếu chưa có hoặc dùng thư mục có sẵn rồi đo dung lượng. Mỗi lần ghi phải vừa cả thư mục quota lẫn Shared Folder.
- Di chuyển tệp vào thư mục quota đã đầy bị từ chối; đổi tên hoặc di chuyển thư mục quota vẫn giữ giới hạn. Không đưa được thư mục quota vào thùng rác (gỡ quota trước).
- Nút *Đo lại* cho Shared Folder khi dung lượng hiển thị sai.
- Files hiện dung lượng của thư mục quota đang mở.

## Quyền Shared Folder và thư mục chia sẻ trong Files — 2026-10-31

- Storage Manager: nút *Quyền truy cập* trên mỗi Shared Folder để cấp quyền Đọc, Đọc và ghi hoặc Từ chối cho tài khoản hoặc nhóm. Từ chối thắng Ghi, Ghi thắng Đọc; không có dòng nào là không có quyền. Người tạo Shared Folder được quyền ghi.
- Files hiện các Shared Folder bạn được cấp quyền trong thanh bên, mỗi thư mục có thùng rác riêng và tính vào quota của Shared Folder. Thư mục chỉ có quyền xem được đánh dấu *Chỉ xem*.
- Khi tạo Shared Folder có thể chọn *Chứa vùng riêng của người dùng*: mỗi tài khoản có thư mục riêng bên trong (chỉ một Shared Folder giữ vai trò này).
- Máy mới cài: chưa có vùng riêng mặc định; Storage Manager nhắc tạo Shared Folder chứa vùng riêng người dùng, Files hiện *Chưa có nơi lưu trữ* cho đến lúc đó. Máy đang có dữ liệu giữ nguyên.
- Chưa có: thư mục quota (quota folder) — để bước sau.

## Trung tâm thông báo và Trung tâm nhật ký (bản tối giản) — 2026-10-31

- Chuông thông báo trên thanh trạng thái Desktop, có số thông báo chưa đọc; bấm vào để xem 8 thông báo mới nhất, đánh dấu đã đọc hoặc mở Trung tâm thông báo.
- Ứng dụng Trung tâm thông báo: xem tất cả hoặc chỉ chưa đọc, đánh dấu từng cái hoặc tất cả đã đọc. Hiện có các thông báo bảo mật: đổi/đặt lại mật khẩu, đổi vai trò, phiên bị thu hồi, đăng nhập sai nhiều lần, khôi phục tài khoản. Thông báo giữ 90 ngày.
- Ứng dụng Trung tâm nhật ký (chỉ System Admin): xem nhật ký kiểm toán của Account Center và Storage Manager, lọc theo nguồn và kết quả, xem chi tiết từng sự kiện. Chỉ đọc.

## Quota lưu trữ và Storage Manager — 2026-10-31

- Quota cho vùng tệp riêng của từng người dùng: Files chặn tải lên, sao chép, lưu văn bản khi hết dung lượng và hiển thị thanh dung lượng.
- Ứng dụng Storage Manager (admin): tạo Shared Folder trên ZFS/Btrfs, đặt quota cho share và từng người dùng, xem volume.
- Cài đặt → Storage Manager → Quota mặc định cho người dùng mới.

## Phase 5: phân quyền Files và quyền truy cập ứng dụng — 2026-10-30

- Files kiểm tra quyền qua Account Center trên mọi yêu cầu (vai trò, quyền ứng dụng, quy tắc chặn).
- Tab Ứng dụng trong Account Center: chặn hoặc bỏ chặn người dùng, nhóm, vai trò khỏi Files; Desktop ẩn Files với người bị chặn.

## Quản lý phiên và bắt buộc đổi mật khẩu — 2026-10-30

- Admin xem và đăng xuất người dùng khỏi mọi thiết bị từ trang chi tiết.
- Sau khi admin đặt lại mật khẩu, người dùng phải tự đặt mật khẩu mới trước khi dùng Desktop hoặc Files (có thể tắt khi đặt lại).

## Nền tảng nhật ký và thông báo (Phase A) — 2026-10-30

- Mỗi yêu cầu có mã truy vết (X-Request-ID, X-Correlation-ID) được lưu cùng nhật ký bảo mật.
- Nhật ký bảo mật gộp về một kho, không sửa/xóa được (trừ dọn sau 365 ngày).
- Ghi sẵn thông báo bảo mật (đổi/đặt lại mật khẩu, đổi vai trò, đăng nhập sai nhiều lần) cho Trung tâm thông báo sau này.

## Sửa lỗi Account Center trên HTTP — 2026-10-30

- Sửa lỗi mọi hộp thoại (tạo tài khoản, nhóm, vai trò, gán vai trò, sửa, khóa, đặt lại/đổi mật khẩu) không mở khi truy cập qua địa chỉ HTTP thường như http://10.1.1.20:8081.

## Account Center dùng được — 2026-10-30

- Admin đặt lại mật khẩu người dùng (thu hồi mọi phiên của họ, có audit); không áp dụng cho operator máy chủ.
- Trang chi tiết người dùng: quyền thực tế, nhóm, hoạt động gần đây và các thao tác quản trị.
- Ghi nhật ký đăng nhập, đăng xuất và đăng nhập thất bại (có giới hạn); nhật ký hiển thị mới nhất trước.

## Cài đặt trung tâm — 2026-10-30

- Thêm mục Ngôn ngữ & Khu vực (ngôn ngữ, múi giờ) dùng chung cho mọi ứng dụng; tên ứng dụng đổi thành MinhAnOS Settings.
- Quản lý người dùng chuyển về Account Center; Cài đặt chỉ giữ mục tài khoản cá nhân.
- Cài đặt được dựng từ khai báo của từng ứng dụng; có tìm kiếm và mở thẳng tới mục (ví dụ Cá nhân hóa → Giao diện).
- Mục tài khoản, bảo mật, phiên và người dùng do Account Center cung cấp; bỏ trang Người dùng trùng lặp cũ.
- Sửa lỗi có thể xảy ra khi một ứng dụng và mục cài đặt của nó cùng mở (đăng ký bản dịch trùng).

## Account Center UI v1 (Phase 4) — 2026-10-30

- Thêm ứng dụng Account Center dùng chung: hồ sơ, người dùng, nhóm, role, phiên, bảo mật và nhật ký; chạy trong Desktop và độc lập tại `/accounts/` trên cả Desktop (8081) lẫn Drive (8082).
- Thêm API đọc một nhóm (`GET /api/v1/identity/groups/{groupId}`): 33 operation.
- Đổi mật khẩu hoặc thu hồi phiên hiện tại sẽ đưa Desktop về màn đăng nhập; dữ liệu phiên cũ không ghi đè phiên mới.
- Chỉnh quyền truy cập ứng dụng vẫn chưa mở.

## Account Center Phase 3B — 2026-10-30

- Mở API phiên, đổi mật khẩu, metadata credential, nhóm/thành viên, role tùy biến/binding và đọc quyền thực tế: tổng 32 operation.
- Giữ giới hạn owner, admin cuối cùng, operator host, audit/replay và permission ceiling; không thêm migration.
- App policy editing và giao diện Account Center vẫn pending. Kiểm tra tập trung, không lặp thử nghiệm trình duyệt/VM.

## Account Center Phase 3A — 2026-10-30

- Bật 14 Identity v1 operations cho Users/My Account/roles-permissions read/audit và reauthentication; thêm version/cursor/replay guards.
- Bảo vệ admin cuối cùng, giới hạn User Admin và tách host operator khỏi quyền System Admin mới.
- Settings tạo admin có xác thực lại vi/en. Các API group/custom-role/app policy, session/password v1 đầy đủ và Account Center UI vẫn pending. Xem docs/account-center/PHASE3_REPORT.md.

## Account Center Phase 2 — 2026-10-30

- Dùng chung AuthCore cho Desktop/Drive; kiểm trạng thái/epoch, xoay cookie khi login và giới hạn đăng nhập có chặn quá tải.
- Thêm primitives đổi mật khẩu/thu hồi cả hai realm và CLI recovery dành OS root với token 15 phút, một lần dùng.
- Giữ giao diện/API legacy; chưa bật Account Center UI hoặc Identity HTTP quản trị. Xem docs/account-center/PHASE2_REPORT.md.

## Account Center Phase 1 — 2026-10-30

- Thêm Identity contracts, generated types/client, schema adoption và public UUID cùng user hiện hữu.
- Thêm CLI backfill có giới hạn; giữ Files storage IDs và session realms. Chưa bật Account Center UI/API/RBAC mới.
- Kiểm thử upgrade DB thật, concurrency, cài mới và smoke VM; xem docs/account-center/PHASE1_REPORT.md.

# Changelog

## 0.2.0 — 2026-10-30

- Rebuilt the shell with an iPadOS-inspired Home, original abstract wallpaper, large app icons, floating Dock and Spotlight-style app search.
- Added live API/process widgets, private note preview, app switcher and Control Center with persistent light/dark settings.
- Kept app instances mounted across Home, minimization and switching. Explicit close still discards unsaved local forms.
- Retained optional draggable/resizable window mode and existing backend authentication/API contracts.
- Added mandatory agent handoff/documentation rules in AGENTS.md.

This is interface V2. Split View, durable background jobs, Terminal and service integrations remain planned.

## Unreleased — File Station design

- Added two interactive File Station design variants and a specification for UI, storage boundaries and task lifecycle.
- Design only; no deployed file operations or API changes.

## Drive 0.1 — 2026-10-30

- Added independently runnable Drive frontend/backend, login/session and standalone Compose.
- Added real private file operations, upload/download, rename/move, trash/restore and UTF-8 text edits with backups.
- Added admin Debian file agent over protected Unix socket. Desktop launches Drive in a separate tab.
- Added file isolation/security/integration tests and deployment/handoff documentation.

## File Station desktop integration — 2026-10-30

- Shared Files UI renders natively in desktop and independently in Drive.
- Added authenticated same-origin gateway using desktop identity, context/overflow menus, multi-select and shared keyboard actions.
- Added bounded regular-file copy and same-root clipboard operations without overwrite.
- Preserved mounted state across Home/app switches; closing asks when text or uploads are pending.

## Platform documentation — 2026-10-30

- Added a maintained user guide and documentation index.
- Recorded Core/Design System/App SDK/peer-app/replaceable-Shell architecture and accepted i18n/theme principles.
- Added foundation milestones, account-locale/error-code migration requirements and mandatory agent documentation updates.
- Documentation only; no new runtime i18n, theme package or API behavior in this change.


## 2026-10-30 — foundation preview

- Thêm locale/theme/timezone theo tài khoản cho Desktop và Drive, có bộ chọn trong Files.
- Thêm i18n ICU/Intl vi/en, catalog menu Files, semantic theme tokens và host SDK adapter độc lập Shell.
- Backend thêm code lỗi tương thích; bỏ phân tích chuỗi tiếng Việt để xác định phiên đăng nhập trong App/Files.
- Một npm workspace/lockfile, kiểm tra catalog/boundary trong build và workflow CI.
- Chuyển đổi từng phần; chưa dịch toàn bộ giao diện hoặc hoàn thành SDK/Design System.


## 2026-10-30 — Platform Contracts v1 preview

- Thêm manifest/API/authz/files/jobs/uploads/events schemas và chính sách migration theo module.
- Sinh shared-types từ schema; tách api-client khỏi Vue SDK, đọc cả legacy và nested-v1 errors.
- Thêm kiểm tra schema, SDK semver/capability, generated drift, OpenAPI lint và breaking-change guard trong CI.
- Các dịch vụ /api/v1 vẫn planned; không thay API legacy, DB hay chức năng upload đang chạy.


## 2026-10-30 — transactional migration runner

- Thêm module DAG validation, immutable checksum ledger và khóa chống startup đồng thời.
- Profile schema dùng migration embedded; tiếp nhận dữ liệu hiện có sau kiểm tra tương thích.
- Thêm integration tests PostgreSQL riêng cho rollback/cancellation/concurrency/connection loss và adoption.
- Chưa chuyển identity bootstrap; chưa có destructive/down/backfill runner hoặc grants/jobs/upload mới.

- Sửa generator shared-types để xuất đúng tên công khai và newline ổn định; thêm TypeScript contract check không dùng skipLibCheck.


## 2026-10-30 — App naming v1

- Chốt tên/App ID/module của 8 ứng dụng bằng catalog có schema và kiểm tra trùng ID/alias trong CI.
- Đổi npm workspace host Files sang minhanos-files; shared package, thư mục, API và deployment giữ nguyên.
- Chuẩn hóa thuật ngữ Desktop/Shell/Workspace; tách Shell variants khỏi theme/layout.
- Tên UI File Station/Drive chưa đổi; các app planned chưa được tạo hoặc kích hoạt.


## 2026-10-30 — Account Center Phase 0

- Bổ sung repository audit, gap/security analysis, Identity architecture và implementation/acceptance plan.
- Ghi checkpoint phê duyệt trước code/migration; không thay chức năng hoặc dữ liệu hiện tại.

## Cài đặt trung tâm (Settings)

Mở **Cài đặt** từ Desktop. Thanh bên trái nhóm theo ứng dụng; gõ vào ô tìm kiếm để lọc mục.

- **MinhAnOS Desktop → Giao diện**: chế độ làm việc, sáng/tối, hình nền (nút *Cá nhân hóa* ở Tổng quan mở thẳng mục này).
- **Account Center → Tài khoản của tôi, Bảo mật, Phiên, Người dùng**: cùng chức năng như trong app Account Center. Mục Phiên/Người dùng chỉ hiện khi bạn có quyền tương ứng.

Chuyển giữa các mục giữ nguyên biểu mẫu đang nhập; đóng Cài đặt khi còn thay đổi chưa lưu sẽ hỏi xác nhận. Trang "Người dùng" cũ trong Cài đặt đã được thay bằng mục của Account Center; nhóm, role và nhật ký vẫn quản lý trong app Account Center.

## Account Center (Phase 4 UI v1)

Mở **Account Center** từ Desktop, hoặc truy cập riêng tại `http://<host>:8081/accounts/` (Desktop) hay `http://<host>:8082/accounts/` (Drive). Bản riêng đăng nhập bằng tài khoản của chính host đó.

- **Tài khoản của tôi**: đổi tên hiển thị, xem quyền thực tế.
- **Người dùng / Nhóm / Role** (cần quyền quản trị phù hợp): tạo, sửa, vô hiệu hóa, xóa mềm user; quản lý nhóm, thành viên, role tùy biến và gán role. Thao tác chính sách yêu cầu nhập lại mật khẩu.
- **Phiên**: xem và thu hồi phiên; thu hồi phiên hiện tại sẽ đăng xuất ngay.
- **Bảo mật**: đổi mật khẩu — mọi phiên ở cả Desktop và Drive bị đăng xuất, cần đăng nhập lại.
- **Nhật ký**: xem sự kiện audit theo quyền.

Đóng app khi có biểu mẫu chưa lưu sẽ hỏi xác nhận; chuyển app khác thì biểu mẫu được giữ. Chưa có: chỉnh quyền truy cập ứng dụng, MFA, SSO, admin reset mật khẩu. Settings → Người dùng vẫn hoạt động song song.

## Backend tài khoản Phase 3B

API đã hỗ trợ xem/thu hồi phiên của chính mình ở cả Desktop và Drive, đổi mật khẩu, quản lý nhóm, role tùy biến có giới hạn và đọc quyền thực tế. Giao diện Account Center mới chưa được đưa vào launcher; Settings hiện tại vẫn dùng như trước.
Đổi mật khẩu cần mật khẩu hiện tại và làm hết hiệu lực mọi phiên, kể cả phiên vừa thực hiện. Thu hồi các phiên khác giữ phiên hiện tại. Thông tin thiết bị/lần hoạt động chưa thu thập được sẽ để trống, không suy đoán. Xóa nhóm không xóa tài khoản hay dữ liệu Files.
System Admin và operator host không được cấp qua role tùy biến/nhóm. Scope user/storage và chỉnh chính sách truy cập ứng dụng chưa mở; không coi policy lưu sẵn là đã chặn Files. [Báo cáo](account-center/PHASE3B_REPORT.md).

## Tài khoản và quyền — Phase 3A

Trong Settings → Người dùng, tạo user thường như trước. Khi chọn Quản trị viên, nhập thêm **mật khẩu hiện tại của bạn** để xác thực trước khi cấp quyền quản trị web. Mật khẩu xác thực được xóa khỏi trạng thái biểu mẫu sau thao tác; không lưu vào hồ sơ. Đây vẫn là trang Settings cũ; ứng dụng Account Center độc lập chưa xuất hiện trong launcher.

System Admin mới không tự có quyền operator host. Các operator cũ được giữ quyền riêng khi nâng cấp; User Admin chỉ quản lý user thường qua API hiện có, chưa có màn hình quản trị riêng. Đổi vai trò/vô hiệu hóa làm hết hiệu lực phiên của tài khoản đích ở cả Desktop và Drive. Không thể vô hiệu hóa/hạ quyền admin đang hoạt động cuối cùng. Hạ vai trò operator sẽ thu hồi quyền host; nâng lại vai trò web không tự khôi phục quyền đó.

Các thao tác nâng cao hiện dùng API có xác thực, không có nút trên Settings. Xóa mềm yêu cầu tài khoản đã disabled, giữ dữ liệu/ownership và hiện chưa có luồng khôi phục qua enable. Không có hard-delete API. API trả IDENTITY_NOT_READY khi dữ liệu lịch sử chưa backfill đủ: operator dùng identity-admin -mode status/backfill theo hướng dẫn Phase 1, từng batch có giới hạn. Đăng nhập thành công chỉ bổ sung UUID còn thiếu của chính user đó.

Role User Admin hiện vẫn có nhãn User trong UI legacy; không coi nhãn đó là kết luận về quyền backend. Quyền host chỉ operator OS-root/DBA quản lý bằng quy trình riêng, chưa có API cấp host từ web. [Báo cáo hiện hành](account-center/PHASE3_REPORT.md).

## Account security foundation Phase 2

Đăng nhập Desktop/Drive vẫn dùng giao diện hiện hữu. Đăng nhập lại thay token cũ của cùng host. Khi đổi mật khẩu qua service/recovery, tất cả phiên ở cả hai host hết hiệu lực; cần đăng nhập lại. Hiện chưa có nút đổi mật khẩu/Account Center mới, API Identity v1 chưa bật.

Login có giới hạn theo IP và tài khoản; quá nhiều lần trả 429, cửa sổ cố định 10 phút. Khi hai KDF trong mỗi process đang bận, API trả 503/AUTH_BUSY; thử lại sau. Không tự vô hiệu hóa vĩnh viễn tài khoản do nhập sai.

### Recovery dành cho operator OS root

Chỉ dùng trên Debian test/installation do bạn quản lý, với user đang active. Không gửi mật khẩu/token qua URL, argv, chat hoặc log. Tạo tệp mật khẩu mới bằng công cụ tin cậy tại /root/new-password, owner root, mode 0600; độ dài 12–256 byte UTF-8. CLI yêu cầu regular file riêng tư, không symlink. Từ thư mục deploy:

```sh
docker compose exec -T -u 0 drive /app/identity-recovery -mode issue -user USERNAME -token-file /tmp/identity-recovery-token
docker compose exec -T -u 0 drive sh -c 'umask 077; cat > /tmp/identity-new-password' < /root/new-password
docker compose exec -T -u 0 drive /app/identity-recovery -mode consume -user USERNAME -token-file /tmp/identity-recovery-token -password-file /tmp/identity-new-password
docker compose exec -T -u 0 drive rm -f -- /tmp/identity-recovery-token /tmp/identity-new-password
```

Sau thành công, xóa an toàn đúng tệp /root/new-password theo quy trình quản lý bí mật của operator. Standalone dùng thêm -f compose.drive.yaml; Desktop-only có cùng binary trong service desktop. Token hết hạn sau 15 phút và chỉ dùng một lần. Issue lại thay token trước đó; issue chưa đổi mật khẩu/phiên. Tệp token đầu ra phải chưa tồn tại; nếu ghi lỗi, xử lý đúng tệp lỗi rồi issue token mới. Restart container làm mất tệp /tmp; issue lại, không cố phục hồi secret từ DB. Không có email reset/MFA/SSO hoặc tự bật tài khoản disabled. [Chi tiết triển khai](account-center/PHASE2_REPORT.md).

## Identity foundation Phase 1

Chưa có mục Account Center mới trong launcher; tiếp tục dùng đăng nhập/cài đặt hiện hữu. Không gọi các endpoint /api/v1/identity trong spec vì chưa có handlers. Operator dùng identity-admin status/backfill theo [báo cáo](account-center/PHASE1_REPORT.md); complete chỉ phản ánh UUID đầy đủ, không phải quyền hoặc trạng thái triển khai API.

## Bổ sung: ngôn ngữ và theme tài khoản (foundation preview)

Mở File Station trong Desktop hoặc Drive độc lập. Thanh đầu ứng dụng có **Ngôn ngữ** và **Giao diện**.
Có Tiếng Việt/English và Sáng/Tối/Theo thiết bị. Lựa chọn lưu theo tài khoản; đăng nhập ở thiết bị khác
hoặc mở Drive dùng cùng DB sẽ nhận lại. Thay đổi trong một tab được thấy ở tab khác khi tải lại/đăng nhập lại.
Đổi locale/theme không đóng app hoặc xóa bộ lọc hiện tại. Trước đăng nhập, locale chọn theo lựa chọn khách,
ngôn ngữ trình duyệt được hỗ trợ, rồi vi-VN. Đổi ngôn ngữ không đổi tên tệp hay nội dung tài liệu.

Đây là bản chuyển đổi từng phần: menu thao tác Files, nhãn hồ sơ, lỗi chung và đồng hồ đã dùng nền đa ngôn ngữ;
các trang cũ còn nhiều tiếng Việt. Dark theme đã có tokens cho Files và portal, còn màu nhấn/bề mặt cũ cần tiếp tục rà soát.
Múi giờ được lưu riêng qua API nhưng chưa có màn hình chỉnh. Upload hiện tại vẫn do trình duyệt quản lý.


# Hướng dẫn sử dụng MinhAnOS

Cập nhật: 2026-10-30. Phạm vi: nhánh `file-station-desktop`, bản desktop và File Station đang chạy thử.
Đọc [nền tảng](platform-foundations.md) để phân biệt chức năng hiện tại với định hướng tiếp theo.

## 1. Truy cập và đăng nhập

- Desktop: <http://10.1.1.20:8081>.
- Mở trực tiếp File Station trong desktop: <http://10.1.1.20:8081/?app=files>.
- Drive độc lập: <http://10.1.1.20:8082>.

Dùng tài khoản đã được cấp. Không có mật khẩu công khai/mặc định trong tài liệu.
File Station trong desktop dùng phiên đăng nhập desktop, không cần đăng nhập Drive lần nữa.
Drive độc lập dùng cùng tài khoản trong cấu hình tích hợp hiện tại, nhưng có phiên riêng;
đăng xuất Drive không đồng nghĩa đăng xuất desktop và ngược lại.
Nếu triển khai Drive với database riêng, tài khoản và dữ liệu không tự đồng bộ sang nền tảng khác.

## 2. Desktop

- Chọn icon trên Home hoặc Dock để mở ứng dụng. File Station mở ngay trong desktop.
- Dùng Home, Dock hoặc bộ chuyển ứng dụng để tiếp tục ứng dụng đang mở.
- Thu nhỏ giữ instance; đóng ứng dụng kết thúc instance đó. Reload không giữ mọi biểu mẫu chưa lưu.
- `Ctrl+Space` mở tìm ứng dụng; `Escape` đóng menu của shell; `Alt+Tab` chuyển ứng dụng trong desktop khi trình duyệt chuyển phím đến trang.
- Cài đặt → Giao diện chọn chế độ tập trung hoặc cửa sổ, sáng/tối và hình nền hiện có.
- Trong chế độ cửa sổ có thể kéo thanh tiêu đề, đổi kích thước và phóng to/khôi phục.

Hiện giao diện chủ yếu là tiếng Việt. Chưa có bộ chọn ngôn ngữ hoặc đồng bộ locale tài khoản.
Theme sáng/tối hiện có vẫn là triển khai cũ, chưa phải hệ Design Tokens nền tảng hoàn chỉnh.

## 3. Không gian tệp

**Tệp của tôi:** dữ liệu tách theo tài khoản. Bấm chọn một mục để xem thông tin;
double click hoặc Enter trên mục đã chọn để mở thư mục/xem văn bản phù hợp.
Nút ⋯ cung cấp thao tác khi dùng cảm ứng hoặc không dùng chuột phải.

**Thùng rác:** chứa các mục cá nhân đã xóa. Chọn mục và Khôi phục để trả về vị trí cũ.
Nếu tên đích đã tồn tại hoặc thư mục cha không còn, thao tác có thể bị từ chối;
ứng dụng không tự ghi đè tệp đích. Chưa có chức năng dọn/xóa vĩnh viễn thùng rác trong UI.

**Hệ thống Debian:** chỉ admin và chỉ khi agent được cấu hình/kết nối.
Đây là tệp thật của máy chủ; không phải kho tệp cá nhân. Thao tác xóa hệ thống là xóa vĩnh viễn
tệp thường hoặc thư mục rỗng, có bước xác nhận; không có thùng rác hệ thống hoặc xóa cây đệ quy.
Không mở symlink, tệp thiết bị, socket và các đường dẫn `/proc`, `/sys`, `/dev`, `/run`.

## 4. Menu, lựa chọn và thao tác tệp

- Chuột phải vào tệp/thư mục hoặc bấm ⋯ để xem thao tác phù hợp.
- Chuột phải vào vùng trống để tạo thư mục, tải lên, dán hoặc đổi chế độ xem.
- Ctrl/Cmd-click chọn nhiều mục; Shift-click chọn một khoảng trong danh sách đang hiển thị.
- Chuột phải lên mục trong nhóm giữ nguyên nhóm đã chọn. Đổi bộ lọc/tệp ẩn sẽ xóa lựa chọn.
- Dùng toolbar hoặc menu để Sao chép, Cắt, Dán, Đổi tên, Di chuyển, Thuộc tính và Thùng rác.
- Đổi tên/Di chuyển chỉ thực hiện cho một mục mỗi lần. Sao chép thư mục chưa được hỗ trợ.

Clipboard là bộ nhớ của ứng dụng, không phải clipboard của hệ điều hành. Nó giữ khi chuyển app/Home,
nhưng mất khi đóng app hoặc reload. Dán chỉ trong cùng không gian lưu trữ; không dán từ kho cá nhân sang Debian.
Sao chép chỉ hỗ trợ tệp thường tối đa 128 MiB; mỗi đợt dán tối đa 100 mục. Cắt thư mục dùng di chuyển cùng filesystem.
Tên trùng báo lỗi thay vì tự ghi đè. Một đợt nhiều mục có thể thành công một phần; xem lỗi từng mục.

Phím tắt dự kiến hoạt động khi focus nằm trong ứng dụng: Ctrl/Cmd+C/X/V/A, F2, Delete và Shift+F10.
Trong menu dùng mũi tên lên/xuống, Home/End và Escape. Ô nhập/trình sửa giữ phím tắt văn bản thông thường.
Nếu trình duyệt/OS chặn tổ hợp phím, dùng toolbar/menu. F2 và Escape đã được kiểm chứng;
kiểm thử Ctrl+X tự động chưa thành công, xem [validation.md](validation.md).

## 5. Tải lên và tải xuống

Chọn Tải lên hoặc kéo tệp vào vùng duyệt. Giới hạn hiện tại **128 MiB/tệp**, chưa tải thư mục đệ quy.
Nút Tác vụ hiển thị tiến độ, hủy và thử lại. Đổi thư mục/chuyển app khi tab còn chạy không tự hủy upload.
100% dữ liệu đã gửi chưa đồng nghĩa server đã xác nhận lưu xong; chờ trạng thái hoàn tất.
Đóng app/trang, reload hoặc mất trình duyệt có thể ngắt truyền. Bản hiện tại chưa có chunk resume/durable upload.

Chọn tệp rồi Tải xuống. Nội dung được tải dạng attachment; chưa có preview ảnh/PDF/media phong phú.

## 6. Xem và sửa văn bản

Chọn Xem / sửa văn bản cho tệp UTF-8 tối đa **256 KiB**. Lưu tạo bản sao cạnh tệp gốc
với tên chứa `.minhanos-backup-...`. Nếu tệp đã thay đổi từ lúc mở, nạp lại/đối chiếu trước khi lưu.
Không tự lưu bản nháp văn bản. Chuyển Home rồi mở lại app giữ bản nháp trong instance hiện tại.

Trình sửa hiện từ chối hard link, mode đặc biệt và tệp hệ thống có ACL/xattr mà nó chưa bảo toàn được.
Đóng app khi có bản nháp/upload có close guard; luồng chấp nhận/hủy native confirm cần kiểm thử thủ công thêm.
Reload vẫn mất bản nháp chưa lưu. Không sửa mật khẩu/khóa bí mật rồi đưa nội dung vào nhật ký hoặc báo lỗi.

## 7. Xử lý lỗi thường gặp

| Tình huống | Cách xử lý |
|---|---|
| Hết phiên | Đăng nhập lại đúng desktop hoặc Drive đang dùng |
| Không thấy Hệ thống Debian | Kiểm tra vai trò admin và cấu hình agent với người vận hành |
| Agent chưa sẵn sàng | Người vận hành kiểm tra service/socket theo `drive.md` |
| Trùng tên | Đổi tên hoặc chọn đích khác; không kỳ vọng ghi đè tự động |
| Tệp quá lớn/không phải UTF-8 | Dùng tải xuống hoặc công cụ phù hợp; không cố sửa bằng trình văn bản |
| Di chuyển khác filesystem | Bản hiện tại chưa hỗ trợ; cần công cụ quản trị khác |
| Dán thành công một phần | Đọc từng lỗi, kiểm tra nguồn/đích trước khi thử lại |

Khi báo lỗi, ghi ứng dụng, thao tác, thời điểm và mã lỗi nếu có. API hiện chưa có bộ error code ổn định đồng nhất;
không gửi mật khẩu, session, khóa riêng hoặc nội dung tệp nhạy cảm.

## 8. Chức năng chưa có

Chia sẻ liên kết, ACL UI/chmod/chown, quota, ZIP, sao chép cây thư mục, tìm toàn văn,
Office/cộng tác, upload resume, tác vụ bền vững, SSO Authentik và adapter NetBird/1Panel chưa hoàn chỉnh.
Đa ngôn ngữ và Theme/Design Tokens nền tảng là kế hoạch đã chốt, chưa có trong bản hiện tại.

## Quy tắc duy trì hướng dẫn

Mỗi thay đổi làm người dùng thao tác khác phải cập nhật mục tương ứng trong cùng commit,
ghi rõ chức năng đã chạy, giới hạn và phần chưa có. Không đổi ngày cập nhật nếu chỉ thêm lời hứa về chức năng tương lai.


## Hợp đồng nền tảng v1

Đợt này thêm schema và kiểm tra phát triển, không đổi thao tác sử dụng hiện tại.
Chưa có upload tiếp tục sau reload, OCR/job nền, ACL mới, app PDF Viewer hay cài plugin từ manifest.
Các đường dẫn /api/v1 trong OpenAPI là đặc tả planned; không dùng như API đang chạy.


## Khi nâng cấp gặp lỗi migration

Bản này bổ sung ledger migration và kiểm tra bảng hồ sơ; thao tác Files/Desktop không đổi.
Nếu log báo checksum drift, unsupported history hoặc incompatible user_profiles, hãy giữ nguyên dữ liệu/volume,
kiểm tra phiên bản đang chạy và đọc [chính sách migration](migration-policy.md).
Không xóa ledger, sửa checksum hoặc xóa bảng để buộc server khởi động. Cần bản migration sửa chữa được kiểm chứng.


## Tên ứng dụng chuẩn

Bộ tên đã chốt trong [ADR 0003](adr/0003-app-naming-and-repository-policy.md).
MinhAnOS Files là tên sản phẩm chuẩn; bản UI hiện tại vẫn hiển thị File Station và MinhAnOS Drive.
Desktop vẫn chạy ở 8081, Drive/Files độc lập ở 8082. Dữ liệu và các URL/API hiện tại giữ nguyên.
Các tên Storage Manager, Container Manager, Resource Monitor đang được dành cho ứng dụng tương lai, chưa có chức năng mới.


## Account Center — checkpoint kiến trúc (lịch sử, xem mục Phase 4 ở đầu)

Account Center mới đang ở checkpoint kiến trúc, chưa có ứng dụng/URL mới để sử dụng.
Quản lý users hiện tại vẫn nằm trong Settings như trước. Chưa có disable/reset password/MFA/SSO/provision Linux mới.

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

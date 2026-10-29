# File Station — bản thiết kế đề xuất

Trạng thái: thiết kế để người dùng xem trước, chưa triển khai dịch vụ tệp.
Ngày: 2026-10-30. Nền: Desktop V2, Vue/TypeScript + Go + PostgreSQL.
Mẫu tương tác: [designs/file-station.html](designs/file-station.html), HTML fragment dùng trong trình xem visualization.
Dữ liệu, tên tệp và tác vụ upload trong mẫu đều minh họa; không đọc/ghi tệp trên VM.

## Hướng giao diện

Một ứng dụng native trong MinhAnOS, mang cảm giác Files của iPadOS và khả năng quản lý rõ ràng của File Station.
Hai phương án dùng chung chức năng: **Không gian** (lưới, thoáng) và **Quản trị** (danh sách, gọn).
Đề xuất mặc định lưới, nhớ lựa chọn lưới/danh sách theo người dùng và thư mục khi triển khai.

- Trái: Tệp của tôi, Gần đây, Yêu thích, Thư mục chung, Thùng rác.
- Trên: tên thư mục, tạo thư mục, tải lên; dòng tìm kiếm trong thư mục và chọn chế độ xem.
- Giữa: breadcrumb và lưới/danh sách tệp; chọn tệp hiện bảng thông tin, mở thư mục điều hướng vào trong.
- Phải: xem trước, tên, loại, kích thước, thời gian sửa, chủ sở hữu, quyền hiệu lực và thao tác theo quyền.
- Dưới: số mục và nút Tác vụ. Tác vụ là vùng riêng, không gắn vòng đời với thư mục đang xem.
- Tablet: bảng thông tin xuống dưới khi không đủ chỗ. Điện thoại: vị trí thành thanh điều hướng gọn;
  bản triển khai có thể chuyển bảng thông tin sang sheet để giữ không gian duyệt tệp.

Không đặt các biểu đồ dung lượng/trạng thái máy chủ lớn vào vùng duyệt tệp. Dung lượng/quota chỉ hiện khi có dữ liệu thật.
Thư mục chung không được coi là truy cập công khai. Quyền phải thể hiện theo folder và người dùng.

## Tương tác và trạng thái

1. Chọn tệp để xem thông tin; mở thư mục bằng click/tap. Bổ sung chọn nhiều và menu thao tác trong triển khai.
2. Thao tác cơ bản: tạo thư mục, tải lên/tải xuống, đổi tên, sao chép/di chuyển, đưa vào thùng rác và khôi phục.
3. Xung đột tên: giữ cả hai/ghi đè/bỏ qua; ghi đè cần xác nhận rõ tệp đích. Không ghi đè mặc định.
4. Thư mục rỗng: hướng dẫn tạo/tải lên. Không có quyền: thông báo quyền và đường quay lại.
5. Mất kết nối: giữ danh sách đã xem nhưng ghi rõ chưa cập nhật; không báo ghi thành công trước xác nhận server.
6. Tác vụ upload: đang tải, tạm dừng, cần chọn lại tệp, mất kết nối, đang xác minh, hoàn tất, lỗi, hủy.
7. Chuyển app/Home không hủy upload khi tab còn chạy. Đóng view khác với hủy tác vụ.
8. Đóng trình duyệt không bảo đảm tiếp tục gửi dữ liệu. Reload phải đối chiếu upload ID/chunks, có thể yêu cầu chọn lại tệp gốc.
9. Copy/move lớn phải có job ID, trạng thái lưu server và báo lỗi từng mục; không chỉ hiện spinner trên UI.
10. Tệp không hỗ trợ preview có nút tải xuống; HTML/SVG và nội dung không tin cậy không chạy cùng origin của desktop.

## Phạm vi triển khai

### Bản đầu sử dụng được

Tệp riêng theo user, duyệt/list/grid/breadcrumb, tìm tên trong thư mục, tạo/đổi tên,
upload/download, thùng rác/khôi phục, bảng thông tin và kiểm tra quyền server.
Làm rõ giới hạn kích thước/quota và khả năng phục hồi trước khi nhận tệp lớn.
Tìm toàn văn, OCR, media transcoding và chỉnh sửa Office chưa thuộc bản đầu.

### Mở rộng tiếp

Upload theo chunk có resume, chọn nhiều, copy/move qua durable jobs, thư mục chung/ACL,
preview ảnh/PDF/text có giới hạn. Sau đó mới chia sẻ liên kết có hạn dùng, lịch sử phiên bản,
SMB/NFS, external mounts và nhiều node. Thiết kế upload ID cần có từ đầu để tránh viết lại vòng đời tác vụ.

## Kiến trúc đề xuất, chưa áp dụng

- Vue: FilesApp, FilesSidebar, FilesToolbar, FileGrid/FileList, FileInspector, TransferCenter;
  Pinia giữ navigation/selection và kết nối transfer chung với shell.
- Go: file service native dùng session hiện có; kiểm tra quyền từng thao tác; giữ CSRF/Origin cho write API.
- Nội dung tệp nằm trong volume riêng có backup; metadata, owner, favorites, trash, upload/job state nằm PostgreSQL.
- Namespace theo user dùng ID nội bộ; thư mục chung có ACL riêng. Client không truyền đường dẫn host tùy ý.
- File service chỉ truy cập storage root được cấp. Dùng rooted filesystem handles, kiểm tra symlink/path traversal,
  race và tên tệp; không dựa duy nhất vào kiểm tra chuỗi prefix. Không gắn toàn bộ / của Debian hoặc data kodbox.
- Quản trị viên ứng dụng không mặc nhiên được duyệt nội dung riêng của mọi user. Nếu cần cơ chế hỗ trợ truy cập,
  phải có quyền riêng, xác nhận và audit trước khi triển khai.
- Filesystem và DB không có transaction chung: cần nhật ký thao tác, temp file + atomic finalize và reconciliation.
- Upload kiểm tra dung lượng/quota, số bytes thật và checksum; không chỉ tin Content-Length hoặc extension.
- Chưa thay đổi API/schema/deployment. API cụ thể được viết sau khi chốt phạm vi.

## Tiêu chí nghiệm thu triển khai

Kiểm tra chéo hai user, IDOR, symlink/path traversal, thao tác đồng thời và xung đột tên;
upload thiếu dung lượng/bị ngắt, checksum, khôi phục thùng rác, giữ trạng thái khi chuyển app;
khôi phục DB/filesystem sau lỗi giữa chừng. Không đánh dấu hoàn thành dựa riêng vào mockup.

## Kiểm chứng bản mẫu

Đã thử trên trình duyệt: điều hướng vào thư mục, lọc tên, chọn tệp/hiện thông tin,
tạo thư mục minh họa, đưa vào Thùng rác, chuyển thiết kế lưới/danh sách và tạm dừng tác vụ giả lập.
Không có dữ liệu thật được gửi đi. Bản mẫu không chứng minh hiệu năng, độ an toàn hay độ bền upload thật.

# Chính sách migration theo module

Trạng thái: chính sách đã chốt; runner theo module chưa triển khai. Schema startup hiện tại vẫn hoạt động như trước.

Mỗi module sở hữu migration bất biến, có ID tăng dần, checksum, phiên bản yêu cầu và dependency constraints.
Platform lập DAG phụ thuộc, phát hiện chu trình/thiếu module/phiên bản không tương thích, lấy advisory lock rồi chạy
mỗi migration theo thứ tự. Lưu module, migration ID, checksum, trạng thái, thời điểm và phiên bản runner vào ledger.
Checksum lệch phải dừng; không tự sửa migration đã áp dụng. Module không được tự ALTER bảng của module khác.

Dùng expand → migrate → contract:

1. Expand: thêm bảng/cột/index tương thích cả phiên bản cũ và mới; đánh giá DDL lock/transaction trước chạy.
2. Migrate: backfill theo batch có checkpoint, idempotent, có metric, kiểm tra dữ liệu và ràng buộc; hỗ trợ resume.
3. Contract: chỉ xóa sau khi tất cả writer/reader cũ đã ngừng, backfill/đối soát hoàn tất và hết thời gian deprecation.

Không mặc định có down migration. Với thay đổi mất dữ liệu: cần maintenance plan, backup trước thao tác,
restore rehearsal trên DB tách biệt, RPO/RTO rõ, người vận hành biết thời gian gián đoạn và cách roll-forward/restore.
Rollback code không đồng nghĩa rollback schema/data. Không chạy destructive migration chỉ vì server khởi động.

Nghiệm thu runner: concurrent startup chỉ chạy một lần; dependency cycle/incompatibility/checksum drift dừng;
crash transaction rollback; nontransactional step có journal/reconciliation; backfill resume không trùng side effect;
kiểm thử nâng từ DB phiên bản trước, fresh DB và restore thật. Chưa ghi các kiểm chứng này là đã qua.

Ledger module mới phải tiếp nhận schema_version cũ bằng migration adoption có kiểm chứng, không tạo lại users,
không đổi user IDs và không xóa profile/sessions/files. Bí mật backup không đưa vào Git.

# Tài liệu MinhAnOS

Tài liệu phải được cập nhật trong cùng commit với thay đổi liên quan. Nội dung đánh dấu
**định hướng/chưa triển khai** không phải chức năng đã có trên bản chạy thử.

| Tài liệu | Mục đích |
|---|---|
| [Hướng dẫn sử dụng](user-guide.md) | Cách dùng bản hiện tại, giới hạn và xử lý lỗi thường gặp |
| [Nền tảng, i18n và theme](platform-foundations.md) | Kiến trúc đích, quy tắc bắt buộc và lộ trình nền tảng |
| [Rà soát và hợp đồng mở rộng](extension-contracts.md) | Mức độ đáp ứng thực tế, API/package conventions và việc tiếp theo |
| [Kiến trúc](architecture.md) | Các thành phần và ranh giới đã triển khai |
| [Drive / File Station](drive.md) | Khả năng, cách triển khai độc lập, agent và lưu trữ |
| [API](api.md) | Hợp đồng API hiện tại và các bổ sung được đánh dấu rõ |
| [Lộ trình](roadmap.md) | Phần đã có và các mốc tiếp theo |
| [Kiểm chứng](validation.md) | Các lần kiểm tra thực tế và giới hạn kiểm thử |
| [Bàn giao](HANDOFF.md) | Điểm bắt đầu cho agent tiếp theo |

## Quy trình cập nhật

1. Thay đổi thao tác, giao diện hoặc chức năng: cập nhật `user-guide.md` và `CHANGELOG.md`.
2. Thay đổi kiến trúc, i18n, theme hoặc SDK: cập nhật tài liệu nền tảng và kiến trúc.
3. Thay đổi API, quyền hoặc dữ liệu: cập nhật `api.md` và tài liệu triển khai liên quan.
4. Kết thúc công việc: cập nhật `HANDOFF.md`, kết quả trong `validation.md` và phần còn thiếu.
5. Thêm tài liệu mới: bổ sung vào bảng này; giữ liên kết tương đối hoạt động.

Không đưa mật khẩu, session/token, private key hoặc nội dung tệp riêng vào tài liệu.
Không ghi chức năng là hoàn thành chỉ vì đã có thiết kế hoặc giao diện minh họa.


## Hợp đồng mở rộng v1

- [Contracts và trạng thái runtime](../contracts/README.md)
- [ADR 0001](adr/0001-platform-contracts-v1.md)
- [Chính sách migration](migration-policy.md)


- [ADR 0002: transactional migration runner](adr/0002-transactional-migration-runner.md)


- [ADR 0003: tên ứng dụng, module và repository](adr/0003-app-naming-and-repository-policy.md)


- [Account Center: Phase 0 và checkpoint kiến trúc](account-center/README.md)

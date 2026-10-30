[File Station compliance review + first-run storage rule](files-app-review.md)

[Storage Q1 — soft quota and Storage Manager](storage-quota.md)

[Storage agent (S1)](storage-agent.md) — ZFS/Btrfs drivers, restricted storage agent, install and verification.

[Phase 5 report](account-center/PHASE5_REPORT.md) — Files authorization through identity policy, app access rules.

[Phase A report](account-center/PHASE_A_REPORT.md) — request IDs, single identity security log, append-only audit, notice outbox.

[Account Center usable v1](account-center/USABLE_V1_REPORT.md) — admin password reset, user detail page, sign-in audit.

[Notification & Log Center review](notifications-logs-review.md) — evaluation of the Codex brief against current audit/event code.

[Storage model review](storage-model-review.md) — evaluation of the DSM shared-folder + Nextcloud proposal, corrections and order.

[ADR 0007 — central Settings](adr/0007-central-settings.md) — Settings contract every app must follow.

[Account Center review 2026-10-30](account-center/REVIEW_2026-10-30.md) — progress, design assessment and central Settings app standard.

[Account Center UI v1 report](account-center/UI_V1_REPORT.md) — shared native/standalone UI, 33 operations, limits and next steps.

[Phase 3B report](account-center/PHASE3B_REPORT.md) · [ADR 0006 — security and bounded delegation](adr/0006-identity-security-and-delegation-apis.md). Current backend APIs, limits and frontend next steps.

[Phase 3A implementation report](account-center/PHASE3_REPORT.md) · [ADR 0005 — policy and host operators](adr/0005-identity-policy-and-host-operators.md). Active v1 subset and remaining Phase 3 work.

[Phase 2 report](account-center/PHASE2_REPORT.md) · [ADR 0004 — AuthCore/local recovery](adr/0004-shared-auth-core-and-recovery.md) — current auth implementation and operator decisions.

[Account Center Phase 1 report](account-center/PHASE1_REPORT.md) — implementation, migration, security review, tests và hướng tiếp tục.

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

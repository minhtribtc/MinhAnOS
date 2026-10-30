# GAP ANALYSIS — Account Center

Status: Phase 0, chờ duyệt. Tất cả risk là suy luận từ source; chưa khai thác hoặc thay đổi runtime.

## Phân loại thành phần

| Giữ nguyên | Refactor có tương thích | Bổ sung | Ngoài bản đầu |
|---|---|---|---|
| users table và dữ liệu | Shared identity repository thay SQL business trùng hai backend | Public UUID/state/version | OIDC Provider production |
| sessions + drive_sessions | Chung SessionService với realm adapters | Session list/revoke/metadata | LDAP/SAML/SCIM/proxy/Flow Designer |
| authn crypto chuẩn | Versioned credential reader, bounded auth workload | Password change/reset/recovery | Tự viết crypto/token/OIDC protocol |
| profile/preferences | Tách Accounts UI khỏi AppContent | Groups/roles/bindings/app access | Provision POSIX/SMB/SSH mặc định |
| migrate/profile SQL 001 | Adopt Identity sau legacy bootstrap commit | Identity owned migrations + upgrade tests | Rekey users.id/rename data directories |
| Files namespace/storage | Authorization hook và system-root policy | ACL/app grants/file identity theo phase | Rewrite ownership dữ liệu cũ |
| SDK/i18n/tokens | Optional manifest nav/settings + lifecycle adapter | Account Center package/native/standalone host | App untrusted cùng origin có quyền thật |

## Risk register và release gates

| Mức | Risk | Bằng chứng | Biện pháp/điều kiện nghiệm thu |
|---|---|---|---|
| P0 | Đổi users.id sang UUID làm mất mapping Files và FK | validID64, /data/users/<id>, sessions/profiles FK | UUID public mới cùng row; giữ legacy storage key; fingerprint và file-read rehearsal |
| P0 | Disable/password reset chỉ chặn một realm, phiên khác còn dùng được | Desktop sessions và drive_sessions riêng | Cùng IdentityService; atomic revoke cả hai; mọi auth path kiểm state/version |
| P0 | Admin role + host root phá cam kết privacy | agent root / và chỉ chặn virtual roots | Tách web System Admin khỏi host-root authority; allowlist/break-glass; test protected storage |
| P0 | Hạ/xóa admin cuối qua race hoặc role/group mutation | Chưa có model/safety guard | Serialize mọi thay đổi bảo vệ admin; active direct SystemAdmin binding ở V0.1; concurrent tests |
| P0 | Spoof appId/header, same-origin app bị coi là principal độc lập | App SDK chưa có delegation; Unix socket tin caller service | Backend verified app context; untrusted app bị chặn đến khi có sandbox/delegation |
| P0 | Legacy endpoint bypass RBAC mới | /api/admin/users và hai login implementations | Legacy adapters gọi cùng service/policy trước khi bật management mới |
| P1 | Bỏ qua identity migration ở standalone Drive hoặc lock ordering sai | Hai startup paths, bootstrap transaction riêng | Adopt sau commit; test fresh/upgrade/Desktop-off; không sửa profile checksum |
| P1 | Brute force phân tán/CPU DoS, limiter reset khi restart | Process-local IP limiter | Bounded hashing concurrency, shared PostgreSQL throttle/backoff, tránh lockout account DoS |
| P1 | Audit không đầy đủ, admin/password actions không reauth | Chỉ vài events, thiếu failure/last activity | Transactional success audit, failure events có redaction, step-up + fresh auth |
| P1 | Password/credential migration mất compatibility | Hash format thiếu algorithm/cost | Legacy reader + version metadata, test Unicode/long/malformed; không forced reset hàng loạt |
| P1 | Tự cấp role cao hơn quyền actor | Coarse admin/user, custom RBAC chưa có | Permission ceiling, reserved roles, protected bindings; test UserAdmin không tự thành SystemAdmin |
| P1 | Xóa user làm mất ownership/UID tombstone hoặc audit | users FK cascade với vài bảng | Soft delete/retain ownership; hard-delete pipeline deferred; UID/GID không tái dùng |
| P1 | Recovery mở endpoint vượt quyền khi Shell lỗi | Chỉ bootstrap-on-empty | Standalone Accounts UI cùng backend; local-console-only recovery, TTL/single-use/audit |
| P2 | Promise RAM/SSO/privacy khi chưa đo/chưa triển khai | Chưa benchmark và chưa có IdP | Mốc đo rõ, feature flags/readiness; không gắn nhãn production-ready sớm |

## Điểm phải được duyệt trước Phase 1

1. User ID UUID ở contract v1 qua users.public_id; users.id cũ giữ làm storage/internal compatibility key.
   Nếu yêu cầu UUID phải là physical primary key ngay, cần migration FK+storage lớn riêng; không chọn mặc định.
2. Identity modular monolith/library dùng chung trong hai binary sẵn có; giữ hai session realm adapters,
   không tạo Identity daemon/DB/session store thứ ba.
3. Cấp quản trị identity không tự cấp read-any-personal-files hoặc host-root access. Host-root operator thực vẫn thuộc
   trust boundary cao nhất; không hứa chống được root/DBA bằng web ACL.
4. V0.1 disable + soft-delete; hard-delete/provision POSIX/SMB/SSH/OIDC đợi phase có recovery/job/security gates.
5. Schema expand trước, UUID backfill/constraints và rollout mới sau test; chưa bật icon Account Center vì chỉ có contract.

Phase 0 không thay đổi quyền root hiện tại, không disable người dùng hay thu hồi phiên nào. Thay đổi giới hạn system root
là thay đổi hành vi có tác động, phải nằm trong phase được duyệt và có test kế thừa.

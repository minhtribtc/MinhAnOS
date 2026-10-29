# API desktop 0.1

JSON cùng origin, prefix `/api`. Error: `{ "error": "thông báo" }`.
Không phải proxy cho API 1Panel/kodbox. Frontend dùng types tại `src/api.ts`.

| Method | Path | Quyền | Kết quả |
|---|---|---|---|
| GET | /health | Public | Trạng thái DB và phiên bản ứng dụng |
| POST | /login | Public, Origin check và rate limit | Nhận username/password; tạo cookie, trả user và csrfToken |
| GET | /session | Session | user và csrfToken |
| POST | /logout | Session + CSRF | Thu hồi session hiện tại |
| GET | /apps | Session | Registry 8 ứng dụng |
| GET | /preferences | Session | Preferences chỉ của user hiện tại |
| PUT | /preferences | Session + CSRF | Lưu theme/wallpaper/mode/windows/notes/pinned |
| GET | /system | Session | Metrics tiến trình/container và trạng thái tích hợp |
| GET | /audit | Session | 60 event gần nhất; admin thấy tất cả, user thấy event của mình |
| GET | /admin/users | Admin | id, username, displayName, role; không có hash/password |
| POST | /admin/users | Admin + CSRF | Tạo user từ username, displayName, role, password |

Các write request ngoài login cần header `X-CSRF-Token` từ session. Backend
chấp nhận mode `focus` hoặc `windows`; preferences cũ thiếu mode được đọc như `focus`.
Backend
không nhận user_id đích từ request preferences. Tên đăng nhập 3–32 ký tự ASCII
lowercase/number/_/-, bắt đầu bằng chữ; password 12–256 byte; role admin hoặc user.
Các request body giới hạn 128 KiB. Login tối đa 20 lần theo IP trong cửa sổ 10 phút.

Chưa có endpoint xóa/sửa user, OIDC, files, package install hay cấu hình API key.

## Desktop: chỉ còn Dock (2026-10-31)

- Thanh trên cùng đã bỏ. **Dock** là thanh duy nhất: Màn hình chính · ứng dụng đã ghim và đang mở · Tìm kiếm, Ứng dụng đang mở · khay cuối gồm **chuông thông báo** (có số chưa đọc), trạng thái kết nối/Trung tâm điều khiển và **đồng hồ**.
- **Chuông** mở bảng thông báo trượt dọc mép phải (như macOS/Windows 11), không thay ứng dụng đang dùng; bấm *Mở Trung tâm thông báo* để xem đầy đủ.
- **Phóng to** (nút ô vuông hoặc nhấp đúp thanh tiêu đề): ứng dụng chiếm toàn màn hình, Dock ẩn đi, chỉ còn một viên nhỏ ở mép có chuông và số thông báo. Trỏ chuột vào mép màn hình hoặc bấm mũi tên trên viên để hiện Dock; bấm lại nút để khôi phục.
- **Đổi vị trí, kích thước Dock**: nhấp chuột phải vào chỗ trống trên Dock (hoặc nút Màn hình chính) → Dưới/Trái/Phải, Nhỏ/Vừa/Lớn; hoặc *Cài đặt → Desktop → Giao diện → Dock*. Trên điện thoại Dock luôn ở dưới.
- **Ghim/bỏ ghim**: nhấp chuột phải vào biểu tượng trên Dock hoặc màn hình chính (bàn phím: Shift+F10). Kéo biểu tượng đã ghim để đổi thứ tự. Ứng dụng đã ghim không hiện lặp lại trên màn hình chính; mọi ứng dụng vẫn tìm được bằng Tìm kiếm (Ctrl+Space).

## Nhật ký hoạt động Files (2026-10-31)

System Admin chọn Files ghi lại bao nhiêu: **Cài đặt → MinhAnOS Files → Nhật ký hoạt động**. Đổi mức nào cũng được ghi vào nhật ký.

| Mức | Ghi lại |
| --- | --- |
| **Tối thiểu** (mặc định) | Chỉ thao tác ghi (tải lên, tạo thư mục, đổi tên, di chuyển, sao chép, xoá, khôi phục, lưu văn bản, chia sẻ). Không ghi tên/đường dẫn trong vùng riêng và Shared Folder. |
| **Thao tác ghi** | Như trên, kèm đường dẫn và tên (từ → đến); cả thao tác ghi bị lỗi. |
| **Truy cập** | Thêm mọi lượt đọc: mở thư mục, mở tệp văn bản, tải tệp, tải ZIP, tìm kiếm, xem Gần đây/Thùng rác; lượt bị từ chối hoặc lỗi; khách mở liên kết công khai. |
| **Đầy đủ** | Thêm địa chỉ IP, trình duyệt, thời gian xử lý, số byte; và **ai đang mở thư mục/tệp nào ngay lúc này**, lượt tải đang chạy. |

- Xem trong **Trung tâm nhật ký**: chọn nguồn *Tệp (Files)*; lọc theo **Người** (tên đăng nhập) và **Đường dẫn / nội dung** (ví dụ `Kế toán/2026`); bấm mũi tên đầu dòng để xem chi tiết (IP, trình duyệt, số byte…). Khách dùng liên kết công khai hiện là *Khách (liên kết công khai)*.
- Ở mức **Đầy đủ**, đầu Trung tâm nhật ký có khung **Đang truy cập Files**: mỗi người đang xem thư mục nào, mở tệp nào, đang tải gì (tự làm mới mỗi 15 giây). Danh sách này chỉ nằm trong bộ nhớ, mất khi khởi động lại; một mục tự biến mất sau khoảng 75 giây người dùng không còn hoạt động.
- Từ mức **Truy cập**, người dùng thấy dòng *Hoạt động trong Files được ghi nhật ký* ở thanh bên Files.
- Nhật ký Files giữ 365 ngày. Mức Truy cập/Đầy đủ tạo nhiều dòng nhật ký hơn (mỗi lần mở thư mục một dòng).

## Khởi tạo lần đầu (2026-10-31)

1. Mở Desktop (ví dụ `http://10.1.1.20:8081`). Máy chưa có tài khoản sẽ hiện **Khởi tạo lần đầu · Bước 1/2**.
2. Nhập **mã cài đặt**: nội dung tệp `secrets/bootstrap_password` trong thư mục cài đặt trên máy chủ (người quản trị máy chủ giữ). Nhập tên hiển thị, tên đăng nhập (chữ thường, 3–32 ký tự) và mật khẩu (tối thiểu 12 ký tự, khác mã cài đặt) hai lần → **Tiếp tục**.
3. **Bước 2/2**: đặt tên tổ chức đầu tiên (ví dụ tên công ty hoặc gia đình) → **Hoàn tất khởi tạo**. Bạn được đăng nhập ngay với vai trò System Admin và là quản trị viên của tổ chức đó.
4. Tiếp theo: Storage Manager → tạo Shared Folder chứa vùng riêng người dùng; Account Center → Tổ chức để thêm người dùng hoặc tạo tổ chức khác.

Lỗi thường gặp: *Mã cài đặt chưa đúng* → kiểm tra lại tệp trên máy chủ (không có khoảng trắng thừa); *MinhAnOS đã được khởi tạo* → máy đã có tài khoản, hãy đăng nhập.

## Tổ chức (2026-10-31)

- Mỗi tài khoản thuộc một hoặc nhiều **tổ chức**. Máy đang dùng có sẵn tổ chức *MinhAnOS* chứa mọi tài khoản.
- **System Admin** tạo tổ chức cấp cao nhất (Account Center → *Tổ chức* → *Tổ chức mới*) rồi cấp dung lượng trong Storage Manager → *Tổ chức* → *Cấp dung lượng* (chọn pool, nhập GB; để trống = không giới hạn).
- **Quản trị viên tổ chức**: Account Center → *Tổ chức* → chọn tổ chức → *Người dùng mới* hoặc *Thêm thành viên* (nhập tên đăng nhập), *Đổi vai trò*, *Gỡ bỏ*; tạo nhóm của tổ chức. Trong Storage Manager tạo Shared Folder (chọn *Tổ chức sở hữu*) và cấp quyền cho thành viên hoặc nhóm của tổ chức.
- **Tổ chức con**: bật *Cho phép tổ chức con* ở tổ chức cha, sau đó *Tổ chức con*. Quản trị viên tổ chức cha cấp hạn mức cho tổ chức con (trừ vào phần của tổ chức cha), tạm ngưng hoặc xóa tổ chức con, nhưng không xem được tệp của tổ chức con.
- **Thành viên**: Files hiện Shared Folder của từng tổ chức thành nhóm riêng trong thanh bên. Khi bị gỡ khỏi tổ chức hoặc tổ chức bị tạm ngưng, các Shared Folder đó biến mất ngay.
- Lỗi thường gặp: *Tổ chức chưa được cấp dung lượng trên vùng lưu trữ này* → nhờ System Admin hoặc quản trị viên tổ chức cha cấp hạn mức; *Vượt quá dung lượng còn lại của tổ chức cha* → giảm hạn mức hoặc xin thêm cho tổ chức cha.

## Desktop và Files (quy chuẩn 2026-10-31)

- Ứng dụng tệp có tên **MinhAnOS Files** ở mọi nơi; mở trong Desktop hoặc tại cổng 8082 đều là cùng một ứng dụng.
- Đổi ngôn ngữ trong *Cài đặt → Ngôn ngữ* thì Desktop, Files và Ghi chú đổi theo, kể cả tên ứng dụng trong Spotlight và Dock.
- Ghim/bỏ ghim ứng dụng: mở Spotlight, bấm biểu tượng ghim ở cuối dòng ứng dụng.
- Giao diện sáng/tối: Control Center → *Giao diện* mở thẳng phần Giao diện trong Cài đặt.
- **Ghi chú** là ứng dụng riêng; nội dung ghi chú cũ vẫn còn.
- Thao tác tệp của bạn được ghi vào **Trung tâm nhật ký** (System Admin xem, nguồn *Files*). Mặc định với vùng riêng chỉ ghi loại thao tác, không ghi tên hay đường dẫn tệp; System Admin có thể tăng mức chi tiết (xem *Nhật ký hoạt động Files*).
- Các ứng dụng cũ Tổng quan, App Center, Kết nối, Nhật ký, Giới thiệu đã bỏ; nếu đã ghim, chúng tự biến mất khỏi Dock.
- Sao chép thư mục: chọn thư mục → *Sao chép* → mở nơi đến → *Dán*. Việc sao chép chạy nền; xem tiến độ hoặc hủy ở nút *Tác vụ* dưới chân Files. Không sao chép được thư mục vào chính nó, và thư mục chứa liên kết (symlink) hoặc tệp đặc biệt sẽ báo lỗi.
- Xuất dữ liệu: trong vùng riêng, bấm *Xuất toàn bộ tệp riêng (.tar)* ở chân trang (không gồm thùng rác). Dữ liệu Desktop (bố cục, ghi chú): mở `/api/export` khi đã đăng nhập.
- Xóa tài khoản: dữ liệu Desktop bị xóa trong 1 giờ, vùng riêng Files bị làm trống sau 30 ngày. Quản trị viên cần cứu dữ liệu thì lấy từ backup hoặc ổ đĩa trong khoảng đó.
- Thông báo dung lượng: Files báo khi dùng 80 %, 95 % và 100 % dung lượng được cấp; khi dung lượng giảm xuống, thông báo được bật lại cho lần sau.

## Thông báo và nhật ký

- **Chuông thông báo** (góc trên bên phải Desktop, cạnh trạng thái kết nối): số đỏ là số thông báo chưa đọc, cập nhật mỗi phút. Bấm chuông để xem thông báo mới nhất, bấm dấu ✓ để đánh dấu đã đọc, hoặc "Mở Trung tâm thông báo".
- **Trung tâm thông báo**: danh sách đầy đủ, lọc "Tất cả"/"Chưa đọc", "Đánh dấu tất cả đã đọc". Thông báo bảo mật ghi "Bảo mật · bắt buộc" và không tắt được. Nếu thấy thông báo đổi mật khẩu hoặc đăng nhập sai mà không phải bạn, hãy đổi mật khẩu và báo quản trị viên.
- **Trung tâm nhật ký** (chỉ System Admin): mọi thao tác quản trị và bảo mật của Account Center và Storage Manager, mới nhất trước. Lọc theo nguồn và kết quả (thành công, bị từ chối, thất bại); bấm mũi tên đầu dòng để xem Request ID và chi tiết. Nhật ký tài khoản giữ 365 ngày.
- Chưa có: tùy chọn tắt thông báo không bắt buộc, thông báo khi gần hết quota. (Nhật ký Files đã có — xem *Nhật ký hoạt động Files*.)

## Storage Manager và quota

- **Storage Manager** (chỉ System Admin): tab *Shared Folder* tạo thư mục dùng chung trên volume ZFS/Btrfs và đặt quota tổng; tab *Vùng riêng người dùng* đặt quota cho từng người, *Đo lại* khi dung lượng hiển thị sai; tab *Volume* xem nơi lưu dữ liệu. Mỗi thay đổi cần nhập mật khẩu của bạn.
- **Quota mặc định:** Cài đặt → Storage Manager → Quota mặc định, áp dụng cho người dùng mới.
- **Files** hiện *Dung lượng của bạn*; khi hết quota, tải lên/sao chép báo *Đã hết dung lượng được cấp*. Giảm quota xuống dưới mức đang dùng không xóa dữ liệu nhưng chặn ghi thêm.
- Quota do ứng dụng kiểm soát khi ghi qua Files; ghi trực tiếp trên máy chủ không được tính.
- **Quyền Shared Folder:** Storage Manager → Shared Folder → *Quyền truy cập*: thêm tài khoản hoặc nhóm, chọn Đọc, Đọc và ghi hoặc Từ chối (Từ chối thắng; không có dòng nào là không có quyền), nhập mật khẩu để lưu.
- **Files:** các Shared Folder được cấp quyền nằm trong thanh bên; thư mục *Chỉ xem* không cho tạo, tải lên, đổi tên, xóa. Thùng rác là của thư mục đang mở.
- **Vùng riêng người dùng:** chỉ một Shared Folder chứa vùng riêng (chọn *Chứa vùng riêng của người dùng* khi tạo). Máy mới chưa có vùng riêng nào; Files báo *Chưa có nơi lưu trữ* cho đến khi admin tạo.
- **Thư mục quota:** Storage Manager → Shared Folder → *Thư mục quota* → *Thêm thư mục quota* (đường dẫn trong Shared Folder và giới hạn GB). Tải lên, sao chép, di chuyển vào thư mục đó bị chặn khi hết dung lượng; thùng rác trả lại dung lượng cho thư mục. Muốn xóa thư mục quota: gỡ quota trước (tệp vẫn giữ). *Đo lại* trên Shared Folder để sửa số liệu.

## Account Center — quyền truy cập ứng dụng

Account Center → **Ứng dụng** (System Admin): xem ai được dùng ứng dụng nào. Với **MinhAnOS Files**, bấm *Chặn truy cập* rồi chọn người dùng, nhóm hoặc vai trò; họ không mở được Files (kể cả qua Desktop, biểu tượng Files bị ẩn). *Bỏ chặn* để khôi phục. Quy tắc chặn áp dụng cả với admin. Account Center không có quy tắc chặn để tránh khóa toàn bộ quản trị viên.

## Account Center — quản trị người dùng

- **Phiên của người dùng:** trang chi tiết liệt kê phiên Desktop/Files; nút *Đăng xuất khỏi mọi thiết bị* kết thúc tất cả.
- **Bắt buộc đổi mật khẩu:** khi đặt lại mật khẩu, ô *Yêu cầu đổi mật khẩu ở lần đăng nhập tới* được chọn sẵn. Người dùng đăng nhập sẽ chỉ thấy màn hình đổi mật khẩu (Files độc lập hướng dẫn mở /accounts/).
- **Xem chi tiết:** Account Center → Người dùng → bấm vào tên. Trang chi tiết có vai trò, quyền thực tế, nhóm và hoạt động gần đây (đăng nhập, đăng nhập thất bại, thay đổi).
- **Đặt lại mật khẩu:** trong trang chi tiết bấm *Đặt lại mật khẩu*, nhập mật khẩu mới hai lần (ít nhất 12 ký tự) và mật khẩu hiện tại của bạn. Người dùng bị đăng xuất khỏi mọi nơi; hãy gửi mật khẩu mới cho họ qua kênh an toàn. Không áp dụng cho chính bạn (dùng mục Bảo mật) và cho operator máy chủ (họ tự đổi hoặc dùng công cụ khôi phục).
- **Nhật ký:** hiển thị mới nhất trước, gồm đăng nhập/đăng xuất. Đăng nhập sai chỉ được ghi ở lần thử 1, 5, 10, 20 trong 10 phút để tránh tràn nhật ký.

## Cài đặt trung tâm (Settings)

Mở **Cài đặt** từ Desktop. Thanh bên trái nhóm theo ứng dụng; gõ vào ô tìm kiếm để lọc mục.

- **MinhAnOS Desktop → Giao diện**: chế độ làm việc, sáng/tối, hình nền (nút *Cá nhân hóa* ở Tổng quan mở thẳng mục này).
- **Hệ thống → Ngôn ngữ & Khu vực**: ngôn ngữ và múi giờ, áp dụng cho mọi ứng dụng của tài khoản.
- **Account Center → Tài khoản của tôi, Bảo mật, Phiên**: cùng chức năng như trong app Account Center. Mục Phiên chỉ hiện khi bạn có quyền.

Chuyển giữa các mục giữ nguyên biểu mẫu đang nhập; đóng Cài đặt khi còn thay đổi chưa lưu sẽ hỏi xác nhận. Quản lý người dùng, nhóm, role và nhật ký nằm trong app **Account Center** (không còn trong Cài đặt). Khi mở Account Center trong Desktop, ngôn ngữ và giao diện được chỉnh qua Cài đặt; bản Account Center độc lập (`/accounts/`) vẫn có bộ chọn riêng.

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
ứng dụng không tự ghi đè tệp đích. Mỗi mục hiện vị trí gốc ("Từ …") và ngày xóa. **Xóa vĩnh viễn** một mục
hoặc **Dọn sạch thùng rác** (nút trên đầu, hoặc chuột phải vào vùng trống) giải phóng dung lượng ngay; không khôi phục được.

**Gần đây:** 50 tệp được sửa gần nhất trong không gian đang mở (vùng riêng hoặc Shared Folder).

**Yêu thích:** chọn tệp/thư mục rồi bấm *Thêm vào Yêu thích* (menu, cột Thông tin). Mục Yêu thích ở thanh bên
gom mọi không gian; bấm để mở thư mục, hoặc mở thư mục chứa tệp và chọn sẵn tệp. Tối đa 500 mục; mục ở
Shared Folder bạn không còn quyền sẽ tự ẩn. Yêu thích bị xóa cùng tài khoản. Khi tệp/thư mục bị xóa, chuyển
vào thùng rác, đổi tên hoặc di chuyển, mục Yêu thích tự bỏ đi hoặc đi theo vị trí mới.

**Chia sẻ (vùng riêng):** chọn tệp/thư mục trong vùng riêng → *Chia sẻ* (menu, cột Thông tin). Hộp thoại có hai phần:
- *Người trong tổ chức*: tìm theo tên hoặc tên đăng nhập (chỉ người cùng tổ chức đang hoạt động), chọn quyền
  **Chỉ xem** hoặc **Được sửa**, bấm *Chia sẻ*. Đổi quyền hoặc ngừng chia sẻ ngay trong danh sách. Với một tệp,
  *Được sửa* chỉ cho sửa nội dung văn bản; với thư mục là tải lên, tạo, đổi tên, di chuyển, xóa bên trong.
- *Liên kết công khai*: ai có liên kết đều mở được, không cần tài khoản. Chọn **Xem và tải về**, hoặc với thư mục
  **Xem, tải lên và tạo thư mục** hay **Chỉ nhận tệp tải lên** (người nhận chỉ gửi tệp, không thấy gì bên trong);
  có thể đặt mật khẩu (≥6 ký tự) và ngày hết hạn (hết ngày đó). Sao chép liên kết bằng nút cạnh địa chỉ; xóa liên
  kết là thu hồi ngay. Mỗi liên kết hiện số lượt dùng.
Tệp người khác tải lên qua chia sẻ tính vào dung lượng của bạn và được ghi nhật ký. Mục đã chia sẻ có biểu tượng
chia sẻ; thanh bên **Đã chia sẻ** liệt kê chúng (bấm để sửa). Đổi tên hay di chuyển vẫn giữ chia sẻ; xóa vào thùng rác
thì chia sẻ và liên kết của mục đó bị gỡ.

**Chia sẻ với tôi:** thanh bên liệt kê mục người khác chia sẻ với bạn; bấm để mở. Mục chỉ xem không cho ghi; mục
được sửa lưu thẳng vào kho của người chia sẻ (xóa thì vào thùng rác của họ). Nếu bạn hoặc họ rời tổ chức, mục tự ẩn.

**Trang liên kết công khai** (`http://<máy chủ>:8082/s/…`): nhập mật khẩu nếu có (nhớ 12 giờ trên trình duyệt đó;
thử sai quá 20 lần trong 10 phút sẽ bị chặn tạm), xem/tải từng tệp, tải tất cả thành .zip, hoặc gửi tệp (≤128 MB mỗi
tệp). Muốn liên kết dùng tên miền khác, đặt `FILES_PUBLIC_URL` (ví dụ `https://files.example.com`) trong `.env`.

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
- Đổi tên/Di chuyển chỉ thực hiện cho một mục mỗi lần.
- **Kéo thả để di chuyển:** kéo một hoặc nhiều mục đã chọn thả lên thư mục hoặc lên breadcrumb; không thả được vào chính nó.
- **Sắp xếp** theo Tên, Kích thước, Ngày sửa hoặc Loại tệp, tăng/giảm dần (thư mục luôn đứng trước); lựa chọn được nhớ trên trình duyệt.
- **Tìm kiếm:** gõ để lọc thư mục đang mở; nhấn **Enter** để tìm theo tên trong thư mục đó và mọi thư mục con
  (tối đa 200 kết quả; nếu báo "chỉ hiện một phần" hãy thu hẹp từ khóa). Xóa ô tìm để quay lại thư mục.

Clipboard là bộ nhớ của ứng dụng, không phải clipboard của hệ điều hành. Nó giữ khi chuyển app/Home,
nhưng mất khi đóng app hoặc reload. Dán chỉ trong cùng không gian lưu trữ; không dán từ kho cá nhân sang Debian.
Sao chép chỉ hỗ trợ tệp thường tối đa 128 MiB; mỗi đợt dán tối đa 100 mục. Cắt thư mục dùng di chuyển cùng filesystem.
Tên trùng báo lỗi thay vì tự ghi đè. Một đợt nhiều mục có thể thành công một phần; xem lỗi từng mục.

Phím tắt dự kiến hoạt động khi focus nằm trong ứng dụng: Ctrl/Cmd+C/X/V/A, F2, Delete và Shift+F10.
Trong menu dùng mũi tên lên/xuống, Home/End và Escape. Ô nhập/trình sửa giữ phím tắt văn bản thông thường.
Nếu trình duyệt/OS chặn tổ hợp phím, dùng toolbar/menu. F2 và Escape đã được kiểm chứng;
kiểm thử Ctrl+X tự động chưa thành công, xem [validation.md](validation.md).

## 5. Tải lên và tải xuống

Chọn Tải lên hoặc kéo tệp vào vùng duyệt. **Tải thư mục lên** (hoặc kéo cả thư mục vào) giữ nguyên cây thư mục,
kể cả thư mục rỗng; thư mục trùng tên được dùng lại, tệp trùng tên vẫn báo lỗi. Giới hạn **128 MiB/tệp**, 10.000 tệp mỗi lần.
Nút Tác vụ hiển thị tiến độ, hủy và thử lại. Đổi thư mục/chuyển app khi tab còn chạy không tự hủy upload.
100% dữ liệu đã gửi chưa đồng nghĩa server đã xác nhận lưu xong; chờ trạng thái hoàn tất.
Đóng app/trang, reload hoặc mất trình duyệt có thể ngắt truyền. Bản hiện tại chưa có chunk resume/durable upload.

Chọn tệp rồi Tải xuống. Chọn một thư mục hoặc nhiều mục rồi Tải xuống để nhận **một tệp .zip** (tạo dần khi tải,
tối đa 1.000 mục được chọn; symlink và tệp đặc biệt bị bỏ qua). Chưa có preview ảnh/PDF/media — sẽ là module riêng.

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

Chia sẻ liên kết, ACL UI/chmod/chown, xem trước/sửa định dạng (module riêng), tìm toàn văn, phiên bản tệp,
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

## Tác vụ nền và lịch chạy

Tác vụ nền là việc máy chủ tiếp tục làm sau khi bạn bấm, ví dụ sao chép cả thư mục hoặc tệp lớn hơn 128 MB trong Files.

- Trong Files, mở bảng *Tải lên và sao chép* để xem tiến độ và bấm hủy khi cần; đóng trang không dừng việc đang chạy.
- Lỗi tạm thời được tự thử lại tối đa 3 lần, và một việc đang chạy không bao giờ bị chạy trùng hai lần.
- Khi tài khoản bị vô hiệu hóa, các tác vụ Files chưa xong của tài khoản đó tự hủy; việc đăng xuất vẫn diễn ra ngay.
- Việc dọn dẹp định kỳ chạy theo lịch chung của hệ thống, mỗi việc chỉ chạy một lần dù có nhiều máy chủ.
- Nhật ký thao tác tệp trong Files được giữ 365 ngày; tác vụ đã kết thúc được giữ 7 ngày rồi tự dọn.
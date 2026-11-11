# Raw Requirement: Quản lý sân cầu lông & ghép trận

Phiên bản: 0.3 (MVP, đã chốt phần lớn câu hỏi với host ngày 2026-11-10; bổ sung giả định tạm ở mục 11 ngày 2026-11-11)
Ngôn ngữ tài liệu: tiếng Việt

---

## 1. Bối cảnh

Host sân cầu lông phong trào (1 đến 3 sân) đang gặp khó khăn khi sắp xếp người chơi vãng lai và cố định vào các trận đấu sao cho:

- Trình độ hai đội tương đương nhau.
- Giới tính đội hình hợp lý.
- Mọi người đánh số trận gần bằng nhau.
- Việc theo dõi sân, người chơi và doanh thu không phải làm thủ công trên giấy hoặc chat.

Hiện tại host tự quản lý toàn bộ bằng trí nhớ và ghi chú. Mục tiêu của app là thay thế việc đó.

## 2. Người dùng

- **Host (người dùng chính, và là người duy nhất thao tác):** mở app trên điện thoại tại sân, quản lý người chơi, sân, trận đấu và sổ thu chi.
- **Người chơi:** không cần tài khoản và không cần xem app trên điện thoại của họ.

MVP chỉ có một host. Host đăng nhập bằng **một tài khoản admin duy nhất**, tên đăng nhập và mật khẩu lấy từ biến môi trường (`ADMIN_USERNAME`, `ADMIN_PASSWORD`), không hard-code trong code. Không có đăng ký, không có quản lý nhiều tài khoản.

Giai đoạn sau MVP có thể mở cho nhiều host khác dùng.

## 3. Mục tiêu MVP

1. Host tạo buổi chơi, nhập người chơi, mở 1 đến 3 sân.
2. Khi có sân trống, app **đề xuất** 4 người và cách chia đội; host là người **duyệt** (xác nhận, đổi người, hoặc bỏ đề xuất).
3. Host theo dõi trạng thái từng sân và từng người trên giao diện (cập nhật bằng polling 3 giây).
4. Khi trận kết thúc, host bấm một nút để giải phóng sân và đưa 4 người về hàng chờ.
5. Cuối buổi, app tổng hợp thu chi và phí chơi, và có báo cáo doanh thu theo buổi và theo tháng.

## 4. Phạm vi

### Trong phạm vi MVP

- Đăng nhập một tài khoản admin.
- Danh bạ người chơi dùng chung giữa các buổi (cố định và vãng lai).
- Quản lý người chơi trong một buổi (thêm, sửa, xóa khỏi buổi, cho nghỉ, đưa vào lại hàng chờ).
- Quản lý sân (1 đến 3 sân, trạng thái trống hoặc đang đánh).
- Hàng chờ dùng chung cho tất cả các sân.
- Ghép trận tự động theo thuật toán xoay vòng và cân bằng level.
- Host có thể đổi tay từng người trước khi xác nhận trận.
- Nút kết thúc trận.
- Tính phí buổi theo giới tính (chỉ người vãng lai), đánh dấu đã thu, ghi thu chi.
- Nhập phí tháng cho người cố định.
- Báo cáo doanh thu phân cấp: tổng → tháng → buổi → người chơi.
- Dữ liệu khởi tạo từ sổ sách cũ (mục 12).
- Giao diện mobile-first cho host, cài được lên điện thoại như PWA.

### Ngoài phạm vi MVP

- Đăng ký, nhiều host (tenant), phân quyền.
- Giao diện cho người chơi.
- Đặt sân online, thanh toán online, thông báo.
- Hệ thống rating ELO tự động.
- Đa ngôn ngữ (chỉ tiếng Việt).
- Huỷ trận (không có cơ chế huỷ trận `ONGOING`).
- Trạng thái `LEFT` (rời buổi).
- Đồng hồ đếm giờ trận, cảnh báo quá giờ.
- Chạy offline.
- Chia đều chi phí, làm tròn tiền.
- Đánh dấu người vắng mặt không báo.

## 5. Yêu cầu chức năng

### 5.1 Người chơi (Player)

- **Danh bạ** (`players`) lưu xuyên suốt các buổi. Mỗi người có: tên, giới tính (nam/nữ), level (Y, Y+, TBY, TB-, TB, TB+), loại (cố định / vãng lai, cột `is_regular`), ghi chú (tùy chọn).
- Xóa khỏi danh bạ là **xóa mềm** (`deleted_at`). Người đã xóa không hiện trong danh sách chọn, nhưng lịch sử buổi cũ vẫn giữ nguyên.
- Trong một buổi, mỗi người (`session_players`) có: số trận đã đánh, độ lệch ưu tiên, số lần nghỉ liên tiếp, thời điểm vào hàng chờ gần nhất, trạng thái, phí, cờ đã thu.
- Trạng thái người chơi trong buổi: `WAITING` (trong hàng chờ), `PLAYING` (đang thi đấu), `RESTING` (tạm nghỉ, không vào hàng chờ). **Không có `LEFT`**: người về sớm thì host chuyển sang `RESTING`.
- Host có thể chuyển `WAITING` ↔ `RESTING`. `PLAYING` chỉ được đặt và gỡ bởi vòng đời trận.
- **Xóa khỏi buổi** chỉ được phép khi người đó chưa đánh trận nào trong buổi và không ở trong trận `DRAFT`/`ONGOING`.
- Được thêm người vào giữa buổi (xem 6.4 về số trận ưu tiên).

### 5.2 Sân (Court)

- Số sân mở trong buổi: 1 đến 3, host có thể thay đổi khi buổi đang diễn ra.
- Không được giảm số sân nếu sân bị đóng đang `IN_USE` hoặc đang có `DRAFT`: chặn và báo lỗi.
- Trạng thái sân: `EMPTY` (trống) hoặc `IN_USE` (đang đánh, gắn với một trận).
- Mỗi sân ghi lại thời điểm bắt đầu trận hiện tại.

### 5.3 Hàng chờ (Queue)

- Hàng chờ dùng chung cho tất cả các sân trong buổi.
- Số người trong hàng chờ không cố định, thường từ 7 đến 16 người trong buổi.
- Người vào hàng chờ khi được check-in, khi được đưa về từ `RESTING`, hoặc khi vừa kết thúc trận. Mỗi lần vào hàng chờ cập nhật `queued_at`.

### 5.4 Ghép trận (Matchmaking)

- App là bên **đề xuất**, host là bên **duyệt**. App không bao giờ tự bắt đầu trận.
- Khi có sân trống và đủ ít nhất 4 người `WAITING` chưa nằm trong đề xuất nào, app tạo một đề xuất (trận `DRAFT`) gồm 4 người và cách chia 2 đội.
- **Đề xuất (`DRAFT`)** là trận nháp chưa bắt đầu:
  - Người trong `DRAFT` vẫn ở trạng thái `WAITING` nhưng được **giữ chỗ**: không xuất hiện trong đề xuất của sân khác.
  - Mỗi sân có tối đa một `DRAFT`. Nhiều sân trống cùng lúc thì đề xuất lần lượt theo thứ tự sân.
  - Host có thể: **xác nhận** (thành `ONGOING`), **đổi người**, hoặc **bỏ đề xuất** (xóa `DRAFT`, 4 người trở lại hàng chờ tự do, không tính gì).
- Chọn người (chi tiết ở 6.4):
  1. Bắt đầu với những người có số trận ưu tiên thấp nhất (`min`), sắp xếp theo thứ tự ưu tiên.
  2. Nếu nhóm đó không tạo được tổ hợp hợp lệ, mở rộng sang người có `min+1`, nhưng luôn giữ tối đa số người ở mức `min`.
  3. Chỉ khi xét hết hàng chờ mà vẫn không có tổ hợp hợp lệ thì báo host "không có đội hình hợp lệ", không tự ghép lệch.
- **Host đổi người:**
  - Quy tắc giới tính (6.3): **chặn cứng**, không cho xác nhận.
  - Cân bằng level (6.2): **chỉ cảnh báo**, host bấm "Vẫn xác nhận" để tiếp tục.
  - Công bằng số trận: không kiểm tra.
  - Người được đổi vào phải đang `WAITING` và không nằm trong `DRAFT` của sân khác.
- Người vừa đánh xong được phép vào trận tiếp theo ngay, không bắt buộc nghỉ.

### 5.5 Vòng đời trận đấu (Match)

- Trạng thái trận: `DRAFT` (đề xuất, chưa xác nhận), `ONGOING` (đang đánh), `FINISHED` (đã kết thúc). **Không có trạng thái huỷ.**
- Khi host xác nhận đội hình, trận chuyển sang `ONGOING`, sân chuyển sang `IN_USE`, 4 người chuyển sang `PLAYING`, và cập nhật `consecutive_rest` cho mọi người (6.4). Tất cả trong một transaction.
- Khi host bấm "Kết thúc trận", trận chuyển sang `FINISHED`, sân về `EMPTY`, 4 người về `WAITING` (cập nhật `queued_at`) và được tính thêm một trận. Tất cả trong một transaction.
- Sau khi kết thúc, nếu sân trống và đủ người, app đề xuất trận tiếp theo cho sân đó.
- Không có huỷ trận `ONGOING`. `DRAFT` bị bỏ thì xóa luôn, không lưu lịch sử.
- Không có đồng hồ đếm giờ hay cảnh báo quá giờ. 15 phút/trận chỉ dùng cho mô phỏng.

### 5.6 Buổi chơi (Session)

- Mỗi buổi có: ngày, giờ bắt đầu, giờ kết thúc, số sân, phí nam, phí nữ, trạng thái (`OPEN`, `ENDED`).
- **Chỉ có một buổi `OPEN` tại một thời điểm.** Tạo buổi mới khi đang có buổi `OPEN` thì chặn.
- Thời lượng 2 tiếng chỉ là mục tiêu tham khảo, **không ràng buộc** các trận đang đánh.
- Khi host kết thúc buổi:
  - Các trận `ONGOING` được tự động đóng thành `FINISHED` và tính trận như bình thường.
  - Các `DRAFT` bị xóa.
  - App tính phí buổi và tạo giao dịch thu (xem 5.7).
- Không có buổi kéo qua nửa đêm.
- Buổi `ENDED` chỉ được sửa phần thu chi (cờ đã thu, phí từng người, giao dịch).

### 5.7 Thu chi và doanh thu

- Phí buổi: nam 70.000 VND, nữ 60.000 VND, lưu trong cấu hình buổi (`fee_male`, `fee_female`), sửa được khi tạo buổi.
- **Chỉ người vãng lai trả phí theo buổi.** Người cố định (`is_regular = true`) có phí buổi = 0.
- **Phí tháng của người cố định:** host **tự nhập số tiền** cho từng người cố định, không có công thức (mức đóng do host thoả thuận, có thể là 0). Mỗi khoản là một giao dịch thu danh mục "Phí tháng", gắn với người chơi (`player_id`) và tháng tương ứng. Người cố định có thể có mức phí tháng mặc định (`monthly_fee`) để điền sẵn khi nhập.
- Thu từ người cố định ("Phí tháng") và thu từ vãng lai ("Phí buổi") luôn **tách riêng** trong mọi báo cáo.
- Phí mặc định từng người = phí theo giới tính của buổi. Host có thể sửa thủ công cho từng người (`fee_override`: miễn phí, giảm giá).
- Mỗi người trong buổi có cờ **"đã thu"** (`is_paid`). Host đánh dấu khi nhận tiền.
- Khi kết thúc buổi, app tạo **một giao dịch thu tổng** (danh mục "Phí buổi") gắn với buổi, số tiền = tổng phí của những người đã đánh dấu đã thu. Nếu host sửa cờ đã thu sau khi buổi kết thúc, giao dịch này được cập nhật theo.
- Danh mục **cố định**:
  - Thu: phí buổi, phí tháng (người cố định), bán cầu, nước, khác.
  - Chi: tiền thuê sân, cầu, điện nước, nhân viên, khác.
- **Tiền thuê sân là giá cố định mỗi sân mỗi buổi** (`court_fee`, cấu hình được, điền sẵn khi tạo buổi). Khi kết thúc buổi, app tự tạo giao dịch chi "Tiền thuê sân" = `court_fee × số sân của buổi`; host sửa được nếu thực tế khác.
  - Bối cảnh: host chọn số sân theo số vãng lai tuyển được. Thường 1 sân cho khoảng 8 người; khoảng 15–16 người trở lên mới đủ lời để mở 2 sân. Báo cáo buổi cần thể hiện rõ lời/lỗ để host thấy điều này.
- Mỗi khoản thu chi có: ngày, loại (thu/chi), danh mục, số tiền (VND, số nguyên), ghi chú, buổi liên quan (có thể để trống), người chơi liên quan (có thể để trống, dùng cho "Phí tháng").
- Xóa giao dịch là xóa mềm.
- Tháng xác định theo **ngày giao dịch**, giờ Việt Nam.

#### Báo cáo doanh thu dạng phân cấp (drill-down)

Màn hình báo cáo là một cây mở rộng từng cấp (bấm để xổ xuống):

1. **Tổng doanh thu** (toàn bộ thời gian): tổng thu, tổng chi, lợi nhuận.
2. → **Theo tháng**: tổng thu, tổng chi, lợi nhuận, số buổi, trung bình thu và lợi nhuận mỗi buổi, chi tiết theo danh mục, và **tách rõ**:
   - Thu từ người cố định (tổng "Phí tháng", kèm từng người đóng bao nhiêu).
   - Thu từ vãng lai (tổng "Phí buổi").
   - Thu khác (cầu, nước, khác).
3. → **Theo ngày (buổi)** trong tháng: số người, số nam/nữ, số vãng lai/cố định, số sân, phí dự kiến, đã thu, chưa thu, thu khác, chi (thuê sân, …), lợi nhuận buổi.
4. → **Chi tiết người chơi buổi đó**: tên, giới tính, level, cố định/vãng lai, số trận đã đánh, phí phải đóng (kể cả sửa tay), đã thu hay chưa.

Giao dịch không gắn với buổi nào (ví dụ "Phí tháng", chi phí chung) hiện ở cấp tháng trong nhóm "Không thuộc buổi".

Buổi nhập từ dữ liệu cũ (chỉ có tổng thu, không có danh sách người) vẫn hiện ở cấp ngày, cấp chi tiết người chơi để trống.

## 6. Quy tắc nghiệp vụ

### 6.1 Level

Thang điểm level (có thể cấu hình): Y=1, Y+=2, TBY=3, TB-=4, TB=5, TB+=6.

### 6.2 Cân bằng đội

- Một trận chia 4 người thành 2 cặp.
- Đội được coi là cân bằng khi chênh lệch tổng điểm level giữa hai đội không vượt quá 2 (cấu hình được).
- Ưu tiên chọn cách chia có chênh lệch nhỏ nhất.

### 6.3 Giới tính

Các đội hình hợp lệ:

- 4 nam: 2 cặp nam-nam.
- 4 nữ: 2 cặp nữ-nữ.
- 2 nam 2 nữ: 2 cặp nam-nữ.
- 3 nam 1 nữ: chỉ khi `trung bình level 3 nam − level nữ ≥ 2` (ngưỡng cấu hình được). Chia đội: cặp nam-nữ đấu cặp nam-nam.
- 3 nữ 1 nam: chỉ khi `trung bình level 3 nữ − level nam ≥ 2` (ngưỡng cấu hình được). Chia đội: cặp nam-nữ đấu cặp nữ-nữ.

Đội hình khác với các trường hợp trên là không hợp lệ.

Ghi chú hiện thực: so sánh `3 × level_lẻ` với `tổng level 3 người` để tránh số thực (ví dụ 3 nam 1 nữ: `tổng_nam − 3 × level_nữ ≥ 3 × ngưỡng`).

### 6.4 Công bằng số trận

- Mục tiêu: chênh lệch số trận giữa người đánh nhiều nhất và ít nhất trong buổi không vượt quá 1 (khi có đủ người và thời gian).
- Thứ tự ưu tiên chọn người:
  1. Số trận ưu tiên (`matches_played + priority_offset`), ít hơn được ưu tiên.
  2. `consecutive_rest`, lớn hơn được ưu tiên.
  3. `queued_at`, sớm hơn được ưu tiên.
- `consecutive_rest`:
  - Tăng 1 mỗi khi một trận được xác nhận bắt đầu mà người đó đang `WAITING` nhưng không nằm trong trận đó.
  - Reset về 0 khi người đó vào trận.
  - Không tăng khi đang `RESTING`.
- **Người vào giữa buổi:** khi check-in, gán `priority_offset = min(số trận ưu tiên của những người đang có mặt)` để họ không được ưu tiên tuyệt đối. `matches_played` vẫn là số trận thật và là số hiển thị, báo cáo.
- Quy tắc giới tính luôn thắng công bằng số trận.

### 6.5 Hạn chế trùng lặp

- "Trùng lặp" nghĩa là ghép lại cùng một cặp đồng đội, hoặc gặp lại cùng đối thủ, mà người đó vừa có trong **1 trận gần nhất** của mình (cấu hình được). Chỉ xét trong buổi hiện tại.
- Ví dụ: An vừa cặp với Bình. Nếu trận kế tiếp có hai phương án ngang nhau về số trận và level, app chọn phương án An không cặp lại với Bình.
- Đây là ưu tiên thấp nhất: chỉ dùng để chọn giữa các phương án đã ngang nhau về công bằng số trận và cân bằng level.

## 7. Yêu cầu phi chức năng

- **Hiệu năng:** ghép trận cho một sân phải hoàn thành dưới 200 ms với tối đa 30 người trong hàng chờ.
- **Giao diện:** tối ưu cho điện thoại, thao tác bằng một tay, các nút chính đủ lớn. Phong cách đơn giản, hiện đại, màu sắc đặc trưng cầu lông, có icon và banner cho sinh động.
- **Cập nhật dữ liệu:** polling 3 giây. Không cần chạy offline.
- **Dữ liệu:** tiền lưu bằng số nguyên VND, thời gian lưu dạng UTC và hiển thị theo múi giờ Việt Nam.
- **Độ tin cậy:** dữ liệu trận và thu chi không được mất khi tải lại trang. Lưu ngay sau mỗi thao tác.
- **Bảo mật:** mọi API (trừ đăng nhập) yêu cầu đã đăng nhập. Thông tin admin lấy từ biến môi trường.
- **Đóng gói:** Go server serve luôn bản build của web. Web là PWA, cài được lên màn hình chính điện thoại.
- **Khả năng mở rộng:** thiết kế để sau này thêm `tenant_id` mà không phải viết lại (MVP chưa thêm cột này).

## 8. Data model

ID là số nguyên tự tăng.

| Bảng | Các cột chính |
|---|---|
| sessions | id, date, start_time, end_time, status (OPEN/ENDED), court_count, fee_male, fee_female, court_fee |
| players | id, name, gender, level, is_regular, monthly_fee, note, deleted_at |
| session_players | id, session_id, player_id, status (WAITING/PLAYING/RESTING), matches_played, priority_offset, consecutive_rest, queued_at, checked_in_at, fee_override, is_paid |
| courts | id, session_id, index, status (EMPTY/IN_USE), current_match_id |
| matches | id, session_id, court_id, status (DRAFT/ONGOING/FINISHED), started_at, finished_at |
| match_players | id, match_id, session_player_id, team (A/B) |
| transactions | id, session_id (nullable), player_id (nullable), type (income/expense), category, amount, note, date, deleted_at |

Cấu hình mặc định toàn app (dùng để điền sẵn khi tạo buổi): `fee_male`, `fee_female`, `court_fee`, ngưỡng level, ngưỡng cân bằng, số trận xét trùng lặp.

## 9. Tiêu chí chấp nhận (Acceptance criteria)

1. Với 2 sân, 16 người, trận 15 phút, 2 tiếng: mỗi người đánh số trận chênh lệch không quá 1 so với người đánh nhiều nhất.
2. Với số người mỗi sân dao động 7 hoặc 8: ghép trận vẫn đúng quy tắc giới tính và không bao giờ có người đang `PLAYING` xuất hiện trong trận khác.
3. Khi host bấm kết thúc trận, sân về `EMPTY` và 4 người về `WAITING` trong cùng một thao tác.
4. Khi một người được đổi trong đề xuất, app kiểm tra lại quy tắc giới tính và chỉ cho xác nhận nếu hợp lệ; lệch level thì cảnh báo.
5. Phí buổi tự tính đúng theo số nam và nữ vãng lai đã check-in, người cố định không bị tính phí buổi.
6. Báo cáo tháng khớp với tổng các giao dịch không bị xóa mềm.
7. Dữ liệu không mất sau khi tải lại trang.
8. Chưa đăng nhập thì không gọi được API nào ngoài đăng nhập.
9. Báo cáo drill-down: tổng của mỗi cấp bằng tổng các mục con của nó; thu cố định và thu vãng lai trong tháng cộng với thu khác bằng tổng thu tháng.
10. Sau khi chạy dữ liệu khởi tạo (mục 12), báo cáo tháng 11/2026 hiện đúng 3 người cố định và 2 buổi cũ.

Mô phỏng: Fable viết chương trình mô phỏng với danh sách người chơi giả lập để kiểm tra thuật toán. Host sẽ tự nhập danh sách thật và tự mô phỏng lại khi app hoàn thành.

## 10. Nhật ký quyết định (2026-11-10)

- Ngưỡng 3-1 giới tính: chênh trung bình ≥ 2. Cửa sổ ứng viên: `min`, mở rộng `min+1`, báo host khi hết cách.
- `DRAFT` giữ chỗ người chơi; mỗi sân tối đa một `DRAFT`.
- Đổi người: giới tính chặn cứng, level chỉ cảnh báo.
- Không có huỷ trận, không có `LEFT`.
- Giảm số sân khi sân đang dùng: chặn. Kết thúc buổi: tự đóng trận đang đánh.
- Người cố định không trả phí buổi.
- Có đăng nhập một tài khoản admin.
- Polling 3 giây, không offline, không đồng hồ trận.
- ID số nguyên, chưa có `tenant_id`.
- Tránh trùng lặp: xét 1 trận gần nhất.
- Trận đang đánh khi kết thúc buổi: được tính trận.
- Phí tháng người cố định: host tự nhập, tách riêng với thu vãng lai trong báo cáo.
- Báo cáo drill-down: tổng → tháng → buổi → người chơi.
- Tiền thuê sân: giá cố định mỗi sân mỗi buổi, tự tạo khoản chi khi kết thúc buổi.
- Đóng gói: một binary Go kèm web và một Dockerfile, chưa deploy.
- Mô phỏng: Fable dùng dữ liệu giả lập; host tự mô phỏng với danh sách thật sau.

- Giá thuê sân (`court_fee`): host tự nhập sau trong phần cấu hình; mặc định 0. Đây là khoản chi.

## 11. Câu hỏi còn mở

Các điểm dưới đây phát sinh khi dựng bản đầu tiên (2026-11-11). Mỗi điểm đang chạy theo **giả định tạm**; host xác nhận hoặc đổi.

1. **Thứ tự xếp hạng đề xuất.** Giả định tạm: (1) số trận ưu tiên, giữ tối đa người ở mức `min`; (2) nhìn trước: lựa chọn không được để những người ít trận còn lại rơi vào nhóm mà quy tắc giới tính/cân bằng không xếp được với nhau; (3) ưu tiên người khó xếp (ít đội hình hợp lệ) khi đang có đội hình cho họ; (4) `consecutive_rest`; (5) chênh lệch level nhỏ nhất; (6) ít trùng lặp; (7) `queued_at`. Bước (2) và (3) không có trong mục 6.4 nhưng cần để mô phỏng đạt chênh lệch ≤ 1. Tối đa 16 người đầu hàng chờ (theo thứ tự ưu tiên) được xét cho một đề xuất.
2. **Nhóm 4 người lặp lại.** Với 8 người 1 sân và mọi người cùng level/giới tính, ưu tiên `consecutive_rest` khiến hai nhóm 4 người luân phiên cố định (chỉ đổi cách chia cặp). Giả định tạm: giữ đúng thứ tự ưu tiên của mục 6.4. Nếu host muốn trộn người nhiều hơn thì cần hạ ưu tiên của `consecutive_rest` xuống dưới "tránh trùng lặp".
3. **Khi nào app tự đề xuất.** Giả định tạm: tự đề xuất cho sân vừa kết thúc trận; các trường hợp khác host bấm "Đề xuất trận" trên sân trống. Lý do: nếu tự đề xuất mọi lúc thì đề xuất vừa bị bỏ sẽ hiện lại ngay.
4. **Đổi người trong đề xuất.** Giả định tạm: sau khi đổi, app chia lại hai đội cho 4 người mới (hợp lệ giới tính, lệch level nhỏ nhất) thay vì giữ nguyên vị trí người bị thay.
5. **Cho người đang được giữ chỗ đi nghỉ.** Giả định tạm: đề xuất chứa người đó bị xóa.
6. **Người không thể xếp.** Ví dụ một nữ duy nhất giữa các nam cùng level: không có đội hình hợp lệ nào chứa người đó, nên họ không được đề xuất. Giả định tạm: app không cảnh báo riêng; host tự xếp bằng cách đổi người (vẫn bị chặn nếu sai quy tắc giới tính). Mô phỏng không tính các trường hợp này là lỗi công bằng.
7. **Giao dịch "Phí buổi" tự động.** Giả định tạm: luôn tạo khi kết thúc buổi (kể cả 0 đồng) để sau đó đồng bộ theo cờ đã thu; host không sửa trực tiếp số tiền, chỉ xóa được. Khoản "Tiền thuê sân" chỉ tạo khi `court_fee × số sân > 0`.
8. **Người cố định/vãng lai trong buổi cũ.** Giả định tạm: đọc `is_regular` hiện tại của danh bạ, nên đổi loại một người sẽ đổi phí mặc định của họ ở các buổi cũ (trừ khi đã sửa tay phí).
9. **Đăng xuất.** Cookie đăng nhập là token ký có hạn 30 ngày, không lưu phía server; đăng xuất chỉ xóa cookie trên máy đó.

## 12. Dữ liệu khởi tạo (seed)

Dữ liệu thật trong quá khứ của host, nạp một lần khi khởi tạo DB (file seed riêng, chạy được lại mà không nhân đôi dữ liệu). Năm: 2026.

**Người cố định:**

| Tên | Giới tính | Level | Phí tháng (`monthly_fee`) |
|---|---|---|---|
| Trí | Nam | TB- | 1.333.000 |
| Trang | Nữ | TB | 666.000 |
| Huyền | Nữ | Y | 0 |

Mỗi người có một giao dịch thu "Phí tháng" **đã thu**, tháng 11/2026 (ngày giao dịch 2026-11-01), gắn `player_id` với đúng số tiền trên. Huyền 0 đồng vẫn tạo để báo cáo hiện đủ người.

**Buổi cũ** (trạng thái `ENDED`, không có danh sách người chơi và trận):

| Ngày | Số sân | Thu ("Phí buổi") | Chi ("Tiền thuê sân") |
|---|---|---|---|
| 2026-11-04 | 1 | 315.000 | không có |
| 2026-11-09 | 2 | 790.000 | 220.000 |

Buổi cũ không tự sinh khoản chi thuê sân theo `court_fee`; chỉ dùng đúng số liệu trong bảng.

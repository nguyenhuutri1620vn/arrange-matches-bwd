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
- **Nhóm đi chung:** host gán số nhóm (Nhóm 1, Nhóm 2, …) cho người trong buổi để đánh dấu những người đi chung (cặp đôi, nhóm bạn), tối đa 4 người một nhóm. Nhóm chỉ có hiệu lực trong buổi đó, host có thể đổi hoặc bỏ bất cứ lúc nào. Xem 6.6.

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

- "Trùng lặp" nghĩa là ghép lại cùng một cặp đồng đội, hoặc gặp lại cùng đối thủ, mà người đó vừa có trong **3 trận gần nhất** của mình (cấu hình được, trước 2026-11-13 là 1). Chỉ xét trong buổi hiện tại.
- **Trộn người giữa các trận:** khi chọn 4 người, ưu tiên phương án có ít cặp vừa chung sân (đồng đội hoặc đối thủ trong các trận xét trùng lặp). Không có quy tắc này thì 8 người 1 sân chia thành 2 nhóm 4 người cố định thay phiên nhau, chỉ đổi cách chia cặp.
- **Không nghỉ quá lâu:** người đã ngồi ngoài từ 2 trận liền trở lên (`consecutive_rest ≥ 2`) được ưu tiên trước quy tắc trộn người.
- **Đổi đồng đội:** khi chia đội, trong các cách chia cân bằng (lệch ≤ ngưỡng 6.2), ưu tiên ít trùng lặp hơn trước rồi mới đến chênh lệch nhỏ nhất.
- Ví dụ: An vừa cặp với Bình. Nếu trận kế tiếp có hai phương án ngang nhau về số trận và level, app chọn phương án An không cặp lại với Bình.
- Trộn người và đổi đồng đội không bao giờ vượt công bằng số trận, quy tắc giới tính hay ngưỡng cân bằng (xem thứ tự ở 6.8).

### 6.6 Hạn chế đánh lệch giới (3-1)

- Trận 3 nam 1 nữ hoặc 3 nữ 1 nam chỉ được đề xuất khi trong số người đang đến lượt (cùng mức công bằng) không còn đội hình 4 nam, 4 nữ hoặc 2 nam 2 nữ nào hợp lệ.
- Không gọi thêm người có số trận cao hơn chỉ để tránh trận 3-1: công bằng số trận vẫn đứng trên.
- Ngoại lệ: nếu người lẻ giới đã bị bỏ qua ít nhất 1 lượt (`consecutive_rest ≥ 1`) thì được xếp vào trận 3-1 kể cả khi còn phương án khác, để không ai phải ngồi chờ mãi. Ví dụ: sân 1 đang đề xuất 4 nữ, chỉ còn 1 nữ rảnh giữa 7 nam thì sân 2 ghép 4 nam, bạn nữ đó chờ một lượt.

### 6.7 Nhóm đi chung

Nhóm là bạn bè hoặc cặp đôi đến cùng nhau (2-4 người), host gắn theo buổi (5.1). Người đi một mình để "Không" và được coi là người ngoài. Mục tiêu: người trong nhóm thường đứng chung đội với nhau và đấu với người ngoài, đổi đồng đội trong nhóm qua các trận, và thỉnh thoảng cả nhóm đánh chung một sân.

- **Đôi cùng nhóm đấu người ngoài:** ưu tiên trận có nhiều đội gồm hai người cùng nhóm. Ví dụ nhóm Thi (Thi nữ, một bạn nữ, hai bạn nam): Thi + bạn nam đấu một cặp nam-nữ khác, Thi + bạn nữ đấu một cặp nữ khác, hai bạn nam đấu một cặp nam khác.
- **Đổi đồng đội trong nhóm:** trong các phương án ngang nhau, ưu tiên đôi cùng nhóm chưa đứng chung đội trong các trận xét trùng lặp (6.5). Khi chia đội, đồng đội cùng nhóm cũng được tính trùng lặp, nên cả nhóm cùng sân sẽ đổi cặp.
- **Lâu lâu cả nhóm đánh chung:** trận có người cùng nhóm đối đầu nhau (ví dụ Thi + bạn nam A đấu bạn nữ + bạn nam B) chỉ được ưu tiên khi không ai trong số đó từng đối đầu người cùng nhóm trên sân này trong các trận xét trùng lặp (mặc định 3 trận). Nếu vừa đối đầu, phương án đó xếp sau phương án không có người cùng nhóm.
- Đối thủ là người ngoài vẫn được trộn theo 6.5, nên đôi cùng nhóm gặp các cặp khác nhau qua các trận.
- **Cả nhóm cùng vào, cùng đợi** (host chọn 2026-11-14), chỉ khi buổi mở từ 2 sân trở lên. Buổi chỉ mở 1 sân thì nhóm chỉ được ưu tiên chơi chung, không ai phải chờ (mô phỏng: 1 sân 7-8 người có tới 38% buổi lệch 2 trận vì người ngoài phải đánh bù cho nhóm). Khi có người trong nhóm đang đánh (`PLAYING`), những người cùng nhóm còn lại ngồi chờ, không được đề xuất. Hàng chờ ghi "Chờ nhóm" cho họ.
- Khi nhóm rảnh, một đề xuất có người trong nhóm phải có đủ mọi người trong nhóm đang chờ (cả nhóm 1 sân; nhóm 2 người là một đôi). Ngoại lệ: nếu còn sân trống khác chưa có đề xuất, nhóm được tách một đôi vào sân này, những người còn lại phải vào đề xuất của sân trống kế tiếp (đề xuất đó ưu tiên lấy đủ họ nếu xếp được). Không xếp được thì họ chờ nhóm như trên.
- Tách nhóm chỉ được giữ nếu không làm người nhiều trận hơn phải vào sân thay người ít trận hơn. App so số trận của 8 người trên sân này và sân kế tiếp, giữa phương án tách nhóm và phương án không tách. Nếu tách nhóm làm hai sân cộng lại có người nhiều trận hơn, app chọn phương án không tách. Ví dụ: hai nhóm 3 người tách thành hai đôi, hai người còn lại là nam nên sân kế tiếp chỉ nhận được nam; người nữ đi một mình (3 trận) phải ngồi ngoài trong khi người nam đi một mình đã 4 trận vào sân. Hai phương án ngang nhau thì vẫn tách.
- Nếu giữ nhóm khiến một người ít trận nhất không còn đội hình hợp lệ nào (ví dụ người nữ đi một mình khi hai nữ còn lại là một nhóm), đề xuất đó được tách nhóm.
- Vì nhóm cùng đợi, người có nhóm có thể lệch quá 1 trận so với người khác. Quy tắc chênh lệch ≤ 1 (6.4) chỉ bắt buộc với người không có nhóm và giữa người trong cùng nhóm. Host chấp nhận tối đa 3% buổi có nhóm (từ 2 sân) vẫn bị lệch 2 trận, chủ yếu khi nhóm 3-4 người.
- Ưu tiên nhóm (chọn phương án nhiều đôi cùng nhóm, đổi đồng đội trong nhóm) đứng sau công bằng số trận, nhìn trước, tránh 3-1 và người khó xếp, nhưng trên người ngồi ngoài 2 trận, trộn người và người nghỉ lâu (6.8). Khi chia đội, đôi cùng nhóm vẫn phải trong ngưỡng cân bằng 6.2, và quy tắc giới tính vẫn chặn cứng (đôi nữ cùng nhóm chỉ đấu được cặp nữ).

### 6.8 Thứ tự xếp hạng đề xuất

Trong cửa sổ ứng viên (5.4), các tổ hợp hợp lệ được xếp theo:

1. Số trận ưu tiên, giữ tối đa người ở mức `min`.
2. Nhìn trước: không để những người ít trận còn lại rơi vào nhóm không xếp được với nhau; các trận kế tiếp đạt số trận ưu tiên thấp nhất.
3. Tránh trận 3-1 (6.6), trừ khi người lẻ giới đã bị bỏ qua 1 lượt. Ngoại lệ này cần để mô phỏng 1 sân 7 người không có buổi lệch 2 trận.
4. Ưu tiên người khó xếp (ít đội hình hợp lệ).
5. Nhóm đi chung (6.7): nhiều đội gồm đôi cùng nhóm hơn (trận cả nhóm đối đầu nhau chỉ tính khi tới lượt, chưa tới lượt thì xếp sau), rồi ít đôi cùng nhóm vừa đứng chung đội hơn.
6. Nhiều người đã ngồi ngoài từ 2 trận liền hơn (6.5).
7. Ít cặp vừa chung sân hơn (trộn người, 6.5). Người cùng nhóm không tính.
8. `consecutive_rest` lớn hơn.
9. Chênh lệch level nhỏ hơn.
10. Ít trùng lặp hơn (6.5).
11. `queued_at` sớm hơn.

Khi chia 4 người thành 2 đội: trong các cách chia cân bằng, chọn cách có nhiều cặp cùng nhóm đứng chung đội nhất, rồi ít trùng lặp nhất, rồi chênh lệch nhỏ nhất.

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
| session_players | id, session_id, player_id, status (WAITING/PLAYING/RESTING), matches_played, priority_offset, consecutive_rest, queued_at, checked_in_at, fee_override, is_paid, group_no (nullable) |
| courts | id, session_id, index, status (EMPTY/IN_USE), current_match_id |
| matches | id, session_id, court_id, status (DRAFT/ONGOING/FINISHED), started_at, finished_at |
| match_players | id, match_id, session_player_id, team (A/B) |
| transactions | id, session_id (nullable), player_id (nullable), type (income/expense), category, amount, note, date, deleted_at |

Cấu hình mặc định toàn app (dùng để điền sẵn khi tạo buổi): `fee_male`, `fee_female`, `court_fee`, ngưỡng level, ngưỡng cân bằng, số trận xét trùng lặp.

## 9. Tiêu chí chấp nhận (Acceptance criteria)

1. Với 2 sân, 16 người, trận 15 phút, 2 tiếng: mỗi người đánh số trận chênh lệch không quá 1 so với người đánh nhiều nhất. Có nhóm đi chung và từ 2 sân thì tiêu chí này áp dụng cho người không có nhóm, còn người trong cùng nhóm lệch nhau tối đa 1; cho phép tối đa 3% buổi không đạt (6.7).
2. Với số người mỗi sân dao động 7 hoặc 8: ghép trận vẫn đúng quy tắc giới tính và không bao giờ có người đang `PLAYING` xuất hiện trong trận khác.
3. Khi host bấm kết thúc trận, sân về `EMPTY` và 4 người về `WAITING` trong cùng một thao tác.
4. Khi một người được đổi trong đề xuất, app kiểm tra lại quy tắc giới tính và chỉ cho xác nhận nếu hợp lệ; lệch level thì cảnh báo.
5. Phí buổi tự tính đúng theo số nam và nữ vãng lai đã check-in, người cố định không bị tính phí buổi.
6. Báo cáo tháng khớp với tổng các giao dịch không bị xóa mềm.
7. Dữ liệu không mất sau khi tải lại trang.
8. Chưa đăng nhập thì không gọi được API nào ngoài đăng nhập.
9. Báo cáo drill-down: tổng của mỗi cấp bằng tổng các mục con của nó; thu cố định và thu vãng lai trong tháng cộng với thu khác bằng tổng thu tháng.
10. Sau khi chạy dữ liệu khởi tạo (mục 12), báo cáo tháng 11/2026 hiện đúng 3 người cố định, 3 buổi (4/11, 9/11, 11/11) và các tổng thu chi ở cuối mục 12.

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
- Tránh trùng lặp: xét 1 trận gần nhất (đổi thành 3 ngày 2026-11-13).
- Trận đang đánh khi kết thúc buổi: được tính trận.
- Phí tháng người cố định: host tự nhập, tách riêng với thu vãng lai trong báo cáo.
- Báo cáo drill-down: tổng → tháng → buổi → người chơi.
- Tiền thuê sân: giá cố định mỗi sân mỗi buổi, tự tạo khoản chi khi kết thúc buổi.
- Đóng gói: một binary Go kèm web và một Dockerfile, chưa deploy.
- Mô phỏng: Fable dùng dữ liệu giả lập; host tự mô phỏng với danh sách thật sau.

- Giá thuê sân (`court_fee`): host tự nhập sau trong phần cấu hình; mặc định 0. Đây là khoản chi.
- 2026-11-11: Host sửa phí tháng của Trang thành 667.000 (trước là 666.000). Thêm hai khoản chi "Chi phí ban đầu của sân" cho Trí và Trang (mục 12).
- 2026-11-11: Thêm chi phí ban đầu tiền sân 1.400.000 và tiền cầu 630.000 (mục 12).
- 2026-11-14: Đổi "hạn chế chờ" thành "cả nhóm cùng vào, cùng đợi" khi mở từ 2 sân (mục 6.7): người trong nhóm chờ khi bạn cùng nhóm đang đánh. 1 sân giữ "hạn chế chờ". Quy tắc chênh lệch ≤ 1 chỉ bắt buộc với người không có nhóm. Host chấp nhận tối đa 3% buổi có nhóm vẫn lệch 2 trận (mô phỏng: 2 nhóm 4 người, 3 sân, 24 người lệch ở 1,6-2,1% buổi). Lý do: host muốn nhóm Thi được chơi chung, không bị tách.
- 2026-11-14: Chỉ giữ cách tách nhóm khi hai sân cộng lại không đưa người nhiều trận hơn vào sân thay người ít trận hơn (mục 6.7). Lý do: mô phỏng 4 nhóm 3 người, 2 sân, 15 người lệch 2 trận ở 2,6-3,5% buổi vì người đi một mình phải đánh bù cho nhóm bị tách; sau khi sửa còn 1,0-1,9%. Host duyệt sửa thuật toán.
- 2026-11-14: Nhóm đi chung tách thành đôi đấu người ngoài, đổi đồng đội trong nhóm, khoảng 3 trận một lần cả nhóm đánh chung (mục 6.7). Host muốn đưa nhóm lên vị trí 4 và người khó xếp xuống vị trí 7, nhưng mô phỏng trượt (1 sân 7-8 người lệch 2 trận, kể cả khi không có nhóm), nên giữ người khó xếp ở vị trí 4 và nhóm ở vị trí 5 (mục 6.8). Lý do: nhóm 4 người trước đây gần như lúc nào cũng tự đánh với nhau.
- 2026-11-13: Trộn người giữa các trận, ưu tiên đổi đồng đội khi vẫn cân bằng, và xét trùng lặp 3 trận gần nhất (mục 6.5, 6.8). Lý do: 8 người 1 sân cứ chia 2 nhóm 4 người cố định.
- 2026-11-13: Host thống kê lại vốn và chi (mục 12): vốn 2.030.000 (Trí 1.353.000, Trang 677.000), bỏ hai khoản chi trùng, thêm 315.000 tiền cầu ngày 12/11, buổi 11/11 có 5 vãng lai chưa thu.
- 2026-11-11: Thêm nhóm đi chung (gán theo buổi, tối đa 4 người) và hạn chế trận 3-1 (mục 6.6 đến 6.8). Host chọn: hạn chế chờ hơn là giữ nhóm.
- 2026-11-11: Host chọn đưa "chung nhóm" lên trên `consecutive_rest`, ngay sau "người khó xếp" (mục 6.8).
- 2026-11-11: Tránh 3-1 đưa lên trên "người khó xếp", kèm ngoại lệ người lẻ giới đã bị bỏ qua 1 lượt (host duyệt sau khi thấy đề xuất 3 nam 1 nữ trong khi còn đủ 4 nam).

## 11. Câu hỏi còn mở

Các điểm dưới đây phát sinh khi dựng bản đầu tiên (2026-11-11). Mỗi điểm đang chạy theo **giả định tạm**; host xác nhận hoặc đổi.

1. **Thứ tự xếp hạng đề xuất.** Đã chốt 2026-11-11, xem 6.8. Bước nhìn trước và người khó xếp không có trong mục 6.4 nhưng cần để mô phỏng đạt chênh lệch ≤ 1. Tối đa 16 người đầu hàng chờ (theo thứ tự ưu tiên) được xét cho một đề xuất.
2. **Nhóm 4 người lặp lại.** Đã chốt 2026-11-13: host muốn trộn người, xem quy tắc trộn người và không nghỉ quá lâu ở 6.5.
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
| Trí | Nam | TB- | 1.353.000 |
| Trang | Nữ | TB | 677.000 |
| Huyền | Nữ | Y | 0 |

Mỗi người có một giao dịch thu "Phí tháng" **đã thu**, tháng 11/2026 (ngày giao dịch 2026-11-01), gắn `player_id` với đúng số tiền trên. Huyền 0 đồng vẫn tạo để báo cáo hiện đủ người.

**Buổi cũ** (trạng thái `ENDED`, không có danh sách người chơi và trận):

| Ngày | Số sân | Thu ("Phí buổi") | Chi ("Tiền thuê sân") |
|---|---|---|---|
| 2026-11-04 | 1 | 315.000 | không có |
| 2026-11-09 | 2 | 790.000 | 220.000 |

Buổi cũ không tự sinh khoản chi thuê sân theo `court_fee`; chỉ dùng đúng số liệu trong bảng.

**Vốn ban đầu** (thống kê lại 2026-11-13): Trí và Trang góp 2.030.000 theo tỉ lệ 2/3 và 1/3, làm tròn nghìn: Trí 1.353.000, Trang 677.000 (chính là hai khoản "Phí tháng" ở trên). Toàn bộ vốn dùng cho:

| Ngày | Danh mục | Số tiền | Ghi chú |
|---|---|---|---|
| 2026-11-01 | Tiền thuê sân | 1.400.000 | Tiền sân ban đầu |
| 2026-11-01 | Cầu | 630.000 | 2 ống cầu |
| 2026-11-12 | Cầu | 315.000 | Mua thêm 1 ống |

Bản trước còn ghi thêm hai khoản "Chi khác" 1.333.000 và 667.000 (tiền của Trí và Trang dùng trang bị sân). Đó là ghi trùng với tiền sân và tiền cầu, nên bị xóa mềm.

**Buổi 2026-11-11** (`ENDED`, không có tiền thuê sân): 5 người vãng lai, 4 nam 1 nữ, chưa ai trả phí (dự kiến 4 × 70.000 + 60.000 = 340.000). Tên tạm "VL nam 1" đến "VL nam 4" và "VL nữ 1", level TBY; host đổi tên, level trong danh bạ sau. Buổi có khoản thu "Phí buổi" tự động 0 đồng, tự cập nhật khi host đánh dấu đã thu. Buổi thử cùng ngày (không có giao dịch nào) bị thay bằng buổi này.

Sau khi nạp, tháng 11/2026: thu 2.030.000 (cố định) + 1.105.000 (phí buổi) = 3.135.000; chi 2.345.000 + 220.000 = 2.565.000; còn 340.000 chưa thu ở buổi 11/11.

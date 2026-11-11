# CLAUDE.md

Hướng dẫn cho Claude khi làm việc trong repo này. Đọc kỹ trước khi sửa code.

Đặc tả nghiệp vụ đầy đủ nằm trong `docs/raw_requirement.md`. Nếu có mâu thuẫn giữa file này và tài liệu yêu cầu, hỏi lại người dùng thay vì tự quyết.

## 1. Tổng quan dự án

App quản lý sân cầu lông phong trào cho host. Host (một người dùng duy nhất trong MVP) quản lý 1 đến 3 sân, người chơi vãng lai và cố định, ghép trận tự động theo level và giới tính, và theo dõi thu chi theo buổi và theo tháng.

Người chơi không dùng app. Chỉ host thao tác.

Ngôn ngữ giao diện và thông báo: tiếng Việt. Tên biến, hàm, comment kỹ thuật: tiếng Anh.

## 2. Tech stack

- **Backend:** Go 1.26+ (driver SQLite yêu cầu), `net/http` (hoặc `chi`), không dùng framework nặng.
- **Database:** SQLite, thông qua `database/sql` và driver `modernc.org/sqlite` (không cần CGO).
- **Migration:** file SQL trong `migrations/`, đánh số tăng dần.
- **Frontend:** React + Vite, TypeScript, PWA mobile-first. Không dùng UI framework nặng, ưu tiên CSS đơn giản.
- **Test:** `testing` của Go cho logic nghiệp vụ, đặc biệt là bộ matchmaking.

Được thêm dependency khi thật sự cần, nhưng trước khi thêm phải kiểm tra bảo mật: thư viện còn được bảo trì, không có lỗ hổng đã biết (`govulncheck`, `npm audit`), số dependency bắc cầu hợp lý. Ghi lý do chọn vào commit message.

## 3. Cấu trúc thư mục

```
cmd/server/          # entrypoint HTTP server
internal/
  domain/            # struct, enum, quy tắc nghiệp vụ thuần (không phụ thuộc DB, HTTP)
  matchmaking/       # thuật toán chọn người và chia đội (thuần, dễ test)
  session/           # usecase buổi chơi, sân, trận, hàng chờ
  finance/           # tính phí buổi, thu chi, báo cáo tháng
  storage/           # repository, SQLite
  httpapi/           # handler, routing, DTO
migrations/
web/                 # frontend React + Vite
docs/
  raw_requirement.md
simulation/          # script mô phỏng công bằng (xem mục 7)
```

Nguyên tắc phụ thuộc: `domain` và `matchmaking` không import `storage` hay `httpapi`. Chỉ `session` và `finance` gọi `storage`.

## 4. Quy tắc nghiệp vụ cốt lõi

Đây là phần quan trọng nhất. Mọi thay đổi liên quan đến ghép trận phải giữ nguyên các quy tắc sau.

### 4.1 Level

Y=1, Y+=2, TBY=3, TB-=4, TB=5, TB+=6. Lưu trong `domain` dưới dạng enum và hàm chuyển sang điểm số. Không hard-code số ở nơi khác.

### 4.2 Giới tính và đội hình hợp lệ

- 4 nam: 2 cặp nam-nam.
- 4 nữ: 2 cặp nữ-nữ.
- 2 nam 2 nữ: 2 cặp nam-nữ.
- 3 nam 1 nữ: chỉ khi `trung bình level 3 nam − level nữ ≥ ngưỡng` (mặc định 2). Chia: cặp nam-nữ đấu cặp nam-nam.
- 3 nữ 1 nam: chỉ khi `trung bình level 3 nữ − level nam ≥ ngưỡng` (mặc định 2). Chia: cặp nam-nữ đấu cặp nữ-nữ.

So sánh bằng số nguyên (nhân 3 hai vế), không dùng float.

Đội hình khác là không hợp lệ và không được trả về.

### 4.3 Cân bằng đội

Chênh lệch tổng điểm giữa hai đội không vượt quá ngưỡng cấu hình (mặc định 2). Trong các cách chia hợp lệ, chọn cách có chênh lệch nhỏ nhất.

### 4.4 Công bằng số trận (ưu tiên cao)

Thứ tự ưu tiên khi chọn người từ hàng chờ:

1. Số trận ưu tiên `matches_played + priority_offset` (ít hơn được ưu tiên).
2. `consecutive_rest` (nghỉ lâu hơn được ưu tiên).
3. `queued_at` (chờ lâu hơn được ưu tiên).

- `consecutive_rest` tăng 1 khi một trận bắt đầu mà người đó đang `WAITING` nhưng không được chọn; reset 0 khi vào trận; không tăng khi `RESTING`.
- Người check-in giữa buổi: `priority_offset = min(số trận ưu tiên của người đang có mặt)`. `matches_played` luôn là số trận thật.
- Cửa sổ ứng viên: bắt đầu với người ở mức `min`; không có tổ hợp hợp lệ thì mở rộng `min+1`, giữ tối đa người ở mức `min`; hết hàng chờ vẫn không có thì báo host, không ghép lệch.

Mục tiêu: chênh lệch số trận giữa người nhiều nhất và ít nhất không vượt quá 1 khi đủ người và thời gian. Quy tắc giới tính luôn thắng công bằng.

### 4.5 Trùng lặp đồng đội và đối thủ

Tránh ghép lại cùng đồng đội hoặc đối thủ mà người đó đã gặp trong 1 trận gần nhất (cấu hình được, chỉ trong buổi). Ưu tiên thấp nhất, chỉ dùng để chọn giữa các phương án đã ngang nhau về công bằng và cân bằng level.

### 4.6 Vòng đời trạng thái

Người chơi trong buổi: `WAITING`, `PLAYING`, `RESTING`. Không có `LEFT`; người về sớm chuyển `RESTING`.
Sân: `EMPTY`, `IN_USE`.
Trận: `DRAFT`, `ONGOING`, `FINISHED`. Không có huỷ trận.

- `DRAFT` là đề xuất của app chờ host duyệt. Người trong `DRAFT` vẫn `WAITING` nhưng được giữ chỗ, không vào đề xuất của sân khác. Mỗi sân tối đa một `DRAFT`. Host bỏ đề xuất thì xóa `DRAFT`.
- Host đổi người trong `DRAFT`: giới tính chặn cứng, lệch level chỉ cảnh báo (host có thể "Vẫn xác nhận").
- Xác nhận trận: trận `ONGOING`, sân `IN_USE`, 4 người `PLAYING`, cập nhật `consecutive_rest`, trong cùng một transaction.
- Kết thúc trận: 4 người về `WAITING` (cập nhật `queued_at`), sân về `EMPTY`, số trận mỗi người tăng 1, trong cùng một transaction. Không được để trạng thái lệch nhau giữa các bảng.
- Kết thúc buổi: trận `ONGOING` tự đóng thành `FINISHED` (tính trận), `DRAFT` bị xóa. Chỉ một buổi `OPEN` tại một thời điểm.
- Không giảm số sân khi sân bị đóng đang `IN_USE` hoặc có `DRAFT`.

Một người không bao giờ được xuất hiện trong hai trận `ONGOING` cùng lúc.

### 4.7 Tiền

- Tất cả số tiền là `int64`, đơn vị VND. Không dùng `float`.
- Phí buổi: nam 70000, nữ 60000, lưu trong cấu hình buổi (`fee_male`, `fee_female`), không hard-code.
- Chỉ người vãng lai trả phí buổi; người cố định (`is_regular`) phí buổi = 0.
- Mỗi người trong buổi có cờ `is_paid`. Kết thúc buổi tạo một giao dịch thu tổng bằng số tiền đã thu.
- Người cố định đóng "Phí tháng" do host tự nhập (giao dịch gắn `player_id`). Báo cáo luôn tách thu cố định và thu vãng lai.
- Tiền thuê sân: `court_fee` cố định mỗi sân mỗi buổi; kết thúc buổi tự tạo khoản chi `court_fee × số sân`.
- Báo cáo dạng drill-down: tổng → tháng → buổi → người chơi. Tổng mỗi cấp phải bằng tổng các mục con.
- Dữ liệu khởi tạo (người cố định, buổi cũ) theo mục 12 của `docs/raw_requirement.md`, seed chạy lại không nhân đôi.
- Xóa giao dịch là xóa mềm (`deleted_at`). Mọi truy vấn báo cáo phải lọc `deleted_at IS NULL`.

### 4.8 Thời gian

- Lưu UTC trong DB.
- Hiển thị theo múi giờ `Asia/Ho_Chi_Minh`.
- Báo cáo tháng tính theo múi giờ Việt Nam, không theo UTC.

## 5. Quy ước code

- Mọi hàm nghiệp vụ nhận `context.Context` làm tham số đầu tiên.
- Lỗi nghiệp vụ dùng sentinel error hoặc kiểu lỗi riêng trong `domain`, không trả chuỗi tự do.
- Không panic trong code thường. Chỉ dùng panic cho lỗi lập trình thật sự không thể phục hồi.
- Handler HTTP mỏng: parse request, gọi usecase, trả response. Không chứa logic nghiệp vụ.
- Mọi thao tác thay đổi trạng thái trận, sân, người chơi phải đi qua một usecase trong `session` và dùng transaction.
- Đặt tên theo domain: `Match`, `Court`, `SessionPlayer`, không dùng `Data`, `Info`, `Manager` chung chung.
- Không viết comment lặp lại code. Comment chỉ để giải thích lý do khi logic không hiển nhiên.

## 6. Quy ước frontend

- Mobile-first, kiểm tra ở chiều rộng 375px trước.
- Nút thao tác chính tối thiểu 44px chiều cao.
- Trạng thái sân và người chơi dùng màu nhất quán: trống (xanh lá), đang đánh (cam), nghỉ (xám).
- Mọi thao tác thay đổi dữ liệu phải có trạng thái loading và báo lỗi rõ ràng bằng tiếng Việt.
- Không dùng `localStorage` làm nguồn sự thật cho dữ liệu buổi chơi. Nguồn sự thật là server.
- Cập nhật trạng thái bằng polling 3 giây. Không cần offline.
- Phong cách đơn giản, hiện đại, màu sắc đặc trưng cầu lông, có icon và banner. Là PWA cài được lên điện thoại; Go server serve bản build web.

## 7. Kiểm thử

### 7.1 Bắt buộc

- Unit test cho `matchmaking`: kiểm tra từng quy tắc giới tính, cân bằng đội, thứ tự ưu tiên.
- Unit test cho chuyển trạng thái trận, sân, người chơi.
- Unit test cho tính phí buổi và báo cáo tháng.

### 7.2 Mô phỏng công bằng

Thư mục `simulation/` chứa một chương trình mô phỏng buổi chơi:

- Chạy với 1, 2, 3 sân.
- Số người mỗi sân dao động 7 hoặc 8, và trường hợp hỗn hợp.
- Trận 15 phút, buổi 2 tiếng.
- Danh sách người chơi (giới tính, level) lấy từ danh sách thật do host cung cấp (giả định: `simulation/players.csv`).

Tiêu chí đạt: chênh lệch số trận giữa người nhiều nhất và ít nhất không quá 1. Nếu không đạt, không được merge thay đổi thuật toán.

Chạy bằng:

```
go run ./simulation -courts 2 -players 16 -runs 1000   # một cấu hình
go run ./simulation -matrix -runs 1000                 # 1-3 sân, 7 đến 8 người mỗi sân
```

Thêm `-v` để in từng buổi không đạt, `-file` để dùng danh sách người chơi khác. Buổi mà quy tắc giới tính/cân bằng khiến không thể công bằng (ví dụ có người không xếp được vào đội hình hợp lệ nào) được đếm riêng, không tính là lỗi.

### 7.3 Lệnh thường dùng

```
go test ./...
cp .env.example .env          # rồi điền ADMIN_USERNAME, ADMIN_PASSWORD
cd web && npm install && npm run build   # Go server nhúng web/dist lúc biên dịch
go run ./cmd/server           # http://localhost:8080
cd web && npm run dev         # giao diện dev, proxy /api sang :8080
docker build -t bwd . && docker run -p 8080:8080 -v bwd-data:/data -e ADMIN_USERNAME=... -e ADMIN_PASSWORD=... bwd
```

Cập nhật các lệnh này khi repo thay đổi.

## 8. Quy trình làm việc với Claude

- **Lần dựng ban đầu** (repo chưa có code): được phép tự quyết thiết kế thuật toán và schema theo các quyết định đã chốt trong `docs/raw_requirement.md`, không cần dừng lại chờ xác nhận. Câu hỏi còn mở thì dùng "giả định tạm" ghi trong tài liệu.
- Sau lần dựng ban đầu: trước khi sửa thuật toán ghép trận hoặc schema DB, tóm tắt kế hoạch và chờ người dùng xác nhận.
- Thứ tự dựng: `domain` + `matchmaking` + mô phỏng đạt → `storage` + `session` → `httpapi` (kèm đăng nhập) → giao diện web → `finance` và báo cáo.
- Khi thêm tính năng mới, cập nhật `docs/raw_requirement.md` trước, rồi mới viết code.
- Mỗi thay đổi cần chạy `go test ./...` và báo kết quả.
- Đăng nhập chỉ là một tài khoản admin lấy từ biến môi trường `ADMIN_USERNAME`, `ADMIN_PASSWORD`. Không hard-code mật khẩu trong code hay commit file chứa mật khẩu.
- Không tự thêm tính năng ngoài phạm vi MVP (đa host, đăng ký, thông báo, ELO, huỷ trận, offline). Nếu thấy cần, hỏi trước.
- Khi gặp câu hỏi nghiệp vụ chưa rõ, ưu tiên mục "Câu hỏi còn mở" trong `docs/raw_requirement.md`. Nếu chưa có câu trả lời, hỏi người dùng thay vì giả định.
- Commit thẳng vào nhánh `develop`. Commit nhỏ, mỗi commit một thay đổi logic rõ ràng.

## 9. Những điều không được làm

- Không dùng `float64` cho tiền.
- Không xóa cứng giao dịch tài chính.
- Không ghép người đang `PLAYING` vào trận khác.
- Không bỏ qua kiểm tra quy tắc giới tính khi host đổi người trong đề xuất.
- Không đưa logic nghiệp vụ vào handler HTTP hoặc frontend.
- Không hard-code phí, ngưỡng level, ngưỡng cân bằng ở nhiều nơi.

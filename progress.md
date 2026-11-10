# Progress: Quản lý sân cầu lông & ghép trận

Ghi lại các bước đã làm trước khi bắt đầu viết code. Cập nhật file này sau mỗi mốc lớn.

Cập nhật lần cuối: 2026-11-10 · Nhánh: `develop`

---

## Trạng thái tổng quan

- Giai đoạn: **Chuẩn bị (chưa có code ứng dụng)**
- Đã có: tài liệu yêu cầu, hướng dẫn cho Claude, repo git.
- Chưa có: `go.mod`, mã Go, migration, frontend, `simulation/`, `.gitignore`.

---

## Lịch sử các bước đã làm

| # | Ngày | Bước | Commit | Ghi chú |
|---|---|---|---|---|
| 1 | 2026-11-10 | Khởi tạo repo git | `c235deb` Initial commit | Repo trống. |
| 2 | 2026-11-10 | Viết `CLAUDE.md` (hướng dẫn cho Claude: stack, cấu trúc thư mục, quy tắc nghiệp vụ, quy ước code, kiểm thử) | `8215b6c` Add project guide and MVP requirements | Nguồn chuẩn cho mọi quy tắc kỹ thuật. |
| 3 | 2026-11-10 | Viết `docs/raw_requirement.md` v0.2 (bối cảnh, phạm vi MVP, yêu cầu chức năng, quy tắc ghép trận, data model, tiêu chí chấp nhận) | `8215b6c` | Đã chốt phần lớn câu hỏi với host. Mục 12 có dữ liệu khởi tạo. |
| 4 | 2026-11-10 | Tạo file `progress.md` này | — | Chưa commit. |

---

## Quyết định đã chốt (tóm tắt)

Chi tiết đầy đủ nằm trong `docs/raw_requirement.md` mục 10.

- MVP chỉ có **một host**, đăng nhập bằng tài khoản admin lấy từ `ADMIN_USERNAME`, `ADMIN_PASSWORD`.
- Trạng thái: người chơi `WAITING` / `PLAYING` / `RESTING` (không có `LEFT`); sân `EMPTY` / `IN_USE`; trận `DRAFT` / `ONGOING` / `FINISHED` (không có huỷ trận).
- App chỉ **đề xuất** trận (`DRAFT`), host duyệt. Mỗi sân tối đa một `DRAFT`.
- Đổi người trong `DRAFT`: lệch giới tính chặn cứng, lệch level chỉ cảnh báo.
- Ghép trận: ưu tiên `matches_played + priority_offset`, rồi `consecutive_rest`, rồi `queued_at`. Mục tiêu chênh lệch số trận ≤ 1.
- Giới tính 3-1 / 1-3: ngưỡng trung bình ≥ 2, so sánh bằng số nguyên.
- Tiền: `int64` VND. Người cố định không trả phí buổi; phí tháng host tự nhập.
- Báo cáo drill-down: tổng → tháng → buổi → người chơi, thu cố định và vãng lai tách riêng.
- Thời gian lưu UTC, hiển thị và tính báo cáo tháng theo `Asia/Ho_Chi_Minh`.
- Polling 3 giây. Không offline. Không đồng hồ đếm giờ trận.
- Chỉ một buổi `OPEN` tại một thời điểm.

---

## Việc còn lại trước khi bắt đầu code

Theo thứ tự dựng trong `CLAUDE.md` mục 8:

- [ ] Thêm `.gitignore` (`.DS_Store`, `web/node_modules`, binary, file DB, `.env`).
- [ ] Thêm `.env.example` với `ADMIN_USERNAME`, `ADMIN_PASSWORD` (không có giá trị thật).
- [ ] Khởi tạo `go.mod` (Go 1.22+).
- [ ] Đặt file `simulation/players.csv` (host cung cấp danh sách thật, hoặc tạm dùng dữ liệu giả lập).
- [ ] Trả lời các câu hỏi mở (hiện mục 11 của tài liệu ghi "Chưa có").

### Thứ tự dựng

1. `domain` + `matchmaking` + mô phỏng đạt tiêu chí (chênh lệch số trận ≤ 1).
2. `storage` + `session`.
3. `httpapi` (kèm đăng nhập).
4. Giao diện web (React + Vite, PWA).
5. `finance` và báo cáo.

Trước bước 1 phải tóm tắt kế hoạch và chờ host xác nhận (`CLAUDE.md` mục 8).

---

## Mốc kiểm tra

- [ ] `go test ./...` chạy được (sau khi có `go.mod`).
- [ ] Mô phỏng `go run ./simulation -courts 2 -players 16 -runs 1000` đạt chênh lệch số trận ≤ 1.
- [ ] Mỗi bước dựng có commit riêng trên `develop`.

---

## Nhật ký thay đổi

- 2026-11-10: Tạo `progress.md`, ghi lại các bước khởi tạo repo, `CLAUDE.md`, `docs/raw_requirement.md`.

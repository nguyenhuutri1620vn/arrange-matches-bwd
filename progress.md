# Progress: Quản lý sân cầu lông & ghép trận

Cập nhật file này sau mỗi mốc lớn.

Cập nhật lần cuối: 2026-11-11 · Nhánh: `develop`

---

## Trạng thái tổng quan

- Giai đoạn: **Bản dựng đầu tiên đã xong** (đủ 5 bước trong `CLAUDE.md` mục 8).
- Chưa làm: deploy, mô phỏng lại với danh sách người chơi thật, xác nhận các giả định tạm ở mục 11 của `docs/raw_requirement.md`.

---

## Lịch sử các bước đã làm

| # | Ngày | Bước | Ghi chú |
|---|---|---|---|
| 1 | 2026-11-10 | Khởi tạo repo, `CLAUDE.md`, `docs/raw_requirement.md` v0.2 | Nguồn chuẩn cho quy tắc nghiệp vụ. |
| 2 | 2026-11-10 | `internal/domain` + `internal/matchmaking` + `simulation/` | Thuật toán chọn người theo số trận ưu tiên, có nhìn trước để không bỏ kẹt người ít trận. |
| 3 | 2026-11-10 | `migrations/` + `internal/storage` + `internal/session` | SQLite (`modernc.org/sqlite`), mọi thay đổi trạng thái trong một transaction, seed mục 12. |
| 4 | 2026-11-10 | `internal/finance` | Thu chi, phí tháng, báo cáo drill-down. |
| 5 | 2026-11-10 | `internal/httpapi` + `cmd/server` | Đăng nhập admin từ biến môi trường, cookie ký, giới hạn đăng nhập sai. |
| 6 | 2026-11-10 | `web/` (React + Vite, PWA) | 5 tab: Buổi chơi, Người chơi, Thu chi, Báo cáo, Cài đặt. |
| 7 | 2026-11-11 | Kiểm tra giao diện ở 375px, Dockerfile, cập nhật tài liệu | Ghi giả định tạm vào mục 11 của tài liệu yêu cầu. |

---

## Mốc kiểm tra

- [x] `go test ./...` đạt (domain, matchmaking, session, finance, httpapi).
- [x] Mô phỏng `go run ./simulation -matrix -runs 1000` đạt: 1, 2, 3 sân, 7 đến 8 người mỗi sân, không có buổi nào lệch quá 1 trận mà lẽ ra xếp công bằng được. Dùng danh sách giả lập `simulation/players.csv`.
- [x] `govulncheck ./...` và `npm audit`: không có lỗ hổng.
- [x] Đã chạy thử trên trình duyệt ở 375px: đăng nhập, mở buổi, check-in, đề xuất, đổi người, xác nhận, thu phí, kết thúc buổi, báo cáo tháng 11/2026 (3 người cố định, 2 buổi cũ), thu chi.
- [ ] Host mô phỏng lại với danh sách người chơi thật.
- [ ] Host xác nhận các giả định tạm (mục 11 tài liệu yêu cầu).
- [ ] Deploy.

---

## Điều cần biết khi làm tiếp

- Go server nhúng `web/dist` lúc biên dịch: sửa giao diện xong phải `npm run build` rồi chạy lại server.
- `go.mod` yêu cầu Go 1.26 (do driver SQLite); Go cũ hơn sẽ tự tải toolchain.
- `.env` (không commit) chứa `ADMIN_USERNAME`, `ADMIN_PASSWORD`, `SESSION_SECRET`. Thiếu `SESSION_SECRET` thì mỗi lần khởi động lại server phải đăng nhập lại.
- Sau lần dựng ban đầu: sửa thuật toán ghép trận hoặc schema phải tóm tắt kế hoạch và chờ host xác nhận (`CLAUDE.md` mục 8), và mô phỏng phải đạt lại.

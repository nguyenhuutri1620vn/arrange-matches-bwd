# Nhóm luân phiên chung/tách, bảng nhóm riêng, chọn nhóm lúc check-in

Ngày: 2026-11-17. Thay phần "nhóm theo tỉ lệ 67%" của `2026-11-16-tranh-3-1-va-nhom-theo-ti-le-design.md`. Phần tránh 3-1 (phương án A) của bản đó giữ nguyên.

## Vì sao

Host thấy nhóm vẫn bị tách lẻ và muốn nhóm đánh theo một nhịp rõ ràng, đoán trước được:

- **Nhóm 4 người:** trận đầu cả 4 đánh chung một sân. Các trận sau tách thành 2 cặp, mỗi cặp đấu với cặp người ngoài. Khi 2 cặp cùng rảnh thì cả 4 lại đánh chung. Mỗi lần tách thì đổi cặp (host chọn).
- **Nhóm 2 người:** 1 trận đứng chung đội, 1 trận tách ra, lặp lại.
- **Nhóm 3 người:** mỗi trận có 2 người trong nhóm đứng chung đội, người còn lại đánh với người ngoài. Trận sau đổi người đứng ngoài (host chọn).
- Host chọn nhóm ngay trong màn "Thêm người vào buổi".
- Host muốn có bảng nhóm riêng cho từng buổi, lưu nhóm đang ở nhịp "chung" hay "tách".

## Mục tiêu

1. Nhóm 2 và nhóm 4 theo nhịp chung → tách → chung. Nhóm 3 luân phiên cặp đôi.
2. Ô "Nhóm đi chung" trong màn "Thêm người", tạo được nhóm mới ngay tại đó.
3. Công bằng số trận vẫn đạt tiêu chí mô phỏng (≤ 1, cho phép tối đa 3% buổi có nhóm từ 2 sân bị lệch 2 trận như đã chốt 2026-11-14).

## Không làm

- Không lưu nhóm cố định qua các buổi (host chọn nhóm theo buổi).
- Không thêm loại "cặp cố định": nhóm 4 đổi cặp mỗi lần tách.
- Không bỏ quy tắc công bằng, giới tính, cân bằng: nhịp nhóm chỉ là ưu tiên, các quy tắc này luôn đứng trên. Khi bị ép, nhịp có thể trễ (ví dụ cả 4 chưa cùng rảnh thì chưa đánh chung).

## Các phương án đã cân nhắc

- **Bảng nhóm riêng theo buổi (đã chọn):** bảng `session_groups` có trạng thái nhịp. Rõ ràng, nhịp đổi trong cùng transaction với xác nhận trận.
- Nhóm cố định trong danh bạ: host không cần.
- Không đổi schema, suy nhịp từ lịch sử trận: làm được nhưng host muốn bảng nhóm, và trạng thái nằm trong DB dễ kiểm tra hơn.

## Thiết kế

### Database (migration `0003_session_groups.sql`)

```sql
CREATE TABLE session_groups (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id       INTEGER NOT NULL REFERENCES sessions (id),
    no               INTEGER NOT NULL,            -- hiển thị "Nhóm 1", "N1"
    phase            TEXT    NOT NULL DEFAULT 'TOGETHER' CHECK (phase IN ('TOGETHER', 'SPLIT')),
    phase_started_at INTEGER NOT NULL,            -- UTC, lúc vào nhịp hiện tại
    created_at       INTEGER NOT NULL,
    UNIQUE (session_id, no)
);
ALTER TABLE session_players ADD COLUMN group_id INTEGER REFERENCES session_groups (id);
```

- Dữ liệu cũ: mỗi cặp `(session_id, group_no)` khác NULL tạo một dòng `session_groups` (`phase = 'TOGETHER'`, thời gian = `checked_in_at` nhỏ nhất của nhóm), rồi gán `group_id`. Sau đó bỏ cột `group_no`.
- Nhóm không còn ai thì xóa dòng nhóm (không phải dữ liệu tài chính nên xóa cứng được). Số `no` của nhóm mới là số nhỏ nhất chưa dùng trong buổi.
- Tối đa 4 người một nhóm, như cũ.

### Nhịp nhóm

Nhóm 1 người không có tác dụng. Nhịp chỉ dùng cho nhóm 2 và 4 người, nhóm 3 luôn ở chế độ cặp đôi luân phiên.

| Nhóm | Nhịp `TOGETHER` hoàn thành khi | Nhịp `SPLIT` hoàn thành khi |
|---|---|---|
| 2 người | một trận xác nhận có 2 người đứng chung đội | mỗi người đã vào ít nhất 1 trận kể từ `phase_started_at` |
| 4 người | một trận xác nhận có cả 4 người | như trên |

- Hoàn thành `TOGETHER` → chuyển `SPLIT`. Hoàn thành `SPLIT` → chuyển `TOGETHER`. `phase_started_at` = lúc xác nhận trận làm đổi nhịp.
- Nhịp đổi trong usecase xác nhận trận, cùng transaction với trận `ONGOING`, sân `IN_USE` và người chơi `PLAYING`.
- Trận vào sân mà không theo nhịp (do công bằng/giới tính ép) không làm đổi nhịp, trừ việc được tính là "đã vào trận" cho `SPLIT`.
- Host đổi nhóm của một người: nhóm mới giữ nhịp hiện tại của nó. Nhóm đổi kích thước thì nhịp vẫn giữ, áp dụng theo kích thước mới.

### Ưu tiên khi đề xuất

Mỗi ứng viên có thêm `GroupSize`, `GroupPhase` và `SplitPlayed` (đã vào trận kể từ `phase_started_at`). Tiêu chí nhóm trong thứ tự xếp hạng (6.8) thay bằng:

- **Nhóm 2, `TOGETHER`:** ưu tiên phương án có cả đôi đứng chung đội.
- **Nhóm 2, `SPLIT`:** ưu tiên phương án không cho đôi này đứng chung đội. Hai người vào trận riêng hoặc đối đầu nhau đều được.
- **Nhóm 4, `TOGETHER`:** ưu tiên phương án có cả 4 người (2 cặp trong nhóm đấu nhau, đổi cặp so với lần gần nhất).
- **Nhóm 4, `SPLIT`:** ưu tiên phương án có một cặp trong nhóm đứng chung đội đấu người ngoài, cặp chưa đứng chung gần đây trước. Người đã đánh trong nhịp `SPLIT` (`SplitPlayed`) không được ưu tiên thêm, để mọi người trong nhóm đều qua một lượt tách.
- **Nhóm 3:** ưu tiên phương án có đúng một cặp trong nhóm đứng chung đội, cặp lâu chưa đứng chung trước. Người thứ ba không vào cùng trận nếu có phương án khác.
- Vị trí tiêu chí này trong 6.8 giống tiêu chí nhóm hiện tại (sau công bằng, nhìn trước, tránh 3-1, người khó xếp). Nếu mô phỏng cho phép, thử đưa lên trên "người khó xếp".

### "Cùng vào, cùng đợi" (từ 2 sân)

Quy tắc chờ nhau hiện có áp dụng theo đơn vị của nhịp:

- **`TOGETHER`:** cả nhóm là một đơn vị (như hiện tại).
- **Nhóm 4 `SPLIT` và nhóm 3:** đơn vị là cặp. Một cặp không phải chờ cặp kia, nhưng một người không chờ ai thì vẫn vào được với người ngoài.
- **Nhóm 2 `SPLIT`:** không ai chờ ai.
- Buổi 1 sân thì vẫn chỉ ưu tiên, không ai chờ.

### Màn "Thêm người vào buổi"

- Trên ô tìm kiếm có dòng "Nhóm đi chung": `Không` · `N1 (2)` · `N2 (3)` · … · `Nhóm mới`. Số trong ngoặc là số người hiện có. Nhóm đủ 4 người không hiện.
- Lựa chọn được giữ cho các lần check-in tiếp theo trong màn. Chọn `Nhóm mới` thì lần check-in đầu tạo nhóm, các lần sau tự chuyển sang nhóm vừa tạo.
- API `POST /api/sessions/{id}/players` nhận thêm `group_id` (số nhóm có sẵn) hoặc `new_group: true`. Tạo nhóm, check-in và gán nhóm nằm trong cùng một transaction. Nhóm đầy thì trả `ErrGroupFull` và không thêm người.
- Phần đổi nhóm trong hàng chờ dùng cùng bộ lựa chọn này. Hàng chờ hiện thêm nhịp của nhóm: "N1 · chung" / "N1 · tách".

### Mô phỏng

- Mô phỏng giữ nhịp nhóm giống app.
- Mô phỏng in thêm tỉ lệ trận 3-1, và với từng loại nhóm: tỉ lệ nhịp `TOGETHER` hoàn thành đúng hạn, cùng số trận trung bình mỗi nhịp.
- Phải PASS với `-matrix`, `-groups 2..4`, `-group-size 2|3|4`.

### Tài liệu

- `docs/raw_requirement.md` 5.1, 6.7, 6.8 viết lại theo nhịp. Bỏ đoạn "tỉ lệ đứng chung" thêm hôm 2026-11-16. Thêm nhật ký 2026-11-17.
- CLAUDE.md 4.4 cập nhật đoạn về nhóm.

## Để người làm tự quyết

- Cấu trúc code của luật nhịp trong `matchmaking` (hàm thuần, có test riêng).
- Cách cài đặt chi tiết quy tắc chờ theo đơn vị cặp trong `GroupQueue`/`groupRule`.
- Nếu mô phỏng không đạt, báo host con số trước khi nới tiêu chí.

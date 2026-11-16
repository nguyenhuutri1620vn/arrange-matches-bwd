# Tránh 3-1 chặt hơn, nhóm đứng chung theo tỉ lệ, chọn nhóm lúc check-in

Ngày: 2026-11-16. Host duyệt hướng: tránh 3-1 theo phương án A, nhóm theo tỉ lệ (cách 1).

## Vì sao

- Trận 3 nam 1 nữ / 3 nữ 1 nam ra quá nhiều. Mô phỏng hiện tại (danh sách 16 nam, 8 nữ): 16–26% số trận ở 1 sân, 10–17% ở 2 sân, 3–11% ở 3 sân.
- Host muốn người đi chung (cặp đôi, bạn bè) phần lớn đứng chung đội, khoảng 4/6 trận, thỉnh thoảng tách ra để đổi gió. Hiện tại đôi 2 người chỉ đứng chung đội 33–37% số trận ở 1 sân, 61–75% ở 2–3 sân.
- Host chỉ gán được nhóm sau khi check-in (chạm tên trong hàng chờ). Thêm cả nhóm thì phải làm hai bước cho từng người.

## Mục tiêu

1. Ít trận 3-1 hơn hẳn, mà chênh lệch số trận vẫn ≤ 1 (tiêu chí mô phỏng mục 7.2 không đổi).
2. Đôi cùng nhóm đứng chung đội khoảng 2/3 số trận của họ trong buổi, phần còn lại được tách ra.
3. Chọn nhóm ngay trong màn "Thêm người".

## Không làm

- Không cấm hẳn 3-1 (phương án B): người lẻ giới sẽ ngồi ngoài lâu.
- Không thêm loại "cặp cố định": host chọn để nhóm thỉnh thoảng tách ra.
- Không đổi schema DB. Số trận chung đội tính từ lịch sử trận trong buổi.
- Không đổi quy tắc "cả nhóm cùng vào, cùng đợi" khi từ 2 sân.

## Không được phá

Công bằng số trận (mục 4.4), quy tắc giới tính và ngưỡng cân bằng (4.2, 4.3) vẫn đứng trên hết. Mô phỏng phải PASS với `-matrix`, với `-groups 2` đến `4`, và với nhóm 3, 4 người.

## Các phương án đã cân nhắc

**3-1:** A, chặt hơn (đã chọn): khi những người đang tới lượt chỉ xếp được 3-1, mở rộng sang người hơn 1 trận để tìm 4-0/2-2. B, cấm hẳn: bỏ vì người lẻ giới bị lệch 2 trận. C, chỉ nâng ngưỡng level: bỏ vì vẫn còn nhiều 3-1.

**Nhóm:** 1, theo tỉ lệ (đã chọn): đôi nào dưới 2/3 thì ưu tiên chung đội cao hơn, đã đủ thì cho tách. 2, nâng ưu tiên cố định: bỏ vì thành "luôn chung", và lần thử 2026-11-14 làm 1 sân lệch 2 trận.

## Thiết kế

### 3-1 (phương án A)

- Hiện tại 3-1 chỉ bị xếp sau trong cùng cửa sổ ứng viên (những người cùng số trận ưu tiên). Sau khi sửa: nếu phương án tốt nhất của cửa sổ là 3-1, app thử thêm cửa sổ mở rộng sang mức `min+1`, vẫn giữ tối đa số người ở mức `min`. Có đội hình 4-0 hoặc 2-2 thì dùng nó, không có thì mới dùng 3-1.
- Giữ ngoại lệ hiện có: người lẻ giới đã ngồi ngoài ít nhất 1 trận thì trận 3-1 có họ không bị phạt, để người nữ duy nhất còn lại không phải chờ mãi. Nếu mô phỏng cho thấy ngoại lệ này là nguồn chính của 3-1, mình sẽ báo con số cho host trước khi đổi.
- Ngưỡng level của 3-1 (`GenderGapThreshold`, mặc định 2) giữ nguyên.

### Nhóm đứng chung theo tỉ lệ

- Mỗi ứng viên có thêm hai số trong buổi: số trận đã đánh khi đang thuộc nhóm hiện tại, và số trận trong đó đồng đội cùng nhóm. Các số này lấy từ lịch sử trận đã kết thúc trong buổi (`session` đã đọc lịch sử này cho quy tắc trùng lặp, chỉ cần đọc hết buổi thay vì 3 trận gần nhất).
- Một người **thiếu tỉ lệ** khi `100 × số trận chung đội < tỉ lệ × số trận`, so bằng số nguyên. Tỉ lệ nằm trong `domain.MatchRules` (`GroupTeamPct`, mặc định 67), không hard-code ở chỗ khác. Người chưa đánh trận nào được tính là thiếu.
- Khi chọn 4 người và chia đội: phương án có nhiều người thiếu tỉ lệ được đứng chung đội với bạn cùng nhóm thì xếp trước. Tiêu chí này nằm trên "người ngồi ngoài 2 trận", trộn người và người nghỉ lâu. Vị trí so với "người khó xếp" sẽ do mô phỏng quyết định: đặt trên nếu vẫn PASS, không thì đặt dưới.
- Người đã đủ tỉ lệ: ưu tiên "đôi cùng nhóm" của họ được hạ xuống dưới trộn người, để app tự tách họ ra đổi gió.
- Quy tắc "cả nhóm cùng vào, cùng đợi" từ 2 sân giữ nguyên. Ở 1 sân, nhóm vẫn chỉ được ưu tiên, không ai phải chờ.

### Chọn nhóm lúc check-in

- Màn "Thêm người vào buổi" có thêm dòng "Nhóm đi chung": `Không` · các nhóm đang có (còn chỗ) · `Nhóm mới`. Mặc định là `Không`.
- Nhóm đã chọn được giữ lại trong màn này. Host check-in lần lượt từng người của nhóm mà không phải chọn lại. Dùng được cho cả "Lưu và check-in" khi thêm người mới.
- Check-in xong, app gọi API gán nhóm đã có (`setGroup`) ngay sau `checkIn`. Không có logic nhóm ở frontend ngoài việc hiển thị lựa chọn. Nhóm đầy (4 người) không hiện trong danh sách.
- Phần đổi nhóm trong hàng chờ giữ như cũ.

### Đo và báo cáo

- Mô phỏng in thêm tỉ lệ trận 3-1 và tỉ lệ đôi cùng nhóm đứng chung đội, để so trước/sau và giữ lại cho các lần sửa sau.
- Báo host bảng trước/sau. Nếu 1 sân không đạt 2/3 mà không phá công bằng, báo con số thật.

### Tài liệu

Cập nhật `docs/raw_requirement.md` trước khi code: mục 6.6 (3-1), 6.7 (tỉ lệ đứng chung), 6.8 (thứ tự ưu tiên), 5.1 (chọn nhóm lúc check-in), thêm dòng nhật ký 2026-11-16.

## Để người làm tự quyết

- Cách cài đặt chi tiết việc mở rộng cửa sổ cho 3-1 trong `proposeFrom`/`bestInWindow`.
- Tiêu chí tỉ lệ nhóm nằm ở đâu trong `option.better` và trong cách chia đội, theo kết quả mô phỏng và trong giới hạn ở trên.
- Unit test cho từng quy tắc mới (3-1 mở rộng cửa sổ, thiếu/đủ tỉ lệ, tính số trận chung đội).

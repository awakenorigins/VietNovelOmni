# Bộ Quy Tắc Kiểm Tra Liền Mạch (30+ rules)

Đây là danh sách kiểm tra CỨNG khi check_consistency. Đối chiếu từng rule có mã, ghi rõ vi phạm (nếu có) kèm trích dẫn nguyên văn làm bằng chứng. Không phán mơ hồ.

## Nhóm THỜI GIAN (T)
- **T01 — Thời gian lùi**: Sự kiện chương sau xảy ra TRƯỚC sự kiện chương trước mà không có hồi tưởng rõ ràng. Kiểm tra mốc thời gian, ngày/giờ, mùa.
- **T02 — Nhảy ngày vô lý**: Khoảng cách thời gian giữa hai cảnh không khớp với hành động (vd "sáng hôm sau" nhưng nhân vật vừa làm việc cả đêm dài hơn).
- **T03 — Dịch chuyển tức thời**: Nhân vật xuất hiện ở địa điểm mới mà không có quá trình di chuyển hợp lý về thời gian/không gian.
- **T04 — Tuổi/thời lượng sai**: Tuổi nhân vật, độ dài quan hệ, thời gian tu luyện mâu thuẫn giữa các chương.

## Nhóm NHÂN VẬT (C)
- **C01 — Chết rồi sống lại**: Nhân vật đã xác nhận chết lại xuất hiện hành động bình thường mà không giải thích (hồi sinh, hiểu lầm, song sinh).
- **C02 — Vi phạm trạng thái phong ấn**: Nhân vật bị thương nặng/trói/phong ấn vẫn hành động như bình thường.
- **C03 — Nhảy cấp năng lực**: Sức mạnh/cảnh giới/kỹ năng tăng vọt không qua quá trình, hoặc giảm vô cớ.
- **C04 — Tính cách đứt gãy**: Hành vi trái ngược tính cách đã thiết lập mà không có sự kiện kích hoạt.
- **C05 — Biến mất khỏi cảnh**: Nhân vật có mặt đầu cảnh rồi biến mất không lý do, hoặc xuất hiện đột ngột giữa cảnh kín.
- **C06 — Biệt danh nhầm người**: Cùng một người bị gọi nhiều tên khiến tưởng là người khác; hoặc hai người khác nhau bị trộn tên.
- **C07 — Quan hệ nhảy vọt**: Từ xa lạ sang tin tưởng tuyệt đối (hoặc thù địch sang thân thiết) trong một chương không có sự kiện đủ nặng.

## Nhóm THẾ GIỚI / VẬT PHẨM (W)
- **W01 — Vi phạm quy tắc thế giới**: Hành động phá vỡ ranh giới luật thế giới đã thiết lập (ma pháp, công nghệ, xã hội) mà không giải thích ngoại lệ.
- **W02 — Vật phẩm trùng/biến mất**: Vật phẩm độc nhất xuất hiện hai nơi, hoặc vật quan trọng biến mất không lý do, hoặc dùng vật đã mất.
- **W03 — Địa lý mâu thuẫn**: Vị trí địa danh, khoảng cách, bố cục không gian mâu thuẫn giữa các chương.
- **W04 — Tiền tệ/đơn vị lệch**: Giá cả, đơn vị đo, hệ thống tiền không nhất quán.

## Nhóm MẠCH TRUYỆN / PHỤC BÚT (P)
- **P01 — Phục bút bỏ rơi**: Manh mối/lời hứa cốt truyện gài ra nhưng không bao giờ thu hồi (theo dõi foreshadow_ledger).
- **P02 — Thu hồi không gài**: Tình tiết giải đáp một phục bút chưa từng được gài trước đó (deus ex machina).
- **P03 — Mâu thuẫn động cơ**: Hành động nhân vật trái với mục tiêu/động cơ đã nêu.
- **P04 — Xung đột tràn**: Xung đột được giải quyết quá dễ so với mức độ đã xây dựng, hoặc kéo dài vô lý.
- **P05 — Thông tin lộ sớm**: Bí mật lớn bị tiết lộ trước thời điểm outline yêu cầu, làm hỏng nút thắt.

## Nhóm ĐẠI TỪ / GIỌNG (V)
- **V01 — Đại từ sai ngôi**: Xưng hô (anh/em/tôi/ta/tao) lệch với quan hệ và POV đã thiết lập.
- **V02 — Giọng nhân vật trộn**: Hai nhân vật nói giống hệt nhau, không phân biệt được nếu bỏ thẻ người nói.
- **V03 — POV nhảy**: Góc nhìn kể chuyện đổi giữa chừng không có chủ đích (đầu chương ngôi thứ ba hạn tri một người, giữa chương biết suy nghĩ người khác).

## Nhóm CHI TIẾT (D)
- **D01 — Ngoại hình đổi**: Màu mắt, tóc, chiều cao, đặc điểm nhận dạng mâu thuẫn.
- **D02 — Trang phục nhảy**: Quần áo/trang bị thay đổi giữa cảnh không có hành động thay.
- **D03 — Vết thương biến mất**: Thương tích nặng khỏi quá nhanh không hợp lý.
- **D04 — Thời tiết/ánh sáng lệch**: Trời/đèn/mùa mâu thuẫn trong cùng một mạch thời gian.
- **D05 — Số lượng sai**: Số người, vật, lần lặp đếm không khớp giữa các đoạn.

## Cách dùng khi kiểm tra
1. Đọc kỹ bản nháp chương + dữ liệu đối chiếu (world_rules, foreshadow_ledger, relationships, alias_map, recent_summaries).
2. Duyệt từng nhóm rule. Với mỗi vi phạm: ghi mã rule + trích nguyên văn câu vi phạm + giải thích ngắn.
3. Phân loại nghiêm trọng: lỗi cứng (T01/C01/W02...) phải sửa; lỗi mềm (D04 nhẹ) ghi chú.
4. Nếu không vi phạm rule nào, xác nhận rõ "không phát hiện mâu thuẫn theo 30 rule".

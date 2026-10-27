# Hội Đồng Nacharium — 13 Góc Phê Bình (tùy chọn)

Khi cần review sâu một chương/cuốn (mode `oum council`), thay vì 1 Editor, đánh giá qua nhiều góc nhìn rồi gộp. Dùng cho chương then chốt, bản thảo cuối, hoặc khi anh Nấng yêu cầu thẩm định kỹ.

## Bộ NHANH (5 góc — mặc định)
1. **Bắt Lỗi Liền Mạch** — đối chiếu 30 rule continuity (T/C/W/P/V/D), trích nguyên văn vi phạm.
2. **Giọng & Văn Phong** — prose có thuần Việt, có nhịp, có hơi thở? Bắt câu sáo AI, em dash, tiếng Anh.
3. **Nhân Vật & Đối Thoại** — động cơ hợp lý? thoại phân biệt được người nói? có ngầm ý?
4. **Nhịp & Cảm Xúc** — đường cong căng thẳng đúng chưa? có chỗ chùng/dồn vô lý?
5. **Móc & Cấu Trúc** — mở hút? kết để lại sức kéo? beat phục vụ tổng thể?

## Bộ ĐẦY ĐỦ (13 góc — khi cần kỹ)
Thêm 8 góc vào bộ nhanh:
6. **Show-don't-tell** — có dán nhãn cảm xúc thay vì thể hiện qua thân thể/lựa chọn?
7. **Phục Bút** — gài đủ? thu hồi đúng lúc? có bỏ rơi (P01) hay deus ex machina (P02)?
8. **Thế Giới Quan** — quy tắc thế giới nhất quán? lộ tự nhiên hay nhồi thiết lập?
9. **Bối Cảnh Việt** — chi tiết địa phương đúng? xưng hô đúng văn hóa? không lai căng?
10. **Logic Cốt** — nhân quả chặt? không trùng hợp tiện lợi giải nút thắt?
11. **Đa Dạng Cú Pháp** — câu dài ngắn xen kẽ? tránh lặp cấu trúc, lặp mở đầu chương?
12. **Thị Trường** — chương này có giữ chân người đọc tiếp không? sức bán?
13. **Tổng Biên** — gộp toàn bộ, ra verdict cuối: accept / polish / rewrite + 3 điểm sửa ưu tiên.

## Cách gộp (merge)
- Mỗi góc cho điểm 0-100 + comment trích dẫn cụ thể.
- Cùng một lỗi 2+ góc cùng flag → nâng mức nghiêm trọng.
- Verdict cuối theo Tổng Biên, có tham chiếu điểm các góc.
- Ngưỡng: trung bình ≥80 pass · 60-79 polish · <60 rewrite.

## Triển khai trong skill
Council chạy ở tầng skill (spawn sub-agents đọc chương đã commit, mỗi agent một bộ góc, gộp feedback), KHÔNG nằm trong engine Go — giữ engine gọn. Engine chỉ lo viết + lưu + export; council là lớp QA thêm khi cần.

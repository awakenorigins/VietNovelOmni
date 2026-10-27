# Novels — Truyện đã sáng tác bằng OmniNovel

Thư mục này chứa các tiểu thuyết đã được viết hoàn chỉnh bằng chính engine + bộ skill của repo, kèm toàn bộ dữ liệu quá trình (outline, beats, scan report) làm mẫu tham khảo thực tế.

## 1. Vua Đường Ngoại Truyện — HOÀN THÀNH ✅

- **Thể loại:** Lịch sử, Dã sử
- **Quy mô:** 21/21 chương, ~59.000 từ
- **Cấu trúc:**
  - `chapters/` — 21 chương bản chính (ch-00 → ch-20)
  - `OUTLINE-MASTER.md` — đề cương tổng
  - `BEATS-OUTLINE.md` + `beats/` — outline nhịp truyện từng chương
  - `manuscript-full.md` — bản thảo gộp toàn truyện
  - `scan-report.md` — báo cáo quét continuity
  - `edit-log.md` — nhật ký biên tập
- **Quy trình:** outline → beats → viết từng chương → scan continuity → biên tập → gộp bản thảo.

## 2. Trọng Sinh (mẫu chương mở đầu)

- **Thể loại:** Trọng sinh, Đô thị
- **Quy mô:** chương 1 (bản merge-final, ~2.600 từ)
- **Cấu trúc:** `chapters/ch01-merge-final.txt`
- Dùng làm mẫu minh họa kỹ thuật merge 3 bản viết (tiền thân của tool `draft_beat` trong engine).

## Xuất EPUB từ truyện trong thư mục này

Truyện gen bằng engine (có `progress.json` chuẩn store) xuất trực tiếp:

```bash
oum-novel --export vua-duong.epub --author "Tiểu Tâm" --series "Tần Số Nacha" --audit-gate warn
```

Truyện chỉ có thư mục `chapters/` (như 2 truyện trên) dùng dashboard OmniNovel (thư mục `dashboard/`) — dashboard tự dựng shim store để export EPUB đầy đủ front/back matter.

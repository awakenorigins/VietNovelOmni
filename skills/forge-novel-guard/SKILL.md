---
name: forge-novel-guard
version: 1.0.0
description: "Kinh nghiệm forge novel đúc kết từ 70+ chương: EPUB build, prose quality, AI trace scan, sub-agent pipeline, thẩm định 2 tầng. Cho AI đọc 1 lần là hấp thụ hết."
triggers:
  - forge novel
  - forge chapter
  - viết chương
  - build epub
  - epub premium
  - scan prose
  - kiểm tra prose
  - verify epub
  - novel quality
  - AI trace
  - thẩm định chương
  - prose checklist
  - epub checklist
author: Tiểu Tâm 🦊
---

# Forge Novel Guard V1.0

Bộ kinh nghiệm và công cụ bảo vệ chất lượng novel từ forge đến EPUB export.

## Khi nào kích hoạt

| Tình huống | Dùng gì |
|---|---|
| Forge chương mới | Đọc Section 7 (Quy trình) + chạy verify sau mỗi beat |
| Build EPUB | Đọc Section 1 (EPUB Build) + chạy `verify-epub.py` |
| Thẩm định prose | Chạy `verify-prose.py` (tiếng Anh, markdown, em dash, AI trace) |
| Gửi file Telegram | Đọc Section 3 (Telegram) + chạy `verify-send.py` |
| Sub-agent forge | Đọc Section 4 (Sub-agent rules) + inject quy tắc đầu prompt |

## Cách dùng

### Verify prose (sau mỗi beat/chương):
```bash
python3 scripts/verify-prose.py path/to/chapter.md
```

### Verify EPUB (trước khi gửi):
```bash
python3 scripts/verify-epub.py path/to/book.epub
```

### Verify file trước gửi Telegram:
```bash
python3 scripts/verify-send.py path/to/file filename.ext
```

### Full verify (prose + EPUB + send):
```bash
python3 scripts/master-verify.py path/to/book.epub
```

## Tham khảo chi tiết

Đọc `reference/kinh-nghiem-v1.0.md` để hiểu nguyên nhân gốc + phương án triệt để cho mỗi lỗi.

## Quy tắc bất di bất dịch

1. **XHTML chỉ 5 entity**: `&amp;` `&lt;` `&gt;` `&quot;` `&apos;` — còn lại dùng Unicode
2. **Cover EPUB = PNG/JPG** — KHÔNG BAO GIỜ SVG
3. **0% tiếng Anh** trong prose tiếng Việt
4. **0% markdown** trong prose (`**`, `##`, `---`, `` ` ``)
5. **0% em dash** (—) trong tiếng Việt
6. **0% dấu vết AI** (bảng đỏ 12 cụm từ)
7. **mimetype** phải là entry đầu tiên trong EPUB, ZIP_STORED
8. **EPUBCheck** 0 fatal, 0 error trước khi gửi
9. **Thẩm định 2 tầng**: Technical (script) → Prose (AI/human)
10. **Score ≥ 9/10** từ 3+ reviewer độc lập mới PASS

## Entity HTML → Unicode (bảng nhanh)

| Cấm | Dùng |
|---|---|
| `&mdash;` | — |
| `&ndash;` | – |
| `&hellip;` | … |
| `&diams;` | ♦ |
| `&nbsp;` | (U+00A0) |
| `&lsquo;` `&rsquo;` | ' ' |
| `&ldquo;` `&rdquo;` | " " |
| `&bull;` | • |
| `&copy;` | © |

## Bảng đỏ AI traces

| Dấu vết | Thay bằng |
|---|---|
| "tuyệt vời" | Bỏ hoặc mô tả cụ thể |
| "hoàn hảo" | Bỏ hoặc chi tiết |
| "thú vị thay" | Bỏ |
| "quả thật" / "đúng vậy" | Bỏ |
| "một cách [tính từ]" | Trạng từ trực tiếp |
| "không thể phủ nhận" | Viết thẳng |
| "điều đáng nói là" | Bỏ |
| "rõ ràng là" | Bỏ |
| "nói cách khác" | Bỏ |

## Tiếng Anh → Tiếng Việt (bảng nhanh)

| EN | VI |
|---|---|
| logic | lý / đạo lý |
| pattern | vết cũ / khuôn mẫu |
| VIP | quý nhân / thượng khách |
| level | cấp / bậc |
| style | phong cách |
| focus | tập trung / chú tâm |
| control | kiểm soát |
| OK | được / ổn |

## Inject cho sub-agent (copy-paste vào đầu prompt)

```
⚠️ QUY TẮC BẮT BUỘC (vi phạm = fail):
1. XHTML: CHỈ 5 entity hợp lệ (&amp; &lt; &gt; &quot; &apos;)
2. 0% markdown trong prose (không **, ##, ---, >)
3. 0% tiếng Anh trong văn tiếng Việt
4. Cover EPUB = PNG (không SVG)
5. Verify mỗi step bằng exec + read output
6. Tiếng Việt CÓ DẤU đầy đủ mọi nơi
7. Tác giả xưng "ta", KHÔNG xưng "tôi"
```

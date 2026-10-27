---
name: oumstudio-novel
description: >
  Engine viết tiểu thuyết dài AUTONOMOUS cho tiếng Việt — fork ainovel-cli (Go, multi-agent
  Coordinator/Architect/Writer/Editor, store-first + checkpoint + context compaction) tích hợp
  lớp chất lượng prose Việt: cổng chặn cứng (0 em dash / 0 tiếng Anh), 30+ rule liền mạch,
  5 style Việt (ngôn tình/tiên hiệp/đô thị/trinh thám/cung đấu), EPUB premium dark/bright.
  Nhập 1 prompt → tự dựng premise/outline/nhân vật/thế giới → viết chương → kiểm tra nhất quán
  → xuất EPUB/TXT. Hỗ trợ truyện rất dài (checkpoint resume), can thiệp realtime, import truyện cũ.
  Triggers: viết tiểu thuyết, viết truyện dài, viết novel, ngôn tình, tiên hiệp, đô thị, trinh thám,
  cung đấu, OUMStudio, oum novel, autonomous novel, viết sách, đóng epub, xuất epub, write novel,
  continue novel, export book, sáng tác truyện, engine viết truyện.
version: 1.0.0
author: Tiểu Tâm × Nấng
---

# OUMStudio-Novel

Engine viết tiểu thuyết dài autonomous. Fork `ainovel-cli` (Go) + nâng cấp chất lượng prose tiếng Việt premium.

## Bản chất

Đây KHÔNG phải prompt-guide. Đây là một **cỗ máy chạy thật** (binary Go): nhập 1 prompt truyện → nó tự gọi tools, tự viết, tự checkpoint, tự xuất sách. Multi-agent: Coordinator điều phối → Architect dựng nền → Writer viết → Editor thẩm định.

## Vị trí

- Binary: `skills/oumstudio-novel/bin/oum-novel`
- Source: `skills/oumstudio-novel/engine/` (build lại: `cd engine && go build -o ../bin/oum-novel ./cmd/ainovel-cli/`)
- Config engine: `~/.ainovel/config.json` (provider + roles)
- Scripts: `skills/oumstudio-novel/scripts/oum-prose-verify.py`
- References: `skills/oumstudio-novel/references/` (continuity-rules, nacharium-council)

## Yêu cầu

- Go ≥ 1.25 để build (đã build sẵn binary).
- Provider LLM trong `~/.ainovel/config.json`. Hiện cấu hình dùng API của Nấng (claudekkg/router9, Sonnet 4.6 chính).
  - ⚠️ Tránh model bỏ tham số `temperature` (vd Opus 4.8 trên proxy báo "temperature deprecated"). Sonnet 4.6 OK.

## Lệnh chính

### Viết truyện mới (headless, autonomous)
```bash
cd <thư-mục-làm-việc>   # output ghi vào ./output/novel/
bin/oum-novel --headless --prompt "Truyện ngôn tình hiện đại VN, 2 chương, nữ chính lao công, nam chính CEO..."
```
Hoặc dùng `--prompt-file <file>` cho prompt dài.

### Xuất sách (headless)
```bash
bin/oum-novel --export ./TenSach.epub          # EPUB 3 premium
bin/oum-novel --export ./TenSach.txt           # TXT thuần
bin/oum-novel --export ./TenSach.epub --export-from 1 --export-to 10 --overwrite
```

### TUI (tương tác đầy đủ, can thiệp realtime)
```bash
bin/oum-novel        # mở giao diện terminal: nhập prompt, /export, /import, /diag, can thiệp giữa chừng
```

## Quy trình chuẩn (skill orchestration)

1. **Chuẩn bị**: chọn thư mục làm việc riêng cho mỗi truyện (output tách biệt). Set style nếu cần (sửa `style` trong config: ngon-tinh/tien-hiep/do-thi/trinh-tham/cung-dau/default).
2. **Viết**: chạy `--headless --prompt` (hoặc `--prompt-file`). Background nếu dài; poll `output/novel/meta/progress.json` field `phase`/`completed_chapters`.
3. **Theo dõi**: engine tự checkpoint. Nếu gián đoạn → chạy lại cùng lệnh, nó resume từ điểm dừng.
4. **Kiểm tra prose** (tùy chọn QA thêm): `python3 scripts/oum-prose-verify.py --json output/novel/chapters/01.md`.
5. **Council** (tùy chọn, chương then chốt): xem `references/nacharium-council.md` — spawn sub-agents review đa góc.
6. **Xuất**: `--export book.epub`. Verify cấu trúc EPUB trước khi giao.
7. **Giao Telegram**: EPUB bị Telegram chặn định dạng → nén `.zip` rồi gửi, kèm PDF preview (convert qua weasyprint từ xhtml chương) cho người dùng xem nhanh.

## Đảm bảo chất lượng prose Việt (điểm mạnh cốt lõi)

Engine có **Cổng Chặn Cứng** trong `commit_chapter`: chương sẽ bị TỪ CHỐI lưu nếu prose có:
- Dấu gạch ngang dài `—` (em dash)
- Từ tiếng Anh thật lọt vào

→ Writer buộc viết lại sạch. Đây là khác biệt lớn với engine gốc: prose bẩn KHÔNG thể vào sách.

Ngoài ra (mức cảnh báo, qua `rules/default.md`): cụm sáo AI, từ nhịp lạm dụng, markdown sót.

## 30+ Rule Liền Mạch

`check_consistency` + Editor đối chiếu bộ rule có mã (`references/continuity-rules-vi.md`):
- T (thời gian), C (nhân vật), W (thế giới/vật phẩm), P (mạch/phục bút), V (đại từ/giọng), D (chi tiết).
Vi phạm cứng (chết sống lại, dịch chuyển, phục bút bịa...) → hạ điểm consistency về fail, buộc sửa.

## EPUB Premium

- CSS typography Việt, tự thích nghi sáng/tối (`prefers-color-scheme`), màu Tiểu Tâm.
- Markdown trong prose tự chuyển: `*nghiêng*`→em, `**đậm**`→strong, phân cách cảnh→hr.
- EPUB 3 chuẩn: mimetype/container/content.opf/nav/cover, entity-safe XHTML, `xml:lang=vi`.
- (TODO) cover ảnh: thêm khi có API image; hiện dùng bìa chữ.

## Style profiles Việt

Sửa `style` trong `~/.ainovel/config.json`:
- `ngon-tinh` · `tien-hiep` · `do-thi` · `trinh-tham` · `cung-dau` · `default` · `romance`/`fantasy`/`suspense` (gốc).

## Quy tắc giao file Telegram (bắt buộc)

- EPUB → nén `.zip` (Telegram chặn `application/epub+zip`).
- Preview → PDF (qua weasyprint), KHÔNG gửi `.md`/`.html` thô.
- Filename ASCII, trong workspace, verify UTF-8.

## Lưu ý vận hành

- Mỗi truyện = 1 thư mục làm việc riêng (output không trộn).
- Rate-limit provider → engine tự failover sang model dự phòng (cấu hình trong `roles.*.fallbacks`).
- Engine ghi `meta/diag-export.md` khi lỗi → đọc để chẩn đoán loop/lỗi tool.

## Kiểm thử

`cd engine && go test ./...` → 19 packages ok. Smoke test: `--headless --prompt "...2 chương..."` rồi `--export`.

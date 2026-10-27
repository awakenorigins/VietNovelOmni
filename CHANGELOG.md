# Changelog

Tất cả thay đổi đáng chú ý của dự án được ghi lại ở đây.
Định dạng dựa trên [Keep a Changelog](https://keepachangelog.com/), tuân theo [Semantic Versioning](https://semver.org/).

## [1.6.0] - 2026-07-03

### Added
- **draft_beat (beat-method):** tool mới cho Writer — mỗi beat viết ĐÚNG 3 bản độc lập (noi_tam / doi_thoai / hanh_dong), tự chấm điểm 0-100, rồi trộn tinh hoa thành `final_content` kèm `merge_rationale`; gate kiểm tra đủ 3 hướng trước khi nhận.
- **Audit-feedback-loop:** kết quả audit chương tự động enqueue rewrite vào hàng đợi (`rewrite_queue`) — chương yếu được viết lại không cần can thiệp tay; kèm test (`audit_feedback_test.go`, `rewrite_queue_test.go`).
- **7 skill scripts** wire vào post-commit hook: `tbd-scan.py`, `style-fingerprint.py`, `prose-metrics.py`, `critique-heuristics.py`, `wiki-extract.py`, `route-check.py`, `novelcore-check.py` — chạy heuristics thật thay vì chỉ prompt.
- **OmniNovel Web App (`dashboard/`):** web UI quản trị trọn vòng đời truyện:
  - Create novel + settings đa provider (Claude + GPT / OpenAI-compatible custom), lưu cấu hình per-provider.
  - Jobs runner + **AUTO mode vô cực**: tự đề tài → seed → viết → audit → export không dừng.
  - **Seedcraft mastery engine** (`seedcraft/`): sinh seed truyện có cấu trúc, chấm điểm, chống nhạt.
  - **Seed registry** (`seedreg.go`) chống trùng đề tài giữa các lần AUTO.
  - **Album library** (`library.go`): duyệt truyện đã hoàn thành, đọc chương, tải EPUB.
  - Export EPUB từ UI + Basic Auth bảo vệ toàn bộ app + `verify-endpoints.sh` smoke test.

### Fixed
- **draft_beat strict schema bug:** property `note` trong variant schema thiếu `Required()` → OpenAI strict tool calling từ chối schema. Đã đánh dấu required (cho phép chuỗi rỗng) để tương thích strict mode.

### Docs
- `novels/README.md` + 2 truyện hoàn chỉnh trong `novels/`: **Vua Đường Ngoại Truyện** (21 chương, ~59k từ) và **Trọng Sinh** (mẫu).

## [1.5.0] - 2026-06-29

### Added — Front/Back matter chuẩn cho EPUB
- EPUB export giờ dựng ĐẦY ĐỦ trang chuẩn của một cuốn sách:
  - Front: cover image + cover text + half-title + title page + copyright + dedication/epigraph + preface
  - Back: end-of-part + afterword + author bio + series note
- `BookMeta` struct + `Options.Meta`; CLI flags: `--meta-file` (JSON), `--author`, `--series`, `--subtitle`, `--cover`.
- Nhúng ảnh bìa thật (png/jpg/webp) qua `--cover` / meta.coverImage → khai báo `properties="cover-image"` (EPUB 3 chuẩn).
- nav.xhtml + content.opf (manifest/spine) tự cập nhật theo các trang matter.
- CSS cho toàn bộ trang matter (dark mode aware).

### Verified
- epubcheck 4.2.6: 0 errors/0 warnings trên EPUB đầy đủ matter + cover image 2.8MB.
- 26 mục XHTML đúng thứ tự đọc. 19 go test pass.

## [1.4.0] - 2026-06-29

### Changed — EPUB metadata lên chuẩn EPUB 3.3 (mới nhất)
- content.opf bổ sung metadata theo EPUB 3.3 (W3C Rec 2023): dc:date, generator, schema.org accessibility (accessMode/accessModeSufficient/accessibilityFeature/accessibilityHazard/accessibilitySummary).
- package version="3.0" GIỮ NGUYÊN — đúng spec EPUB 3.3 (version attribute không đổi cho minor versions; EPUB 3.2 valid = EPUB 3.3 valid).
- Verified: epubcheck 4.2.6 → 0 errors / 0 warnings.

## [1.3.0] - 2026-06-29

### Added — Pre-export audit gate (kiểm duyệt trước khi xuất)
- Export giờ ĐỌC báo cáo skill-audit của các chương TRƯỚC khi xuất.
- Flag `--audit-gate`: `off` (tắt) / `warn` (cảnh báo, mặc định) / `block` (chặn xuất nếu prose lỗi cứng thật).
- `block` mode: từ chối xuất + liệt kê lý do nếu có chương em dash/tiếng Anh.
- `warn` mode: in cảnh báo continuity (guardian/master) + prose nhưng vẫn xuất.
- Fix hook: novel-guardian cần tên chNN.md, novel-master cần file chNN.md positional → cả 4 skill scan đúng.

### Testing
- 19 package pass. Verified: block chặn chương em dash/tiếng Anh, warn hiện cảnh báo guardian+master trên seed "Người Giữ Giờ".

## [1.2.0] - 2026-06-29

### Added — Auto-wired skill pipeline (FULL integration)
- **Post-commit hook**: engine giờ TỰ ĐỘNG chạy toàn bộ skill verify trên MỖI chương ngay sau khi commit.
- Config mới: `post_commit_hook` + `skills_dir` (bootstrap/config.go).
- `oumstudio/post-commit-hook.sh`: orchestrator gọi oum-prose-verify + forge-novel-guard + novel-guardian + novel-master → ghi `meta/skill-audit/ch{N}.json` (JSON hợp lệ, Python builder).
- `Dockerfile.omni`: image FULL (node + python + toàn bộ skill nhúng) để pipeline chạy tự động đầu-cuối trong container.
- `deploy/`: compose + run.sh dùng image omni.
- Engine: `CommitChapterTool.WithPostCommitHook()` gọi hook ở cả path chính + rewrite.

### Fixed
- Ghi nhận: `forge-novel-guard/verify-prose.py` báo nhầm tiếng Việt không dấu là tiếng Anh (regex thô) → hook để "chỉ tham khảo", dùng oum-prose-verify (blacklist chuẩn) làm checker chính.

### Testing
- 19 package pass, 0 FAIL. Forge thật + hook fire verified (3 truyện: Người Giữ Giờ, gác hải đăng, bán hoa đêm).

## [1.1.0] - 2026-06-29

### Added — Omni Novel Suite
- Hợp nhất engine autonomous + **10 skill quy trình** vào một repo duy nhất (`skills/`).
- Bundle: cw-brainstorming, cw-prose-writing, cw-story-critique, cw-official-docs, cw-style-skill-creator, cw-router, novel-guardian, novel-master, novelcore-ai, forge-novel-guard.
- `SKILLS.md` — bản đồ pipeline 7 bước (brainstorm → style → forge → scan → critique → verify → wiki → export).
- README cập nhật thành Omni Suite (bảng so sánh engine vs skill, kiến trúc hợp nhất).
- NOTICE bổ sung attribution sub-component (novel-guardian MIT).

## [1.0.0] - 2026-06-29

### Added
- Lần phát hành chính thức đầu tiên của **OUMStudio-Novel**.
- Engine multi-agent autonomous (Coordinator / Architect / Writer / Editor) — fork tiếng Việt của `voocel/ainovel-cli`.
- Store-first + checkpoint resume cho tiểu thuyết rất dài.
- Context compaction (ctxpack) cho truyện hàng trăm chương.
- Lớp chất lượng prose tiếng Việt: cổng chặn cứng 0 em dash / 0 tiếng Anh.
- 30+ rule liền mạch (continuity) tiếng Việt.
- 9 style Việt + hội đồng phê bình Nacharium.
- Xuất EPUB premium dark/bright adaptive + TXT.
- Import truyện cũ → phân tích → viết tiếp.
- License Apache 2.0 + NOTICE attribution.

### Testing
- 19 package pass, 0 FAIL (`go test ./...`).

[1.1.0]: https://github.com/NachaFromMars/oumstudio-novel/releases/tag/v1.1.0
[1.0.0]: https://github.com/NachaFromMars/oumstudio-novel/releases/tag/v1.0.0

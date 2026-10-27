# Post-Commit Hook — Auto-wired Skill Pipeline

Từ v1.2.0, OmniNovel tự động chạy TOÀN BỘ skill verify trên mỗi chương ngay sau khi `commit_chapter` thành công. Không còn phải chạy tay.

## Cơ chế

```
Writer commit_chapter
   → [prose gate nội bộ: chặn em dash / tiếng Anh]
   → save chapters/{N}.md + checkpoint
   → ⚙️ POST-COMMIT HOOK (mới):
        oumstudio/post-commit-hook.sh chạy với env:
          OMNI_CHAPTER, OMNI_CHAPTER_FILE, OMNI_OUTPUT_DIR, OMNI_SKILLS_DIR
        → oum-prose-verify  (checker chính, blacklist tiếng Anh)
        → forge-novel-guard (tham khảo em dash/markdown/AI)
        → novel-guardian    (scan 10 rule continuity)
        → novel-master      (check 5 lớp)
        → ghi meta/skill-audit/ch{N}.json
```

Hook **best-effort**: lỗi skill chỉ ghi vào report, KHÔNG chặn commit (chương đã lưu an toàn).

## Bật hook

Trong `~/.ainovel/config.json`:

```json
{
  "post_commit_hook": "/opt/omni/oumstudio/post-commit-hook.sh",
  "skills_dir": "/opt/omni/skills"
}
```

Để trống `post_commit_hook` = tắt hook (engine chạy như cũ).

## Image FULL (khuyến nghị)

Image `Dockerfile.omni` đã nhúng node + python + toàn bộ skill, hook chạy sẵn:

```bash
docker build -f Dockerfile.omni -t oumstudio-novel:omni .
```

Image engine gốc (`Dockerfile`, alpine) KHÔNG có node/python → hook chỉ chạy được các skill python, các skill node sẽ báo `available:false`.

## Report

Mỗi chương sinh 1 file `meta/skill-audit/ch{N}.json`:

```json
{
  "chapter": 1,
  "audited_at": "2026-06-29T08:34:53Z",
  "results": {
    "oum_prose_verify": {"available": true, "exit": 1, "parsed": {...}},
    "forge_novel_guard_prose": {"available": true, "note": "...", "output": "..."},
    "novel_guardian_scan": {"available": true, "output": "..."},
    "novel_master_check": {"available": true, "output": "..."}
  },
  "pass_all": true
}
```

`pass_all` = false chỉ khi oum-prose-verify phát hiện lỗi cứng THẬT (em dash / tiếng Anh); markdown `#` tiêu đề không tính (gỡ khi export EPUB).

## Lưu ý skill

`forge-novel-guard/verify-prose.py` dùng regex `[A-Za-z]{3,}` để dò tiếng Anh → báo nhầm tiếng Việt không dấu ("khi", "tay") và tên riêng ("Seiko"). Vì vậy hook chỉ lấy nó để tham khảo em dash/markdown/AI, KHÔNG tính vào `pass_all`. Checker chính là `oum-prose-verify` (dùng blacklist từ tiếng Anh phổ biến, chính xác).

# SKILLS.md — Bản đồ Omni Novel Suite

Bộ OUMStudio-Novel hợp nhất 1 engine autonomous + lớp prose + 10 skill quy trình thành một pipeline viết tiểu thuyết tiếng Việt trọn vẹn.

## Pipeline đề xuất

```
1. cw-brainstorming        → outline, beats, character arc, magic system
        ↓
2. cw-style-skill-creator  → tạo style guide (1 lần/project)
        ↓
3. ENGINE (oum-novel)      → chạy autonomous: Coordinator→Architect→Writer→Editor
   + cw-prose-writing      → văn phong theo style guide
   + oumstudio gate        → 0 em dash / 0 tiếng Anh
        ↓
4. novel-guardian scan     → 10 rule liền mạch (T/C/W/P/V/D), pacing, style
   novel-master check      → 5 lớp: bible/continuity/prose/pace/voice (30+ rule)
        ↓
5. cw-story-critique       → phê bình pacing/nhân vật/thoại
        ↓
6. forge-novel-guard       → verify prose/epub/send trước khi giao
        ↓
7. cw-official-docs        → cập nhật wiki nhân vật/địa danh/lore
        ↓
   EXPORT → EPUB premium / TXT
```

`cw-router` tự chọn skill phù hợp khi không chắc dùng cái nào.
`novelcore-ai` tối ưu cách viết prompt cho từng beat (4 khối, 3 bản, token budget, context phân vùng).

## Danh mục skill

| # | Skill | Loại | Vị trí |
|---|-------|------|--------|
| 0 | OUMStudio engine | Binary Go autonomous | `cmd/` + `internal/` + `oumstudio/` |
| 1 | cw-brainstorming | Prompt skill | `skills/cw-brainstorming/` |
| 2 | cw-style-skill-creator | Prompt skill | `skills/cw-style-skill-creator/` |
| 3 | cw-prose-writing | Prompt skill | `skills/cw-prose-writing/` |
| 4 | cw-story-critique | Prompt skill | `skills/cw-story-critique/` |
| 5 | cw-official-docs | Prompt skill | `skills/cw-official-docs/` |
| 6 | cw-router | Dispatcher | `skills/cw-router/` |
| 7 | novel-guardian | Node CLI engine | `skills/novel-guardian/` |
| 8 | novel-master | Node CLI engine | `skills/novel-master/` |
| 9 | novelcore-ai | Prompt + scripts | `skills/novelcore-ai/` |
| 10 | forge-novel-guard | Python verify | `skills/forge-novel-guard/` |

Mỗi skill có `SKILL.md` riêng mô tả chi tiết cách dùng.

### Skill scripts (v1.6.0)

7 script heuristics mới được wire vào post-commit hook — chạy kiểm tra thật thay vì chỉ prompt: `cw-brainstorming/scripts/tbd-scan.py`, `cw-style-skill-creator/scripts/style-fingerprint.py`, `cw-prose-writing/scripts/prose-metrics.py`, `cw-story-critique/scripts/critique-heuristics.py`, `cw-official-docs/scripts/wiki-extract.py`, `cw-router/scripts/route-check.py`, `novelcore-ai/scripts/novelcore-check.py`.

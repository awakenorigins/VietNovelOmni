#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""route-check.py — router self-test: suggest which OmniNovel skills should run.

Usage: route-check.py [--json] [--chapter N] [--words N | --file chapter.md]
                      [--has-outline yes|no] [--baseline prev-fingerprint.json]
Prints JSON routing suggestion. Exit 0 on success.
"""
import sys, os, json

def main():
    argv = sys.argv[1:]
    opts = {"chapter": 0, "words": 0, "file": None, "has_outline": None, "baseline": None}
    i = 0
    while i < len(argv):
        a = argv[i]
        if a == "--json":
            pass
        elif a == "--chapter":
            i += 1; opts["chapter"] = int(argv[i]) if i < len(argv) else 0
        elif a == "--words":
            i += 1; opts["words"] = int(argv[i]) if i < len(argv) else 0
        elif a == "--file":
            i += 1; opts["file"] = argv[i] if i < len(argv) else None
        elif a == "--has-outline":
            i += 1; opts["has_outline"] = (argv[i].lower() in ("yes", "true", "1")) if i < len(argv) else None
        elif a == "--baseline":
            i += 1; opts["baseline"] = argv[i] if i < len(argv) else None
        elif not a.startswith("--") and opts["file"] is None:
            opts["file"] = a
        i += 1

    if opts["file"] and os.path.isfile(opts["file"]) and not opts["words"]:
        try:
            with open(opts["file"], encoding="utf-8", errors="replace") as f:
                opts["words"] = len(f.read().split())
        except Exception:
            pass

    routes = []
    def add(skill, script, reason, priority="normal"):
        routes.append({"skill": skill, "script": script, "reason": reason, "priority": priority})

    # always-on verifiers for any committed chapter
    add("oumstudio-novel", "oum-prose-verify.py", "verify chính (blacklist/hard errors) — quyết định pass_all", "required")
    add("novelcore-ai", "novelcore-check.py", "check blueprint + AI-sáo cho mọi chương", "high")
    add("cw-prose-writing", "prose-metrics.py", "thống kê văn phong mọi chương", "normal")
    add("cw-story-critique", "critique-heuristics.py", "heuristic hook/mở chương/telling", "normal")
    add("cw-brainstorming", "tbd-scan.py", "quét placeholder còn sót", "high")
    add("cw-official-docs", "wiki-extract.py", "trích tên riêng cập nhật wiki", "low")

    if opts["has_outline"] is False:
        add("cw-brainstorming", "SKILL.md (prompt)", "chưa có outline → cần brainstorm outline trước khi viết tiếp", "high")

    ch = opts["chapter"]
    if ch and ch > 1:
        add("cw-style-skill-creator", "style-fingerprint.py --baseline <ch%02d.json>" % (ch - 1),
            "so drift văn phong với chương trước", "normal")
    else:
        add("cw-style-skill-creator", "style-fingerprint.py", "tạo baseline fingerprint chương đầu", "normal")

    w = opts["words"]
    warnings = []
    if w and w < 800:
        warnings.append("chương ngắn (%d từ) — cân nhắc kiểm tra độ đầy đủ nội dung" % w)
    if w and w > 6000:
        warnings.append("chương rất dài (%d từ) — cân nhắc tách chương" % w)

    report = {
        "skill": "cw-router",
        "input": {"chapter": ch, "words": w, "file": opts["file"],
                  "has_outline": opts["has_outline"], "baseline": opts["baseline"]},
        "routes": routes,
        "warnings": warnings,
        "issues": [],
        "ok": True,
    }
    print(json.dumps(report, ensure_ascii=False, indent=2))
    return 0

if __name__ == "__main__":
    sys.exit(main())

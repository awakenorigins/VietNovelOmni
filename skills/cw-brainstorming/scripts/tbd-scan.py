#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""tbd-scan.py — scan for leftover [TBD]/TODO/placeholders (advisory).

Usage: tbd-scan.py [--json] <chapter-or-outline.md> [more files...]
"""
import sys, os, json, re

PATTERNS = [
    (r'\[TBD[^\]]*\]', "TBD"),
    (r'\bTODO\b', "TODO"),
    (r'\bFIXME\b', "FIXME"),
    (r'\bXXX\b', "XXX"),
    (r'\[placeholder[^\]]*\]', "placeholder"),
    (r'\bplaceholder\b', "placeholder"),
    (r'<<[^>]{0,60}>>', "angle-placeholder"),
    (r'\{\{[^}]{0,60}\}\}', "brace-placeholder"),
    (r'\.\.\.\s*\(.{0,40}(sau|thêm|viết tiếp).{0,40}\)', "deferred-note"),
    (r'\[(chưa|cần)[^\]]{0,60}\]', "vi-note"),
]

def main():
    files = [a for a in sys.argv[1:] if a != "--json"]
    if not files:
        print(json.dumps({"skill": "cw-brainstorming", "error": "usage: tbd-scan.py [--json] <file.md> [...]"}, ensure_ascii=False))
        return 2
    findings = []
    scanned = []
    for path in files:
        if not os.path.isfile(path):
            findings.append({"file": path, "line": 0, "type": "missing-file", "text": "file not found"})
            continue
        scanned.append(path)
        with open(path, encoding="utf-8", errors="replace") as f:
            for ln, line in enumerate(f, 1):
                for pat, kind in PATTERNS:
                    for m in re.finditer(pat, line, flags=re.I):
                        findings.append({"file": path, "line": ln, "type": kind,
                                         "text": line.strip()[:200]})
    # dedupe identical file/line entries
    seen = set()
    uniq = []
    for x in findings:
        k = (x["file"], x["line"], x["type"])
        if k not in seen:
            seen.add(k)
            uniq.append(x)
    report = {
        "skill": "cw-brainstorming",
        "files_scanned": scanned,
        "placeholders_found": len(uniq),
        "findings": uniq,
        "issues": ["%s:%d [%s] %s" % (f["file"], f["line"], f["type"], f["text"][:80]) for f in uniq],
        "ok": not uniq,
    }
    print(json.dumps(report, ensure_ascii=False, indent=2))
    return 0

if __name__ == "__main__":
    sys.exit(main())

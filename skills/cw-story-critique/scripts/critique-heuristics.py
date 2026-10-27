#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""critique-heuristics.py — story critique heuristics (advisory).

Usage: critique-heuristics.py [--json] <chapter.md>
"""
import sys, os, json, re

TELLING_WORDS = ["cảm thấy", "nhận ra rằng", "biết rằng", "hiểu rằng", "cảm nhận được"]
WEAK_OPENERS = [
    r'^\s*#*\s*đêm\b', r'^\s*#*\s*màn đêm\b', r'^\s*#*\s*sáng sớm\b', r'^\s*#*\s*buổi sáng\b',
    r'tỉnh dậy', r'giật mình tỉnh', r'mở mắt',
]

def main():
    args = [a for a in sys.argv[1:] if a != "--json"]
    if not args:
        print(json.dumps({"skill": "cw-story-critique", "error": "usage: critique-heuristics.py [--json] <chapter.md>"}, ensure_ascii=False))
        return 2
    path = args[0]
    if not os.path.isfile(path):
        print(json.dumps({"skill": "cw-story-critique", "error": "file not found: %s" % path}, ensure_ascii=False))
        return 2
    with open(path, encoding="utf-8", errors="replace") as f:
        text = f.read()

    issues = []
    low = text.lower()
    paragraphs = [p.strip() for p in re.split(r'\n\s*\n', text) if p.strip()]
    # skip heading-only paragraphs for body analysis
    body = [p for p in paragraphs if not re.match(r'^#{1,6}\s', p)]

    # --- ending hook ---
    last = body[-1] if body else ""
    has_question = "?" in last
    has_dialogue = bool(re.search(r'["“”]', last)) or bool(re.match(r'^\s*[\-–—]\s', last))
    crisis_words = ["đột nhiên", "bỗng", "chợt", "vang lên", "gõ cửa", "máu", "hét", "thét",
                    "biến mất", "không còn", "sụp", "vỡ", "nổ", "chết", "cửa mở", "bóng người"]
    has_crisis = any(w in last.lower() for w in crisis_words)
    hook_ok = has_question or has_dialogue or has_crisis
    if not hook_ok:
        issues.append("đoạn cuối chương thiếu hook (không có câu hỏi/khủng hoảng/lời thoại)")

    # --- weak chapter opening ---
    first_para = body[0].lower() if body else ""
    first_slice = first_para[:200]
    weak_open = None
    for pat in WEAK_OPENERS:
        if re.search(pat, first_slice):
            weak_open = pat
            issues.append("mở chương kiểu sáo (đêm/sáng sớm/tỉnh dậy): khớp mẫu %r" % pat)
            break

    # --- telling density ---
    telling = {}
    telling_total = 0
    for w in TELLING_WORDS:
        c = low.count(w)
        if c:
            telling[w] = c
            telling_total += c
    if telling_total > 5:
        issues.append("mật độ 'tóm-kể' (telling) cao: %d từ cảm xúc dán nhãn (>5)" % telling_total)

    report = {
        "skill": "cw-story-critique",
        "file": path,
        "ending_hook": {"has_question": has_question, "has_dialogue": has_dialogue,
                        "has_crisis": has_crisis, "ok": hook_ok},
        "weak_opening_pattern": weak_open,
        "telling_words": telling,
        "telling_total": telling_total,
        "issues": issues,
        "ok": not issues,
    }
    print(json.dumps(report, ensure_ascii=False, indent=2))
    return 0

if __name__ == "__main__":
    sys.exit(main())

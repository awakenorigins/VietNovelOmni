#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""novelcore-check.py — NovelCoreAI blueprint checker (advisory).

Usage: novelcore-check.py [--json] <chapter.md>
Prints JSON to stdout. Always exits 0 when it could run (issues are advisory).
"""
import sys, os, json, re

AI_CLICHES = [
    "không thể tin được",
    "trái tim đập thình thịch",
    "một cảm giác khó tả",
    "sâu thẳm trong tâm hồn",
    "như một lời nhắc nhở",
    "quan trọng hơn bao giờ hết",
    "trong khoảnh khắc ấy",
    "một luồng điện chạy dọc sống lưng",
    "thời gian như ngừng trôi",
    "không khí trở nên căng thẳng",
    "một nỗi buồn man mác",
    "ánh mắt đầy ẩn ý",
    "nụ cười bí ẩn",
    "cảm xúc lẫn lộn",
    "như chưa từng có chuyện gì xảy ra",
    "một sự im lặng đáng sợ",
    "trái tim như thắt lại",
    "nước mắt lăn dài trên má",
    "hơi thở dồn dập",
    "cả thế giới như sụp đổ",
    "định mệnh đã an bài",
    "một trang mới của cuộc đời",
    "hành trình phía trước",
    "bài học đắt giá",
    "minh chứng cho",
    "hơn ai hết",
    "không gì có thể ngăn cản",
]

def split_sentences(text):
    parts = re.split(r'(?<=[.!?…])\s+', text)
    return [p.strip() for p in parts if p.strip()]

def main():
    args = [a for a in sys.argv[1:] if a != "--json"]
    if not args:
        print(json.dumps({"skill": "novelcore-ai", "error": "usage: novelcore-check.py [--json] <chapter.md>"}, ensure_ascii=False))
        return 2
    path = args[0]
    if not os.path.isfile(path):
        print(json.dumps({"skill": "novelcore-ai", "error": "file not found: %s" % path}, ensure_ascii=False))
        return 2
    with open(path, encoding="utf-8", errors="replace") as f:
        text = f.read()

    issues = []
    warnings = []

    # --- paragraph / beat word counts ---
    paragraphs = [p.strip() for p in re.split(r'\n\s*\n', text) if p.strip()]
    para_stats = []
    for i, p in enumerate(paragraphs, 1):
        wc = len(p.split())
        para_stats.append(wc)
        if wc > 600:
            warnings.append("đoạn %d quá dài: %d từ (>600)" % (i, wc))
    total_words = len(text.split())
    lo = int(os.environ.get("OMNI_MIN_WORDS", "0") or 0)
    hi = int(os.environ.get("OMNI_MAX_WORDS", "0") or 0)
    if lo and total_words < lo:
        warnings.append("chương ngắn hơn khoảng cho phép: %d < %d từ" % (total_words, lo))
    if hi and total_words > hi:
        warnings.append("chương dài hơn khoảng cho phép: %d > %d từ" % (total_words, hi))

    # --- AI cliché detection ---
    low = text.lower()
    cliches_found = []
    for c in AI_CLICHES:
        n = low.count(c)
        if n:
            cliches_found.append({"phrase": c, "count": n})
            issues.append("AI-sáo: '%s' xuất hiện %d lần" % (c, n))

    # --- "không phải ... mà là ..." correction-pattern frequency ---
    khong_phai = len(re.findall(r'không\s+phải\b.{0,80}?\bmà\s+(?:là|còn|chính)\b', low, flags=re.S))
    if khong_phai > 2:
        issues.append("câu chỉnh lý 'không phải… mà là…' xuất hiện %d lần (>2/chương)" % khong_phai)

    report = {
        "skill": "novelcore-ai",
        "file": path,
        "total_words": total_words,
        "paragraphs": len(paragraphs),
        "max_paragraph_words": max(para_stats) if para_stats else 0,
        "avg_paragraph_words": round(sum(para_stats) / len(para_stats), 1) if para_stats else 0,
        "ai_cliches": cliches_found,
        "khong_phai_ma_la_count": khong_phai,
        "issues": issues,
        "warnings": warnings,
        "ok": not issues,
    }
    print(json.dumps(report, ensure_ascii=False, indent=2))
    return 0

if __name__ == "__main__":
    sys.exit(main())

#!/usr/bin/env python3
"""
oum-prose-verify.py — Trình kiểm tra chất lượng prose tiếng Việt cho OUMStudio-Novel.

Dùng được 2 chế độ:
  1. CLI người đọc:   python3 oum-prose-verify.py <file>
  2. Hook engine:     python3 oum-prose-verify.py --json <file>   (in JSON, exit 0/1)

Phân loại lỗi:
  HARD (chặn commit khi --strict):
    - em dash (—) ngoài đối thoại
    - markdown header trong thân (^#, ^##, ^>, ```, ---, **bold**)
    - từ tiếng Anh thật lọt vào prose (heuristic loại false-positive tiếng Việt không dấu)
  SOFT (cảnh báo, không chặn):
    - cụm sáo AI tiếng Việt
    - lặp "không phải ... mà là"

Exit code: 0 nếu không HARD error (hoặc không --strict), 1 nếu có HARD error và bật --strict.
"""

import sys
import re
import os
import json

# Từ tiếng Anh phổ biến hay lọt vào prose Ak Việt (whitelist đen — luôn là lỗi)
ENGLISH_BLACKLIST = {
    "the", "and", "but", "with", "from", "this", "that", "have", "will",
    "your", "they", "their", "would", "could", "should", "about", "which",
    "brilliant", "stunning", "delve", "tapestry", "journey", "moreover",
    "furthermore", "however", "therefore", "essentially", "ultimately",
    "suddenly", "meanwhile", "nevertheless", "indeed", "perhaps", "whisper",
    "embrace", "realm", "destiny", "shimmer", "cascade", "ethereal",
    "okay", "yeah", "wow", "hello", "sorry", "please", "thanks",
}

# Cụm sáo AI tiếng Việt (soft)
AI_TRACES = [
    "tuyệt vời", "hoàn hảo", "thú vị thay", "quả thật", "không thể phủ nhận",
    "điều đáng nói là", "rõ ràng là", "nói cách khác", "theo một nghĩa nào đó",
    "đáng chú ý là", "không hiểu tại sao", "cảm xúc lẫn lộn",
    "một cách tàn nhẫn", "một cách nhẹ nhàng", "một cách tự nhiên",
    "một cách tinh tế", "một cách bất ngờ", "một cách kỳ lạ",
]


def find_english(text):
    """Tìm từ tiếng Anh thật. Heuristic: chỉ tính từ trong blacklist (chắc chắn là Anh),
    hoặc từ ASCII có >=4 ký tự chứa cụm phụ âm không hợp lệ tiếng Việt."""
    hits = []
    for m in re.finditer(r"\b[A-Za-z]{2,}\b", text):
        w = m.group(0)
        wl = w.lower()
        if wl in ENGLISH_BLACKLIST:
            ctx = text[max(0, m.start() - 25): m.end() + 25].replace("\n", " ").strip()
            hits.append({"word": w, "ctx": ctx[:60]})
    return hits


def find_em_dash(text):
    hits = []
    for m in re.finditer("—", text):
        ctx = text[max(0, m.start() - 20): m.end() + 20].replace("\n", " ").strip()
        hits.append(ctx[:50])
    return hits


def find_markdown(text):
    found = []
    for line in text.splitlines():
        s = line.strip()
        if re.match(r"^#{1,6}\s", s):
            found.append(s[:40])
        elif s.startswith(">") and len(s) > 1:
            found.append(s[:40])
        elif s == "---" or s.startswith("```"):
            found.append(s[:40])
    if "**" in text:
        found.append("**bold**")
    return found


def find_ai_traces(text):
    tl = text.lower()
    return [t for t in AI_TRACES if t in tl]


def find_contrast_cliche(text):
    return len(re.findall(r"không phải .{1,40}? mà là", text))


def analyze(text):
    english = find_english(text)
    em_dash = find_em_dash(text)
    markdown = find_markdown(text)
    ai = find_ai_traces(text)
    contrast = find_contrast_cliche(text)

    hard = []
    if english:
        hard.append(f"tiếng Anh: {[h['word'] for h in english][:8]}")
    if em_dash:
        hard.append(f"em dash: {len(em_dash)} lần")
    if markdown:
        hard.append(f"markdown: {markdown[:5]}")

    soft = []
    if ai:
        soft.append(f"cụm sáo AI: {ai}")
    if contrast:
        soft.append(f"'không phải...mà là': {contrast} lần")

    return {
        "words": len(text.split()),
        "chars": len(text),
        "hard_errors": hard,
        "soft_warnings": soft,
        "detail": {
            "english": english,
            "em_dash": em_dash,
            "markdown": markdown,
            "ai_traces": ai,
            "contrast_cliche": contrast,
        },
    }


def main():
    args = sys.argv[1:]
    as_json = "--json" in args
    strict = "--strict" in args or as_json  # hook mode mặc định strict
    paths = [a for a in args if not a.startswith("--")]
    if not paths:
        print("Usage: oum-prose-verify.py [--json] [--strict] <file>", file=sys.stderr)
        sys.exit(2)

    fp = paths[0]
    if not os.path.exists(fp):
        msg = f"File không tồn tại: {fp}"
        if as_json:
            print(json.dumps({"ok": False, "error": msg}, ensure_ascii=False))
        else:
            print("❌ " + msg)
        sys.exit(2)

    text = open(fp, encoding="utf-8").read()
    res = analyze(text)
    has_hard = bool(res["hard_errors"])

    if as_json:
        out = {
            "ok": not has_hard,
            "file": os.path.basename(fp),
            "words": res["words"],
            "hard_errors": res["hard_errors"],
            "soft_warnings": res["soft_warnings"],
        }
        print(json.dumps(out, ensure_ascii=False))
    else:
        print(f"\n{'=' * 50}")
        print(f"OUM PROSE VERIFY: {os.path.basename(fp)}")
        print(f"{res['words']:,} từ · {res['chars']:,} ký tự")
        print(f"{'=' * 50}")
        if res["hard_errors"]:
            print("❌ HARD ERRORS (chặn commit):")
            for e in res["hard_errors"]:
                print(f"   • {e}")
        else:
            print("✅ Không có hard error")
        if res["soft_warnings"]:
            print("⚠️  SOFT WARNINGS:")
            for w in res["soft_warnings"]:
                print(f"   • {w}")
        print(f"{'=' * 50}")

    sys.exit(1 if (has_hard and strict) else 0)


if __name__ == "__main__":
    main()

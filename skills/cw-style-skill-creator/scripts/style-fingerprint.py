#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""style-fingerprint.py — build a style fingerprint JSON; optionally compare to baseline.

Usage: style-fingerprint.py [--json] [--baseline prev.json] <chapter.md>
"""
import sys, os, json, re
from collections import Counter

STOP = set("và của là một những các có không được cho với tôi anh em cô ông bà nó họ này đó khi đã sẽ đang thì mà ở ra vào lại cũng như trong trên dưới về từ đến nhưng nếu vì nên rồi còn chỉ rất".split())

def split_sentences(text):
    parts = re.split(r'(?<=[.!?…])\s+', text)
    return [p.strip() for p in parts if p.strip()]

def fingerprint(text):
    words = [w.lower() for w in re.findall(r'[\wÀ-ỹ]+', text)]
    content = [w for w in words if w not in STOP and len(w) > 1 and not w.isdigit()]
    top = Counter(content).most_common(20)
    sentences = split_sentences(text)
    lens = [len(s.split()) for s in sentences]
    avg_len = round(sum(lens) / len(lens), 2) if lens else 0
    # punctuation profile per 1000 chars
    nchars = max(1, len(text))
    punct = {}
    for ch, name in [(",", "comma"), (".", "period"), ("?", "question"), ("!", "exclaim"),
                     ("…", "ellipsis"), (";", "semicolon"), (":", "colon"), ("—", "emdash"),
                     ('"', "dquote"), ("“", "lquote")]:
        punct[name] = round(text.count(ch) * 1000 / nchars, 2)
    return {
        "total_words": len(words),
        "sentences": len(sentences),
        "avg_sentence_words": avg_len,
        "top_words": [{"word": w, "count": c} for w, c in top],
        "punctuation_per_1000_chars": punct,
    }

def drift_pct(a, b):
    """Relative drift % between two numbers."""
    if a == 0 and b == 0: return 0.0
    base = max(abs(a), abs(b), 1e-9)
    return round(abs(a - b) * 100 / base, 1)

def main():
    argv = sys.argv[1:]
    baseline_path = None
    files = []
    i = 0
    while i < len(argv):
        if argv[i] == "--json":
            pass
        elif argv[i] == "--baseline":
            i += 1
            baseline_path = argv[i] if i < len(argv) else None
        else:
            files.append(argv[i])
        i += 1
    if not files:
        print(json.dumps({"skill": "cw-style-skill-creator", "error": "usage: style-fingerprint.py [--json] [--baseline prev.json] <chapter.md>"}, ensure_ascii=False))
        return 2
    path = files[0]
    if not os.path.isfile(path):
        print(json.dumps({"skill": "cw-style-skill-creator", "error": "file not found: %s" % path}, ensure_ascii=False))
        return 2
    with open(path, encoding="utf-8", errors="replace") as f:
        text = f.read()
    fp = fingerprint(text)
    report = {"skill": "cw-style-skill-creator", "file": path, "fingerprint": fp, "issues": [], "ok": True}

    if baseline_path:
        if os.path.isfile(baseline_path):
            try:
                with open(baseline_path, encoding="utf-8") as f:
                    base = json.load(f)
                bfp = base.get("fingerprint", base)
                d_len = drift_pct(fp["avg_sentence_words"], bfp.get("avg_sentence_words", 0))
                a_top = set(x["word"] for x in fp["top_words"][:15])
                b_top = set(x["word"] for x in bfp.get("top_words", [])[:15])
                overlap = len(a_top & b_top) / max(1, len(a_top | b_top))
                d_vocab = round((1 - overlap) * 100, 1)
                pa = fp["punctuation_per_1000_chars"]
                pb = bfp.get("punctuation_per_1000_chars", {})
                keys = set(pa) | set(pb)
                d_punct = round(sum(drift_pct(pa.get(k, 0), pb.get(k, 0)) for k in keys) / max(1, len(keys)), 1)
                report["drift"] = {
                    "baseline": baseline_path,
                    "avg_sentence_len_drift_pct": d_len,
                    "top_vocab_drift_pct": d_vocab,
                    "punctuation_drift_pct": d_punct,
                }
                if d_len > 40:
                    report["issues"].append("độ dài câu trung bình lệch %.1f%% so với baseline" % d_len)
                if d_vocab > 80:
                    report["issues"].append("từ vựng top lệch %.1f%% so với baseline" % d_vocab)
                report["ok"] = not report["issues"]
            except Exception as e:
                report["drift"] = {"baseline": baseline_path, "error": str(e)}
        else:
            report["drift"] = {"baseline": baseline_path, "error": "baseline not found"}
    print(json.dumps(report, ensure_ascii=False, indent=2))
    return 0

if __name__ == "__main__":
    sys.exit(main())

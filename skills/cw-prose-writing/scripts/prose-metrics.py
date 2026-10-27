#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""prose-metrics.py — prose style statistics (advisory).

Usage: prose-metrics.py [--json] <chapter.md>
"""
import sys, os, json, re

def split_sentences(text):
    parts = re.split(r'(?<=[.!?…])\s+', text)
    return [p.strip() for p in parts if p.strip()]

def main():
    args = [a for a in sys.argv[1:] if a != "--json"]
    if not args:
        print(json.dumps({"skill": "cw-prose-writing", "error": "usage: prose-metrics.py [--json] <chapter.md>"}, ensure_ascii=False))
        return 2
    path = args[0]
    if not os.path.isfile(path):
        print(json.dumps({"skill": "cw-prose-writing", "error": "file not found: %s" % path}, ensure_ascii=False))
        return 2
    with open(path, encoding="utf-8", errors="replace") as f:
        text = f.read()

    issues = []

    sentences = split_sentences(text)
    lens = [len(s.split()) for s in sentences]
    n = len(lens)
    avg = round(sum(lens) / n, 1) if n else 0
    dist = {"short_<=8": 0, "medium_9_20": 0, "long_21_35": 0, "very_long_>35": 0}
    for L in lens:
        if L <= 8: dist["short_<=8"] += 1
        elif L <= 20: dist["medium_9_20"] += 1
        elif L <= 35: dist["long_21_35"] += 1
        else: dist["very_long_>35"] += 1

    # dialogue vs narration ratio (by lines containing quotes or dash-dialogue)
    lines = [l for l in text.splitlines() if l.strip()]
    dlg_re = re.compile(r'^[\-–—]\s|["“”"\']')
    dlg_words = 0
    nar_words = 0
    for l in lines:
        w = len(l.split())
        if re.search(r'["“”]', l) or re.match(r'^\s*[\-–—]\s', l):
            dlg_words += w
        else:
            nar_words += w
    total = dlg_words + nar_words
    dialogue_ratio = round(dlg_words / total, 3) if total else 0.0

    # repeated sentence openers (>=3 consecutive sentences starting with same word)
    openers = []
    for s in sentences:
        m = re.match(r'^[\W_]*([\wÀ-ỹ]+)', s)
        openers.append(m.group(1).lower() if m else "")
    repeats = []
    i = 0
    while i < len(openers):
        j = i
        while j + 1 < len(openers) and openers[j + 1] == openers[i] and openers[i]:
            j += 1
        run = j - i + 1
        if run >= 3:
            repeats.append({"word": openers[i], "count": run, "sentence_index": i + 1})
            issues.append("%d câu liên tiếp mở đầu bằng '%s' (từ câu %d)" % (run, openers[i], i + 1))
        i = j + 1

    # "-một cách" adverb ratio
    mot_cach = len(re.findall(r'một\s+cách\s+[\wÀ-ỹ]+', text, flags=re.I))
    mot_cach_per_1k = round(mot_cach * 1000 / max(1, len(text.split())), 2)
    if mot_cach_per_1k > 3:
        issues.append("lạm dụng trạng từ 'một cách …': %d lần (%.2f/1000 từ)" % (mot_cach, mot_cach_per_1k))

    report = {
        "skill": "cw-prose-writing",
        "file": path,
        "sentences": n,
        "avg_sentence_words": avg,
        "sentence_length_distribution": dist,
        "dialogue_ratio": dialogue_ratio,
        "narration_ratio": round(1 - dialogue_ratio, 3) if total else 0.0,
        "repeated_openers": repeats,
        "mot_cach_count": mot_cach,
        "mot_cach_per_1000_words": mot_cach_per_1k,
        "issues": issues,
        "ok": not issues,
    }
    print(json.dumps(report, ensure_ascii=False, indent=2))
    return 0

if __name__ == "__main__":
    sys.exit(main())

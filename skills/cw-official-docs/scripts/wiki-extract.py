#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""wiki-extract.py — extract capitalized proper-noun candidates (>=2 occurrences) for wiki update.

Usage: wiki-extract.py [--json] <chapter.md>
"""
import sys, os, json, re
from collections import Counter

# Common Vietnamese sentence-initial words that are not names
COMMON = set("""Tôi Anh Em Cô Chú Bác Ông Bà Nó Họ Chúng Ta Mình Người Cả Có Không Nhưng Và Rồi Khi Nếu Vì Thế Sau Trước Một Hai Ba Bốn Năm Sáu Bảy Tám Chín Mười Đó Đây Này Kia Ngày Đêm Sáng Chiều Tối Trưa Hôm Buổi Lúc Bây Giờ Chuyện Điều Việc Cái Con Chiếc Căn Ngôi Cửa Nhà Đường Phố Trong Ngoài Trên Dưới Giữa Bên Cạnh Từ Đến Về Theo Cùng Với Của Cho Được Bị Đã Sẽ Đang Vẫn Còn Cũng Chỉ Rất Quá Lại Ra Vào Lên Xuống Đi Chạy Nói Hỏi Nghe Nhìn Thấy Biết Nghĩ Nhớ Quên Muốn Cần Phải Nên Thì Mà Ở Như Hay Hoặc Dù Tuy Mặc Bởi Do Tại Sao Gì Ai Nào Đâu Bao Nhiêu Mấy Thứ Lần Chương""".split())

def main():
    args = [a for a in sys.argv[1:] if a != "--json"]
    if not args:
        print(json.dumps({"skill": "cw-official-docs", "error": "usage: wiki-extract.py [--json] <chapter.md>"}, ensure_ascii=False))
        return 2
    path = args[0]
    if not os.path.isfile(path):
        print(json.dumps({"skill": "cw-official-docs", "error": "file not found: %s" % path}, ensure_ascii=False))
        return 2
    with open(path, encoding="utf-8", errors="replace") as f:
        text = f.read()

    # strip markdown headings to reduce noise
    body = re.sub(r'^#{1,6}\s.*$', '', text, flags=re.M)

    UP = 'A-ZĐÀÁẢÃẠĂẰẮẲẴẶÂẦẤẨẪẬÈÉẺẼẸÊỀẾỂỄỆÌÍỈĨỊÒÓỎÕỌÔỒỐỔỖỘƠỜỚỞỠỢÙÚỦŨỤƯỪỨỬỮỰỲÝỶỸỴ'
    LOW = 'a-zđàáảãạăằắẳẵặâầấẩẫậèéẻẽẹêềếểễệìíỉĩịòóỏõọôồốổỗộơờớởỡợùúủũụưừứửữựỳýỷỹỵ'
    # multi-word capitalized sequences (e.g. "Trần Vỹ", "Hà Nội") or single capitalized words
    token = r'[%s][%s]+' % (UP, LOW)
    seq_re = re.compile(r'\b(%s(?:\s+%s){0,3})\b' % (token, token))

    counts = Counter()
    for m in seq_re.finditer(body):
        name = m.group(1)
        words = name.split()
        # drop leading common words (often sentence-initial)
        while words and words[0] in COMMON:
            words = words[1:]
        if not words:
            continue
        name = " ".join(words)
        if len(words) == 1 and (name in COMMON or len(name) < 2):
            continue
        # single sentence-initial word: only count if it also appears mid-sentence
        counts[name] += 1

    # filter: single words must appear mid-sentence at least once to count as name
    def mid_sentence(name):
        for m in re.finditer(re.escape(name), body):
            i = m.start()
            prev = body[:i].rstrip()
            if prev and prev[-1] not in '.!?…"\n“':
                return True
        return False

    candidates = []
    for name, c in counts.most_common():
        if c < 2:
            continue
        if " " not in name and not mid_sentence(name):
            continue
        kind = "person_or_place"
        candidates.append({"name": name, "count": c, "kind": kind,
                           "suggestion": "kiểm tra/cập nhật trang wiki cho '%s'" % name})

    report = {
        "skill": "cw-official-docs",
        "file": path,
        "candidates": candidates,
        "candidate_count": len(candidates),
        "issues": [],
        "ok": True,
    }
    print(json.dumps(report, ensure_ascii=False, indent=2))
    return 0

if __name__ == "__main__":
    sys.exit(main())

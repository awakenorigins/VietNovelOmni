#!/usr/bin/env python3
"""
verify-prose.py — Scan prose tiếng Việt tìm lỗi chất lượng
Chạy: python3 verify-prose.py <file.md|file.txt>

Kiểm tra:
1. Tiếng Anh lọt vào
2. Markdown artifacts
3. Em dash (—)
4. Dấu vết AI (bảng đỏ 12 cụm)
5. Xưng "tôi" sai ngôi (trong Lời Tác Giả / Lời Bạt)
"""

import sys, re, os

# === CONFIG ===
VN_FALSE_POSITIVES = {
    'trong', 'trang', 'nguy', 'trung', 'ngay', 'khong', 'cung',
    'chang', 'chung', 'thanh', 'ngang', 'vang', 'lung', 'bang',
    'dung', 'tang', 'sang', 'mang', 'lang', 'hang', 'rang',
    'gang', 'phan', 'than', 'chan', 'nhan', 'quan', 'khoan',
    'thong', 'hong', 'dong', 'long', 'song', 'cong', 'phong',
    'nhanh', 'loang', 'quanh', 'khinh', 'hui', 'the', 'then',
    'back', 'can', 'cat', 'con', 'dam', 'dan', 'dat', 'tao',
    'bay', 'lay', 'may', 'hay', 'day', 'say', 'pay', 'ray',
    'van', 'ban', 'tan', 'man', 'nan', 'san', 'ran', 'han',
    'ang', 'ong', 'eng', 'ung', 'anh', 'inh', 'unh',
}

AI_TRACES = [
    'tuyệt vời', 'hoàn hảo', 'thú vị thay', 'quả thật', 'đúng vậy',
    'tất nhiên rồi', 'không thể phủ nhận', 'điều đáng nói là',
    'rõ ràng là', 'quan trọng hơn', 'nói cách khác',
    'một cách tàn nhẫn', 'một cách nhẹ nhàng', 'một cách tự nhiên',
    'một cách tinh tế', 'một cách bất ngờ', 'một cách kỳ lạ',
]

MARKDOWN_PATTERNS = ['**', '## ', '# ', '---', '___', '```', '> ']

def scan_english(text):
    """Tìm từ tiếng Anh 3+ ký tự trong văn tiếng Việt"""
    candidates = re.findall(r'\b[A-Za-z]{3,}\b', text)
    real = [w for w in candidates if w.lower() not in VN_FALSE_POSITIVES]
    if real:
        unique = set(real)
        print(f"  ❌ Tiếng Anh ({len(unique)} từ): {unique}")
        for word in list(unique)[:5]:
            for m in re.finditer(re.escape(word), text):
                ctx = text[max(0, m.start()-30):m.end()+30].strip()
                print(f"     '{word}' → ...{ctx[:70]}...")
                break
        return False
    print("  ✅ Không có tiếng Anh")
    return True

def scan_markdown(text):
    """Tìm markdown artifacts trong prose"""
    found = []
    for p in MARKDOWN_PATTERNS:
        if p in text:
            found.append(p)
    if found:
        print(f"  ❌ Markdown: {found}")
        return False
    print("  ✅ Không có markdown")
    return True

def scan_em_dash(text):
    """Tìm em dash (—) trong prose"""
    count = text.count('—')
    if count > 0:
        print(f"  ❌ Em dash: {count} lần")
        for m in list(re.finditer('—', text))[:3]:
            ctx = text[max(0, m.start()-25):m.end()+25].strip()
            print(f"     → ...{ctx[:60]}...")
        return False
    print("  ✅ Không có em dash")
    return True

def scan_ai_traces(text):
    """Tìm dấu vết AI (bảng đỏ)"""
    found = []
    text_lower = text.lower()
    for trace in AI_TRACES:
        count = text_lower.count(trace.lower())
        if count > 0:
            found.append(f"'{trace}': {count}x")
    if found:
        print(f"  ⚠️ AI traces ({len(found)}): {', '.join(found)}")
        return True  # Warning only, not fail
    print("  ✅ Không có dấu vết AI")
    return True

def check_author_voice(text):
    """Kiểm tra 'tôi' trong phần tác giả"""
    sections = ['Lời Tác Giả', 'Lời Bạt', 'Về Tác Giả']
    issues = []
    for section_name in sections:
        start = text.find(section_name)
        if start < 0:
            continue
        section = text[start:start+2000]
        toi_count = len(re.findall(r'\btôi\b', section, re.IGNORECASE))
        if toi_count > 0:
            issues.append(f"'{section_name}': {toi_count} lần 'tôi'")
    if issues:
        print(f"  ❌ Xưng 'tôi' sai ngôi: {', '.join(issues)}")
        return False
    print("  ✅ Không xưng 'tôi' sai ngôi")
    return True

def main():
    if len(sys.argv) < 2:
        print("Usage: python3 verify-prose.py <file.md|file.txt>")
        sys.exit(1)

    filepath = sys.argv[1]
    if not os.path.exists(filepath):
        print(f"❌ File không tồn tại: {filepath}")
        sys.exit(1)

    with open(filepath, 'r', encoding='utf-8') as f:
        text = f.read()

    print(f"\n{'='*50}")
    print(f"VERIFY PROSE: {os.path.basename(filepath)}")
    print(f"Kích thước: {len(text):,} chars, ~{len(text.split()):,} từ")
    print(f"{'='*50}\n")

    results = []
    results.append(("Tiếng Anh", scan_english(text)))
    results.append(("Markdown", scan_markdown(text)))
    results.append(("Em dash", scan_em_dash(text)))
    results.append(("AI traces", scan_ai_traces(text)))
    results.append(("Xưng ngôi", check_author_voice(text)))

    fails = sum(1 for _, ok in results if not ok)

    print(f"\n{'='*50}")
    if fails == 0:
        print("🎉 ALL CHECKS PASSED!")
    else:
        print(f"❌ {fails} CHECK(S) FAILED — fix rồi chạy lại")
    print(f"{'='*50}")

    sys.exit(0 if fails == 0 else 1)

if __name__ == '__main__':
    main()

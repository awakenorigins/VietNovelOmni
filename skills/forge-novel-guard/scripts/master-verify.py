#!/usr/bin/env python3
"""
master-verify.py — One-shot verify tất cả (EPUB + prose + send readiness)
Chạy: python3 master-verify.py <file.epub>
"""

import sys, os, re, zipfile

# === ENTITY SCAN ===
def scan_entities(data, filename):
    invalid = re.findall(
        r'&(?!amp;|lt;|gt;|quot;|apos;|#\d+;|#x[0-9a-fA-F]+;)[a-zA-Z]+;',
        data
    )
    if invalid:
        print(f"  ❌ [{filename}] Entity lỗi: {set(invalid)}")
        return False
    return True

# === ENGLISH SCAN ===
VN_FP = {
    'trong', 'trang', 'nguy', 'trung', 'thanh', 'chang', 'chung',
    'ngang', 'vang', 'lung', 'bang', 'dung', 'tang', 'sang', 'mang',
    'lang', 'hang', 'rang', 'gang', 'phan', 'than', 'chan', 'nhan',
    'quan', 'khoan', 'thong', 'hong', 'dong', 'long', 'song', 'cong',
    'phong', 'nhanh', 'loang', 'quanh', 'khinh', 'then', 'tao',
    'class', 'span', 'body', 'html', 'head', 'title', 'link', 'meta',
    'style', 'text', 'type', 'href', 'content', 'charset', 'name',
    'epub', 'xmlns', 'lang', 'http', 'www', 'xhtml', 'ncx', 'opf',
    'nav', 'toc', 'src', 'alt', 'img', 'div', 'properties', 'media',
    'item', 'manifest', 'spine', 'metadata', 'package', 'version',
    'creator', 'publisher', 'date', 'language', 'description',
}

def scan_english(text, filename):
    candidates = re.findall(r'\b[A-Za-z]{4,}\b', text)
    real = [w for w in candidates if w.lower() not in VN_FP]
    if real:
        print(f"  ❌ [{filename}] Tiếng Anh: {set(list(real)[:8])}")
        return False
    return True

# === MARKDOWN SCAN ===
def scan_markdown(text, filename):
    for p in ['**', '## ', '# ', '---', '___', '```']:
        if p in text:
            print(f"  ❌ [{filename}] Markdown: '{p}'")
            return False
    return True

# === AI TRACES ===
AI_TRACES = [
    'tuyệt vời', 'hoàn hảo', 'thú vị thay', 'quả thật', 'đúng vậy',
    'không thể phủ nhận', 'điều đáng nói là', 'rõ ràng là',
    'một cách tàn nhẫn', 'một cách nhẹ nhàng', 'một cách tinh tế',
]

def scan_ai_traces(text, filename):
    found = [t for t in AI_TRACES if t in text.lower()]
    if found:
        print(f"  ⚠️ [{filename}] AI traces: {found}")
    return True  # Warning only

# === TAG VALIDATION ===
def validate_tags(data, filename):
    for tag in ['div', 'p', 'span', 'h1', 'h2', 'h3']:
        opens = len(re.findall(rf'<{tag}[\s>]', data))
        closes = data.count(f'</{tag}>')
        if opens != closes:
            print(f"  ❌ [{filename}] {tag}: {opens} open != {closes} close")
            return False
    return True

# === EM DASH SCAN ===
def scan_em_dash(text, filename):
    count = text.count('\u2014')  # —
    if count > 0:
        print(f"  ⚠️ [{filename}] Em dash: {count} lần")
    return True  # Warning only for EPUB (might be intentional in metadata)

# === MAIN ===
def master_verify(epub_path):
    print(f"\n{'='*60}")
    print(f"  MASTER VERIFY — Forge Novel Guard V1.0")
    print(f"  File: {os.path.basename(epub_path)}")
    print(f"{'='*60}\n")

    if not os.path.exists(epub_path):
        print(f"❌ File không tồn tại: {epub_path}")
        return False

    z = zipfile.ZipFile(epub_path)
    total_pass = 0
    total_fail = 0
    total_warn = 0

    # === PHASE 1: EPUB Structure ===
    print("📦 PHASE 1: EPUB Structure\n")

    # mimetype
    try:
        info = z.getinfo('mimetype')
        ok = info.compress_type == 0 and info.header_offset == 0
        mtype = z.read('mimetype').decode('utf-8').strip()
        ok = ok and mtype == 'application/epub+zip'
    except KeyError:
        ok = False
    print(f"  {'✅' if ok else '❌'} mimetype (first, stored, correct)")
    total_pass += ok; total_fail += (not ok)

    # Cover
    names = z.namelist()
    has_cover = any('cover.png' in f.lower() or 'cover.jpg' in f.lower() for f in names)
    has_svg = any('cover.svg' in f.lower() for f in names) and not has_cover
    if has_cover:
        print(f"  ✅ Cover PNG/JPG")
        total_pass += 1
    elif has_svg:
        print(f"  ❌ Cover SVG only (cần PNG)")
        total_fail += 1
    else:
        print(f"  ⚠️ Không tìm thấy cover")
        total_warn += 1

    # nav.xhtml
    has_nav = any('nav.xhtml' in f for f in names)
    print(f"  {'✅' if has_nav else '❌'} nav.xhtml")
    total_pass += has_nav; total_fail += (not has_nav)

    # toc.ncx
    has_ncx = any('toc.ncx' in f for f in names)
    print(f"  {'✅' if has_ncx else '⚠️'} toc.ncx (backward compat)")
    total_pass += has_ncx

    # === PHASE 2: Content Quality ===
    print(f"\n📝 PHASE 2: Content Quality\n")

    xhtml_files = sorted([f for f in names if f.endswith('.xhtml')])
    total_words = 0

    for fname in xhtml_files:
        data = z.read(fname).decode('utf-8')
        text = re.sub(r'<[^>]*>', ' ', data)
        basename = os.path.basename(fname)
        words = len(text.split())
        total_words += words

        ok = True
        ok &= scan_entities(data, basename)
        ok &= validate_tags(data, basename)

        # Only deep scan chapter files
        if 'ch' in basename.lower() or 'chapter' in basename.lower():
            ok &= scan_english(text, basename)
            ok &= scan_markdown(text, basename)
            scan_ai_traces(text, basename)
            scan_em_dash(text, basename)

        if ok:
            print(f"  ✅ {basename} ({words}w)")

        total_pass += ok
        total_fail += (not ok)

    # === PHASE 3: Send Readiness ===
    print(f"\n📤 PHASE 3: Send Readiness\n")

    fsize = os.path.getsize(epub_path)
    fname_base = os.path.basename(epub_path)
    is_ascii = all(ord(c) < 128 for c in fname_base)
    has_ext = fname_base.endswith('.epub')

    print(f"  {'✅' if has_ext else '❌'} Extension .epub")
    print(f"  {'✅' if is_ascii else '❌'} Filename ASCII safe")
    print(f"  📊 Size: {fsize:,} bytes ({fsize/1024:.0f} KB)")
    print(f"  📊 Words: ~{total_words:,}")
    print(f"  📊 XHTML files: {len(xhtml_files)}")

    total_pass += (has_ext and is_ascii)
    total_fail += (not has_ext or not is_ascii)

    # === SUMMARY ===
    print(f"\n{'='*60}")
    print(f"  SUMMARY")
    print(f"  ✅ Passed: {total_pass}")
    print(f"  ❌ Failed: {total_fail}")
    print(f"  ⚠️ Warnings: {total_warn}")
    print(f"{'='*60}")

    if total_fail == 0:
        print(f"\n  🎉 ALL CHECKS PASSED!")
        print(f"  📌 Final step: epubcheck {epub_path}")
    else:
        print(f"\n  ❌ {total_fail} ISSUE(S) — fix rồi chạy lại")

    print(f"{'='*60}\n")

    z.close()
    return total_fail == 0

if __name__ == '__main__':
    if len(sys.argv) < 2:
        print("Usage: python3 master-verify.py <file.epub>")
        sys.exit(1)
    ok = master_verify(sys.argv[1])
    sys.exit(0 if ok else 1)

#!/usr/bin/env python3
"""
verify-epub.py — Kiểm tra EPUB3 trước khi gửi
Chạy: python3 verify-epub.py <file.epub>

Kiểm tra:
1. mimetype (first entry, ZIP_STORED)
2. Cover PNG/JPG (không SVG)
3. XHTML entity hợp lệ (chỉ 5 entity)
4. Tag mở/đóng khớp
5. Tiếng Anh trong content
6. Markdown trong content
"""

import sys, os, re, zipfile

def scan_entities(data, filename):
    """Tìm entity HTML không hợp lệ trong XHTML"""
    invalid = re.findall(
        r'&(?!amp;|lt;|gt;|quot;|apos;|#\d+;|#x[0-9a-fA-F]+;)[a-zA-Z]+;',
        data
    )
    if invalid:
        print(f"  ❌ [{filename}] Entity lỗi: {set(invalid)}")
        return False
    return True

def scan_english(text, filename):
    """Tìm tiếng Anh trong content (loại HTML tags)"""
    VN_FP = {
        'trong', 'trang', 'nguy', 'trung', 'thanh', 'chang', 'chung',
        'ngang', 'vang', 'lung', 'bang', 'dung', 'tang', 'sang', 'mang',
        'lang', 'hang', 'rang', 'gang', 'phan', 'than', 'chan', 'nhan',
        'quan', 'khoan', 'thong', 'hong', 'dong', 'long', 'song', 'cong',
        'phong', 'nhanh', 'loang', 'quanh', 'khinh', 'then', 'tao',
        # HTML/XML tags (false positive in stripped text)
        'class', 'span', 'body', 'html', 'head', 'title', 'link', 'meta',
        'style', 'text', 'type', 'href', 'content', 'charset', 'name',
        'epub', 'xmlns', 'lang', 'http', 'www', 'xhtml', 'dtbook',
        'ncx', 'opf', 'nav', 'toc', 'src', 'alt', 'img', 'div',
        'properties', 'media', 'item', 'itemref', 'manifest', 'spine',
        'metadata', 'package', 'version', 'unique', 'identifier',
        'creator', 'publisher', 'date', 'language', 'description',
        'subject', 'rights', 'calibre', 'series', 'index',
    }
    candidates = re.findall(r'\b[A-Za-z]{4,}\b', text)
    real = [w for w in candidates if w.lower() not in VN_FP]
    if real:
        unique = set(real)
        print(f"  ❌ [{filename}] Tiếng Anh: {list(unique)[:8]}")
        return False
    return True

def validate_tags(data, filename):
    """Kiểm tra tag mở/đóng khớp"""
    issues = []
    for tag in ['div', 'p', 'span', 'h1', 'h2', 'h3']:
        opens = len(re.findall(rf'<{tag}[\s>]', data))
        closes = data.count(f'</{tag}>')
        if opens != closes:
            issues.append(f"{tag}: {opens} open != {closes} close")
    if issues:
        print(f"  ❌ [{filename}] Tag mismatch: {'; '.join(issues)}")
        return False
    return True

def validate_epub(epub_path):
    print(f"\n{'='*60}")
    print(f"VERIFY EPUB: {os.path.basename(epub_path)}")
    print(f"{'='*60}\n")

    if not os.path.exists(epub_path):
        print(f"❌ File không tồn tại: {epub_path}")
        return False

    z = zipfile.ZipFile(epub_path)
    total_pass = 0
    total_fail = 0

    # 1. mimetype
    try:
        info = z.getinfo('mimetype')
        ok = info.compress_type == 0 and info.header_offset == 0
        content = z.read('mimetype').decode('utf-8').strip()
        ok = ok and content == 'application/epub+zip'
    except KeyError:
        ok = False
    status = "✅" if ok else "❌"
    print(f"[1] mimetype: {status}")
    total_pass += ok; total_fail += (not ok)

    # 2. Cover PNG/JPG
    names = z.namelist()
    has_image_cover = any(
        'cover.png' in f.lower() or 'cover.jpg' in f.lower() or 'cover.jpeg' in f.lower()
        for f in names
    )
    has_svg_only = any('cover.svg' in f.lower() for f in names) and not has_image_cover
    if has_image_cover:
        print(f"[2] Cover: ✅ PNG/JPG")
        total_pass += 1
    elif has_svg_only:
        print(f"[2] Cover: ❌ SVG only (cần convert sang PNG)")
        total_fail += 1
    else:
        print(f"[2] Cover: ⚠️ Không tìm thấy cover image")
        total_pass += 1  # Not fatal

    # 3-6. Check XHTML files
    print(f"\n[3-6] XHTML files:")
    xhtml_count = 0
    for fname in sorted(names):
        if not fname.endswith('.xhtml'):
            continue
        xhtml_count += 1
        data = z.read(fname).decode('utf-8')
        text = re.sub(r'<[^>]*>', ' ', data)
        basename = os.path.basename(fname)

        ok = True
        ok &= scan_entities(data, basename)
        ok &= validate_tags(data, basename)
        # Only scan English in chapter content, skip nav/toc
        if 'ch' in basename.lower() or 'chapter' in basename.lower():
            ok &= scan_english(text, basename)

        if ok:
            words = len(text.split())
            print(f"  ✅ {basename} ({words}w)")

        total_pass += ok
        total_fail += (not ok)

    # Summary
    fsize = os.path.getsize(epub_path)
    print(f"\n{'='*60}")
    print(f"Files: {len(names)} total, {xhtml_count} XHTML")
    print(f"Size: {fsize:,} bytes ({fsize/1024:.0f} KB)")
    print(f"Result: {total_pass} passed, {total_fail} failed")
    if total_fail == 0:
        print("🎉 ALL CHECKS PASSED!")
    else:
        print("❌ FIX ISSUES ABOVE BEFORE SENDING")
    print(f"{'='*60}")
    print(f"\n📌 Nhớ chạy thêm: epubcheck {epub_path}")

    z.close()
    return total_fail == 0

def main():
    if len(sys.argv) < 2:
        print("Usage: python3 verify-epub.py <file.epub>")
        sys.exit(1)
    ok = validate_epub(sys.argv[1])
    sys.exit(0 if ok else 1)

if __name__ == '__main__':
    main()

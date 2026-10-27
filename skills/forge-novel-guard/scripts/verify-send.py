#!/usr/bin/env python3
"""
verify-send.py — Kiểm tra file trước khi gửi Telegram
Chạy: python3 verify-send.py <filepath> <filename>

Kiểm tra:
1. File tồn tại
2. File có nội dung
3. Filename có extension
4. Tên ASCII safe (không dấu)
5. File trong workspace
6. HTML: không JS, không external resources
"""

import sys, os, re

def verify_before_send(filepath, filename):
    print(f"\n{'='*50}")
    print(f"VERIFY SEND: {filename}")
    print(f"Path: {filepath}")
    print(f"{'='*50}\n")

    checks = []

    # 1. File tồn tại
    exists = os.path.exists(filepath)
    checks.append(("File tồn tại", exists))

    # 2. File có nội dung
    size = os.path.getsize(filepath) if exists else 0
    checks.append(("Có nội dung", size > 0))
    if size > 0:
        print(f"  📊 Size: {size:,} bytes ({size/1024:.0f} KB)")

    # 3. Filename có extension
    has_ext = '.' in filename and len(filename.rsplit('.', 1)[-1]) >= 2
    checks.append(("Có extension", has_ext))

    # 4. ASCII safe
    is_ascii = all(ord(c) < 128 for c in filename)
    checks.append(("ASCII safe", is_ascii))
    if not is_ascii:
        bad_chars = [c for c in filename if ord(c) >= 128]
        print(f"  ⚠️ Ký tự không ASCII: {''.join(bad_chars)}")

    # 5. Trong workspace
    abs_path = os.path.abspath(filepath)
    in_workspace = '/root/.openclaw/workspace/' in abs_path
    checks.append(("Trong workspace", in_workspace))

    # 6. HTML specific checks
    if filename.endswith('.html') or filename.endswith('.htm'):
        if exists:
            with open(filepath, 'r', encoding='utf-8') as f:
                content = f.read()
            has_js = bool(re.search(r'<script', content, re.IGNORECASE))
            has_ext_css = bool(re.search(r'@import\s+url|<link[^>]+stylesheet[^>]+http', content, re.IGNORECASE))
            has_ext_font = bool(re.search(r'fonts\.googleapis|fonts\.gstatic', content, re.IGNORECASE))
            checks.append(("Không JavaScript", not has_js))
            checks.append(("Không external CSS", not has_ext_css))
            checks.append(("Không external fonts", not has_ext_font))

    # Print results
    all_pass = True
    for name, ok in checks:
        status = "✅" if ok else "❌"
        print(f"  {status} {name}")
        if not ok:
            all_pass = False

    print(f"\n{'='*50}")
    if all_pass:
        print("🎉 READY TO SEND!")
        print(f"\n  message action=send filePath={filepath} filename=\"{filename}\"")
    else:
        print("❌ FIX ISSUES BEFORE SENDING")
    print(f"{'='*50}")

    return all_pass

def main():
    if len(sys.argv) < 3:
        print("Usage: python3 verify-send.py <filepath> <filename>")
        print("Example: python3 verify-send.py /root/.openclaw/workspace/book.epub Book-Premium.epub")
        sys.exit(1)
    ok = verify_before_send(sys.argv[1], sys.argv[2])
    sys.exit(0 if ok else 1)

if __name__ == '__main__':
    main()

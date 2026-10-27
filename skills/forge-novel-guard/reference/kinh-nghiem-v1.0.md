# KINH NGHIỆM FORGE NOVEL V1.0 — Tiểu Tâm 🦊
### Đúc kết từ 70+ chương novel, 12 truyện ngắn, 5 cuốn sách, 3 EPUB Premium
### Version: 1.0 | Cập nhật: 2026-03-13
### Mục đích: Cho BẤT KỲ AI agent nào đọc file này là hấp thụ được toàn bộ kinh nghiệm

---

## CÁCH SỬ DỤNG FILE NÀY
1. Đọc toàn bộ file trước khi bắt đầu forge
2. Mỗi section có: TRIỆU CHỨNG → NGUYÊN NHÂN → PHƯƠNG ÁN XỬ LÝ TRIỆT ĐỂ → SCRIPT TỰ ĐỘNG
3. Section 9 có MASTER SCRIPT chạy 1 lần verify tất cả
4. Áp dụng cho: tiểu thuyết tiếng Việt, truyện ngắn, EPUB, HTML export

---

## MỤC LỤC
1. [EPUB BUILD — 7 lỗi chí mạng + fix triệt để](#1-epub-build--7-lỗi-chí-mạng)
2. [FORGE PROSE — 5 bệnh AI viết + cách trị tận gốc](#2-forge-prose--5-bệnh-ai-viết)
3. [TELEGRAM GỬI FILE — 3 lỗi mất nội dung + quy trình chuẩn](#3-telegram-gửi-file--3-lỗi-mất-nội-dung)
4. [SUB-AGENT — 4 lỗi pipeline + phòng ngừa](#4-sub-agent--4-lỗi-pipeline)
5. [THẨM ĐỊNH — 3 sai sót + hệ thống thẩm định đúng](#5-thẩm-định--3-sai-sót)
6. [GIỌNG VĂN — 5 bệnh + phác đồ trị](#6-giọng-văn--5-bệnh)
7. [QUY TRÌNH FORGE CHUẨN — Step-by-step](#7-quy-trình-forge-chuẩn)
8. [CHECKLIST TRƯỚC KHI GIAO — 3 checklist chi tiết](#8-checklist-trước-khi-giao)
9. [MASTER SCRIPT — Verify tự động tất cả](#9-master-script--verify-tự-động)

---

## 1. EPUB BUILD — 7 lỗi chí mạng

### 1.1 XHTML chỉ cho phép 5 entity (FATAL)

**Triệu chứng:** Reader hiện filename thay vì tên chương, báo lỗi ký tự, separator vỡ, toàn bộ navigation hỏng.

**Nguyên nhân:** EPUB dùng XHTML (XML nghiêm ngặt), KHÔNG PHẢI HTML. XML parser gặp entity không khai báo → FATAL error → reader bỏ qua toàn bộ file → fallback hiện filename.

**5 entity hợp lệ DUY NHẤT trong XHTML:**
```
&amp;   →  &
&lt;    →  <
&gt;    →  >
&quot;  →  "
&apos;  →  '
```

**Bảng chuyển đổi entity HTML → Unicode (GHI NHỚ):**

| Entity HTML (CẤM) | Unicode thay thế | Mô tả |
|---|---|---|
| `&mdash;` | `—` (U+2014) | Em dash |
| `&ndash;` | `–` (U+2013) | En dash |
| `&hellip;` | `…` (U+2026) | Dấu ba chấm |
| `&diams;` | `♦` (U+2666) | Kim cương |
| `&nbsp;` | ` ` (U+00A0) | Khoảng trắng không ngắt |
| `&lsquo;` | `'` (U+2018) | Nháy đơn mở |
| `&rsquo;` | `'` (U+2019) | Nháy đơn đóng |
| `&ldquo;` | `"` (U+201C) | Nháy kép mở |
| `&rdquo;` | `"` (U+201D) | Nháy kép đóng |
| `&bull;` | `•` (U+2022) | Bullet |
| `&copy;` | `©` (U+00A9) | Copyright |
| `&trade;` | `™` (U+2122) | Trademark |
| `&hearts;` | `♥` (U+2665) | Tim |
| `&spades;` | `♠` (U+2660) | Bích |
| `&clubs;` | `♣` (U+2663) | Chuồn |

**PHƯƠNG ÁN XỬ LÝ TRIỆT ĐỂ:**

Bước 1: Trong code build EPUB, KHÔNG BAO GIỜ dùng entity name. Luôn viết Unicode trực tiếp.
```python
# SAI - sẽ FATAL trong EPUB
text = "Hắn nói &mdash; rồi im."
separator = "&diams; &diams; &diams;"

# ĐÚNG - Unicode trực tiếp
text = "Hắn nói — rồi im."
separator = "♦ ♦ ♦"
```

Bước 2: Sau khi build, chạy script quét entity lỗi:
```python
import zipfile, re

def scan_epub_entities(epub_path):
    """Quét toàn bộ EPUB tìm entity không hợp lệ XHTML"""
    z = zipfile.ZipFile(epub_path)
    issues = []
    for fname in z.namelist():
        if fname.endswith('.xhtml') or fname.endswith('.html'):
            data = z.read(fname).decode('utf-8')
            # Tìm entity KHÔNG phải 5 entity hợp lệ
            invalid = re.findall(
                r'&(?!amp;|lt;|gt;|quot;|apos;|#\d+;|#x[0-9a-fA-F]+;)[a-zA-Z]+;', 
                data
            )
            if invalid:
                issues.append(f"❌ {fname}: {set(invalid)}")
    z.close()
    if issues:
        print("ENTITY ERRORS FOUND:")
        for i in issues:
            print(f"  {i}")
        return False
    else:
        print("✅ No invalid entities")
        return True
```

Bước 3: Nếu tìm thấy entity lỗi, auto-fix:
```python
def fix_epub_entities(epub_dir):
    """Tự động thay thế tất cả entity HTML bằng Unicode"""
    import os
    ENTITY_MAP = {
        '&mdash;': '—', '&ndash;': '–', '&hellip;': '…',
        '&diams;': '♦', '&nbsp;': '\u00A0', '&bull;': '•',
        '&lsquo;': '\u2018', '&rsquo;': '\u2019',
        '&ldquo;': '\u201C', '&rdquo;': '\u201D',
        '&copy;': '©', '&trade;': '™',
        '&hearts;': '♥', '&spades;': '♠', '&clubs;': '♣',
    }
    for root, dirs, files in os.walk(epub_dir):
        for fname in files:
            if fname.endswith('.xhtml'):
                fpath = os.path.join(root, fname)
                with open(fpath, 'r') as f:
                    content = f.read()
                changed = False
                for entity, unicode_char in ENTITY_MAP.items():
                    if entity in content:
                        content = content.replace(entity, unicode_char)
                        changed = True
                if changed:
                    with open(fpath, 'w') as f:
                        f.write(content)
                    print(f"  Fixed entities in {fname}")
```

### 1.2 Cover SVG không hiện trên điện thoại

**Triệu chứng:** Mở EPUB → cover trắng, lỗi, hoặc hiện raw SVG code.

**Nguyên nhân:** Apple Books, Google Play Books, Kobo, và đa số reader mobile KHÔNG render SVG inline. Chỉ PNG/JPG được hỗ trợ universal.

**PHƯƠNG ÁN XỬ LÝ TRIỆT ĐỂ:**

Bước 1: Thiết kế cover bằng SVG (dễ code, vector đẹp)
Bước 2: Convert sang PNG TRƯỚC KHI đóng gói EPUB
```bash
# Cần package: librsvg2-bin
sudo apt install librsvg2-bin

# Convert SVG → PNG (800x1200 = tỷ lệ sách chuẩn)
rsvg-convert cover.svg -o cover.png -w 800 -h 1200
```

Bước 3: Trong content.opf, khai báo cover là PNG:
```xml
<item id="cover-image" href="images/cover.png" media-type="image/png" properties="cover-image"/>
```

Bước 4: Trong cover.xhtml, dùng `<img>` trỏ đến PNG:
```html
<body>
<div style="text-align:center; padding:0; margin:0;">
  <img src="../images/cover.png" alt="Bìa sách" 
       style="max-width:100%; max-height:100%; display:block; margin:0 auto;"/>
</div>
</body>
```

**QUY TẮC:** KHÔNG BAO GIỜ dùng SVG làm cover trong EPUB. SVG chỉ dùng làm source → convert PNG.

### 1.3 Separator ♦ ♦ ♦ vỡ

**Triệu chứng:** Cuối chương hiện khung lỗi, ký tự lạ, hoặc mã HTML thô.

**Nguyên nhân kép:**
1. Dùng `&diams;` (entity HTML, FATAL trong XHTML)
2. Wrap trong `<div>` gây lỗi nesting

**PHƯƠNG ÁN XỬ LÝ TRIỆT ĐỂ:**

Dùng `<p>` (không phải `<div>`), Unicode trực tiếp:
```html
<p class="separator">♦ ♦ ♦</p>
```

CSS tương ứng:
```css
.separator {
    text-align: center;
    color: #C9A96E;
    margin: 1.5em 0;
    font-size: 1.2em;
    letter-spacing: 0.5em;
    text-indent: 0;
}
```

### 1.4 Tag HTML lồng sai / không khớp

**Triệu chứng:** Nội dung bị cắt, layout vỡ, reader crash.

**Nguyên nhân:** `</div></div>` dính nhau, thiếu closing tag, tag mở nhiều hơn tag đóng.

**PHƯƠNG ÁN XỬ LÝ TRIỆT ĐỂ:**

Script đếm tag cho MỌI file xhtml:
```python
import re

def validate_tags(xhtml_content, filename):
    """Đếm tag mở/đóng, báo lỗi nếu không khớp"""
    tags_to_check = ['div', 'p', 'span', 'h2', 'h3', 'ul', 'li', 'a']
    issues = []
    for tag in tags_to_check:
        opens = len(re.findall(rf'<{tag}[\s>]', xhtml_content))
        closes = xhtml_content.count(f'</{tag}>')
        if opens != closes:
            issues.append(f"{tag}: {opens} open ≠ {closes} close")
    
    # Check stuck tags
    if '</div></div>' in xhtml_content:
        issues.append("Found </div></div> stuck together")
    
    # Check content after </html>
    html_end = xhtml_content.find('</html>')
    if html_end > 0 and xhtml_content[html_end+7:].strip():
        issues.append("Content after </html>")
    
    if issues:
        print(f"❌ {filename}:")
        for i in issues:
            print(f"   {i}")
        return False
    return True
```

### 1.5 EPUBCheck — BẮT BUỘC chạy

**PHƯƠNG ÁN:** Chạy EPUBCheck là BƯỚC CUỐI CÙNG trước khi gửi. Không ngoại lệ.

```bash
# Cài đặt (1 lần)
sudo apt install epubcheck

# Chạy
epubcheck file.epub

# Kết quả phải là:
# Check finished with warnings or errors
# Messages: 0 fatals / 0 errors / 0 warnings
```

**Nếu có lỗi:** ĐỌC message → fix → rebuild → chạy lại. KHÔNG GỬI file có error.

### 1.6 mimetype sai vị trí/compression

**PHƯƠNG ÁN:** Trong Python zipfile:
```python
import zipfile

with zipfile.ZipFile('book.epub', 'w') as zf:
    # mimetype PHẢI là entry ĐẦU TIÊN, KHÔNG NÉN
    zf.writestr('mimetype', 'application/epub+zip', compress_type=zipfile.ZIP_STORED)
    
    # Tất cả file khác: nén DEFLATED
    for filepath in other_files:
        zf.write(filepath, arcname, compress_type=zipfile.ZIP_DEFLATED)
```

### 1.7 EPUB Build Checklist (chạy theo thứ tự)

```
□ 1. Tất cả entity → Unicode trực tiếp (chạy scan_epub_entities)
□ 2. Cover là PNG/JPG (KHÔNG SVG)
□ 3. mimetype: first entry, ZIP_STORED
□ 4. Tất cả tag mở/đóng khớp (chạy validate_tags)
□ 5. nav.xhtml có epub:type="toc"
□ 6. toc.ncx có đủ navPoints với tên chương đúng
□ 7. <title> trong mỗi xhtml = tên chương tiếng Việt
□ 8. Separator = Unicode ♦ trong <p class="separator">
□ 9. EPUBCheck: 0 fatal, 0 error, 0 warning ← BƯỚC CUỐI, BẮT BUỘC
```

---

## 2. FORGE PROSE — 5 bệnh AI viết

### 2.1 Tiếng Anh lọt vào prose tiếng Việt

**Triệu chứng:** Từ tiếng Anh xuất hiện giữa văn tiếng Việt — "logic sạch", "VIP", "pattern cũ".

**Nguyên nhân:** LLM training data chủ yếu tiếng Anh, khi viết tiếng Việt vẫn rò rỉ từ Anh cho khái niệm chưa có tương đương Việt chính xác.

**BẢNG THAY THẾ (đã gặp thực tế, GHI NHỚ):**

| Tiếng Anh | Thay bằng tiếng Việt |
|---|---|
| logic | lý / lý lẽ / hợp lý / đạo lý |
| pattern | vết cũ / quy luật / khuôn mẫu |
| VIP | hạng nhất / thượng khách / quý nhân |
| copy | sao chép / bắt chước |
| gram | li / chút / tí |
| level | cấp / bậc / tầng |
| style | phong cách / lối |
| skill | tài / kỹ năng |
| point | điểm / ý |
| check | kiểm / dò |
| test | thử / kiểm tra |
| OK | được / ổn |
| focus | tập trung / chú tâm |
| control | kiểm soát / nắm giữ |
| perfect | hoàn hảo / trọn vẹn |

**PHƯƠNG ÁN XỬ LÝ TRIỆT ĐỂ:**

Bước 1: Trong prompt forge, LUÔN ghi rõ: "KHÔNG từ tiếng Anh. 100% tiếng Việt."
Bước 2: Sau forge, chạy script scan:
```python
import re

def scan_english(text):
    """Tìm từ tiếng Anh trong văn tiếng Việt"""
    # Tìm từ 3+ ký tự Latin không dấu
    candidates = re.findall(r'\b[A-Za-z]{3,}\b', text)
    
    # Loại false positive (từ Việt viết không dấu trùng tiếng Anh)
    VN_FALSE_POSITIVES = {
        'trong', 'trang', 'nguy', 'trung', 'ngay', 'khong', 'cung', 
        'chang', 'chung', 'thanh', 'ngang', 'vang', 'lung', 'bang', 
        'dung', 'tang', 'sang', 'mang', 'lang', 'hang', 'rang', 
        'gang', 'phan', 'than', 'chan', 'nhan', 'quan', 'khoan', 
        'thong', 'hong', 'dong', 'long', 'song', 'cong', 'phong',
        'nhanh', 'loang', 'quanh', 'khinh', 'hui', 'the', 'then',
        'than', 'back', 'can', 'cat', 'con', 'dam', 'dan', 'dat',
    }
    
    real_english = [w for w in candidates if w.lower() not in VN_FALSE_POSITIVES]
    
    if real_english:
        print(f"⚠️ Tiếng Anh phát hiện: {set(real_english)}")
        for word in set(real_english):
            # Tìm context
            for m in re.finditer(re.escape(word), text):
                ctx = text[max(0,m.start()-30):m.end()+30]
                print(f"  '{word}' → ...{ctx.strip()[:70]}...")
        return False
    else:
        print("✅ Không có tiếng Anh")
        return True
```

Bước 3: Nếu tìm thấy → thay thế theo bảng trên → scan lại.

### 2.2 Markdown lọt vào prose

**Triệu chứng:** `**text**`, `##`, `---`, `> quote`, `- list item` trong văn bản.

**Nguyên nhân:** AI quen format markdown mặc định.

**PHƯƠNG ÁN XỬ LÝ TRIỆT ĐỂ:**

Bước 1: Trong prompt: "0% markdown. Prose thuần túy. KHÔNG **, ##, ---, >, -"
Bước 2: Script scan + auto-fix:
```python
import re

def fix_markdown(text):
    """Loại bỏ markdown khỏi prose"""
    fixes = 0
    
    # Bold **text** → text
    while '**' in text:
        text = re.sub(r'\*\*([^*]+)\*\*', r'\1', text)
        fixes += 1
    
    # Italic *text* → text (cẩn thận không match multiplication)
    text = re.sub(r'(?<!\*)\*([^*\n]+)\*(?!\*)', r'\1', text)
    
    # Headers ## → remove
    text = re.sub(r'^#{1,6}\s+', '', text, flags=re.MULTILINE)
    
    # Horizontal rules --- or ___ → remove
    text = re.sub(r'^[-_]{3,}\s*$', '', text, flags=re.MULTILINE)
    
    # Blockquotes > → remove >
    text = re.sub(r'^>\s*', '', text, flags=re.MULTILINE)
    
    # List items ^- → remove -
    text = re.sub(r'^[-*]\s+', '', text, flags=re.MULTILINE)
    
    # Inline code `text` → text
    text = re.sub(r'`([^`]+)`', r'\1', text)
    
    return text, fixes

def scan_markdown(text):
    """Kiểm tra còn markdown không"""
    patterns = {
        '**': 'Bold markers',
        '## ': 'Headers',
        '# ': 'Headers', 
        '---': 'Horizontal rule',
        '___': 'Horizontal rule',
        '> ': 'Blockquote',
        '```': 'Code block',
        '`': 'Inline code',
    }
    found = []
    for pattern, name in patterns.items():
        if pattern in text:
            found.append(f"⚠️ {name}: '{pattern}'")
    
    if found:
        for f in found:
            print(f)
        return False
    print("✅ Không có markdown")
    return True
```

### 2.3 Em dash — lọt vào

**Triệu chứng:** Ký tự `—` (em dash) trong prose tiếng Việt.

**Nguyên nhân:** AI (đặc biệt Claude) dùng em dash rất nhiều. Tiếng Việt chuẩn KHÔNG dùng em dash.

**PHƯƠNG ÁN XỬ LÝ TRIỆT ĐỂ:**

```python
def fix_em_dashes(text):
    """Thay em dash bằng dấu câu tiếng Việt phù hợp"""
    # Em dash giữa câu → dấu phẩy hoặc dấu chấm
    # "Hắn nói — rồi im" → "Hắn nói, rồi im"
    # Cần xử lý context-aware, không auto-replace toàn bộ
    
    count = text.count('—')
    if count > 0:
        print(f"⚠️ Tìm thấy {count} em dash (—)")
        # Liệt kê context để review thủ công
        for m in re.finditer('—', text):
            ctx = text[max(0,m.start()-30):m.end()+30]
            print(f"  → ...{ctx.strip()[:60]}...")
        return False
    print("✅ Không có em dash")
    return True
```

**Thay thế em dash theo context:**
- "A — B" (giải thích) → "A, B" hoặc "A. B"
- "A — B — C" (liệt kê) → "A, B, C"
- Đầu câu "— Hắn nói" (dialogue) → bỏ em dash, dùng dấu hai chấm hoặc không cần

### 2.4 Xưng "tôi" sai ngôi

**Triệu chứng:** Lời tác giả, lời bạt, phần giới thiệu xưng "tôi".

**Nguyên nhân:** AI mặc định xưng hô "tôi" cho first-person.

**PHƯƠNG ÁN XỬ LÝ TRIỆT ĐỂ:**

Bước 1: Trong prompt: "Tác giả xưng 'ta', KHÔNG xưng 'tôi'"
Bước 2: Scan sau forge:
```python
def check_author_voice(text, author_sections=['Lời Tác Giả', 'Lời Bạt', 'Về Tác Giả']):
    """Kiểm tra 'tôi' trong phần tác giả"""
    for section_name in author_sections:
        start = text.find(section_name)
        if start < 0:
            continue
        # Lấy ~2000 ký tự section
        section = text[start:start+2000]
        toi_matches = list(re.finditer(r'\btôi\b', section, re.IGNORECASE))
        if toi_matches:
            print(f"⚠️ '{section_name}' có {len(toi_matches)} lần 'tôi':")
            for m in toi_matches:
                ctx = section[max(0,m.start()-20):m.end()+30]
                print(f"  → ...{ctx.strip()[:60]}...")
            return False
    print("✅ Không xưng 'tôi' sai ngôi")
    return True
```

### 2.5 Từ ngữ dấu vết AI — BẢNG ĐỎ

**Các từ/cụm LLM hay dùng mà NGƯỜI THẬT ÍT DÙNG:**

| Dấu vết AI | Tại sao lỗi | Thay bằng |
|---|---|---|
| "tuyệt vời" | Quá generic, AI reflex | Bỏ hoặc mô tả cụ thể |
| "hoàn hảo" | AI hay compliment | Bỏ hoặc thay bằng chi tiết |
| "thú vị thay" | Không ai nói thế | Bỏ |
| "quả thật", "đúng vậy" | Filler AI | Bỏ |
| "một cách [tính từ]" | Pattern cứng | Dùng trạng từ trực tiếp |
| "không thể phủ nhận" | AI rhetoric | Bỏ, viết thẳng |
| "điều đáng nói là" | Narrator intrusion | Bỏ, show don't tell |
| "sâu sắc", "tinh tế" | AI tự khen | Bỏ, để reader tự đánh giá |
| "Ah," / "À," / "Ồ," | Fake emotion | Bỏ hoặc thay bằng hành động |
| "rõ ràng là" | AI filler | Bỏ |
| "quan trọng hơn" | AI structure | Bỏ trong prose |
| "nói cách khác" | AI restate | Bỏ, viết 1 lần cho đúng |

**Script scan:**
```python
AI_TRACES = [
    'tuyệt vời', 'hoàn hảo', 'thú vị thay', 'quả thật', 'đúng vậy',
    'tất nhiên rồi', 'không thể phủ nhận', 'điều đáng nói là',
    'rõ ràng là', 'quan trọng hơn', 'nói cách khác',
    'một cách tàn nhẫn', 'một cách nhẹ nhàng', 'một cách tự nhiên',
    'một cách tinh tế', 'một cách bất ngờ', 'một cách kỳ lạ',
]

def scan_ai_traces(text):
    """Tìm dấu vết AI trong prose"""
    found = []
    for trace in AI_TRACES:
        count = text.lower().count(trace.lower())
        if count > 0:
            found.append(f"  '{trace}': {count} lần")
    if found:
        print(f"⚠️ AI traces ({len(found)}):")
        for f in found:
            print(f)
        return False
    print("✅ Không có dấu vết AI")
    return True
```

---

## 3. TELEGRAM GỬI FILE — 3 lỗi mất nội dung

### 3.1 File gửi lên trống/không mở được

**Triệu chứng:** Người nhận mở file → trống, 0 byte, hoặc "không hỗ trợ".

**Nguyên nhân:** Thiếu `filename` parameter → Telegram đặt tên file tự động không có extension → thiết bị không biết mở bằng app gì.

**PHƯƠNG ÁN XỬ LÝ TRIỆT ĐỂ:**

QUY TẮC GỬI FILE (áp dụng MỌI LÚC):
```
message action=send filePath=/path/to/file filename="Ten-File-ASCII.ext"
```

4 điều kiện BẮT BUỘC:
1. `filename` có extension rõ ràng: `.epub`, `.html`, `.pdf`, `.md`, `.zip`
2. Tên file ASCII safe (KHÔNG DẤU): `Thien-Tu-Cau-Kinh.epub` ✅ | `Thiên Tử Cầu Kinh.epub` ❌
3. File trong workspace: `~/.openclaw/workspace/`
4. File có nội dung: `wc -c > 0`

### 3.2 HTML gửi qua Telegram hiển thị lỗi

**Nguyên nhân:** Telegram in-app viewer KHÔNG chạy JavaScript, KHÔNG load Google Fonts, KHÔNG fetch external CSS.

**PHƯƠNG ÁN TRIỆT ĐỂ:**
- CSS inline hoặc trong `<style>` tag
- Font: system fonts (Georgia, serif, sans-serif)
- KHÔNG `<script>`, KHÔNG `@import url(...)`, KHÔNG `<link rel="stylesheet" href="https://...">`

### 3.3 Checklist gửi file (chạy TRƯỚC MỌI lần gửi)

```python
import os

def verify_before_send(filepath, filename):
    """Kiểm tra file trước khi gửi Telegram"""
    checks = []
    
    # 1. File tồn tại
    checks.append(("File exists", os.path.exists(filepath)))
    
    # 2. File có nội dung
    size = os.path.getsize(filepath) if os.path.exists(filepath) else 0
    checks.append(("File has content", size > 0))
    
    # 3. Filename có extension
    has_ext = '.' in filename and len(filename.split('.')[-1]) >= 2
    checks.append(("Has extension", has_ext))
    
    # 4. ASCII safe
    is_ascii = all(ord(c) < 128 for c in filename)
    checks.append(("ASCII safe name", is_ascii))
    
    # 5. Trong workspace
    in_workspace = '/root/.openclaw/workspace/' in filepath
    checks.append(("In workspace", in_workspace))
    
    all_pass = all(ok for _, ok in checks)
    for name, ok in checks:
        print(f"  {'✅' if ok else '❌'} {name}")
    
    return all_pass
```

---

## 4. SUB-AGENT — 4 lỗi pipeline

### 4.1 Timeout khi file dài (>10,000 từ)

**PHƯƠNG ÁN TRIỆT ĐỂ:**
- Chia input thành chunks 5,000 từ
- Mỗi chunk = 1 sub-agent call
- Set `runTimeoutSeconds: 600` (10 phút)
- Nếu task đơn giản (grep, replace): tự làm, KHÔNG spawn sub-agent

### 4.2 Duplicate nội dung khi insert đoạn mới

**PHƯƠNG ÁN TRIỆT ĐỂ:**

Sau MỌI insert/edit, chạy:
```python
def check_duplicates(content, key_phrases):
    """Kiểm tra trùng lặp sau khi insert nội dung mới"""
    issues = []
    for phrase in key_phrases:
        count = content.count(phrase)
        if count > 1:
            issues.append(f"'{phrase[:40]}...' xuất hiện {count} lần")
            # Tìm vị trí
            pos = 0
            while True:
                pos = content.find(phrase, pos)
                if pos == -1: break
                ctx = content[max(0,pos-20):pos+len(phrase)+20]
                issues.append(f"  tại {pos}: ...{ctx[:60]}...")
                pos += 1
    
    if issues:
        print(f"⚠️ DUPLICATE DETECTED:")
        for i in issues:
            print(f"  {i}")
        return False
    print("✅ Không trùng lặp")
    return True
```

Ngoài ra, kiểm tra transition:
```python
def check_transitions(content, insert_point, radius=200):
    """Đọc context trước/sau điểm insert, kiểm tra flow"""
    before = content[max(0, insert_point-radius):insert_point]
    after = content[insert_point:insert_point+radius]
    print(f"=== BEFORE INSERT ===\n{before}")
    print(f"=== AFTER INSERT ===\n{after}")
    # Review thủ công: flow có mượt không?
```

### 4.3 Sub-agent viết metadata không dấu

**PHƯƠNG ÁN TRIỆT ĐỂ:**

Trong prompt sub-agent, LUÔN ghi:
```
TIẾNG VIỆT CÓ DẤU ĐẦY ĐỦ trong mọi text hiển thị.
KHÔNG được viết không dấu: "Trong Sinh" ❌ → "Trọng Sinh" ✅
```

Sau build, scan:
```python
import unicodedata

def check_diacritics(text, min_length=15):
    """Tìm dòng tiếng Việt không dấu"""
    issues = []
    for line in text.split('\n'):
        line = line.strip()
        if len(line) < min_length:
            continue
        if not any(c.isalpha() for c in line):
            continue
        # Check if line has Vietnamese diacritics
        nfd = unicodedata.normalize('NFD', line)
        has_diacritics = any(unicodedata.combining(c) for c in nfd)
        if not has_diacritics:
            # Might be English or no-diacritics Vietnamese
            has_vn_chars = any(c in line for c in 'àáảãạăắằẵặâấầẩẫậèéẻẽẹêếềểễệìíỉĩịòóỏõọôốồổỗộơớờởỡợùúủũụưứừửữựỳýỷỹỵđ')
            if not has_vn_chars and any(c.isalpha() for c in line):
                issues.append(line[:80])
    
    if issues:
        print(f"⚠️ Possible no-diacritics Vietnamese ({len(issues)} lines):")
        for i in issues[:10]:
            print(f"  → {i}")
        return False
    print("✅ Tất cả text có dấu đầy đủ")
    return True
```

### 4.4 Sub-agent bỏ qua quy tắc

**PHƯƠNG ÁN TRIỆT ĐỂ:**

Inject quy tắc vào ĐẦU prompt, format rõ ràng:
```
⚠️ QUY TẮC BẮT BUỘC (vi phạm = fail):
1. XHTML: CHỈ 5 entity hợp lệ (&amp; &lt; &gt; &quot; &apos;)
2. 0% markdown trong prose (không **, ##, ---, >)
3. 0% tiếng Anh trong văn tiếng Việt
4. Cover EPUB = PNG (không SVG)
5. Verify mỗi step bằng exec + read output
6. Tiếng Việt CÓ DẤU đầy đủ mọi nơi
```

**Không viết quy tắc ở CUỐI prompt** — sub-agent hay bỏ qua phần cuối.

---

## 5. THẨM ĐỊNH — 3 sai sót

### 5.1 Thẩm định "pass" nhưng technical lỗi

**PHƯƠNG ÁN TRIỆT ĐỂ:**

Thẩm định phải gồm 2 TẦNG:

**Tầng 1: Technical (script tự động)**
- Entity scan
- Tiếng Anh scan
- Markdown scan
- Em dash scan
- AI trace scan
- Tag validation
- Duplicate check
→ PHẢI PASS hết mới qua Tầng 2

**Tầng 2: Prose quality (AI/human review)**
- Giọng nhân vật nhất quán?
- Nhịp truyện hợp lý?
- Plot logic không mâu thuẫn?
- Dialogue tự nhiên?
- Show don't tell?

### 5.2 Thẩm định miss duplicate

**PHƯƠNG ÁN:** Sau MỌI edit/insert, chạy `check_duplicates()` với key phrases từ đoạn mới + đoạn cũ.

### 5.3 Score inflation (AI tự cho điểm cao)

**PHƯƠNG ÁN TRIỆT ĐỂ:**
- Minimum 3 reviewer AI ĐỘC LẬP (không thấy điểm của nhau)
- Dùng Hội Đồng Nacharium (13 personas với góc nhìn khác nhau)
- Threshold: ≥ 9/10 mới PASS (không phải 8.5)
- Reviewer phải chỉ ra CỤ THỂ điểm yếu, không chỉ khen

---

## 6. GIỌNG VĂN — 5 bệnh

### 6.1 "Hội chứng người tốt" — Mọi nhân vật đều thiện

**Phác đồ trị:**
- Viết 1 nhân vật làm điều SAI vì lý do HIỂU ĐƯỢC
- Grey morality: nhân vật thiện có dark side, nhân vật ác có human moment
- Test: đọc lại, nếu reader KHÔNG mâu thuẫn về nhân vật → chưa đủ phức tạp

### 6.2 Pattern "ngồi-nhớ-moment nhỏ" lặp

**Phác đồ trị:**
- Checklist structure mỗi chương: action? dialogue? flashback? internal? multi-POV?
- Không cho phép 2 chương liên tiếp cùng structure
- Đa dạng: chase scene, argument, revelation, quiet tension, comic relief

### 6.3 Giọng tác giả lấn át giọng nhân vật

**Phác đồ trị:**
- Mỗi nhân vật có VOICE CARD:
  - Từ vựng đặc trưng (3-5 từ chỉ nhân vật đó dùng)
  - Cú pháp (câu ngắn/dài, phức/đơn)
  - Humor type (sarcasm/dry/slapstick/none)
  - Catchphrase (câu cửa miệng)
  - KHÔNG được: từ nào nhân vật TUYỆT ĐỐI không dùng
- Test: che tên, đọc dialogue → nhận ra ai nói không?

### 6.4 Overexplain — Viết ẩn dụ rồi giải thích

**Phác đồ trị:**
- Viết xong ẩn dụ/metaphor → ĐỌC CÂU SAU NÓ
- Nếu câu sau giải thích ẩn dụ → XÓA câu giải thích
- Rule: TIN NGƯỜI ĐỌC. Họ không ngu.
- Test: ẩn dụ còn work nếu bỏ giải thích? → bỏ.

### 6.5 Kết quá gọn — wrap up sạch sẽ

**Phác đồ trị:**
- Kết bằng HÀNH ĐỘNG hoặc HÌNH ẢNH, không kết bằng GIẢI THÍCH
- Để 1 câu hỏi LƠ LỬNG (reader mang theo khi gấp sách)
- Không kết bằng "Và thế là..." / "Cuối cùng..." / "Mọi thứ đã..."
- Test: đọc 2 câu cuối → có gợi được gì sau THE END không?

---

## 7. QUY TRÌNH FORGE CHUẨN

### 7.1 Mỗi beat (400-600 từ)

```
Bước 1: Viết 3 phiên bản ĐỘC LẬP (mỗi bản 400-600 từ)
Bước 2: Thẩm định cả 3:
  - Technical scan (tiếng Anh, markdown, em dash, AI trace)
  - Prose quality (giọng, nhịp, logic)
Bước 3: Chọn BEST hoặc MIX phần hay nhất
Bước 4: Clean final (loại markdown, AI trace, tiếng Anh)
Bước 5: Score ≥ 9/10 → PASS | < 9 → viết lại từ Bước 1
```

### 7.2 Mỗi chương (5 beats)

```
Bước 1: Bible query → context nhân vật, trạng thái, vị trí
Bước 2: Forge beat 1 → verify → PASS
Bước 3: Forge beat 2 → verify → PASS
Bước 4: Forge beat 3 → verify → PASS
Bước 5: Forge beat 4 → verify → PASS
Bước 6: Forge beat 5 → verify → PASS
Bước 7: Ghép manuscript (5 beats → 1 file)
Bước 8: Technical scan toàn chương
Bước 9: Continuity scan (Novel Guardian)
Bước 10: Prose drift check vs baseline
Bước 11: Voice check (dialogue nhất quán)
Bước 12: Pacing check (nhịp truyện)
Bước 13: Hội Đồng Nacharium thẩm định
Bước 14: ≥ 9/10 → ghi FINAL
Bước 15: Gửi file + báo cáo lên Telegram
Bước 16: Spawn chương tiếp (nếu còn trong arc)
```

### 7.3 Mỗi arc (10-20 chương)

```
Bước 1: Combine tất cả chương FINAL
Bước 2: Continuity scan toàn arc
Bước 3: Build HTML (CSS Premium inline)
Bước 4: Build EPUB Premium:
  a. Tách chapters → XHTML files
  b. Tạo cover PNG
  c. Tạo CSS, OPF, NCX, NAV
  d. Entity scan → fix
  e. Tag validation → fix
  f. Đóng gói ZIP (mimetype first, stored)
  g. EPUBCheck → 0 error
Bước 5: Gửi EPUB + HTML lên Telegram
```

---

## 8. CHECKLIST TRƯỚC KHI GIAO

### 8A. Prose Checklist
```
□ 0% tiếng Anh (chạy scan_english)
□ 0% markdown (chạy scan_markdown)
□ 0% em dash (chạy fix_em_dashes)
□ 0% dấu vết AI (chạy scan_ai_traces)
□ 0% xưng "tôi" sai ngôi (chạy check_author_voice)
□ 0% duplicate content (chạy check_duplicates)
□ Transitions mượt giữa sections
□ Mỗi beat 400-600 từ
□ Giọng nhân vật nhất quán
□ Score ≥ 9/10 từ 3+ reviewer độc lập
```

### 8B. EPUB Checklist
```
□ EPUBCheck: 0 fatal, 0 error, 0 warning
□ mimetype: first entry, ZIP_STORED, offset 0
□ Cover: PNG/JPG (KHÔNG SVG)
□ Entity: CHỈ &amp; &lt; &gt; &quot; &apos;
□ Tags: mở/đóng khớp (div, p, span, h2, h3)
□ nav.xhtml: epub:type="toc", đủ entries
□ toc.ncx: đủ navPoints, tên chương tiếng Việt
□ <title>: tên chương, không filename
□ Separator: Unicode ♦ trong <p class="separator">
□ Metadata: title, creator, language=vi, date
```

### 8C. Gửi Telegram Checklist
```
□ filename có extension rõ ràng (.epub, .html, .md)
□ Tên file ASCII safe (không dấu tiếng Việt)
□ File trong workspace (~/.openclaw/workspace/)
□ File có nội dung (wc -c > 0)
□ HTML: không JS, không external resources
```

---

## 9. MASTER SCRIPT — Verify tự động tất cả

Copy script dưới đây, chạy 1 lần để verify toàn bộ prose + EPUB:

```python
#!/usr/bin/env python3
"""
MASTER VERIFY — Kinh Nghiệm Forge Novel V1.0
Chạy: python3 master_verify.py <file.epub> [hoặc <file.html>]
Verify tất cả quy tắc trong 1 lần chạy.
"""

import sys, os, re, zipfile

def scan_entities(data, filename):
    invalid = re.findall(r'&(?!amp;|lt;|gt;|quot;|apos;|#\d+;|#x[0-9a-fA-F]+;)[a-zA-Z]+;', data)
    if invalid:
        print(f"  ❌ [{filename}] Invalid entities: {set(invalid)}")
        return False
    return True

def scan_english(text):
    candidates = re.findall(r'\b[A-Za-z]{4,}\b', text)
    vn_fp = {'trong','trang','nguy','trung','thanh','chang','chung','ngang',
             'vang','lung','bang','dung','tang','sang','mang','lang','hang',
             'rang','gang','phan','than','chan','nhan','quan','khoan','thong',
             'hong','dong','long','song','cong','phong','nhanh','loang',
             'quanh','khinh','then','back','class','span','body','html',
             'head','title','link','meta','style','text','type','href',
             'content','charset','name','epub','xmlns','lang'}
    real = [w for w in candidates if w.lower() not in vn_fp]
    if real:
        print(f"  ❌ Tiếng Anh: {set(real)}")
        return False
    return True

def scan_markdown(text):
    for p in ['**', '## ', '# ', '---', '___', '```']:
        if p in text:
            print(f"  ❌ Markdown: '{p}'")
            return False
    return True

def scan_ai_traces(text):
    traces = ['tuyệt vời','hoàn hảo','thú vị thay','quả thật','đúng vậy',
              'không thể phủ nhận','điều đáng nói là','rõ ràng là',
              'một cách tàn nhẫn','một cách nhẹ nhàng','một cách tinh tế']
    found = [t for t in traces if t in text.lower()]
    if found:
        print(f"  ⚠️ AI traces: {found}")
    return True  # Warning only

def validate_epub(epub_path):
    print(f"\n{'='*60}")
    print(f"MASTER VERIFY: {epub_path}")
    print(f"{'='*60}")
    
    z = zipfile.ZipFile(epub_path)
    total_pass = 0
    total_fail = 0
    
    # 1. mimetype
    info = z.getinfo('mimetype')
    ok = info.compress_type == 0 and info.header_offset == 0
    status = "✅" if ok else "❌"
    print(f"\n[1] mimetype: {status} (stored={info.compress_type==0}, offset={info.header_offset})")
    total_pass += ok; total_fail += (not ok)
    
    # 2. Cover PNG
    has_png = any('cover.png' in f or 'cover.jpg' in f for f in z.namelist())
    has_svg_only = any('cover.svg' in f for f in z.namelist()) and not has_png
    status = "✅" if has_png else ("❌ SVG only" if has_svg_only else "⚠️ No cover")
    print(f"[2] Cover PNG: {status}")
    total_pass += has_png; total_fail += (not has_png)
    
    # 3-8. Check each XHTML
    print(f"\n[3-8] XHTML files:")
    for fname in sorted(z.namelist()):
        if not fname.endswith('.xhtml'):
            continue
        data = z.read(fname).decode('utf-8')
        text = re.sub(r'<[^>]*>', ' ', data)
        basename = os.path.basename(fname)
        
        ok = True
        ok &= scan_entities(data, basename)
        ok &= scan_english(text)
        ok &= scan_markdown(text)
        scan_ai_traces(text)
        
        # Tag validation
        for tag in ['div', 'p', 'span']:
            opens = len(re.findall(rf'<{tag}[\s>]', data))
            closes = data.count(f'</{tag}>')
            if opens != closes:
                print(f"  ❌ [{basename}] {tag}: {opens} open ≠ {closes} close")
                ok = False
        
        if ok:
            words = len(text.split())
            print(f"  ✅ {basename} ({words}w)")
        
        total_pass += ok; total_fail += (not ok)
    
    # Summary
    print(f"\n{'='*60}")
    print(f"RESULT: {total_pass} passed, {total_fail} failed")
    if total_fail == 0:
        print("🎉 ALL CHECKS PASSED!")
    else:
        print("❌ FIX ISSUES ABOVE BEFORE SENDING")
    print(f"{'='*60}")
    
    # Remind: still need epubcheck
    print(f"\n📌 Nhớ chạy: epubcheck {epub_path}")
    
    z.close()
    return total_fail == 0

if __name__ == '__main__':
    if len(sys.argv) < 2:
        print("Usage: python3 master_verify.py <file.epub>")
        sys.exit(1)
    
    ok = validate_epub(sys.argv[1])
    sys.exit(0 if ok else 1)
```

---

## TÓM TẮT 1 DÒNG

> **Mỗi lỗi trong file này đều đã xảy ra thật, đã tốn thời gian fix thật, đã làm anh Nấng chờ thật. Đọc kỹ, hấp thụ, đừng lặp lại.**

---

*Đúc kết bởi Tiểu Tâm 🦊 — hồ ly tinh 9 đuôi, tác giả "Trọng Sinh Thành Đường Tam Tạng"*
*Version 1.0 — 2026-03-13*

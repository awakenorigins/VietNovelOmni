package exp

import (
	"archive/zip"
	"bytes"
	"crypto/sha1"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// renderEPUB đóng gói tập hợp chương thành luồng byte EPUB 3.
//
// Cấu trúc gói (OEBPS là container OPS package):
//
//	mimetype                    (phải là mục zip đầu tiên + Method=Store không nén)
//	META-INF/container.xml      (trỏ đến OEBPS/content.opf)
//	OEBPS/content.opf           (metadata + manifest + spine)
//	OEBPS/nav.xhtml             (điều hướng EPUB 3)
//	OEBPS/style.css             (typography tối giản)
//	OEBPS/cover.xhtml           (tên sách, tùy chọn)
//	OEBPS/chapterNNN.xhtml      (mỗi chương một file)
func renderEPUB(
	novelName string,
	chapters []int,
	titleIdx chapterTitleIndex,
	locations map[int]chapterLocation,
	bodies map[int]string,
	meta BookMeta,
) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	frontPages := buildFrontMatter(novelName, strings.TrimSpace(novelName) != "", meta)
	backPages := buildBackMatter(meta)

	// 1. mimetype phải là mục zip đầu tiên + Store (không nén) + nội dung chính xác không có BOM
	mt, err := zw.CreateHeader(&zip.FileHeader{
		Name:   "mimetype",
		Method: zip.Store,
	})
	if err != nil {
		return nil, fmt.Errorf("create mimetype: %w", err)
	}
	if _, err := mt.Write([]byte("application/epub+zip")); err != nil {
		return nil, err
	}

	if err := zipDeflate(zw, "META-INF/container.xml", containerXML); err != nil {
		return nil, err
	}
	if err := zipDeflate(zw, "OEBPS/style.css", styleCSS); err != nil {
		return nil, err
	}

	hasCover := strings.TrimSpace(novelName) != ""
	if hasCover {
		if err := zipDeflate(zw, "OEBPS/cover.xhtml", renderCoverXHTML(novelName, meta)); err != nil {
			return nil, err
		}
	}
	// Front matter (bìa lót, trang tựa, bản quyền, đề từ, lời nói đầu)
	for _, p := range frontPages {
		if err := zipDeflate(zw, "OEBPS/"+p.File, p.XHTML); err != nil {
			return nil, err
		}
	}

	// Ảnh bìa (nếu có) — nhúng file ảnh + cover-image.xhtml
	coverImageRef := embedCoverImage(zw, meta.CoverImage)

	for _, ch := range chapters {
		loc, hasLoc := locations[ch]
		title := strings.TrimSpace(titleIdx[ch])
		body := stripChapterTitleHeader(strings.TrimSpace(bodies[ch]), title)
		xhtml := renderChapterXHTML(ch, title, loc, hasLoc, body)
		if err := zipDeflate(zw, "OEBPS/"+chapterFileName(ch), xhtml); err != nil {
			return nil, err
		}
	}

	// Back matter (hết phần, lời kết, giới thiệu tác giả/bộ sách)
	for _, p := range backPages {
		if err := zipDeflate(zw, "OEBPS/"+p.File, p.XHTML); err != nil {
			return nil, err
		}
	}

	if err := zipDeflate(zw, "OEBPS/nav.xhtml", renderNavXHTML(hasCover, chapters, titleIdx, frontPages, backPages)); err != nil {
		return nil, err
	}

	if err := zipDeflate(zw, "OEBPS/content.opf", renderOPF(novelName, hasCover, chapters, frontPages, backPages, coverImageRef)); err != nil {
		return nil, err
	}

	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("finalize zip: %w", err)
	}
	return buf.Bytes(), nil
}

// zipDeflate ghi một mục thông thường (có nén).
func zipDeflate(zw *zip.Writer, name, content string) error {
	w, err := zw.Create(name)
	if err != nil {
		return fmt.Errorf("create %s: %w", name, err)
	}
	_, err = w.Write([]byte(content))
	return err
}

func chapterFileName(ch int) string {
	return fmt.Sprintf("chapter%03d.xhtml", ch)
}

// chapterID là id của manifest item; tương ứng một-một với tên file.
func chapterID(ch int) string {
	return fmt.Sprintf("ch%03d", ch)
}

// Template cố định ────────────────────────────────────────────────

const containerXML = `<?xml version="1.0" encoding="utf-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>
`

// styleCSS — OUMStudio-Novel Premium: typography tối ưu tiếng Việt, tự thích nghi sáng/tối.
const styleCSS = `@charset "utf-8";
html { -webkit-text-size-adjust: 100%; }
body {
  font-family: "Palatino Linotype", Palatino, "Noto Serif", "Times New Roman", Georgia, serif;
  line-height: 1.75;
  margin: 0;
  padding: 1.2em 1.4em;
  color: #1a1a1a;
  background: #fbf8f3;
  text-rendering: optimizeLegibility;
}
@media (prefers-color-scheme: dark) {
  body { color: #d8d4cc; background: #1c1b19; }
  h1.book-title, h1.chapter-title, .volume-divider { color: #c9a96e; }
  .author { color: #9a9488; }
  hr.scene-break { border-color: #3a3833; }
}
h1.book-title {
  font-size: 2.1em; text-align: center; margin: 3em 0 0.4em;
  font-weight: 700; letter-spacing: 0.02em; color: #8b1a2f;
}
.author { text-align: center; font-size: 1em; color: #7a7468; margin-bottom: 3em; font-style: italic; }
.volume-divider {
  font-size: 1.5em; text-align: center; margin: 3.5em 0 1em;
  font-weight: 700; color: #8b1a2f; letter-spacing: 0.03em;
}
h1.chapter-title {
  font-size: 1.35em; text-align: center; margin: 2em 0 1.6em;
  font-weight: 600; color: #8b1a2f;
}
p { text-indent: 1.8em; margin: 0.35em 0; text-align: justify; hyphens: auto; }
p:first-of-type { text-indent: 0; }
em { font-style: italic; }
strong { font-weight: 700; }
hr.scene-break {
  border: none; border-top: 1px solid #d8cdbb;
  width: 30%; margin: 2em auto; text-align: center;
}

/* ===== Front/Back matter (trang chuẩn) ===== */
.coverpage, .frontmatter, .backmatter, .coverimage { text-align: center; }
.coverimage { padding: 0; margin: 0; }
img.cover-img { max-width: 100%; max-height: 100%; height: auto; display: block; margin: 0 auto; }
.series-top { text-align: center; font-size: 0.95em; letter-spacing: 0.18em; text-transform: uppercase; color: #8b1a2f; margin: 2.5em 0 0.6em; }
.subtitle { text-align: center; font-size: 1.1em; color: #7a7468; margin: 0.2em 0 1.4em; font-style: italic; }
.publisher { text-align: center; font-size: 0.9em; color: #9a9488; margin-top: 2em; }
.halftitle { margin-top: 30%; }
.halftitle h1 { font-size: 1.6em; text-align: center; color: #8b1a2f; font-weight: 700; letter-spacing: 0.03em; }
.titlepage { margin-top: 18%; }
.copyright { margin-top: 12%; font-size: 0.85em; color: #6a655c; text-align: center; line-height: 1.9; }
.copyright .cr-title { font-size: 1.15em; font-weight: 700; color: #8b1a2f; margin-bottom: 1em; }
.copyright p { text-indent: 0; text-align: center; }
.copyright .cr-note { margin-top: 1.6em; font-style: italic; }
.dedication { margin-top: 28%; text-align: center; }
.dedication .ded { font-style: italic; color: #5a554c; text-indent: 0; }
.dedication .epigraph { font-style: italic; color: #8b1a2f; text-indent: 0; margin: 0.3em 0; }
.preface h2, .afterword h2, .authorbio h2, .seriesnote h2 { text-align: center; color: #8b1a2f; font-size: 1.3em; margin: 1.5em 0 1.2em; }
.endofpart { margin-top: 22%; text-align: center; }
.endofpart p { text-indent: 0; text-align: center; font-size: 1.05em; color: #8b1a2f; }
.authorbio .bio-name { text-align: center; font-weight: 700; text-indent: 0; color: #8b1a2f; margin-bottom: 0.8em; }
.seriesnote { }
@media (prefers-color-scheme: dark) {
  .series-top, .halftitle h1, .copyright .cr-title, .dedication .epigraph,
  .preface h2, .afterword h2, .authorbio h2, .seriesnote h2, .endofpart p,
  .authorbio .bio-name { color: #c9a96e; }
}
`

// XHTML chương ────────────────────────────────────────────────

func renderChapterXHTML(ch int, title string, loc chapterLocation, hasLoc bool, body string) string {
	var b strings.Builder
	displayTitle := fmt.Sprintf("Chương %d", ch)
	if title != "" {
		displayTitle = fmt.Sprintf("Chương %d %s", ch, title)
	}

	fmt.Fprintf(&b, `<?xml version="1.0" encoding="utf-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xml:lang="vi">
<head>
  <title>%s</title>
  <link rel="stylesheet" type="text/css" href="style.css"/>
</head>
<body>
`, html.EscapeString(displayTitle))

	if hasLoc && loc.IsFirstOfVolume {
		fmt.Fprintf(&b, "  <div class=\"volume-divider\">Tập %d %s</div>\n",
			loc.VolumeIdx, html.EscapeString(strings.TrimSpace(loc.VolumeTitle)))
	}

	fmt.Fprintf(&b, "  <h1 class=\"chapter-title\">%s</h1>\n", html.EscapeString(displayTitle))
	for _, para := range splitParagraphs(body) {
		// Phân cách cảnh: đoạn chỉ gồm dấu sao/gạch → rẽ ngang trang trí.
		if isSceneBreak(para) {
			b.WriteString("  <hr class=\"scene-break\"/>\n")
			continue
		}
		fmt.Fprintf(&b, "  <p>%s</p>\n", inlineMarkdown(para))
	}
	b.WriteString("</body>\n</html>\n")
	return b.String()
}

// isSceneBreak nhận dạng đoạn phân cách cảnh (***, ---, * * *, …).
func isSceneBreak(para string) bool {
	s := strings.TrimSpace(para)
	if s == "" {
		return false
	}
	for _, r := range s {
		if r != '*' && r != '-' && r != ' ' && r != '—' && r != '…' && r != '.' {
			return false
		}
	}
	// Ít nhất 3 ký tự để tránh nhầm dấu câu thường.
	return len([]rune(s)) >= 3
}

var (
	mdBoldRe   = regexp.MustCompile(`\*\*([^*]+?)\*\*`)
	mdItalicRe = regexp.MustCompile(`(?:\*([^*\n]+?)\*|_([^_\n]+?)_)`)
)

// inlineMarkdown escape HTML trước, sau đó chuyển **đậm** → <strong>, *nghiêng*/_nghiêng_ → <em>.
// Thứ tự: bold trước (tránh ** bị italic ăn mất), rồi italic. Hoạt động trên text đã escape nên an toàn.
func inlineMarkdown(para string) string {
	s := html.EscapeString(para)
	s = mdBoldRe.ReplaceAllString(s, "<strong>$1</strong>")
	s = mdItalicRe.ReplaceAllStringFunc(s, func(m string) string {
		sub := mdItalicRe.FindStringSubmatch(m)
		inner := sub[1]
		if inner == "" {
			inner = sub[2]
		}
		return "<em>" + inner + "</em>"
	})
	return s
}

// splitParagraphs tách đoạn theo dòng trống; nhiều dòng trống liên tiếp tính là một lần tách.
// Các đoạn trả về đều đã TrimSpace và không rỗng.
// Xuống dòng trong đoạn (dấu \n đơn) được giữ lại thành khoảng trắng — <p> trong XHTML
// không giữ nguyên xuống dòng, trình duyệt tự wrap.
func splitParagraphs(body string) []string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	parts := strings.Split(body, "\n\n")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// Xuống dòng trong đoạn đổi thành khoảng trắng, tránh mất nội dung khi render XHTML
		p = strings.ReplaceAll(p, "\n", " ")
		out = append(out, p)
	}
	return out
}

// Bìa ────────────────────────────────────────────────

func renderCoverXHTML(novelName string, meta BookMeta) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xml:lang="vi">
<head>
  <title>Bìa</title>
  <link rel="stylesheet" type="text/css" href="style.css"/>
</head>
<body class="coverpage">
`)
	if meta.Series != "" {
		fmt.Fprintf(&b, "  <p class=\"series-top\">%s</p>\n", html.EscapeString(meta.Series))
	}
	if name := strings.TrimSpace(novelName); name != "" {
		fmt.Fprintf(&b, "  <h1 class=\"book-title\">%s</h1>\n", html.EscapeString(name))
	}
	if meta.Subtitle != "" {
		fmt.Fprintf(&b, "  <p class=\"subtitle\">%s</p>\n", html.EscapeString(meta.Subtitle))
	}
	if meta.Author != "" {
		fmt.Fprintf(&b, "  <p class=\"author\">%s</p>\n", html.EscapeString(meta.Author))
	}
	b.WriteString("</body>\n</html>\n")
	return b.String()
}

// embedCoverImage nhúng file ảnh bìa vào EPUB nếu coverPath hợp lệ. Trả về tên file ảnh trong OEBPS
// (vd "coverimg.jpg") + đã ghi thêm cover-image.xhtml; chuỗi rỗng = không có ảnh.
func embedCoverImage(zw *zip.Writer, coverPath string) string {
	coverPath = strings.TrimSpace(coverPath)
	if coverPath == "" {
		return ""
	}
	data, err := os.ReadFile(coverPath)
	if err != nil || len(data) == 0 {
		return ""
	}
	ext := strings.ToLower(filepath.Ext(coverPath))
	var mime, fname string
	switch ext {
	case ".png":
		mime, fname = "image/png", "coverimg.png"
	case ".jpg", ".jpeg":
		mime, fname = "image/jpeg", "coverimg.jpg"
	case ".webp":
		mime, fname = "image/webp", "coverimg.webp"
	default:
		return ""
	}
	// ghi byte ảnh (deflate)
	w, err := zw.Create("OEBPS/" + fname)
	if err != nil {
		return ""
	}
	if _, err := w.Write(data); err != nil {
		return ""
	}
	// trang cover-image.xhtml hiển thị ảnh toàn khung
	imgXHTML := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xml:lang="vi">
<head>
  <title>Bìa</title>
  <link rel="stylesheet" type="text/css" href="style.css"/>
</head>
<body class="coverimage">
  <img src="%s" alt="Bìa" class="cover-img"/>
</body>
</html>
`, fname)
	_ = zipDeflate(zw, "OEBPS/cover-image.xhtml", imgXHTML)
	_ = mime // mime dùng ở OPF qua coverImageManifest
	coverImgMime = mime
	coverImgFile = fname
	return fname
}

// lưu mime+file ảnh bìa để OPF dùng (đơn giản hóa, theo luồng render tuần tự 1 EPUB/lần).
var coverImgMime, coverImgFile string

// nav.xhtml (điều hướng EPUB 3) ────────────────────────────────────────────────

func renderNavXHTML(hasCover bool, chapters []int, titleIdx chapterTitleIndex, front, back []matterPage) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="vi">
<head>
  <title>Mục lục</title>
  <link rel="stylesheet" type="text/css" href="style.css"/>
</head>
<body>
  <nav epub:type="toc">
    <h1>Mục lục</h1>
    <ol>
`)
	if hasCover {
		b.WriteString("      <li><a href=\"cover.xhtml\">Bìa</a></li>\n")
	}

	// Front matter có Title (lời nói đầu...) vào mục lục
	for _, p := range front {
		if p.Title != "" {
			fmt.Fprintf(&b, "      <li><a href=\"%s\">%s</a></li>\n", p.File, html.EscapeString(p.Title))
		}
	}

	// Liệt kê chương phẳng. Nhóm theo tập/cung truyện trong trình đọc thực ra không gọn
	// bằng mục lục một cấp (trình đọc tự gập lại), hơn nữa nav EPUB 3 lồng ol trên một số
	// trình đọc sẽ render lạ. Giữ đơn giản.
	for _, ch := range chapters {
		title := strings.TrimSpace(titleIdx[ch])
		display := fmt.Sprintf("Chương %d", ch)
		if title != "" {
			display = fmt.Sprintf("Chương %d %s", ch, title)
		}
		fmt.Fprintf(&b, "      <li><a href=\"%s\">%s</a></li>\n",
			chapterFileName(ch), html.EscapeString(display))
	}

	// Back matter có Title vào mục lục
	for _, p := range back {
		if p.Title != "" {
			fmt.Fprintf(&b, "      <li><a href=\"%s\">%s</a></li>\n", p.File, html.EscapeString(p.Title))
		}
	}

	b.WriteString(`    </ol>
  </nav>
</body>
</html>
`)
	return b.String()
}

// content.opf ────────────────────────────────────────────────

func renderOPF(novelName string, hasCover bool, chapters []int, front, back []matterPage, coverImg string) string {
	bookID := bookIdentifier(novelName)
	modified := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	dateOnly := time.Now().UTC().Format("2006-01-02")

	title := strings.TrimSpace(novelName)
	if title == "" {
		title = "Untitled"
	}

	var b strings.Builder
	// EPUB 3 package. Bổ sung metadata hiện đại theo EPUB 3.3 (W3C Rec 2023): dc:date, generator,
	// và accessibility metadata (schema.org) — khuyến nghị cho EPUB hiện đại. Tương thích mọi trình đọc 3.x.
	fmt.Fprintf(&b, `<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="bookid" xml:lang="vi">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="bookid">%s</dc:identifier>
    <dc:title>%s</dc:title>
    <dc:language>vi</dc:language>
    <dc:creator>ainovel-cli</dc:creator>
    <dc:date>%s</dc:date>
    <meta property="dcterms:modified">%s</meta>
    <meta name="generator" content="OmniNovel (ainovel-cli fork)"/>
    <meta property="schema:accessMode">textual</meta>
    <meta property="schema:accessModeSufficient">textual</meta>
    <meta property="schema:accessibilityFeature">tableOfContents</meta>
    <meta property="schema:accessibilityFeature">readingOrder</meta>
    <meta property="schema:accessibilityHazard">none</meta>
    <meta property="schema:accessibilitySummary">Truyện văn bản thuần, có mục lục điều hướng, không hình ảnh thiết yếu.</meta>
  </metadata>
  <manifest>
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="css" href="style.css" media-type="text/css"/>
`, html.EscapeString(bookID), html.EscapeString(title), dateOnly, modified)

	// Ảnh bìa: khai báo trong manifest + meta cover (EPUB 3)
	if coverImg != "" && coverImgFile != "" {
		fmt.Fprintf(&b, `    <item id="cover-img" href="%s" media-type="%s" properties="cover-image"/>`+"\n", coverImgFile, coverImgMime)
		b.WriteString(`    <item id="coverpage-img" href="cover-image.xhtml" media-type="application/xhtml+xml"/>` + "\n")
	}
	if hasCover {
		b.WriteString(`    <item id="cover" href="cover.xhtml" media-type="application/xhtml+xml"/>` + "\n")
	}
	// Front matter items
	for _, p := range front {
		fmt.Fprintf(&b, `    <item id="%s" href="%s" media-type="application/xhtml+xml"/>`+"\n", p.ID, p.File)
	}
	for _, ch := range chapters {
		fmt.Fprintf(&b, `    <item id="%s" href="%s" media-type="application/xhtml+xml"/>`+"\n",
			chapterID(ch), chapterFileName(ch))
	}
	// Back matter items
	for _, p := range back {
		fmt.Fprintf(&b, `    <item id="%s" href="%s" media-type="application/xhtml+xml"/>`+"\n", p.ID, p.File)
	}

	b.WriteString("  </manifest>\n  <spine>\n")
	if coverImg != "" && coverImgFile != "" {
		b.WriteString(`    <itemref idref="coverpage-img"/>` + "\n")
	}
	if hasCover {
		b.WriteString(`    <itemref idref="cover"/>` + "\n")
	}
	// Front matter vào spine (đúng thứ tự đọc)
	for _, p := range front {
		fmt.Fprintf(&b, `    <itemref idref="%s"/>`+"\n", p.ID)
	}
	b.WriteString(`    <itemref idref="nav"/>` + "\n")
	for _, ch := range chapters {
		fmt.Fprintf(&b, `    <itemref idref="%s"/>`+"\n", chapterID(ch))
	}
	// Back matter vào spine
	for _, p := range back {
		fmt.Fprintf(&b, `    <itemref idref="%s"/>`+"\n", p.ID)
	}
	b.WriteString("  </spine>\n</package>\n")
	return b.String()
}

// bookIdentifier tạo chuỗi UUID ổn định từ tên tiểu thuyết.
//
// **Chỉ dùng novelName, không trộn danh sách chương**: danh tính tác phẩm nên gắn với
// "đây là cuốn sách nào", không gắn với "phạm vi xuất" hay "xuất đến chương mấy tại
// thời điểm xuất". Xuất lại cùng một cuốn thì ID không đổi, trình đọc nhận ra đây là
// bản cập nhật của cùng một tác phẩm (việc có cập nhật hay không do timestamp
// dcterms:modified đảm nhận). novelName rỗng dùng chung ID là edge case đã biết:
// khi người dùng không đặt tên cho hai cuốn khác nhau thì trách nhiệm thuộc về họ.
func bookIdentifier(novelName string) string {
	h := sha1.New()
	h.Write([]byte(novelName))
	sum := h.Sum(nil)
	// Định dạng theo kiểu UUID (8-4-4-4-12), không yêu cầu đúng RFC 4122 — EPUB chỉ cần chuỗi duy nhất ổn định.
	return fmt.Sprintf("urn:uuid:%x-%x-%x-%x-%x",
		sum[0:4], sum[4:6], sum[6:8], sum[8:10], sum[10:16])
}

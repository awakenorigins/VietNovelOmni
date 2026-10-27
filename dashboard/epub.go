// epub.go — trình đóng gói EPUB 3 thuần Go stdlib cho các truyện KHÔNG có store format engine.
// Sinh EPUB hợp lệ: mimetype (stored, entry đầu) + container.xml + OPF + nav.xhtml + XHTML chương
// + trang bìa chữ, trang tựa, bản quyền, lời tựa, lời bạt, về tác giả.
package main

import (
	"archive/zip"
	"crypto/rand"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type epubMeta struct {
	Title     string
	Subtitle  string
	Author    string
	Series    string
	Preface   string
	Afterword string
	AuthorBio string
	Copyright string
}

type epubChapter struct {
	Num   int
	Title string
	Body  string // markdown đơn giản
}

func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// mdToXHTML chuyển markdown đơn giản thành các đoạn XHTML an toàn.
// Hỗ trợ: heading (#..) bị bỏ qua nếu trùng tiêu đề chương, đoạn cách dòng trống, --- thành hr.
func mdToXHTML(md string, skipFirstHeading bool) string {
	var sb strings.Builder
	paras := strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n\n")
	first := true
	for _, p := range paras {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if p == "---" || p == "***" {
			sb.WriteString("<hr/>\n")
			continue
		}
		if strings.HasPrefix(p, "#") {
			line := strings.TrimSpace(strings.TrimLeft(p, "# "))
			if first && skipFirstHeading {
				first = false
				continue
			}
			sb.WriteString("<h3>" + html.EscapeString(line) + "</h3>\n")
			first = false
			continue
		}
		first = false
		// gộp dòng đơn trong đoạn
		lines := strings.Split(p, "\n")
		var parts []string
		for _, l := range lines {
			l = strings.TrimSpace(l)
			if l != "" {
				parts = append(parts, html.EscapeString(l))
			}
		}
		sb.WriteString("<p>" + strings.Join(parts, "<br/>") + "</p>\n")
	}
	return sb.String()
}

func xhtmlPage(title, bodyHTML string) string {
	return `<?xml version="1.0" encoding="utf-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="vi" lang="vi">
<head><meta charset="utf-8"/><title>` + html.EscapeString(title) + `</title>
<link rel="stylesheet" type="text/css" href="style.css"/></head>
<body>
` + bodyHTML + `</body>
</html>
`
}

const epubCSS = `body { font-family: serif; line-height: 1.7; margin: 5% 6%; color:#2C2C2C; }
h1,h2,h3 { font-family: sans-serif; line-height:1.3; }
h1 { font-size:1.7em; } h2 { font-size:1.3em; margin-top:2em; }
p { margin: 0 0 .2em; text-indent: 1.4em; }
hr { border:0; text-align:center; margin:1.6em 0; }
hr:after { content:"◆ ◆ ◆"; color:#C9A96E; letter-spacing:.6em; }
.cover { text-align:center; margin-top:22%; }
.cover h1 { font-size:2.1em; border:none; }
.cover .sub { color:#6b6459; font-size:1.05em; margin-top:.6em; }
.cover .author { margin-top:3em; font-size:1.15em; letter-spacing:.15em; }
.cover .rule { color:#C9A96E; margin:1.2em 0; letter-spacing:.5em; }
.front p { text-indent:0; }
.copyright { margin-top:40%; font-size:.85em; color:#555; text-align:center; }
.copyright p { text-indent:0; }
nav ol { list-style:none; padding-left:0; } nav li { margin:.35em 0; }
`

// buildEPUB đóng gói EPUB 3 hợp lệ. outPath ghi đè nếu tồn tại.
func buildEPUB(outPath string, meta epubMeta, chapters []epubChapter) error {
	if meta.Title == "" {
		meta.Title = "Không tên"
	}
	if len(chapters) == 0 {
		return fmt.Errorf("không có chương nào để xuất")
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)

	// 1) mimetype: entry đầu, KHÔNG nén.
	mw, err := zw.CreateHeader(&zip.FileHeader{Name: "mimetype", Method: zip.Store})
	if err != nil {
		return err
	}
	if _, err := mw.Write([]byte("application/epub+zip")); err != nil {
		return err
	}

	add := func(name, content string) error {
		w, err := zw.Create(name)
		if err != nil {
			return err
		}
		_, err = w.Write([]byte(content))
		return err
	}

	// 2) container.xml
	if err := add("META-INF/container.xml", `<?xml version="1.0" encoding="utf-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>
`); err != nil {
		return err
	}

	if err := add("OEBPS/style.css", epubCSS); err != nil {
		return err
	}

	type item struct{ id, href, title string }
	var spine []item

	// Bìa chữ
	coverBody := `<div class="cover"><h1>` + html.EscapeString(meta.Title) + `</h1>`
	if meta.Subtitle != "" {
		coverBody += `<div class="sub">` + html.EscapeString(meta.Subtitle) + `</div>`
	}
	if meta.Series != "" {
		coverBody += `<div class="sub">` + html.EscapeString(meta.Series) + `</div>`
	}
	coverBody += `<div class="rule">◆ ◆ ◆</div>`
	if meta.Author != "" {
		coverBody += `<div class="author">` + html.EscapeString(meta.Author) + `</div>`
	}
	coverBody += `</div>`
	if err := add("OEBPS/cover.xhtml", xhtmlPage(meta.Title, coverBody)); err != nil {
		return err
	}
	spine = append(spine, item{"cover", "cover.xhtml", "Bìa"})

	// Trang tựa
	titleBody := `<div class="cover"><h1>` + html.EscapeString(meta.Title) + `</h1>`
	if meta.Subtitle != "" {
		titleBody += `<div class="sub">` + html.EscapeString(meta.Subtitle) + `</div>`
	}
	if meta.Author != "" {
		titleBody += `<div class="author">` + html.EscapeString(meta.Author) + `</div>`
	}
	titleBody += `</div>`
	if err := add("OEBPS/titlepage.xhtml", xhtmlPage("Trang tựa", titleBody)); err != nil {
		return err
	}
	spine = append(spine, item{"titlepage", "titlepage.xhtml", "Trang tựa"})

	// Bản quyền
	cop := meta.Copyright
	if cop == "" {
		who := meta.Author
		if who == "" {
			who = "Tác giả"
		}
		cop = fmt.Sprintf("© %d %s. Mọi quyền được bảo lưu.", time.Now().Year(), who)
	}
	if err := add("OEBPS/copyright.xhtml", xhtmlPage("Bản quyền",
		`<div class="copyright"><p>`+html.EscapeString(cop)+`</p><p>Xuất bản bởi VietNovelOmni Studio.</p></div>`)); err != nil {
		return err
	}
	spine = append(spine, item{"copyright", "copyright.xhtml", "Bản quyền"})

	if meta.Preface != "" {
		if err := add("OEBPS/preface.xhtml", xhtmlPage("Lời tựa",
			`<div class="front"><h2>Lời tựa</h2>`+mdToXHTML(meta.Preface, false)+`</div>`)); err != nil {
			return err
		}
		spine = append(spine, item{"preface", "preface.xhtml", "Lời tựa"})
	}

	for _, ch := range chapters {
		name := fmt.Sprintf("ch%03d.xhtml", ch.Num)
		title := ch.Title
		if title == "" {
			title = fmt.Sprintf("Chương %d", ch.Num)
		}
		body := `<h2>` + html.EscapeString(title) + `</h2>` + "\n" + mdToXHTML(ch.Body, true)
		if err := add("OEBPS/"+name, xhtmlPage(title, body)); err != nil {
			return err
		}
		spine = append(spine, item{fmt.Sprintf("ch%03d", ch.Num), name, title})
	}

	if meta.Afterword != "" {
		if err := add("OEBPS/afterword.xhtml", xhtmlPage("Lời bạt",
			`<div class="front"><h2>Lời bạt</h2>`+mdToXHTML(meta.Afterword, false)+`</div>`)); err != nil {
			return err
		}
		spine = append(spine, item{"afterword", "afterword.xhtml", "Lời bạt"})
	}
	if meta.AuthorBio != "" {
		if err := add("OEBPS/about.xhtml", xhtmlPage("Về tác giả",
			`<div class="front"><h2>Về tác giả</h2>`+mdToXHTML(meta.AuthorBio, false)+`</div>`)); err != nil {
			return err
		}
		spine = append(spine, item{"about", "about.xhtml", "Về tác giả"})
	}

	// nav.xhtml
	var navList strings.Builder
	for _, it := range spine {
		navList.WriteString(`      <li><a href="` + it.href + `">` + html.EscapeString(it.title) + "</a></li>\n")
	}
	nav := `<?xml version="1.0" encoding="utf-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="vi" lang="vi">
<head><meta charset="utf-8"/><title>Mục lục</title>
<link rel="stylesheet" type="text/css" href="style.css"/></head>
<body>
  <nav epub:type="toc" id="toc">
    <h2>Mục lục</h2>
    <ol>
` + navList.String() + `    </ol>
  </nav>
</body>
</html>
`
	if err := add("OEBPS/nav.xhtml", nav); err != nil {
		return err
	}

	// content.opf
	uid := newUUID()
	now := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	var manifest, spineXML strings.Builder
	manifest.WriteString(`    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>` + "\n")
	manifest.WriteString(`    <item id="css" href="style.css" media-type="text/css"/>` + "\n")
	for _, it := range spine {
		manifest.WriteString(`    <item id="` + it.id + `" href="` + it.href + `" media-type="application/xhtml+xml"/>` + "\n")
		spineXML.WriteString(`    <itemref idref="` + it.id + `"/>` + "\n")
	}
	creator := ""
	if meta.Author != "" {
		creator = `    <dc:creator id="creator">` + html.EscapeString(meta.Author) + `</dc:creator>` + "\n"
	}
	series := ""
	if meta.Series != "" {
		series = `    <meta property="belongs-to-collection" id="series">` + html.EscapeString(meta.Series) + `</meta>` + "\n"
	}
	opf := `<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="pub-id" xml:lang="vi">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="pub-id">urn:uuid:` + uid + `</dc:identifier>
    <dc:title>` + html.EscapeString(meta.Title) + `</dc:title>
    <dc:language>vi</dc:language>
` + creator + series + `    <meta property="dcterms:modified">` + now + `</meta>
  </metadata>
  <manifest>
` + manifest.String() + `  </manifest>
  <spine>
` + spineXML.String() + `  </spine>
</package>
`
	if err := add("OEBPS/content.opf", opf); err != nil {
		return err
	}
	return zw.Close()
}

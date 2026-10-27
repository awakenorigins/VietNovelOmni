package exp

import (
	"fmt"
	"html"
	"strings"
)

// matterPage là một trang front/back matter đã render: id (cho manifest/spine), tên file, nội dung XHTML.
type matterPage struct {
	ID    string
	File  string
	Title string // tiêu đề nav (rỗng = không đưa vào nav)
	XHTML string
}

// xhtmlDoc bọc body thành 1 tài liệu XHTML EPUB hợp lệ.
func xhtmlDoc(title, bodyClass, body string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xml:lang="vi">
<head>
  <title>%s</title>
  <link rel="stylesheet" type="text/css" href="style.css"/>
</head>
<body class="%s">
%s
</body>
</html>
`, html.EscapeString(title), bodyClass, body)
}

// paras chuyển text nhiều đoạn (cách nhau dòng trống) thành các thẻ <p>.
func paras(text string) string {
	var b strings.Builder
	for _, p := range strings.Split(strings.TrimSpace(text), "\n\n") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// gộp xuống dòng đơn trong cùng đoạn thành khoảng trắng
		p = strings.ReplaceAll(p, "\n", " ")
		fmt.Fprintf(&b, "  <p>%s</p>\n", html.EscapeString(p))
	}
	return b.String()
}

// buildFrontMatter dựng các trang đầu sách theo metadata. Trang nào thiếu dữ liệu thì bỏ qua.
// Thứ tự chuẩn: cover → half-title → title page → copyright → dedication/epigraph → preface.
func buildFrontMatter(novelName string, hasCover bool, m BookMeta) []matterPage {
	var pages []matterPage

	// 1. Half-title (bìa lót): chỉ tên sách, tối giản
	if strings.TrimSpace(novelName) != "" {
		body := fmt.Sprintf("  <div class=\"halftitle\">\n    <h1>%s</h1>\n  </div>\n", html.EscapeString(novelName))
		pages = append(pages, matterPage{ID: "halftitle", File: "halftitle.xhtml", XHTML: xhtmlDoc("Bìa lót", "frontmatter", body)})
	}

	// 2. Title page (trang tựa): tên sách + phụ đề + tác giả + series + NXB
	{
		var b strings.Builder
		b.WriteString("  <div class=\"titlepage\">\n")
		if m.Series != "" {
			fmt.Fprintf(&b, "    <p class=\"series-top\">%s</p>\n", html.EscapeString(m.Series))
		}
		if strings.TrimSpace(novelName) != "" {
			fmt.Fprintf(&b, "    <h1 class=\"book-title\">%s</h1>\n", html.EscapeString(novelName))
		}
		if m.Subtitle != "" {
			fmt.Fprintf(&b, "    <p class=\"subtitle\">%s</p>\n", html.EscapeString(m.Subtitle))
		}
		if m.Author != "" {
			fmt.Fprintf(&b, "    <p class=\"author\">%s</p>\n", html.EscapeString(m.Author))
		}
		if m.Publisher != "" {
			fmt.Fprintf(&b, "    <p class=\"publisher\">%s</p>\n", html.EscapeString(m.Publisher))
		}
		b.WriteString("  </div>\n")
		pages = append(pages, matterPage{ID: "titlepage", File: "titlepage.xhtml", XHTML: xhtmlDoc("Trang tựa", "frontmatter", b.String())})
	}

	// 3. Copyright (bản quyền)
	if m.Copyright != "" || m.Publisher != "" || m.Author != "" {
		var b strings.Builder
		b.WriteString("  <div class=\"copyright\">\n")
		if strings.TrimSpace(novelName) != "" {
			fmt.Fprintf(&b, "    <p class=\"cr-title\">%s</p>\n", html.EscapeString(novelName))
		}
		if m.Author != "" {
			fmt.Fprintf(&b, "    <p>Tác giả: %s</p>\n", html.EscapeString(m.Author))
		}
		if m.Series != "" {
			fmt.Fprintf(&b, "    <p>Thuộc bộ sách: %s</p>\n", html.EscapeString(m.Series))
		}
		if m.Copyright != "" {
			fmt.Fprintf(&b, "    <p>%s</p>\n", html.EscapeString(m.Copyright))
		}
		if m.Publisher != "" {
			fmt.Fprintf(&b, "    <p>%s</p>\n", html.EscapeString(m.Publisher))
		}
		b.WriteString("    <p class=\"cr-note\">Tác phẩm hư cấu. Mọi nhân vật, sự kiện và bối cảnh đều do tác giả sáng tạo.</p>\n")
		b.WriteString("  </div>\n")
		pages = append(pages, matterPage{ID: "copyright", File: "copyright.xhtml", XHTML: xhtmlDoc("Bản quyền", "frontmatter", b.String())})
	}

	// 4. Dedication / Epigraph (đề từ / lời đề tặng)
	if m.Dedication != "" || m.Tagline != "" {
		var b strings.Builder
		b.WriteString("  <div class=\"dedication\">\n")
		if m.Dedication != "" {
			fmt.Fprintf(&b, "    <p class=\"ded\">%s</p>\n", html.EscapeString(m.Dedication))
		}
		if m.Tagline != "" {
			// tagline có thể nhiều dòng (\n)
			for _, line := range strings.Split(m.Tagline, "\n") {
				line = strings.TrimSpace(line)
				if line != "" {
					fmt.Fprintf(&b, "    <p class=\"epigraph\">%s</p>\n", html.EscapeString(line))
				}
			}
		}
		b.WriteString("  </div>\n")
		pages = append(pages, matterPage{ID: "dedication", File: "dedication.xhtml", XHTML: xhtmlDoc("Đề từ", "frontmatter", b.String())})
	}

	// 5. Preface (lời nói đầu)
	if strings.TrimSpace(m.Preface) != "" {
		body := "  <div class=\"preface\">\n    <h2>Lời nói đầu</h2>\n" + paras(m.Preface) + "  </div>\n"
		pages = append(pages, matterPage{ID: "preface", File: "preface.xhtml", Title: "Lời nói đầu", XHTML: xhtmlDoc("Lời nói đầu", "frontmatter", body)})
	}

	return pages
}

// buildBackMatter dựng các trang cuối sách. Thứ tự: end-of-part → afterword → author bio → series note.
func buildBackMatter(m BookMeta) []matterPage {
	var pages []matterPage

	// 1. Hết Phần N + teaser
	if strings.TrimSpace(m.EndOfPart) != "" {
		body := "  <div class=\"endofpart\">\n" + paras(m.EndOfPart) + "  </div>\n"
		pages = append(pages, matterPage{ID: "endofpart", File: "endofpart.xhtml", Title: "Hết phần", XHTML: xhtmlDoc("Hết phần", "backmatter", body)})
	}

	// 2. Afterword (lời kết / hậu ký)
	if strings.TrimSpace(m.Afterword) != "" {
		body := "  <div class=\"afterword\">\n    <h2>Lời kết</h2>\n" + paras(m.Afterword) + "  </div>\n"
		pages = append(pages, matterPage{ID: "afterword", File: "afterword.xhtml", Title: "Lời kết", XHTML: xhtmlDoc("Lời kết", "backmatter", body)})
	}

	// 3. Giới thiệu tác giả
	if strings.TrimSpace(m.AuthorBio) != "" {
		body := "  <div class=\"authorbio\">\n    <h2>Về tác giả</h2>\n"
		if m.Author != "" {
			body += fmt.Sprintf("    <p class=\"bio-name\">%s</p>\n", html.EscapeString(m.Author))
		}
		body += paras(m.AuthorBio) + "  </div>\n"
		pages = append(pages, matterPage{ID: "authorbio", File: "authorbio.xhtml", Title: "Về tác giả", XHTML: xhtmlDoc("Về tác giả", "backmatter", body)})
	}

	// 4. Giới thiệu bộ sách
	if strings.TrimSpace(m.SeriesNote) != "" {
		body := "  <div class=\"seriesnote\">\n"
		if m.Series != "" {
			body += fmt.Sprintf("    <h2>%s</h2>\n", html.EscapeString(m.Series))
		} else {
			body += "    <h2>Bộ sách</h2>\n"
		}
		body += paras(m.SeriesNote) + "  </div>\n"
		pages = append(pages, matterPage{ID: "seriesnote", File: "seriesnote.xhtml", Title: "Bộ sách", XHTML: xhtmlDoc("Bộ sách", "backmatter", body)})
	}

	return pages
}

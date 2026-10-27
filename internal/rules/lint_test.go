package rules

import (
	"strings"
	"testing"
)

func TestLint_CleanText(t *testing.T) {
	// Văn xuôi tiếng Việt sạch (chữ Latin có dấu) phải pass — đây là điểm khác cốt lõi so với bản gốc tiếng Trung.
	if vs := Lint("# Chương 1 Gió Nổi\nAnh sải bước về phía trước.\nMàn đêm dần buông."); len(vs) != 0 {
		t.Errorf("clean Vietnamese text should pass: %+v", vs)
	}
}

func TestLint_MarkdownResidue(t *testing.T) {
	text := "# Chương 1\nĐây là nội dung **trọng điểm**.\n## Tiểu mục\nChính văn."
	vs := Lint(text)
	bold := findViolation(vs, "markdown_residue", "**")
	if bold == nil || bold.Actual != 2 {
		t.Errorf("expected ** residue x2: %+v", vs)
	}
	heading := findViolation(vs, "markdown_residue", "#")
	if heading == nil || heading.Actual != 1 {
		t.Errorf("expected 1 heading beyond first line: %+v", vs)
	}
}

func TestLint_EnglishWords(t *testing.T) {
	// Từ tiếng Anh thật (the, journey) lọt vào prose tiếng Việt → error.
	text := "# Chương 1\nĐây là một journey tuyệt vời, the kết thúc đẹp."
	vs := Lint(text)
	var v *Violation
	for i := range vs {
		if vs[i].Rule == "english_words" {
			v = &vs[i]
			break
		}
	}
	if v == nil {
		t.Fatalf("expected english_words violation: %+v", vs)
	}
	if v.Actual != 2 {
		t.Errorf("total count: got %v want 2", v.Actual)
	}
	if !strings.Contains(v.Target, "journey") || !strings.Contains(v.Target, "the") {
		t.Errorf("examples should include detected words: %q", v.Target)
	}
	if v.Severity != SeverityError {
		t.Errorf("severity: %v", v.Severity)
	}
}

func TestLint_VietnameseNotFlaggedAsEnglish(t *testing.T) {
	// Chữ Việt KHÔNG dấu thường gặp (con, ban, tan, han...) KHÔNG được tính là tiếng Anh.
	text := "Con thuyen tro ve ben song, ban tay anh nam chat."
	vs := Lint(text)
	for _, v := range vs {
		if v.Rule == "english_words" {
			t.Errorf("Vietnamese-without-diacritics must not be flagged as English: %+v", v)
		}
	}
}

func TestLint_EmDash(t *testing.T) {
	text := "Anh bước vào — căn phòng tối om — không một tiếng động."
	vs := Lint(text)
	var v *Violation
	for i := range vs {
		if vs[i].Rule == "em_dash" {
			v = &vs[i]
			break
		}
	}
	if v == nil {
		t.Fatalf("expected em_dash violation: %+v", vs)
	}
	if v.Actual != 2 {
		t.Errorf("em dash count: got %v want 2", v.Actual)
	}
	if v.Severity != SeverityError {
		t.Errorf("severity should be error: %v", v.Severity)
	}
}

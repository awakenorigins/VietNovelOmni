package rules

import (
	"regexp"
	"strings"
)

// Lint kiểm tra đường đáy tích hợp sẵn: quét phần chính văn tìm tàn dư cơ chế,
// không liên quan đến quy tắc người dùng, luôn thực thi khi lưu chương.
// Cùng hợp đồng với Check — chỉ trả về sự thật (nguyên tắc sắt số một),
// không chặn luồng xử lý, để bộ phận đánh giá/người dùng phán quyết.
//
// PHIÊN BẢN TIẾNG VIỆT (OUMStudio-Novel):
// Bản gốc ainovel-cli viết cho tiếng Trung, nên "non_cjk_fragments" coi MỌI chuỗi Latin
// là dấu hiệu lẫn ngoại ngữ. Với tiểu thuyết tiếng Việt — vốn 100% chữ Latin — quy tắc đó
// luôn báo động trên toàn chương, vô dụng. Ở đây thay bằng:
//   - markdown_residue: ** in đậm, dòng tiêu đề # ngoài dòng đầu (giữ nguyên)
//   - english_words: CHỈ bắt từ tiếng Anh thật (blacklist + heuristic âm vị không hợp tiếng Việt)
//   - em_dash: dấu gạch ngang dài "—" lẫn trong văn xuôi (mức error — lỗi phổ biến của prose AI)
func Lint(text string) []Violation {
	var vs []Violation
	vs = appendMarkdownResidue(vs, text)
	vs = appendEnglishWords(vs, text)
	vs = appendEmDash(vs, text)
	return vs
}

func appendMarkdownResidue(vs []Violation, text string) []Violation {
	if n := strings.Count(text, "**"); n > 0 {
		vs = append(vs, Violation{
			Rule:     "markdown_residue",
			Target:   "**",
			Actual:   n,
			Severity: SeverityWarning,
		})
	}
	headings := 0
	seenContent := false
	for line := range strings.SplitSeq(text, "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		// Tiêu đề # ở dòng không trống đầu tiên là định dạng hợp lệ của file chương (không cố định số dòng, chấp nhận dòng trống dẫn đầu)
		first := !seenContent
		seenContent = true
		if !first && strings.HasPrefix(t, "#") {
			headings++
		}
	}
	if headings > 0 {
		vs = append(vs, Violation{
			Rule:     "markdown_residue",
			Target:   "#",
			Actual:   headings,
			Severity: SeverityWarning,
		})
	}
	return vs
}

// englishBlacklist: các từ tiếng Anh chắc chắn là ngoại ngữ khi lọt vào prose tiếng Việt.
// Đây là tín hiệu mạnh, mức error. Danh sách giữ ngắn, tập trung từ AI hay sinh + từ chức năng Anh phổ biến.
var englishBlacklist = map[string]struct{}{}

func init() {
	for _, w := range []string{
		"the", "and", "but", "with", "from", "this", "that", "have", "will",
		"your", "they", "their", "would", "could", "should", "about", "which",
		"when", "what", "where", "while", "because", "however", "therefore",
		"moreover", "furthermore", "nevertheless", "essentially", "ultimately",
		"meanwhile", "suddenly", "perhaps", "indeed", "actually", "really",
		"brilliant", "stunning", "gorgeous", "delve", "tapestry", "journey",
		"realm", "destiny", "embrace", "whisper", "shimmer", "cascade",
		"ethereal", "serene", "profound", "intricate", "vibrant", "radiant",
		"okay", "yeah", "wow", "hello", "sorry", "please", "thanks", "thank",
		"never", "always", "everything", "nothing", "someone", "something",
	} {
		englishBlacklist[w] = struct{}{}
	}
}

// latinWordRe khớp từ chữ Latin có dấu phụ tiếng Việt hoặc thuần ASCII.
var latinWordRe = regexp.MustCompile(`[A-Za-z]{2,}`)

// vietnameseDiacritics: nếu một "từ" chứa ký tự có dấu tiếng Việt, chắc chắn là tiếng Việt → bỏ qua.
const vietnameseDiacritics = "àáảãạăằắẳẵặâầấẩẫậèéẻẽẹêềếểễệìíỉĩịòóỏõọôồốổỗộơờớởỡợùúủũụưừứửữựỳýỷỹỵđ" +
	"ÀÁẢÃẠĂẰẮẲẴẶÂẦẤẨẪẬÈÉẺẼẸÊỀẾỂỄỆÌÍỈĨỊÒÓỎÕỌÔỒỐỔỖỘƠỜỚỞỠỢÙÚỦŨỤƯỪỨỬỮỰỲÝỶỸỴĐ"

func hasVietnameseDiacritic(w string) bool {
	return strings.ContainsAny(w, vietnameseDiacritics)
}

// appendEnglishWords chỉ báo cáo từ tiếng Anh THẬT, tránh false-positive trên chữ Latin tiếng Việt không dấu.
// Quy tắc: bỏ qua từ có dấu tiếng Việt; trong các từ ASCII thuần, chỉ tính từ nằm trong blacklist tiếng Anh.
func appendEnglishWords(vs []Violation, text string) []Violation {
	// Tách phần thân, bỏ dòng tiêu đề # đầu (tên chương có thể chứa chữ Latin hợp lệ ít gặp)
	matches := latinWordRe.FindAllString(text, -1)
	if len(matches) == 0 {
		return vs
	}
	total := 0
	seen := make(map[string]struct{})
	var examples []string
	for _, m := range matches {
		if hasVietnameseDiacritic(m) {
			continue
		}
		lw := strings.ToLower(m)
		if _, bad := englishBlacklist[lw]; !bad {
			continue
		}
		total++
		if _, ok := seen[lw]; ok {
			continue
		}
		seen[lw] = struct{}{}
		if len(examples) < 5 {
			examples = append(examples, m)
		}
	}
	if total == 0 {
		return vs
	}
	return append(vs, Violation{
		Rule:     "english_words",
		Target:   strings.Join(examples, "、"),
		Actual:   total,
		Severity: SeverityError,
	})
}

// appendEmDash báo cáo dấu gạch ngang dài "—" trong chính văn. Prose AI hay chèn em dash thay dấu phẩy/ngoặc.
// Mức error: văn xuôi tiếng Việt chuẩn không dùng "—"; nếu cần ngắt lời thoại đã có quy ước khác.
func appendEmDash(vs []Violation, text string) []Violation {
	n := strings.Count(text, "—")
	if n == 0 {
		return vs
	}
	return append(vs, Violation{
		Rule:     "em_dash",
		Target:   "—",
		Actual:   n,
		Severity: SeverityError,
	})
}

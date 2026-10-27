package tools

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/voocel/ainovel-cli/internal/rules"
)

// story_guards.go — VietNovelOmni: guard biên kịch mức chương, loop-safe by design.
//
// Triết lý: các guard ở đây CHỈ trả về sự thật (violation mức warning) hoặc chuỗi
// cảnh báo đưa vào kết quả commit — KHÔNG chặn cứng. Chặn cứng heuristic văn chương
// dễ tạo fail-loop (model không thoát nổi tiêu chí mơ hồ); thay vào đó tín hiệu được
// Editor 7 chiều + audit-feedback-loop tiêu thụ, nơi đã có sẵn cơ chế rewrite có giới hạn.

// hookMonotonyThreshold: cùng một loại hook lặp liên tiếp đủ N chương (tính cả chương
// đang commit) thì phát cảnh báo đơn điệu.
const hookMonotonyThreshold = 3

// hookMonotonyWarning kiểm tra hook_type chương mới có làm chuỗi lặp cùng loại
// đạt ngưỡng đơn điệu không. history là HookHistory các chương đã hoàn thành
// (theo thứ tự chương), newHook là hook_type chương đang commit.
// Trả về chuỗi rỗng nếu không có vấn đề.
func hookMonotonyWarning(history []string, newHook string) string {
	nh := strings.TrimSpace(strings.ToLower(newHook))
	if nh == "" {
		return ""
	}
	run := 1 // chương đang commit
	for i := len(history) - 1; i >= 0; i-- {
		h := strings.TrimSpace(strings.ToLower(history[i]))
		if h == "" || h != nh {
			break
		}
		run++
	}
	if run < hookMonotonyThreshold {
		return ""
	}
	return fmt.Sprintf(
		"hook đơn điệu: %d chương liên tiếp cùng loại móc %q — độc giả sẽ nhờn. Chương sau đổi loại móc (crisis/mystery/desire/emotion/choice luân phiên).",
		run, nh)
}

// weakOpeningPatterns: các mở đầu sáo mòn giết tỷ lệ giữ chân 15 giây đầu của web-novel.
// Chỉ áp cho CHƯƠNG 1 (cửa sinh tử của truyện); các chương sau đã có style_stats.opening_time_rate lo.
var weakOpeningPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^(buổi\s+)?(sáng|trưa|chiều|tối|đêm)\s`),
	regexp.MustCompile(`(?i)(tỉnh dậy|giật mình tỉnh|mở mắt ra|thức giấc|choàng tỉnh)`),
	regexp.MustCompile(`(?i)^(mặt trời|ánh nắng|bầu trời|cơn mưa|tiếng chuông báo thức)`),
	regexp.MustCompile(`(?i)^(ngày\s+(xửa|xưa)|thuở\s+xưa|từ\s+rất\s+lâu)`),
}

// firstChapterHookIssues đánh giá đoạn mở của CHƯƠNG 1 theo chuẩn giữ chân web-novel:
//   - đoạn mở không được rơi vào mẫu sáo (tỉnh dậy / thời tiết / cổ tích)
//   - trong ~200 từ đầu phải có tín hiệu chuyển động (đối thoại, hành động, xung đột,
//     câu hỏi) thay vì tả tĩnh thuần
//
// Trả về danh sách Violation mức warning (advisory) — Editor sẽ trích dẫn và ép sửa
// qua vòng review, không chặn commit trực tiếp.
func firstChapterHookIssues(content string) []rules.Violation {
	body := stripLeadingTitle(content)
	var vs []rules.Violation

	firstPara := firstNonEmptyParagraph(body)
	for _, re := range weakOpeningPatterns {
		if loc := re.FindStringIndex(firstPara); loc != nil {
			vs = append(vs, rules.Violation{
				Rule:     "ch1_weak_opening",
				Target:   strings.TrimSpace(firstPara[loc[0]:min(loc[1], len(firstPara))]),
				Actual:   1,
				Severity: rules.SeverityWarning,
			})
			break
		}
	}

	head := firstNWords(body, 200)
	hasMotion := strings.ContainsAny(head, "“”\"?!") ||
		strings.Contains(head, "?") ||
		regexp.MustCompile(`(?i)(nói|hét|gào|quát|hỏi|chạy|lao|đấm|giáng|siết|né|rút|bắn|vồ|xé|đập|gãy|máu|chết|nổ)`).MatchString(head)
	if !hasMotion {
		vs = append(vs, rules.Violation{
			Rule:     "ch1_static_opening",
			Target:   "200 từ đầu thiếu chuyển động (không đối thoại/hành động/xung đột/câu hỏi)",
			Actual:   1,
			Severity: rules.SeverityWarning,
		})
	}
	return vs
}

// stripLeadingTitle bỏ dòng tiêu đề "# ..." đầu file (nếu có) để soi thẳng chính văn.
func stripLeadingTitle(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if t == "" {
			continue
		}
		if strings.HasPrefix(t, "#") {
			return strings.Join(lines[i+1:], "\n")
		}
		break
	}
	return s
}

func firstNonEmptyParagraph(s string) string {
	for _, p := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(p); t != "" {
			return t
		}
	}
	return ""
}

func firstNWords(s string, n int) string {
	fields := strings.Fields(s)
	if len(fields) > n {
		fields = fields[:n]
	}
	return strings.Join(fields, " ")
}

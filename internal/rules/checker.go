package rules

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Check thực hiện kiểm tra cơ học nội dung chương theo các quy tắc có cấu trúc, trả về danh sách vi phạm thực tế.
//
// Hợp đồng thiết kế:
//   - Chỉ trả về sự thật, không ra lệnh (nguyên tắc sắt)
//   - Không chặn bất kỳ luồng gọi nào
//   - severity được ánh xạ cố định theo loại quy tắc (xem bảng chú thích trong types.go)
//
// Tham số:
//   - text: nội dung chương (bản cuối hoặc bản nháp đều được)
//   - wordCount: số từ của chương (đếm theo rune). Nếu <0, checker tự tính để tránh caller quét O(n) lặp lại.
//   - s: quy tắc có cấu trúc đã hợp nhất; nếu IsEmpty thì trả về nil luôn.
func Check(text string, wordCount int, s Structured) []Violation {
	if s.IsEmpty() {
		return nil
	}
	if wordCount < 0 {
		wordCount = utf8.RuneCountInString(text)
	}

	var violations []Violation
	violations = appendForbiddenChars(violations, text, s.ForbiddenChars)
	violations = appendForbiddenPhrases(violations, text, s.ForbiddenPhrases)
	violations = appendFatigueWords(violations, text, s.FatigueWords)
	violations = appendChapterWords(violations, wordCount, s.ChapterWords)
	return violations
}

// forbidden_chars: xuất hiện ≥1 lần là error.
// Mỗi quy tắc chỉ tạo một violation, actual là số lần xuất hiện.
func appendForbiddenChars(vs []Violation, text string, list []string) []Violation {
	for _, ch := range list {
		if ch == "" {
			continue
		}
		n := strings.Count(text, ch)
		if n == 0 {
			continue
		}
		vs = append(vs, Violation{
			Rule:     "forbidden_chars",
			Target:   ch,
			Actual:   n,
			Severity: SeverityError,
		})
	}
	return vs
}

// forbidden_phrases: xuất hiện ≥1 lần là error; hành vi giống forbidden_chars, chỉ khác tên rule.
func appendForbiddenPhrases(vs []Violation, text string, list []string) []Violation {
	for _, ph := range list {
		if ph == "" {
			continue
		}
		n := strings.Count(text, ph)
		if n == 0 {
			continue
		}
		vs = append(vs, Violation{
			Rule:     "forbidden_phrases",
			Target:   ph,
			Actual:   n,
			Severity: SeverityError,
		})
	}
	return vs
}

// fatigue_words: vi phạm khi số lần xuất hiện trong chương vượt ngưỡng, mức warning.
// Không tích lũy qua nhiều chương — vấn đề liên chương sẽ xử lý sau bằng công cụ chẩn đoán.
func appendFatigueWords(vs []Violation, text string, m map[string]int) []Violation {
	for word, limit := range m {
		if word == "" || limit <= 0 {
			continue
		}
		n := strings.Count(text, word)
		if n <= limit {
			continue
		}
		vs = append(vs, Violation{
			Rule:     "fatigue_words",
			Target:   word,
			Limit:    limit,
			Actual:   n,
			Severity: SeverityWarning,
		})
	}
	return vs
}

// chapter_words: độ lệch số từ.
// Độ lệch < 20%: warning; độ lệch ≥ 20%: error.
// Công thức độ lệch: thấp hơn min dùng (min-actual)/min; cao hơn max dùng (actual-max)/max.
func appendChapterWords(vs []Violation, wordCount int, rng *WordRange) []Violation {
	if rng == nil {
		return vs
	}
	var deviation float64
	switch {
	case wordCount < rng.Min:
		if rng.Min == 0 {
			return vs
		}
		deviation = float64(rng.Min-wordCount) / float64(rng.Min)
	case wordCount > rng.Max:
		if rng.Max == 0 {
			return vs
		}
		deviation = float64(wordCount-rng.Max) / float64(rng.Max)
	default:
		return vs // trong phạm vi cho phép
	}

	severity := SeverityWarning
	if deviation >= ChapterWordsDeviationThreshold {
		severity = SeverityError
	}
	vs = append(vs, Violation{
		Rule:      "chapter_words",
		Limit:     fmt.Sprintf("%d-%d", rng.Min, rng.Max),
		Actual:    wordCount,
		Deviation: deviation,
		Severity:  severity,
	})
	return vs
}

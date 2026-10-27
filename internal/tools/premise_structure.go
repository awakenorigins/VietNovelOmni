package tools

import (
	"strings"

	"github.com/voocel/ainovel-cli/internal/domain"
)

// premiseHeadingAliasesVI (VietNovelOmni): ánh xạ tiêu đề premise TIẾNG VIỆT về khóa canonical.
// Lý do sống còn: engine gốc chỉ nhận heading tiếng Trung; model viết heading tiếng Việt
// (đúng như prompt architect tiếng Việt yêu cầu) → template_ready=false vĩnh viễn →
// architect dựng lại nền MỖI LẦN resume = vòng lặp lãng phí vô hạn (đã dính thực chiến 03/07/2026).
// So khớp qua normalizePremiseHeading: bỏ dấu ngoặc đơn hậu tố, bỏ phần sau dấu hai chấm,
// hạ chữ thường — nên "Định vị thể loại (độc giả mục tiêu...)" vẫn khớp.
var premiseHeadingAliasesVI = map[string]string{
	"thể loại và tông điệu":        "题材和基调",
	"thể loại và sắc thái":         "题材和基调",
	"định vị thể loại":             "题材定位",
	"xung đột cốt lõi":             "核心冲突",
	"mục tiêu nhân vật chính":      "主角目标",
	"hướng kết cục":                "终局方向",
	"hướng kết thúc":               "终局方向",
	"vùng cấm viết":                "写作禁区",
	"điểm bán hàng khác biệt":      "差异化卖点",
	"điểm bán khác biệt":           "差异化卖点",
	"điểm móc khác biệt":           "差异化钩子",
	"cam kết thực hiện cốt lõi":    "核心兑现承诺",
	"động cơ truyện":               "故事引擎",
	"động cơ câu chuyện":           "故事引擎",
	"tuyến quan hệ/phát triển":     "关系/成长主线",
	"tuyến quan hệ và phát triển":  "关系/成长主线",
	"lộ trình nâng cấp":            "升级路径",
	"bước ngoặt giữa chuyện":       "中段转折",
	"bước ngoặt giữa truyện":       "中段转折",
	"chuyển hướng giữa truyện":     "中段转折",
	"mệnh đề kết cục":              "终局命题",
	"tính phù hợp truyện ngắn":     "短篇适配性",
	"độ phù hợp truyện ngắn":       "短篇适配性",
}

// normalizePremiseHeading chuẩn hóa tiêu đề trước khi tra bảng alias tiếng Việt:
// cắt hậu tố trong ngoặc đơn "(...)", cắt phần sau dấu hai chấm, gọn khoảng trắng, hạ chữ thường.
func normalizePremiseHeading(title string) string {
	if i := strings.IndexAny(title, "(（"); i >= 0 {
		title = title[:i]
	}
	if i := strings.IndexAny(title, ":："); i >= 0 {
		title = title[:i]
	}
	return strings.ToLower(strings.Join(strings.Fields(title), " "))
}

var premiseHeadingAliases = map[string]string{
	"题材定位":    "题材定位",
	"题材和基调":   "题材和基调",
	"核心冲突":    "核心冲突",
	"主角目标":    "主角目标",
	"结局方向":    "终局方向",
	"终局方向":    "终局方向",
	"写作禁区":    "写作禁区",
	"差异化卖点":   "差异化卖点",
	"差异化钩子":   "差异化钩子",
	"核心兑现承诺":  "核心兑现承诺",
	"故事引擎":    "故事引擎",
	"关系/成长主线": "关系/成长主线",
	"升级路径":    "升级路径",
	"中段转折":    "中段转折",
	"中期转向":    "中段转折",
	"终局命题":    "终局命题",
	"短篇适配性":   "短篇适配性",
	"本作为什么适合短篇/单卷收束": "短篇适配性",
}

func parsePremiseSections(premise string) map[string]string {
	lines := strings.Split(premise, "\n")
	sections := make(map[string]string)
	var current string
	var body []string

	flush := func() {
		if current == "" {
			return
		}
		text := strings.TrimSpace(strings.Join(body, "\n"))
		if text != "" {
			sections[current] = text
		}
		body = body[:0]
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if heading, ok := canonicalPremiseHeading(trimmed); ok {
			flush()
			current = heading
			continue
		}
		if current != "" {
			body = append(body, line)
		}
	}
	flush()
	return sections
}

func canonicalPremiseHeading(line string) (string, bool) {
	if !strings.HasPrefix(line, "#") {
		return "", false
	}
	title := strings.TrimSpace(strings.TrimLeft(line, "#"))
	if title == "" {
		return "", false
	}
	if canonical, ok := premiseHeadingAliases[title]; ok {
		return canonical, true
	}
	// Fallback tiếng Việt: chuẩn hóa rồi tra bảng alias Việt.
	if canonical, ok := premiseHeadingAliasesVI[normalizePremiseHeading(title)]; ok {
		return canonical, true
	}
	return "", false
}

func premiseStructure(premise string, tier domain.PlanningTier) map[string]any {
	sections := parsePremiseSections(premise)
	required := requiredPremiseHeadings(tier)
	found := make([]string, 0, len(required))
	var missing []string
	for _, heading := range required {
		if _, ok := sections[heading]; ok {
			found = append(found, heading)
			continue
		}
		missing = append(missing, heading)
	}

	structure := map[string]any{
		"template_ready": len(missing) == 0,
		"found":          found,
		"missing":        missing,
	}
	if len(sections) > 0 {
		structure["section_count"] = len(sections)
	}
	return structure
}

func requiredPremiseHeadings(tier domain.PlanningTier) []string {
	common := []string{
		"题材和基调",
		"题材定位",
		"核心冲突",
		"主角目标",
		"终局方向",
		"写作禁区",
		"差异化卖点",
		"差异化钩子",
		"核心兑现承诺",
	}

	switch tier {
	case domain.PlanningTierLong:
		return append(common,
			"故事引擎",
			"关系/成长主线",
			"升级路径",
			"中段转折",
			"终局命题",
		)
	case domain.PlanningTierMid:
		return append(common,
			"故事引擎",
			"中段转折",
		)
	case domain.PlanningTierShort:
		return append(common,
			"短篇适配性",
		)
	default:
		return common
	}
}

package exp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// skillAuditReport ánh xạ tối thiểu cấu trúc meta/skill-audit/ch{N}.json do post-commit-hook.sh sinh ra.
type skillAuditReport struct {
	Chapter int  `json:"chapter"`
	PassAll bool `json:"pass_all"`
	Results map[string]struct {
		Available bool            `json:"available"`
		Exit      int             `json:"exit"`
		Parsed    json.RawMessage `json:"parsed"`
		Output    string          `json:"output"`
	} `json:"results"`
}

// proseAuditParsed ánh xạ phần parsed của oum-prose-verify (checker chính).
type proseAuditParsed struct {
	OK         bool     `json:"ok"`
	HardErrors []string `json:"hard_errors"`
}

// runAuditGate đọc toàn bộ báo cáo skill-audit trong storeDir/meta/skill-audit và đánh giá.
// Trả về (warnings, blocked). blocked=true khi mode="block" và có chương lỗi prose cứng THẬT
// (em dash / tiếng Anh — không tính markdown tiêu đề).
func runAuditGate(storeDir, mode string, chapters []int) (warnings []string, blocked bool) {
	if mode == "" || mode == "off" {
		return nil, false
	}
	auditDir := filepath.Join(storeDir, "meta", "skill-audit")
	want := make(map[int]struct{}, len(chapters))
	for _, c := range chapters {
		want[c] = struct{}{}
	}

	hardFail := []int{}
	for _, ch := range chapters {
		p := filepath.Join(auditDir, fmt.Sprintf("ch%02d.json", ch))
		b, err := os.ReadFile(p)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("chương %d: chưa có báo cáo skill-audit (hook chưa chạy?)", ch))
			continue
		}
		var rep skillAuditReport
		if err := json.Unmarshal(b, &rep); err != nil {
			warnings = append(warnings, fmt.Sprintf("chương %d: báo cáo audit lỗi định dạng", ch))
			continue
		}

		// oum-prose-verify: lỗi cứng THẬT (bỏ qua markdown tiêu đề)
		if r, ok := rep.Results["oum_prose_verify"]; ok && len(r.Parsed) > 0 {
			var pv proseAuditParsed
			if json.Unmarshal(r.Parsed, &pv) == nil {
				for _, e := range pv.HardErrors {
					if len(e) >= 8 && e[:8] == "markdown" {
						continue // markdown tiêu đề: gỡ khi export, không tính
					}
					warnings = append(warnings, fmt.Sprintf("chương %d [prose]: %s", ch, e))
					hardFail = append(hardFail, ch)
				}
			}
		}

		// novel-guardian / novel-master: ghi nhận có fail continuity (cảnh báo, không chặn)
		if r, ok := rep.Results["novel_guardian_scan"]; ok && r.Available && r.Output != "" {
			if containsFail(r.Output) {
				warnings = append(warnings, fmt.Sprintf("chương %d [guardian]: có cảnh báo liền mạch (xem meta/skill-audit/ch%02d.json)", ch, ch))
			}
		}
		if r, ok := rep.Results["novel_master_check"]; ok && r.Available && hasWarningSeverity(r.Output) {
			warnings = append(warnings, fmt.Sprintf("chương %d [master]: có WARNING liền mạch (xem meta/skill-audit/ch%02d.json)", ch, ch))
		}
	}

	sort.Ints(hardFail)
	if mode == "block" && len(hardFail) > 0 {
		blocked = true
	}
	return warnings, blocked
}

func containsFail(s string) bool {
	for i := 0; i+4 <= len(s); i++ {
		if s[i:i+4] == "fail" {
			return true
		}
	}
	return false
}

func hasWarningSeverity(s string) bool {
	for i := 0; i+7 <= len(s); i++ {
		if s[i:i+7] == "WARNING" {
			return true
		}
	}
	return false
}

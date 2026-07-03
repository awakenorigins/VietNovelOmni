package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/voocel/ainovel-cli/internal/store"
)

// 3 bản độc lập, từ khóa KHÁC nhau theo hướng — giống thực tế beat-method.
const (
	vNoiTam   = "Hắn ngồi im nghe tim mình đập chậm, ký ức cũ trỗi dậy trong lồng ngực tối. Hắn ngồi im nghe tim mình đập chậm, ký ức cũ trỗi dậy trong lồng ngực tối. "
	vDoiThoai = "Anh nợ tôi một mạng, gã nói, giọng thấp mà sắc như lưỡi dao kề cổ. Anh nợ tôi một mạng, gã nói, giọng thấp mà sắc như lưỡi dao kề cổ. "
	vHanhDong = "Hắn vụt đứng dậy, xô ghế, lao qua cửa trước khi tiếng thở kịp thoát ra. Hắn vụt đứng dậy, xô ghế, lao qua cửa trước khi tiếng thở kịp thoát ra. "
	// finalMerge trộn dấu vết cả 3 bản (điểm sát nhau → merge thật).
	finalMerge = "Hắn ngồi im nghe tim mình đập chậm, ký ức cũ trỗi dậy trong lồng ngực tối. " +
		"Anh nợ tôi một mạng, gã nói, giọng thấp mà sắc như lưỡi dao kề cổ. " +
		"Hắn vụt đứng dậy, xô ghế, lao qua cửa trước khi tiếng thở kịp thoát ra."
)

// beatArgs dựng args hợp lệ mặc định cho draft_beat; test chỉnh từng trường để tạo case lỗi.
func beatArgs(t *testing.T, mutate func(m map[string]any)) json.RawMessage {
	t.Helper()
	m := map[string]any{
		"chapter":   1,
		"beat":      1,
		"beat_goal": "Mở chương: thiết lập khủng hoảng nợ máu",
		"variants": []map[string]any{
			{"focus": "noi_tam", "content": vNoiTam, "score": 74, "note": "sâu nhưng chậm"},
			{"focus": "doi_thoai", "content": vDoiThoai, "score": 79, "note": "căng, có ngầm ý"},
			{"focus": "hanh_dong", "content": vHanhDong, "score": 77, "note": "nhanh, hơi mỏng"},
		},
		"final_content":   finalMerge,
		"merge_rationale": "Lấy khung đối thoại bản B, chèn hai nhịp nội tâm bản A, giữ cú chốt hành động bản C",
	}
	if mutate != nil {
		mutate(m)
	}
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	return raw
}

func newBeatStore(t *testing.T) *store.Store {
	t.Helper()
	s := store.NewStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Progress.Init("test", 10); err != nil {
		t.Fatalf("Progress.Init: %v", err)
	}
	return s
}

func TestDraftBeatHappyPathAppendsAndRecords(t *testing.T) {
	s := newBeatStore(t)
	tool := NewDraftBeatTool(s)

	out, err := tool.Execute(context.Background(), beatArgs(t, nil))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var res map[string]any
	if err := json.Unmarshal(out, &res); err != nil {
		t.Fatalf("Unmarshal result: %v", err)
	}
	if res["written"] != true || res["gate_passed"] != true {
		t.Fatalf("expected written+gate_passed, got %v", res)
	}
	if res["merge_audit"] != "passed" {
		t.Fatalf("merge_audit should be passed, got %v", res["merge_audit"])
	}
	if res["best_variant"] != "doi_thoai" {
		t.Fatalf("best_variant should be doi_thoai (score 79), got %v", res["best_variant"])
	}

	draft, err := s.Drafts.LoadDraft(1)
	if err != nil || draft == "" {
		t.Fatalf("draft should exist after beat append, err=%v", err)
	}
	if !strings.Contains(draft, "lao qua cửa") {
		t.Fatalf("draft should contain final_content")
	}

	// Hồ sơ beat phải tồn tại và gate_passed=true
	var rec map[string]any
	data, err := s.Drafts.LoadBeatRecordRaw(1, 1)
	if err != nil {
		t.Fatalf("beat record missing: %v", err)
	}
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatalf("beat record not json: %v", err)
	}
	if rec["gate_passed"] != true {
		t.Fatalf("beat record gate_passed should be true")
	}

	// Beat thứ hai append tiếp (variants + final khác nội dung nhưng vẫn merge thật)
	out2, err := tool.Execute(context.Background(), beatArgs(t, func(m map[string]any) {
		m["beat"] = 2
		m["variants"] = []map[string]any{
			{"focus": "noi_tam", "content": "Mưa đêm dội xuống mái tôn, hắn nằm nghe từng hạt gõ vào giấc ngủ chưa kịp đến. Mưa đêm dội xuống mái tôn, hắn nằm nghe từng hạt gõ vào giấc ngủ chưa kịp đến. ", "score": 70, "note": "tĩnh"},
			{"focus": "doi_thoai", "content": "Ngủ được không, nàng hỏi khẽ bên kia vách, câu hỏi rơi vào khoảng lặng dày đặc. Ngủ được không, nàng hỏi khẽ bên kia vách, câu hỏi rơi vào khoảng lặng dày đặc. ", "score": 72, "note": "ấm"},
			{"focus": "hanh_dong", "content": "Hắn trở mình, kéo chăn lên tận cằm, tay siết chặt mép giường cho tới khi đau. Hắn trở mình, kéo chăn lên tận cằm, tay siết chặt mép giường cho tới khi đau. ", "score": 71, "note": "nhỏ"},
		}
		m["final_content"] = "Mưa đêm dội xuống mái tôn, hắn nằm nghe từng hạt gõ vào giấc ngủ chưa kịp đến. " +
			"Ngủ được không, nàng hỏi khẽ bên kia vách, câu hỏi rơi vào khoảng lặng dày đặc. " +
			"Hắn trở mình, kéo chăn lên tận cằm, tay siết chặt mép giường."
	}))
	if err != nil {
		t.Fatalf("Execute beat 2: %v", err)
	}
	var res2 map[string]any
	_ = json.Unmarshal(out2, &res2)
	full, _ := s.Drafts.LoadDraft(1)
	if !strings.Contains(full, "mái tôn") || !strings.Contains(full, "lao qua cửa") {
		t.Fatalf("draft should contain both beats")
	}
}

func TestDraftBeatGateRejectsEmDashAndEnglish(t *testing.T) {
	s := newBeatStore(t)
	tool := NewDraftBeatTool(s)

	// Em dash trong bản cuối → gate chặn, draft KHÔNG được ghi.
	// Giữ dấu vết 3 bản để qua merge-audit, chỉ chèn em dash để trip prose-gate.
	_, err := tool.Execute(context.Background(), beatArgs(t, func(m map[string]any) {
		m["final_content"] = "Hắn ngồi im nghe tim mình đập chậm, ký ức cũ trỗi dậy — trong lồng ngực tối. " +
			"Anh nợ tôi một mạng, gã nói, giọng thấp mà sắc như lưỡi dao. " +
			"Hắn vụt đứng dậy, xô ghế, lao qua cửa."
	}))
	if err == nil || !strings.Contains(err.Error(), "KHÔNG QUA GATE") {
		t.Fatalf("expected gate rejection for em dash, got %v", err)
	}
	if draft, _ := s.Drafts.LoadDraft(1); draft != "" {
		t.Fatalf("draft must stay empty after gate fail, got %q", draft)
	}

	// Tiếng Anh lọt → gate chặn (vẫn giữ dấu vết 3 bản).
	_, err = tool.Execute(context.Background(), beatArgs(t, func(m map[string]any) {
		m["final_content"] = "Hắn ngồi im nghe tim mình đập chậm, ký ức cũ trỗi dậy suddenly trong lồng ngực tối. " +
			"Anh nợ tôi một mạng, gã nói, giọng thấp mà sắc như lưỡi dao. " +
			"Hắn vụt đứng dậy, xô ghế, lao qua cửa."
	}))
	if err == nil || !strings.Contains(err.Error(), "KHÔNG QUA GATE") {
		t.Fatalf("expected gate rejection for english word, got %v", err)
	}
}

func TestDraftBeatMergeAuditRejectsClone(t *testing.T) {
	s := newBeatStore(t)
	tool := NewDraftBeatTool(s)
	// final bê gần nguyên bản doi_thoai → clone → bị chặn.
	_, err := tool.Execute(context.Background(), beatArgs(t, func(m map[string]any) {
		m["final_content"] = vDoiThoai + vDoiThoai
	}))
	if err == nil || !strings.Contains(err.Error(), "merge-audit") || !strings.Contains(err.Error(), "gần nguyên") {
		t.Fatalf("expected clone rejection, got %v", err)
	}
	if draft, _ := s.Drafts.LoadDraft(1); draft != "" {
		t.Fatalf("draft must stay empty after merge-audit clone fail, got %q", draft)
	}
}

func TestDraftBeatMergeAuditRequiresRealMerge(t *testing.T) {
	s := newBeatStore(t)
	tool := NewDraftBeatTool(s)
	// Điểm sát nhau nhưng final viết rời rạc, không mang dấu vết bản nào → bị chặn.
	_, err := tool.Execute(context.Background(), beatArgs(t, func(m map[string]any) {
		m["final_content"] = "Cơn giông kéo ngang thành phố, sấm rền qua những tòa nhà xám xịt lặng thinh. " +
			"Đèn đường chớp tắt, vũng nước loang lổ phản chiếu bầu trời rách nát phía trên."
	}))
	if err == nil || !strings.Contains(err.Error(), "merge-audit") {
		t.Fatalf("expected real-merge requirement rejection, got %v", err)
	}
}

func TestDraftBeatMergeAuditDominantMustInherit(t *testing.T) {
	s := newBeatStore(t)
	tool := NewDraftBeatTool(s)
	// Bản doi_thoai áp đảo (điểm 95 vs 60/62) nhưng final bỏ hẳn bản đó → bị chặn.
	_, err := tool.Execute(context.Background(), beatArgs(t, func(m map[string]any) {
		v := m["variants"].([]map[string]any)
		v[0]["score"] = 60
		v[1]["score"] = 95
		v[2]["score"] = 62
		m["final_content"] = "Hắn ngồi im nghe tim mình đập chậm, ký ức cũ trỗi dậy trong lồng ngực tối. " +
			"Hắn vụt đứng dậy, xô ghế, lao qua cửa trước khi tiếng thở kịp thoát ra. " +
			"Cơn gió luồn qua khe cửa, mang theo mùi đất ẩm và tiếng chó sủa xa xăm cuối con hẻm."
	}))
	if err == nil || !strings.Contains(err.Error(), "áp đảo") {
		t.Fatalf("expected dominant-inherit rejection, got %v", err)
	}
}

func TestDraftBeatValidatesVariants(t *testing.T) {
	s := newBeatStore(t)
	tool := NewDraftBeatTool(s)

	// Thiếu bản (2/3)
	_, err := tool.Execute(context.Background(), beatArgs(t, func(m map[string]any) {
		v := m["variants"].([]map[string]any)
		m["variants"] = v[:2]
	}))
	if err == nil || !strings.Contains(err.Error(), "đúng 3 bản") {
		t.Fatalf("expected 3-variant requirement, got %v", err)
	}

	// Focus trùng
	_, err = tool.Execute(context.Background(), beatArgs(t, func(m map[string]any) {
		v := m["variants"].([]map[string]any)
		v[1]["focus"] = "noi_tam"
	}))
	if err == nil || !strings.Contains(err.Error(), "trùng lặp") {
		t.Fatalf("expected duplicate focus rejection, got %v", err)
	}

	// Bản quá ngắn (làm phép)
	_, err = tool.Execute(context.Background(), beatArgs(t, func(m map[string]any) {
		v := m["variants"].([]map[string]any)
		v[0]["content"] = "Ngắn quá."
	}))
	if err == nil || !strings.Contains(err.Error(), "quá ngắn") {
		t.Fatalf("expected short-variant rejection, got %v", err)
	}

	// Score ngoài khoảng
	_, err = tool.Execute(context.Background(), beatArgs(t, func(m map[string]any) {
		v := m["variants"].([]map[string]any)
		v[2]["score"] = 120
	}))
	if err == nil || !strings.Contains(err.Error(), "0-100") {
		t.Fatalf("expected score range rejection, got %v", err)
	}

	// Thiếu merge_rationale
	_, err = tool.Execute(context.Background(), beatArgs(t, func(m map[string]any) {
		m["merge_rationale"] = "  "
	}))
	if err == nil || !strings.Contains(err.Error(), "merge_rationale") {
		t.Fatalf("expected merge_rationale requirement, got %v", err)
	}
}

func TestDraftBeatRespectsCompletedChapter(t *testing.T) {
	s := newBeatStore(t)
	if err := s.Progress.MarkChapterComplete(1, 3000, "", ""); err != nil {
		t.Fatalf("MarkChapterComplete: %v", err)
	}
	tool := NewDraftBeatTool(s)
	out, err := tool.Execute(context.Background(), beatArgs(t, nil))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var res map[string]any
	_ = json.Unmarshal(out, &res)
	if res["skipped"] != true {
		t.Fatalf("completed chapter (not in rewrite queue) should be skipped, got %v", res)
	}
}

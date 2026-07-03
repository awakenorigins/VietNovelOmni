package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/voocel/ainovel-cli/internal/store"
)

// Ba bản độc lập, nhãn hướng TỰ DO, từ khóa khác nhau — giống thực tế beat-method.
// Mỗi bản ~ đủ dài; final gộp ~ 400-600 chữ.
func rep(s string, n int) string { return strings.Repeat(s, n) }

var (
	vA = rep("Hắn ngồi im nghe tim mình đập chậm, ký ức cũ trỗi dậy trong lồng ngực tối như mực. ", 6)
	vB = rep("Anh nợ tôi một mạng, gã nói, giọng thấp mà sắc lạnh tựa lưỡi dao kề ngang cổ họng. ", 6)
	vC = rep("Hắn vụt đứng dậy, xô ghế lăn kềnh, lao thẳng qua khung cửa trước khi tiếng thở kịp thoát. ", 6)
)

// finalMerge gộp dấu vết cả ba bản, đủ 400-600 chữ.
func finalMergeText() string {
	block := "Hắn ngồi im nghe tim mình đập chậm, ký ức cũ trỗi dậy trong lồng ngực tối như mực. " +
		"Anh nợ tôi một mạng, gã nói, giọng thấp mà sắc lạnh tựa lưỡi dao kề ngang cổ họng. " +
		"Hắn vụt đứng dậy, xô ghế lăn kềnh, lao thẳng qua khung cửa trước khi tiếng thở kịp thoát. "
	return rep(block, 8)
}

func beatArgs(t *testing.T, mutate func(m map[string]any)) json.RawMessage {
	t.Helper()
	m := map[string]any{
		"chapter":   1,
		"beat":      1,
		"beat_goal": "Mở chương: thiết lập khủng hoảng nợ máu",
		"variants": []map[string]any{
			{"angle": "nội tâm dồn nén", "content": vA, "score": 74, "highlights": []string{"ký ức cũ trỗi dậy trong lồng ngực tối"}, "note": "sâu nhưng chậm"},
			{"angle": "đối đầu nảy lửa", "content": vB, "score": 79, "highlights": []string{"sắc lạnh tựa lưỡi dao kề ngang cổ họng"}, "note": "căng, có ngầm ý"},
			{"angle": "hành động dứt khoát", "content": vC, "score": 77, "highlights": []string{"lao thẳng qua khung cửa"}, "note": "nhanh, hơi mỏng"},
		},
		"decision":        "merge",
		"final_content":   finalMergeText(),
		"merge_rationale": "Lấy khung đối đầu bản B, chèn nhịp nội tâm bản A, giữ cú chốt hành động bản C",
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

func TestDraftBeatHappyPathMergeAppendsAndRecords(t *testing.T) {
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
	if res["decision"] != "merge" {
		t.Fatalf("decision should be merge, got %v", res["decision"])
	}
	if res["best_variant"] != "đối đầu nảy lửa" {
		t.Fatalf("best_variant should be highest-score angle, got %v", res["best_variant"])
	}
	draft, err := s.Drafts.LoadDraft(1)
	if err != nil || draft == "" {
		t.Fatalf("draft should exist after beat append, err=%v", err)
	}
}

func TestDraftBeatPickBestPassesWhenDominant(t *testing.T) {
	s := newBeatStore(t)
	tool := NewDraftBeatTool(s)
	// Bản B vượt trội (95 vs 60/62), final lấy nguyên bản B → pick_best hợp lệ.
	out, err := tool.Execute(context.Background(), beatArgs(t, func(m map[string]any) {
		v := m["variants"].([]map[string]any)
		v[0]["score"] = 60
		v[1]["score"] = 95
		v[2]["score"] = 62
		m["decision"] = "pick_best"
		m["final_content"] = rep("Anh nợ tôi một mạng, gã nói, giọng thấp mà sắc lạnh tựa lưỡi dao kề ngang cổ họng. ", 24)
		m["merge_rationale"] = "Bản đối đầu vượt trội hẳn, giữ nguyên xương sống, chỉnh nhẹ nhịp"
	}))
	if err != nil {
		t.Fatalf("pick_best dominant should pass, got %v", err)
	}
	var res map[string]any
	_ = json.Unmarshal(out, &res)
	if res["decision"] != "pick_best" || res["gate_passed"] != true {
		t.Fatalf("expected pick_best pass, got %v", res)
	}
}

func TestDraftBeatPickBestRejectedWhenNotDominant(t *testing.T) {
	s := newBeatStore(t)
	tool := NewDraftBeatTool(s)
	// Điểm sát nhau nhưng dùng pick_best → phải bị chặn (yêu cầu merge).
	_, err := tool.Execute(context.Background(), beatArgs(t, func(m map[string]any) {
		m["decision"] = "pick_best"
		m["final_content"] = rep("Anh nợ tôi một mạng, gã nói, giọng thấp mà sắc lạnh tựa lưỡi dao kề ngang cổ họng. ", 24)
	}))
	if err == nil || !strings.Contains(err.Error(), "vượt trội") {
		t.Fatalf("expected pick_best-not-dominant rejection, got %v", err)
	}
}

func TestDraftBeatMergeRejectsClone(t *testing.T) {
	s := newBeatStore(t)
	tool := NewDraftBeatTool(s)
	// decision=merge nhưng final bê nguyên bản B → clone → bị chặn.
	_, err := tool.Execute(context.Background(), beatArgs(t, func(m map[string]any) {
		m["final_content"] = rep("Anh nợ tôi một mạng, gã nói, giọng thấp mà sắc lạnh tựa lưỡi dao kề ngang cổ họng. ", 24)
	}))
	if err == nil || !strings.Contains(err.Error(), "giống gần nguyên") {
		t.Fatalf("expected clone rejection, got %v", err)
	}
	if draft, _ := s.Drafts.LoadDraft(1); draft != "" {
		t.Fatalf("draft must stay empty after clone fail, got %q", draft)
	}
}

func TestDraftBeatMergeRequiresRealMerge(t *testing.T) {
	s := newBeatStore(t)
	tool := NewDraftBeatTool(s)
	// merge nhưng final rời rạc, không mang dấu vết bản nào → bị chặn.
	_, err := tool.Execute(context.Background(), beatArgs(t, func(m map[string]any) {
		m["final_content"] = rep("Cơn giông kéo ngang thành phố, sấm rền qua những tòa nhà xám xịt lặng thinh dưới trời. ", 26)
	}))
	if err == nil || !strings.Contains(err.Error(), "dấu vết") {
		t.Fatalf("expected real-merge rejection, got %v", err)
	}
}

func TestDraftBeatEnforces400to600Words(t *testing.T) {
	s := newBeatStore(t)
	tool := NewDraftBeatTool(s)
	// Quá ngắn
	_, err := tool.Execute(context.Background(), beatArgs(t, func(m map[string]any) {
		m["final_content"] = vA + vB // ~ dưới 340 từ
	}))
	if err == nil || !strings.Contains(err.Error(), "quá ngắn") {
		t.Fatalf("expected too-short rejection, got %v", err)
	}
	// Quá dài
	_, err = tool.Execute(context.Background(), beatArgs(t, func(m map[string]any) {
		m["final_content"] = rep(finalMergeText(), 3)
	}))
	if err == nil || !strings.Contains(err.Error(), "quá dài") {
		t.Fatalf("expected too-long rejection, got %v", err)
	}
}

func TestDraftBeatGateRejectsEmDashAndEnglish(t *testing.T) {
	s := newBeatStore(t)
	tool := NewDraftBeatTool(s)
	// Em dash trong final (vẫn giữ dấu vết ≥2 bản + đủ dài) → prose gate chặn.
	_, err := tool.Execute(context.Background(), beatArgs(t, func(m map[string]any) {
		f := finalMergeText()
		m["final_content"] = f + "Hắn dừng lại — lặng lẽ nhìn ra ngoài trời tối mịt mờ sương."
	}))
	if err == nil || !strings.Contains(err.Error(), "KHÔNG QUA GATE") {
		t.Fatalf("expected em dash gate rejection, got %v", err)
	}
	if draft, _ := s.Drafts.LoadDraft(1); draft != "" {
		t.Fatalf("draft must stay empty after gate fail, got %q", draft)
	}
	// Tiếng Anh lọt
	_, err = tool.Execute(context.Background(), beatArgs(t, func(m map[string]any) {
		f := finalMergeText()
		m["final_content"] = f + "Hắn nhìn suddenly ra ngoài cửa sổ lòng nặng trĩu bao điều."
	}))
	if err == nil || !strings.Contains(err.Error(), "KHÔNG QUA GATE") {
		t.Fatalf("expected english gate rejection, got %v", err)
	}
}

func TestDraftBeatValidatesVariants(t *testing.T) {
	s := newBeatStore(t)
	tool := NewDraftBeatTool(s)

	// Chỉ 1 bản (< 2)
	_, err := tool.Execute(context.Background(), beatArgs(t, func(m map[string]any) {
		v := m["variants"].([]map[string]any)
		m["variants"] = v[:1]
	}))
	if err == nil || !strings.Contains(err.Error(), "ít nhất") {
		t.Fatalf("expected min-2-variant requirement, got %v", err)
	}

	// Nhãn hướng trùng
	_, err = tool.Execute(context.Background(), beatArgs(t, func(m map[string]any) {
		v := m["variants"].([]map[string]any)
		v[1]["angle"] = "nội tâm dồn nén"
	}))
	if err == nil || !strings.Contains(err.Error(), "trùng lặp") {
		t.Fatalf("expected duplicate-angle rejection, got %v", err)
	}

	// Bản quá ngắn
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
		t.Fatalf("expected score-range rejection, got %v", err)
	}

	// decision không hợp lệ
	_, err = tool.Execute(context.Background(), beatArgs(t, func(m map[string]any) {
		m["decision"] = "whatever"
	}))
	if err == nil || !strings.Contains(err.Error(), "decision") {
		t.Fatalf("expected decision validation, got %v", err)
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

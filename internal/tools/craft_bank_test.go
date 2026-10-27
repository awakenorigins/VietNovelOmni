package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/voocel/ainovel-cli/internal/domain"
	"github.com/voocel/ainovel-cli/internal/rules"
	"github.com/voocel/ainovel-cli/internal/store"
)

// sentinel nội dung craft bank để chứng minh runtime inject thật.
const craftBankSentinel = "SENTINEL_CRAFT_BANK_XZ42 psychic distance deadpan free indirect discourse"

func newStoreForCraftTest(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	s := store.NewStore(dir)
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Outline.SavePremise("## 题材和基调\n测试。\n\n## 核心冲突\n测试冲突。\n"); err != nil {
		t.Fatalf("SavePremise: %v", err)
	}
	if err := s.Outline.SaveOutline([]domain.OutlineEntry{
		{Chapter: 1, Title: "一", CoreEvent: "事件", Scenes: []string{"场景"}},
	}); err != nil {
		t.Fatalf("SaveOutline: %v", err)
	}
	if err := s.Progress.Init("t", 1); err != nil {
		t.Fatalf("InitProgress: %v", err)
	}
	return s
}

// extractRefKeys lấy map references từ payload novel_context.
func extractRefKeys(t *testing.T, result []byte) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(result, &payload); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	pack, ok := payload["reference_pack"].(map[string]any)
	if !ok {
		t.Fatalf("reference_pack missing or wrong type")
	}
	refs, ok := pack["references"].(map[string]any)
	if !ok {
		t.Fatalf("references missing under reference_pack")
	}
	return refs
}

// TestCraftBankInjectedForWriter chứng minh: khi References.CraftBank được set,
// key craft_bank (kèm đúng nội dung) xuất hiện trong reference pack của WRITER (chapter mode).
func TestCraftBankInjectedForWriter(t *testing.T) {
	s := newStoreForCraftTest(t)
	tool := NewContextTool(s, References{CraftBank: craftBankSentinel}, "default", rules.LoadOptions{})

	args, _ := json.Marshal(map[string]any{"chapter": 1})
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	refs := extractRefKeys(t, result)
	val, ok := refs["craft_bank"]
	if !ok {
		t.Fatalf("craft_bank KHÔNG được inject cho writer — wire FAIL")
	}
	if s, _ := val.(string); !strings.Contains(s, "SENTINEL_CRAFT_BANK_XZ42") {
		t.Fatalf("craft_bank inject nhưng nội dung sai: %q", s)
	}
}

// TestCraftBankInjectedForArchitect chứng minh craft_bank cũng vào reference pack của ARCHITECT.
func TestCraftBankInjectedForArchitect(t *testing.T) {
	s := newStoreForCraftTest(t)
	tool := NewContextTool(s, References{CraftBank: craftBankSentinel}, "default", rules.LoadOptions{})

	// architect mode: chapter = 0
	args, _ := json.Marshal(map[string]any{"chapter": 0})
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	refs := extractRefKeys(t, result)
	if _, ok := refs["craft_bank"]; !ok {
		t.Fatalf("craft_bank KHÔNG được inject cho architect — wire FAIL")
	}
}

// TestCraftBankAbsentWhenEmpty: nếu không set thì không có key rác (đảm bảo add() bỏ rỗng).
func TestCraftBankAbsentWhenEmpty(t *testing.T) {
	s := newStoreForCraftTest(t)
	tool := NewContextTool(s, References{}, "default", rules.LoadOptions{})
	args, _ := json.Marshal(map[string]any{"chapter": 1})
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	refs := extractRefKeys(t, result)
	if _, ok := refs["craft_bank"]; ok {
		t.Fatalf("craft_bank xuất hiện dù CraftBank rỗng — add() không lọc rỗng")
	}
}

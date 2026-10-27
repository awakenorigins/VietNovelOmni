package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/voocel/ainovel-cli/internal/domain"
	"github.com/voocel/ainovel-cli/internal/store"
)

// writeFakeHook tạo hook giả sinh report pass_all theo tham số — mô phỏng post-commit-hook.sh thật.
func writeFakeHook(t *testing.T, dir string, passAll bool) string {
	t.Helper()
	pass := "false"
	if passAll {
		pass = "true"
	}
	script := `#!/usr/bin/env bash
mkdir -p "$OMNI_OUTPUT_DIR/meta/skill-audit"
printf '{"chapter": %d, "pass_all": ` + pass + `}' "$OMNI_CHAPTER" > "$OMNI_OUTPUT_DIR/meta/skill-audit/ch$(printf %02d $OMNI_CHAPTER).json"
exit 0
`
	p := filepath.Join(dir, "fake-hook.sh")
	if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake hook: %v", err)
	}
	return p
}

// commitChapterWithHook dựng store + draft + commit chương 1 qua tool có hook.
func commitChapterWithHook(t *testing.T, hookPass bool) (*store.Store, string) {
	t.Helper()
	dir := t.TempDir()
	s := store.NewStore(dir)
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Progress.Init("test", 10); err != nil {
		t.Fatalf("Progress.Init: %v", err)
	}
	if err := s.Drafts.SaveDraft(1, "Đêm xuống chậm trên xóm chài, hắn ngồi vá lưới, nghe sóng đập vào kè đá từng nhịp nặng nề."); err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}
	hook := writeFakeHook(t, dir, hookPass)
	tool := NewCommitChapterTool(s).WithPostCommitHook(hook, "", dir)

	args, _ := json.Marshal(map[string]any{
		"chapter":    1,
		"summary":    "chương mở đầu xóm chài",
		"characters": []string{"hắn"},
		"key_events": []string{"vá lưới trong đêm"},
	})
	if _, err := tool.Execute(context.Background(), args); err != nil {
		t.Fatalf("commit: %v", err)
	}
	return s, dir
}

func TestAuditFeedbackEnqueuesOnFail(t *testing.T) {
	s, dir := commitChapterWithHook(t, false)

	p, _ := s.Progress.Load()
	if len(p.PendingRewrites) != 1 || p.PendingRewrites[0] != 1 {
		t.Fatalf("chapter 1 should be auto-enqueued on audit fail, got %v", p.PendingRewrites)
	}
	if p.Flow != domain.FlowRewriting {
		t.Fatalf("flow should switch to rewriting, got %s", p.Flow)
	}
	if _, err := os.Stat(filepath.Join(dir, "meta", "skill-audit", "ch01.enqueued")); err != nil {
		t.Fatalf("enqueue marker should exist: %v", err)
	}
}

func TestAuditFeedbackNoEnqueueOnPass(t *testing.T) {
	s, dir := commitChapterWithHook(t, true)

	p, _ := s.Progress.Load()
	if len(p.PendingRewrites) != 0 {
		t.Fatalf("no enqueue expected on pass, got %v", p.PendingRewrites)
	}
	if p.Flow == domain.FlowRewriting {
		t.Fatal("flow must stay out of rewriting on pass")
	}
	if _, err := os.Stat(filepath.Join(dir, "meta", "skill-audit", "ch01.enqueued")); !os.IsNotExist(err) {
		t.Fatalf("no marker expected on pass, err=%v", err)
	}
}

func TestAuditFeedbackMarkerBlocksSecondEnqueue(t *testing.T) {
	dir := t.TempDir()
	s := store.NewStore(dir)
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Progress.Init("test", 10); err != nil {
		t.Fatalf("Progress.Init: %v", err)
	}
	if err := s.Progress.MarkChapterComplete(1, 3000, "", ""); err != nil {
		t.Fatalf("MarkChapterComplete: %v", err)
	}

	// Report fail + marker đã tồn tại (đã enqueue một lần trước đó, chương được viết lại xong,
	// hàng đợi đã drain) → applyAuditFeedback KHÔNG enqueue lần hai.
	auditDir := filepath.Join(dir, "meta", "skill-audit")
	if err := os.MkdirAll(auditDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(auditDir, "ch01.json"), []byte(`{"chapter":1,"pass_all":false}`), 0o644); err != nil {
		t.Fatalf("write report: %v", err)
	}
	if err := os.WriteFile(filepath.Join(auditDir, "ch01.enqueued"), []byte("x\n"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}

	tool := NewCommitChapterTool(s).WithPostCommitHook("/bin/true", "", dir)
	tool.applyAuditFeedback(1, dir)

	p, _ := s.Progress.Load()
	if len(p.PendingRewrites) != 0 {
		t.Fatalf("marker must block second auto-enqueue, got %v", p.PendingRewrites)
	}
}

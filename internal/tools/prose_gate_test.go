package tools

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/voocel/ainovel-cli/internal/store"
)

// setupCommitStore tạo store sẵn sàng commit chương 1 (chương đầu, không cần boundary/foundation phức tạp).
func setupCommitStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	s := store.NewStore(dir)
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Progress.Init("test", 10); err != nil {
		t.Fatalf("InitProgress: %v", err)
	}
	return s
}

func commitArgs(t *testing.T, chapter int) json.RawMessage {
	t.Helper()
	args, err := json.Marshal(map[string]any{
		"chapter":         chapter,
		"summary":         "tóm tắt chương",
		"characters":      []string{"Lam", "Khang"},
		"key_events":      []string{"gặp gỡ"},
		"timeline_events": []any{},
	})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	return args
}

// TestProseGate_RejectsEmDash xác minh commit bị chặn khi prose chứa em dash.
func TestProseGate_RejectsEmDash(t *testing.T) {
	s := setupCommitStore(t)
	dirty := "# Chương 1\n\nLam bước vào phòng — căn phòng tối om. Cô lau bàn cẩn thận."
	if err := s.Drafts.SaveDraft(1, dirty); err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}
	tool := NewCommitChapterTool(s) // proseGate mặc định ON
	_, err := tool.Execute(context.Background(), commitArgs(t, 1))
	if err == nil {
		t.Fatal("expected commit rejected due to em dash")
	}
	if !strings.Contains(err.Error(), "em dash") && !strings.Contains(err.Error(), "prose tiếng Việt") {
		t.Errorf("error should mention prose standard: %v", err)
	}
}

// TestProseGate_RejectsEnglish xác minh commit bị chặn khi prose lọt từ tiếng Anh thật.
func TestProseGate_RejectsEnglish(t *testing.T) {
	s := setupCommitStore(t)
	dirty := "# Chương 1\n\nĐây là một journey đẹp, the kết thúc viên mãn cho cả hai người."
	if err := s.Drafts.SaveDraft(1, dirty); err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}
	tool := NewCommitChapterTool(s)
	_, err := tool.Execute(context.Background(), commitArgs(t, 1))
	if err == nil {
		t.Fatal("expected commit rejected due to English words")
	}
}

// TestProseGate_AllowsCleanVietnamese xác minh prose tiếng Việt sạch commit thành công.
func TestProseGate_AllowsCleanVietnamese(t *testing.T) {
	s := setupCommitStore(t)
	clean := "# Chương 1\n\nLam bước vào phòng làm việc. Căn phòng tối om, chỉ còn ánh đèn bàn hắt ra một vệt sáng vàng vọt. Cô lau từng ngóc ngách thật cẩn thận, lòng thầm mong mọi thứ sạch sẽ tinh tươm."
	if err := s.Drafts.SaveDraft(1, clean); err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}
	tool := NewCommitChapterTool(s)
	if _, err := tool.Execute(context.Background(), commitArgs(t, 1)); err != nil {
		t.Fatalf("clean Vietnamese prose should commit, got: %v", err)
	}
	if _, err := os.Stat(s.Dir() + "/chapters/01.md"); err != nil {
		t.Fatalf("chapter 1 should be persisted: %v", err)
	}
}

// TestProseGate_OffAllowsDirty xác minh khi tắt gate, prose bẩn vẫn commit (giữ tính tương thích).
func TestProseGate_OffAllowsDirty(t *testing.T) {
	s := setupCommitStore(t)
	dirty := "# Chương 1\n\nLam bước vào — phòng tối."
	if err := s.Drafts.SaveDraft(1, dirty); err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}
	tool := NewCommitChapterTool(s).WithProseGate(false)
	if _, err := tool.Execute(context.Background(), commitArgs(t, 1)); err != nil {
		t.Fatalf("with gate off, commit should succeed: %v", err)
	}
}

package store

import (
	"testing"

	"github.com/voocel/ainovel-cli/internal/domain"
)

func newProgressForQueue(t *testing.T) *Store {
	t.Helper()
	s := NewStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Progress.Init("test", 10); err != nil {
		t.Fatalf("Progress.Init: %v", err)
	}
	return s
}

func TestEnqueueRewriteAddsCompletedChapter(t *testing.T) {
	s := newProgressForQueue(t)
	if err := s.Progress.MarkChapterComplete(1, 3000, "", ""); err != nil {
		t.Fatalf("MarkChapterComplete: %v", err)
	}
	if err := s.Progress.EnqueueRewrite(1, "audit fail"); err != nil {
		t.Fatalf("EnqueueRewrite: %v", err)
	}
	p, _ := s.Progress.Load()
	if len(p.PendingRewrites) != 1 || p.PendingRewrites[0] != 1 {
		t.Fatalf("queue should contain chapter 1, got %v", p.PendingRewrites)
	}
	if p.Flow != domain.FlowRewriting {
		t.Fatalf("flow should switch to rewriting, got %s", p.Flow)
	}
	if p.RewriteReason != "audit fail" {
		t.Fatalf("reason should be recorded, got %q", p.RewriteReason)
	}
}

func TestEnqueueRewriteIsIdempotentAndMerges(t *testing.T) {
	s := newProgressForQueue(t)
	for ch := 1; ch <= 3; ch++ {
		if err := s.Progress.MarkChapterComplete(ch, 3000, "", ""); err != nil {
			t.Fatalf("MarkChapterComplete(%d): %v", ch, err)
		}
	}
	// Hàng đợi có sẵn chương 2 (editor đặt) — enqueue chương 3 phải GIỮ chương 2.
	if err := s.Progress.SetPendingRewrites([]int{2}, "editor polish"); err != nil {
		t.Fatalf("SetPendingRewrites: %v", err)
	}
	if err := s.Progress.SetFlow(domain.FlowRewriting); err != nil {
		t.Fatalf("SetFlow: %v", err)
	}
	if err := s.Progress.EnqueueRewrite(3, "audit fail ch3"); err != nil {
		t.Fatalf("EnqueueRewrite: %v", err)
	}
	p, _ := s.Progress.Load()
	if len(p.PendingRewrites) != 2 {
		t.Fatalf("queue should keep existing + new, got %v", p.PendingRewrites)
	}
	// Enqueue lại chương 3 → không nhân đôi
	if err := s.Progress.EnqueueRewrite(3, "audit fail ch3 again"); err != nil {
		t.Fatalf("EnqueueRewrite twice: %v", err)
	}
	p, _ = s.Progress.Load()
	if len(p.PendingRewrites) != 2 {
		t.Fatalf("enqueue must be idempotent, got %v", p.PendingRewrites)
	}
	// Reason được nối, không mất lý do cũ
	if p.RewriteReason == "" || p.RewriteReason == "audit fail ch3 again" {
		t.Fatalf("reasons should merge, got %q", p.RewriteReason)
	}
}

func TestEnqueueRewriteRejectsUncompletedChapter(t *testing.T) {
	s := newProgressForQueue(t)
	if err := s.Progress.MarkChapterComplete(1, 3000, "", ""); err != nil {
		t.Fatalf("MarkChapterComplete: %v", err)
	}
	if err := s.Progress.EnqueueRewrite(5, "audit fail"); err == nil {
		t.Fatal("uncompleted chapter must be rejected from rewrite queue")
	}
	p, _ := s.Progress.Load()
	if len(p.PendingRewrites) != 0 {
		t.Fatalf("queue should stay empty, got %v", p.PendingRewrites)
	}
	if p.Flow == domain.FlowRewriting {
		t.Fatal("flow must not switch when enqueue fails")
	}
}

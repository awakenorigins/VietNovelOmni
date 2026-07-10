package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/voocel/agentcore/schema"
	"github.com/voocel/ainovel-cli/internal/domain"
	"github.com/voocel/ainovel-cli/internal/errs"
	"github.com/voocel/ainovel-cli/internal/rules"
	"github.com/voocel/ainovel-cli/internal/store"
)

// CommitChapterTool lưu chương: tải nội dung → lưu bản chính → tạo tóm tắt → cập nhật trạng thái → cập nhật tiến độ.
type CommitChapterTool struct {
	store     *store.Store
	rulesOpts rules.LoadOptions // Tùy chọn; khi LoadOptions rỗng sẽ không tạo rule_violations
	proseGate bool             // OUMStudio-Novel: chặn cứng commit khi prose có lỗi error chắc chắn (em dash / tiếng Anh lọt)

	// OmniNovel post-commit hook: chạy SAU KHI commit thành công, audit chương qua toàn bộ skill.
	postCommitHook string // đường dẫn script; trống = tắt
	skillsDir      string // thư mục gốc chứa skills/ (truyền cho hook)
	outputDir      string // thư mục output/novel (truyền cho hook)
}

func NewCommitChapterTool(store *store.Store) *CommitChapterTool {
	// OUMStudio-Novel: bật proseGate mặc định để bảo đảm chất lượng prose tiếng Việt.
	return &CommitChapterTool{store: store, proseGate: true}
}

// WithProseGate bật/tắt cổng chặn cứng prose (mặc định bật).
// Khi bật, commit_chapter từ chối lưu nếu Lint phát hiện vi phạm mức error chắc chắn về cơ học
// (em_dash, english_words), buộc writer gọi draft_chapter viết lại. Các cảnh báo mức warning
// (markdown_residue, fatigue, cụm sáo) vẫn chỉ báo cáo, không chặn.
func (t *CommitChapterTool) WithProseGate(on bool) *CommitChapterTool {
	t.proseGate = on
	return t
}

// proseHardErrors lọc các vi phạm Lint mức error mà OUM coi là chặn cứng.
func proseHardErrors(content string, wordCount int) []rules.Violation {
	var hard []rules.Violation
	for _, v := range rules.Lint(content) {
		if v.Severity != rules.SeverityError {
			continue
		}
		// Chỉ chặn các loại chắc chắn là lỗi cơ học, không gây tranh cãi.
		if v.Rule == "em_dash" || v.Rule == "english_words" {
			hard = append(hard, v)
		}
	}
	return hard
}

// WithRules nhận tùy chọn tải quy tắc người dùng, giúp rule_violations kèm theo kết quả kiểm tra quy tắc người dùng.
// Nếu không gọi phương thức này thì chỉ thực thi Lint giới hạn tối thiểu tích hợp sẵn (kiểm tra cơ chế tồn dư, luôn bật).
func (t *CommitChapterTool) WithRules(opts rules.LoadOptions) *CommitChapterTool {
	t.rulesOpts = opts
	return t
}

// WithPostCommitHook (OmniNovel) cấu hình script chạy sau khi commit mỗi chương.
// hookPath trống = tắt. skillsDir/outputDir truyền xuống hook qua biến môi trường.
func (t *CommitChapterTool) WithPostCommitHook(hookPath, skillsDir, outputDir string) *CommitChapterTool {
	t.postCommitHook = hookPath
	t.skillsDir = skillsDir
	t.outputDir = outputDir
	return t
}

// runPostCommitHook chạy hook OmniNovel sau commit, rồi đóng vòng audit-feedback:
// đọc report meta/skill-audit/chNN.json do hook sinh ra; nếu pass_all=false thì tự đưa chương
// vào hàng đợi viết lại (EnqueueRewrite) — không chỉ ghi log. Mỗi chương chỉ bị tự đưa vào
// hàng đợi đúng MỘT lần (marker meta/skill-audit/chNN.enqueued) để chặn vòng lặp vô hạn
// commit→fail→rewrite→commit→fail. Bản thân hook vẫn best-effort: hook lỗi/thiếu report
// không phá pipeline.
func (t *CommitChapterTool) runPostCommitHook(chapter int) {
	if t.postCommitHook == "" {
		return
	}
	outDir := t.outputDir
	if outDir == "" {
		outDir = t.store.Dir()
	}
	chFile := filepath.Join(outDir, fmt.Sprintf("chapters/%02d.md", chapter))
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", t.postCommitHook)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("OMNI_CHAPTER=%d", chapter),
		"OMNI_CHAPTER_FILE="+chFile,
		"OMNI_OUTPUT_DIR="+outDir,
		"OMNI_SKILLS_DIR="+t.skillsDir,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		slog.Warn("post-commit hook lỗi (không chặn)", "module", "omni-hook", "chapter", chapter, "err", err, "output", strings.TrimSpace(string(output)))
		return
	}
	slog.Info("post-commit hook xong", "module", "omni-hook", "chapter", chapter, "output", strings.TrimSpace(string(output)))

	t.applyAuditFeedback(chapter, outDir)
}

// auditReport là phần tối thiểu của meta/skill-audit/chNN.json mà feedback-loop cần.
type auditReport struct {
	Chapter int  `json:"chapter"`
	PassAll bool `json:"pass_all"`
	Results map[string]struct {
		Parsed *struct {
			HardErrors []string `json:"hard_errors"`
		} `json:"parsed"`
	} `json:"results"`
}

// auditFailDetail rút lý do fail cụ thể từ report để writer vá CÓ MỤC TIÊU thay vì
// viết lại mò (rewrite mù dễ fail lần hai rồi buông). Giới hạn 3 lỗi đầu cho gọn.
func auditFailDetail(rep auditReport) string {
	var details []string
	for _, r := range rep.Results {
		if r.Parsed == nil {
			continue
		}
		for _, e := range r.Parsed.HardErrors {
			if strings.HasPrefix(e, "markdown") {
				continue // markdown không tính fail cứng (đồng bộ với hook)
			}
			details = append(details, e)
			if len(details) >= 3 {
				return strings.Join(details, "; ") + "…"
			}
		}
	}
	return strings.Join(details, "; ")
}

// applyAuditFeedback đọc report audit của chương và đóng vòng phản hồi:
// pass_all=false → EnqueueRewrite. Marker file chặn tái-enqueue lần hai.
func (t *CommitChapterTool) applyAuditFeedback(chapter int, outDir string) {
	reportPath := filepath.Join(outDir, "meta", "skill-audit", fmt.Sprintf("ch%02d.json", chapter))
	data, err := os.ReadFile(reportPath)
	if err != nil {
		// Không có report = hook không sinh được — không suy diễn gì thêm.
		return
	}
	var rep auditReport
	if err := json.Unmarshal(data, &rep); err != nil {
		slog.Warn("audit report không parse được (bỏ qua)", "module", "omni-hook", "chapter", chapter, "err", err)
		return
	}
	if rep.PassAll {
		return
	}

	// Chống lặp vô hạn: mỗi chương chỉ tự enqueue đúng một lần. Lần commit sau khi viết lại
	// nếu vẫn fail thì chỉ log — để editor/người dùng phán quyết, không xử tự động nữa.
	markerPath := filepath.Join(outDir, "meta", "skill-audit", fmt.Sprintf("ch%02d.enqueued", chapter))
	if _, err := os.Stat(markerPath); err == nil {
		slog.Warn("audit vẫn fail sau khi đã viết lại một lần — không tự enqueue nữa, cần editor/người dùng xử lý",
			"module", "omni-hook", "chapter", chapter, "report", reportPath)
		return
	}

	reason := fmt.Sprintf("skill-audit fail (ch%02d): xem %s", chapter, reportPath)
	if detail := auditFailDetail(rep); detail != "" {
		reason = fmt.Sprintf("skill-audit fail (ch%02d) — lỗi cụ thể phải vá: %s (chi tiết: %s)", chapter, detail, reportPath)
	}
	if err := t.store.Progress.EnqueueRewrite(chapter, reason); err != nil {
		slog.Warn("không đưa được chương vào hàng đợi viết lại (không chặn)", "module", "omni-hook", "chapter", chapter, "err", err)
		return
	}
	if err := os.WriteFile(markerPath, []byte(time.Now().Format(time.RFC3339)+"\n"), 0o644); err != nil {
		slog.Warn("không ghi được marker enqueue (nguy cơ lặp nếu lần sau vẫn fail)", "module", "omni-hook", "chapter", chapter, "err", err)
	}
	slog.Info("audit-feedback: chương fail audit đã vào hàng đợi viết lại", "module", "omni-hook", "chapter", chapter, "reason", reason)
}

// commitOutput nhúng thêm trường mở rộng lên trên domain.CommitResult, giữ package domain không phụ thuộc rules.
// Vì trường nhúng được JSON marshaler đưa lên cấp trên (promoted), kết quả serialize tương đương cấu trúc phẳng.
type commitOutput struct {
	domain.CommitResult
	RuleViolations     []rules.Violation `json:"rule_violations,omitempty"`
	StoryGuardWarnings []string          `json:"story_guard_warnings,omitempty"` // VietNovelOmni: cảnh báo biên kịch advisory (hook đơn điệu...)
}

func (t *CommitChapterTool) Name() string { return "commit_chapter" }
func (t *CommitChapterTool) Description() string {
	return "Lưu bản chính của chương. Tải bản nháp và lưu thành bản chính, cập nhật timeline, phục bút, quan hệ, trạng thái nhân vật và tiến độ." +
		"Trả về dữ liệu có cấu trúc: next_chapter / review_required / arc_end / volume_end / needs_expansion / book_complete / flow, v.v."
}
func (t *CommitChapterTool) Label() string { return "Lưu chương" }

// Công cụ ghi (thao tác nguyên tử liên miền: bản nháp→bản chính→tóm tắt→tiến độ→điểm khôi phục), cấm đồng thời.
func (t *CommitChapterTool) ReadOnly(_ json.RawMessage) bool        { return false }
func (t *CommitChapterTool) ConcurrencySafe(_ json.RawMessage) bool { return false }

func (t *CommitChapterTool) Schema() map[string]any {
	timelineSchema := schema.Object(
		schema.Property("time", schema.String("Thời gian trong truyện")).Required(),
		schema.Property("event", schema.String("Mô tả sự kiện")).Required(),
		schema.Property("characters", schema.Array("Nhân vật liên quan", schema.String(""))),
	)
	foreshadowSchema := schema.Object(
		schema.Property("id", schema.String("ID phục bút")).Required(),
		schema.Property("action", schema.Enum("Thao tác", "plant", "advance", "resolve")).Required(),
		schema.Property("description", schema.String("Mô tả phục bút (chỉ bắt buộc khi plant)")),
	)
	relationshipSchema := schema.Object(
		schema.Property("character_a", schema.String("Nhân vật A")).Required(),
		schema.Property("character_b", schema.String("Nhân vật B")).Required(),
		schema.Property("relation", schema.String("Mô tả quan hệ hiện tại")).Required(),
	)
	stateChangeSchema := schema.Object(
		schema.Property("entity", schema.String("Tên nhân vật hoặc thực thể")).Required(),
		schema.Property("field", schema.String("Thuộc tính thay đổi")).Required(),
		schema.Property("old_value", schema.String("Giá trị trước khi thay đổi")),
		schema.Property("new_value", schema.String("Giá trị sau khi thay đổi")).Required(),
		schema.Property("reason", schema.String("Lý do thay đổi")),
	)
	// OUMStudio-Novel: feedback đổi sang chuỗi tự do để tránh lỗi InputValidationError khi model
	// trả object dưới dạng JSON-encoded string (hay gặp khi failover model). Nội dung vẫn ghi nhận
	// nguyên vẹn; OutlineFeedback có UnmarshalJSON khoan dung nhận cả object lẫn string.
	feedbackSchema := schema.String("Phản hồi về đề cương (tùy chọn): mô tả sai lệch và đề xuất điều chỉnh cho phần tiếp theo, viết thành văn xuôi ngắn gọn")
	return schema.Object(
		schema.Property("chapter", schema.Int("Số chương")).Required(),
		schema.Property("summary", schema.String("Tóm tắt nội dung chương này (tối đa 200 chữ)")).Required(),
		schema.Property("characters", schema.Array("Tên nhân vật xuất hiện trong chương này", schema.String(""))).Required(),
		schema.Property("key_events", schema.Array("Các sự kiện chính của chương này", schema.String(""))).Required(),
		schema.Property("timeline_events", schema.Array("Sự kiện timeline trong chương này", timelineSchema)),
		schema.Property("foreshadow_updates", schema.Array("Thao tác phục bút", foreshadowSchema)),
		schema.Property("relationship_changes", schema.Array("Thay đổi quan hệ", relationshipSchema)),
		schema.Property("state_changes", schema.Array("Thay đổi trạng thái nhân vật/thực thể", stateChangeSchema)),
		schema.Property("cast_intros", schema.Array("Giới thiệu nhân vật phụ xuất hiện lần đầu trong chương này và có thể xuất hiện lại (không bao gồm nhân vật chính và các nhân vật đã có trong characters.json)", schema.Object(
			schema.Property("name", schema.String("Tên nhân vật")).Required(),
			schema.Property("brief_role", schema.String("Định vị một câu (ví dụ: chủ quán trọ / tay đánh bạc)")).Required(),
		))),
		schema.Property("hook_type", schema.Enum("Loại điểm móc cuối chương", "crisis", "mystery", "desire", "emotion", "choice")),
		schema.Property("dominant_strand", schema.Enum("Tuyến kể chủ đạo của chương này", "quest", "fire", "constellation")),
		schema.Property("feedback", feedbackSchema),
		schema.Property("self_review", schema.Object(
			schema.Property("strengths", schema.Array("1-3 điểm mạnh của chương này đáng giữ lại", schema.String(""))),
			schema.Property("weaknesses", schema.Array("1-3 điểm yếu của chương này cần sửa ở chương sau", schema.String(""))),
			schema.Property("lesson", schema.String("Bài học cụ thể tự rút ra sau khi đọc lại chương, áp dụng ngay cho chương sau (tự tiến hóa)")),
		)),
	)
}

func (t *CommitChapterTool) Execute(_ context.Context, args json.RawMessage) (json.RawMessage, error) {
	var a struct {
		Chapter             int                        `json:"chapter"`
		Summary             string                     `json:"summary"`
		Characters          []string                   `json:"characters"`
		KeyEvents           []string                   `json:"key_events"`
		TimelineEvents      []domain.TimelineEvent     `json:"timeline_events"`
		ForeshadowUpdates   []domain.ForeshadowUpdate  `json:"foreshadow_updates"`
		RelationshipChanges []domain.RelationshipEntry `json:"relationship_changes"`
		StateChanges        []domain.StateChange       `json:"state_changes"`
		CastIntros          []domain.CastIntro         `json:"cast_intros"`
		HookType            string                     `json:"hook_type"`
		DominantStrand      string                     `json:"dominant_strand"`
		Feedback            *domain.OutlineFeedback    `json:"feedback"`
		SelfReview          *domain.SelfReview         `json:"self_review"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("invalid args: %w: %w", errs.ErrToolArgs, err)
	}
	if a.Chapter <= 0 {
		return nil, fmt.Errorf("chapter must be > 0: %w", errs.ErrToolArgs)
	}
	if t.store.Progress.IsChapterCompleted(a.Chapter) {
		// Dọn dẹp PendingCommit có thể còn tồn đọng (xảy ra khi crash sau ProgressMarked, trước ClearPendingCommit)
		if pending, _ := t.store.Signals.LoadPendingCommit(); pending != nil && pending.Chapter == a.Chapter {
			_ = t.store.Signals.ClearPendingCommit()
		}
		// Đường dẫn đánh bóng/viết lại: chương đã hoàn thành nhưng vẫn trong pending_rewrites, cho phép ghi đè và drain hàng đợi
		progress, _ := t.store.Progress.Load()
		if progress != nil && slices.Contains(progress.PendingRewrites, a.Chapter) {
			return t.executeRewriteCommit(a.Chapter, a.Summary, a.Characters, a.KeyEvents,
				a.HookType, a.DominantStrand, progress)
		}
		return t.buildSkipResult(a.Chapter, progress)
	}
	existingPending, err := t.store.Signals.LoadPendingCommit()
	if err != nil {
		return nil, fmt.Errorf("load pending commit: %w: %w", errs.ErrStoreRead, err)
	}
	if existingPending != nil && existingPending.Chapter != a.Chapter {
		return nil, fmt.Errorf("tồn tại lần lưu chương chưa khôi phục: chương %d (giai đoạn %s), vui lòng khôi phục hoặc nộp lại chương đó trước: %w", existingPending.Chapter, existingPending.Stage, errs.ErrToolConflict)
	}
	if err := t.store.Progress.ValidateChapterWork(a.Chapter); err != nil {
		// Xung đột hàng đợi giữ nguyên (đã có phân loại ErrToolConflict); các lỗi IO khác xếp vào Precondition.
		if errors.Is(err, errs.ErrToolConflict) {
			return nil, err
		}
		return nil, fmt.Errorf("chương hiện tại không được phép lưu: %w: %w", errs.ErrToolPrecondition, err)
	}

	// Chặn vượt giới hạn chế độ phân lớp: phải đặt trước mọi thao tác ghi, nếu không commit vượt giới
	// sẽ làm hỏng file chương, tóm tắt và Progress. boundary được tái sử dụng ở bước 6b để tính tín hiệu cung/cuốn.
	var boundary *store.ArcBoundary
	if progress, perr := t.store.Progress.Load(); perr == nil && progress != nil && progress.Layered {
		b, bErr := t.store.Outline.CheckArcBoundary(a.Chapter)
		if bErr != nil {
			return nil, fmt.Errorf("kiểm tra biên giới cung thất bại chapter=%d: %w: %w", a.Chapter, errs.ErrStoreRead, bErr)
		}
		if b == nil {
			return nil, fmt.Errorf(
				"chương %d nằm ngoài phạm vi đề cương phân lớp: cần expand_arc để mở rộng cung hoặc append_volume để thêm cuốn trước khi viết; nếu toàn bộ sách đã hoàn thành hãy gọi save_foundation type=complete_book: %w",
				a.Chapter, errs.ErrToolPrecondition)
		}
		boundary = b
	}

	// 1. Tải nội dung chương
	content, wordCount, err := t.store.Drafts.LoadChapterContent(a.Chapter)
	if err != nil {
		return nil, fmt.Errorf("load chapter content: %w: %w", errs.ErrStoreRead, err)
	}
	if content == "" {
		return nil, fmt.Errorf("no content found for chapter %d: %w", a.Chapter, errs.ErrToolPrecondition)
	}

	// 1b. OUMStudio-Novel — CỔNG CHỞsN CỨNG PROSE: từ chối commit khi prose có lỗi error cơ học chắc chắn.
	if t.proseGate {
		if hard := proseHardErrors(content, wordCount); len(hard) > 0 {
			parts := make([]string, 0, len(hard))
			for _, v := range hard {
				parts = append(parts, fmt.Sprintf("%s(%q ×%d)", v.Rule, v.Target, v.Actual))
			}
			return nil, fmt.Errorf("chương %d không đạt chuẩn prose tiếng Việt: %s. Prose phải 0 em dash (—), 0 từ tiếng Anh lọt. Hãy gọi draft_chapter(mode=write, chapter=%d) viết lại, bỏ các lỗi trên, rồi commit_chapter lại: %w",
				a.Chapter, strings.Join(parts, ", "), a.Chapter, errs.ErrToolPrecondition)
		}
	}

	now := time.Now().Format(time.RFC3339)
	pending := domain.PendingCommit{
		Chapter:        a.Chapter,
		Stage:          domain.CommitStageStarted,
		Summary:        a.Summary,
		HookType:       a.HookType,
		DominantStrand: a.DominantStrand,
		StartedAt:      now,
		UpdatedAt:      now,
	}
	if err := t.store.Signals.SavePendingCommit(pending); err != nil {
		return nil, fmt.Errorf("save pending commit: %w: %w", errs.ErrStoreWrite, err)
	}

	// 2. Lưu bản chính
	if err := t.store.Drafts.SaveFinalChapter(a.Chapter, content); err != nil {
		return nil, fmt.Errorf("save final chapter: %w: %w", errs.ErrStoreWrite, err)
	}

	// 3. Lưu tóm tắt
	summary := domain.ChapterSummary{
		Chapter:    a.Chapter,
		Summary:    a.Summary,
		Characters: a.Characters,
		KeyEvents:  a.KeyEvents,
	}
	if err := t.store.Summaries.SaveSummary(summary); err != nil {
		return nil, fmt.Errorf("save summary: %w: %w", errs.ErrStoreWrite, err)
	}

	// 4. Cập nhật gia số trạng thái
	if len(a.TimelineEvents) > 0 {
		for i := range a.TimelineEvents {
			a.TimelineEvents[i].Chapter = a.Chapter
		}
		if err := t.store.World.AppendTimelineEvents(a.TimelineEvents); err != nil {
			return nil, fmt.Errorf("append timeline: %w: %w", errs.ErrStoreWrite, err)
		}
	}
	if len(a.ForeshadowUpdates) > 0 {
		if err := t.store.World.UpdateForeshadow(a.Chapter, a.ForeshadowUpdates); err != nil {
			return nil, fmt.Errorf("update foreshadow: %w: %w", errs.ErrStoreWrite, err)
		}
	}
	if len(a.RelationshipChanges) > 0 {
		for i := range a.RelationshipChanges {
			a.RelationshipChanges[i].Chapter = a.Chapter
		}
		if err := t.store.World.UpdateRelationships(a.RelationshipChanges); err != nil {
			return nil, fmt.Errorf("update relationships: %w: %w", errs.ErrStoreWrite, err)
		}
	}
	if len(a.StateChanges) > 0 {
		for i := range a.StateChanges {
			a.StateChanges[i].Chapter = a.Chapter
		}
		if err := t.store.World.AppendStateChanges(a.StateChanges); err != nil {
			return nil, fmt.Errorf("append state changes: %w: %w", errs.ErrStoreWrite, err)
		}
	}

	// 4b. Tích lũy danh sách nhân vật phụ: nhân vật không phải cốt lõi xuất hiện trong chương này được đưa vào cast_ledger để novel_context truy xuất.
	// Khi thất bại chỉ cảnh báo, không chặn lưu chương — danh sách này là dữ liệu phụ, có thể tự phục hồi qua lần lưu chương tiếp theo.
	if len(a.Characters) > 0 {
		coreNames := loadCoreCharacterNameSet(t.store)
		if err := t.store.Cast.MergeAppearances(a.Chapter, a.Characters, a.CastIntros, coreNames); err != nil {
			slog.Warn("tích lũy danh sách nhân vật phụ thất bại, bỏ qua", "module", "commit", "chapter", a.Chapter, "err", err)
		}
	}

	pending.Stage = domain.CommitStageStateApplied
	pending.UpdatedAt = time.Now().Format(time.RFC3339)
	if err := t.store.Signals.SavePendingCommit(pending); err != nil {
		return nil, fmt.Errorf("update pending commit stage: %w: %w", errs.ErrStoreWrite, err)
	}

	// 5. Cập nhật tiến độ
	if err := t.store.Progress.MarkChapterComplete(a.Chapter, wordCount, a.HookType, a.DominantStrand); err != nil {
		return nil, fmt.Errorf("mark chapter complete: %w: %w", errs.ErrStoreWrite, err)
	}

	// 5b. OmniForge self-absorption: lưu tự review của Người viết (nếu có) để feed bài học tới chương sau.
	if a.SelfReview != nil && strings.TrimSpace(a.SelfReview.Lesson) != "" {
		sr := *a.SelfReview
		sr.Chapter = a.Chapter
		if sr.CreatedAt == "" {
			sr.CreatedAt = time.Now().Format(time.RFC3339)
		}
		if err := t.store.World.SaveSelfReview(a.Chapter, sr); err != nil {
			slog.Warn("không lưu được self_review", "module", "omni-hook", "chapter", a.Chapter, "err", err)
		}
	}

	// 6. Kiểm tra xem có cần xét duyệt không
	progress, err := t.store.Progress.Load()
	if err != nil {
		return nil, fmt.Errorf("load progress: %w: %w", errs.ErrStoreRead, err)
	}
	completedCount := 0
	if progress != nil {
		completedCount = len(progress.CompletedChapters)
	}

	// 6b. Tín hiệu cung/cuốn chế độ dài: boundary đã được xác thực ở đầu vào, đảm bảo khác nil khi Layered
	var arcEnd, volumeEnd, needsExpansion, needsNewVolume bool
	var vol, arc, nextVol, nextArc int
	if progress != nil && progress.Layered && boundary != nil {
		arcEnd = boundary.IsArcEnd
		volumeEnd = boundary.IsVolumeEnd
		vol = boundary.Volume
		arc = boundary.Arc
		needsExpansion = boundary.NeedsExpansion
		needsNewVolume = boundary.NeedsNewVolume
		nextVol = boundary.NextVolume
		nextArc = boundary.NextArc
		_ = t.store.Progress.UpdateVolumeArc(vol, arc)
	}

	var reviewRequired bool
	var reviewReason string
	if progress != nil && progress.Layered {
		reviewRequired, reviewReason = domain.ShouldArcReview(arcEnd, volumeEnd, vol, arc)
	} else {
		reviewRequired, reviewReason = domain.ShouldReview(completedCount)
	}

	// 7. Xây dựng tín hiệu có cấu trúc
	result := domain.CommitResult{
		Chapter:        a.Chapter,
		Committed:      true,
		WordCount:      wordCount,
		NextChapter:    a.Chapter + 1,
		ReviewRequired: reviewRequired,
		ReviewReason:   reviewReason,
		HookType:       a.HookType,
		DominantStrand: a.DominantStrand,
		Feedback:       a.Feedback,
		ArcEnd:         arcEnd,
		VolumeEnd:      volumeEnd,
		Volume:         vol,
		Arc:            arc,
		NeedsExpansion: needsExpansion,
		NeedsNewVolume: needsNewVolume,
		NextVolume:     nextVol,
		NextArc:        nextArc,
	}

	// 8. Xác định trạng thái hoàn tất: không phân lớp viết xong chương cuối / phân lớp viết xong chương cuối của cuốn cuối → MarkComplete
	if t.applyCompletion(&result, progress) {
		result.BookComplete = true
	}
	if p, _ := t.store.Progress.Load(); p != nil {
		result.Flow = string(p.Flow)
	}

	pending.Stage = domain.CommitStageProgressMarked
	pending.Result = &result
	pending.UpdatedAt = time.Now().Format(time.RFC3339)
	if err := t.store.Signals.SavePendingCommit(pending); err != nil {
		return nil, fmt.Errorf("update pending commit result: %w: %w", errs.ErrStoreWrite, err)
	}

	// 9. Xóa trạng thái trung gian của tiến độ
	if err := t.store.Progress.ClearInProgress(); err != nil {
		return nil, fmt.Errorf("clear in-progress: %w: %w", errs.ErrStoreWrite, err)
	}
	if err := t.store.Signals.ClearPendingCommit(); err != nil {
		return nil, fmt.Errorf("clear pending commit: %w: %w", errs.ErrStoreWrite, err)
	}

	// 10. Thêm điểm khôi phục
	if _, err := t.store.Checkpoints.AppendArtifact(
		domain.ChapterScope(a.Chapter), "commit",
		fmt.Sprintf("chapters/%02d.md", a.Chapter),
	); err != nil {
		return nil, fmt.Errorf("checkpoint commit: %w: %w", errs.ErrStoreWrite, err)
	}

	// 11. Kiểm tra quy tắc cơ học (chỉ trả về dữ liệu thực tế, không chặn)
	violations := t.checkRules(content, wordCount)

	// 11b. VietNovelOmni — guard biên kịch advisory (loop-safe: chỉ phát tín hiệu, không chặn).
	// Chương 1 = cửa sinh tử giữ chân độc giả: soi mở đầu sáo + mở đầu tĩnh.
	if a.Chapter == 1 {
		violations = append(violations, firstChapterHookIssues(content)...)
	}
	// Hook đơn điệu: cùng loại móc lặp liên tiếp — trả cảnh báo trong kết quả commit để coordinator/writer chương sau thấy.
	var storyGuardWarnings []string
	if progress != nil {
		hist := progress.HookHistory
		if len(hist) > 0 {
			hist = hist[:len(hist)-1] // bỏ mục của chính chương vừa MarkChapterComplete
		}
		if w := hookMonotonyWarning(hist, a.HookType); w != "" {
			storyGuardWarnings = append(storyGuardWarnings, w)
			slog.Warn("hook monotony", "module", "story-guard", "chapter", a.Chapter, "hook", a.HookType)
		}
	}

	// 12. OmniNovel post-commit hook: audit chương qua toàn bộ skill (best-effort, không chặn).
	t.runPostCommitHook(a.Chapter)

	return json.Marshal(commitOutput{CommitResult: result, RuleViolations: violations, StoryGuardWarnings: storyGuardWarnings})
}

// checkRules kiểm tra cơ học nội dung chương: Lint giới hạn tối thiểu tích hợp sẵn (kiểm tra cơ chế tồn dư, luôn thực thi)
// + Check quy tắc người dùng (khi rulesOpts rỗng hoàn toàn, loader trả về layers rỗng, checker trả về nil).
func (t *CommitChapterTool) checkRules(text string, wordCount int) []rules.Violation {
	violations := rules.Lint(text)
	bundle := rules.Merge(rules.Load(t.rulesOpts))
	return append(violations, rules.Check(text, wordCount, bundle.Structured)...)
}

// executeRewriteCommit xử lý lưu chương khi đánh bóng/viết lại: ghi đè bản chính và tóm tắt, cập nhật số từ, drain hàng đợi.
// Bỏ qua toàn bộ thao tác bổ sung trạng thái thế giới (timeline / foreshadow / relationship / state_changes) và kiểm tra biên giới cung,
// vì những thứ này đã được áp dụng trong lần lưu gốc của chương.
func (t *CommitChapterTool) executeRewriteCommit(
	chapter int,
	summary string,
	characters, keyEvents []string,
	hookType, dominantStrand string,
	progress *domain.Progress,
) (json.RawMessage, error) {
	// 1. Tải nội dung sau khi đánh bóng
	content, wordCount, err := t.store.Drafts.LoadChapterContent(chapter)
	if err != nil {
		return nil, fmt.Errorf("rewrite: load chapter content: %w: %w", errs.ErrStoreRead, err)
	}
	if content == "" {
		return nil, fmt.Errorf("no content found for chapter %d: %w", chapter, errs.ErrToolPrecondition)
	}

	// 2. Kiểm tra cứng: drafts hoàn toàn giống bản chính hiện tại → chưa thực sự đánh bóng/viết lại (writer bỏ qua draft_chapter)
	// Từ chối lưu, buộc writer gọi draft_chapter(mode=write) trước để ghi phiên bản mới.
	existingFinal, _ := t.store.Drafts.LoadChapterText(chapter)
	if existingFinal != "" && existingFinal == content {
		mode := "viết lại"
		if progress != nil && progress.Flow == domain.FlowPolishing {
			mode = "đánh bóng"
		}
		return nil, fmt.Errorf("nội dung drafts và chapters của chương %d hoàn toàn giống nhau, không phát hiện thay đổi %s. Vui lòng gọi draft_chapter(mode=write, chapter=%d) để ghi nội dung mới sau khi %s, rồi mới commit_chapter: %w",
			chapter, mode, chapter, mode, errs.ErrToolPrecondition)
	}

	// 3. Ghi đè bản chính
	if err := t.store.Drafts.SaveFinalChapter(chapter, content); err != nil {
		return nil, fmt.Errorf("rewrite: save final chapter: %w: %w", errs.ErrStoreWrite, err)
	}

	// 3. Ghi đè tóm tắt
	if err := t.store.Summaries.SaveSummary(domain.ChapterSummary{
		Chapter:    chapter,
		Summary:    summary,
		Characters: characters,
		KeyEvents:  keyEvents,
	}); err != nil {
		return nil, fmt.Errorf("rewrite: save summary: %w: %w", errs.ErrStoreWrite, err)
	}

	// 4. Cập nhật số từ (MarkChapterComplete với chương đã hoàn thành là idempotent: thay thế số từ, slice.Contains ngăn thêm trùng vào hàng đợi)
	if err := t.store.Progress.MarkChapterComplete(chapter, wordCount, hookType, dominantStrand); err != nil {
		return nil, fmt.Errorf("rewrite: update word count: %w: %w", errs.ErrStoreWrite, err)
	}

	// 5. Drain hàng đợi chờ xử lý; khi hàng đợi rỗng CompleteRewrite sẽ tự chuyển flow về writing
	if err := t.store.Progress.CompleteRewrite(chapter); err != nil {
		return nil, fmt.Errorf("rewrite: complete rewrite: %w: %w", errs.ErrStoreWrite, err)
	}

	// 6. Điểm khôi phục
	if _, err := t.store.Checkpoints.AppendArtifact(
		domain.ChapterScope(chapter), "commit",
		fmt.Sprintf("chapters/%02d.md", chapter),
	); err != nil {
		return nil, fmt.Errorf("rewrite: checkpoint commit: %w: %w", errs.ErrStoreWrite, err)
	}

	// 7. Đọc snapshot Progress sau khi drain, trả về dữ liệu thực tế
	mode := "rewrite"
	if progress.Flow == domain.FlowPolishing {
		mode = "polish"
	}
	latest, _ := t.store.Progress.Load()
	remaining := []int{}
	nextChapter := chapter + 1
	flow := string(domain.FlowWriting)
	if latest != nil {
		remaining = append(remaining, latest.PendingRewrites...)
		nextChapter = latest.NextChapter()
		flow = string(latest.Flow)
	}
	drained := len(remaining) == 0

	// Sau khi hàng đợi rỗng mới kiểm tra hoàn tất: lưu sau khi viết lại không đi qua applyCompletion của đường chính,
	// hoàn tất chỉ có thể kích hoạt tại đây.
	//   - Phân lớp + viết tiến về phía trước: dùng layeredBookComplete cấp chất lượng (yêu cầu các tuyến dài thu lại), chưa thỏa thì nhường kiến trúc sư.
	//   - Phân lớp + viết lại reopen (ReopenedFromComplete): viết lại chỉ sửa chương đã có, không thêm/bớt cấu trúc,
	//     khi cấu trúc còn nguyên vẹn thì hoàn tất lại — nếu viết lại làm xáo trộn một tuyến truyện thì kẹt ở writing,
	//     cuối cuốn cuối sẽ rơi vào vòng lặp vô hạn do viết vượt giới hạn.
	//   - Không phân lớp: viết đủ TotalChapters là hoàn tất (viết lại không thêm/bớt chương, ban đầu đã đủ).
	bookComplete := false
	if drained && latest != nil {
		reComplete := false
		switch {
		case latest.Layered && latest.ReopenedFromComplete:
			reComplete = t.layeredStructurallyComplete(latest)
		case latest.Layered:
			reComplete = t.layeredBookComplete(latest)
		default:
			reComplete = latest.TotalChapters > 0 && len(latest.CompletedChapters) >= latest.TotalChapters
		}
		if reComplete {
			if cerr := t.store.Progress.MarkComplete(); cerr == nil {
				bookComplete = true
				if p, _ := t.store.Progress.Load(); p != nil {
					flow = string(p.Flow)
				}
			}
		}
	}

	// Giống đường chính: rewrite/polish cũng kiểm tra cơ học và kèm rule_violations
	violations := t.checkRules(content, wordCount)

	// OmniNovel post-commit hook (rewrite path)
	t.runPostCommitHook(chapter)

	return json.Marshal(map[string]any{
		"chapter":         chapter,
		"rewritten":       true,
		"mode":            mode,
		"word_count":      wordCount,
		"remaining_queue": remaining,
		"queue_drained":   drained,
		"next_chapter":    nextChapter,
		"flow":            flow,
		"book_complete":   bookComplete,
		"rule_violations": violations,
	})
}

// buildSkipResult tạo kết quả trả về cho "lưu trùng lặp chương đã hoàn thành", căn chỉnh với commit bình thường.
// Điều phối viên dựa vào đây để quyết định phân phát tiếp (writer/editor/architect), tránh ảo giác do nhận prose.
func (t *CommitChapterTool) buildSkipResult(chapter int, progress *domain.Progress) (json.RawMessage, error) {
	_, wordCount, _ := t.store.Drafts.LoadChapterContent(chapter)

	result := domain.CommitResult{
		Chapter:     chapter,
		Committed:   true,
		WordCount:   wordCount,
		NextChapter: chapter + 1,
	}

	if progress != nil && progress.Layered {
		if boundary, _ := t.store.Outline.CheckArcBoundary(chapter); boundary != nil {
			result.ArcEnd = boundary.IsArcEnd
			result.VolumeEnd = boundary.IsVolumeEnd
			result.Volume = boundary.Volume
			result.Arc = boundary.Arc
			result.NeedsExpansion = boundary.NeedsExpansion
			result.NeedsNewVolume = boundary.NeedsNewVolume
			result.NextVolume = boundary.NextVolume
			result.NextArc = boundary.NextArc
		}
		result.ReviewRequired, result.ReviewReason = domain.ShouldArcReview(result.ArcEnd, result.VolumeEnd, result.Volume, result.Arc)
	} else if progress != nil {
		result.ReviewRequired, result.ReviewReason = domain.ShouldReview(len(progress.CompletedChapters))
	}

	if progress != nil {
		if progress.Phase == domain.PhaseComplete {
			result.BookComplete = true
		}
		result.Flow = string(progress.Flow)
	}

	return json.Marshal(result)
}

// loadCoreCharacterNameSet tải tập hợp tên nhân vật đã có trong characters.json (bao gồm bí danh).
// Dùng làm tập lọc "nhân vật cốt lõi đã biết" cho cast_ledger — nhân vật cốt lõi không vào danh sách phụ.
// Khi tải thất bại trả về nil (khi merge tất cả characters đều vào ledger, chấp nhận được).
func loadCoreCharacterNameSet(s *store.Store) map[string]bool {
	chars, err := s.Characters.Load()
	if err != nil || len(chars) == 0 {
		return nil
	}
	set := make(map[string]bool, len(chars)*2)
	for _, c := range chars {
		if c.Name != "" {
			set[c.Name] = true
		}
		for _, alias := range c.Aliases {
			if alias != "" {
				set[alias] = true
			}
		}
	}
	return set
}

// applyCompletion kiểm tra xem lần lưu này có khiến toàn bộ sách hoàn tất không; nếu có thì MarkComplete và trả về true.
//   - Không phân lớp: viết đủ tổng số chương đã hẹn là hoàn tất.
//   - Phân lớp: kiến trúc sư gọi save_foundation type=complete_book là đường chính; đây thêm một lớp đảm bảo
//     tự động — khi sách đã khách quan thỏa điều kiện hoàn tất (xem layeredBookComplete) thì tự kết thúc.
//     Ngăn model không gọi append_volume cũng không gọi complete_book ở điểm cuối, dẫn đến "người viết viết vượt giới →
//     StopGuard chặn → thử lại vô hạn" (livelock, nguyên nhân gốc của trường hợp ch204..347).
func (t *CommitChapterTool) applyCompletion(result *domain.CommitResult, progress *domain.Progress) bool {
	if progress == nil {
		return false
	}
	if progress.Layered {
		if t.layeredBookComplete(progress) {
			_ = t.store.Progress.MarkComplete()
			return true
		}
		return false
	}
	if progress.TotalChapters > 0 && result.NextChapter > progress.TotalChapters {
		_ = t.store.Progress.MarkComplete()
		return true
	}
	return false
}

// layeredStructurallyComplete kiểm định phân lớp dài có "hoàn tất về mặt cấu trúc" không:
// hàng đợi viết lại rỗng + không còn cung xương sống chờ mở rộng + tất cả chương đã mở rộng đều đã viết.
// Đây là dữ liệu thực tế xác định về trạng thái kết thúc, không bao gồm đánh giá ngữ nghĩa như phục bút/tuyến dài —
// dùng làm lưới an toàn "chống vòng lặp vô hạn trạng thái kết thúc" (tái hoàn tất sau khi hàng đợi viết lại rỗng).
func (t *CommitChapterTool) layeredStructurallyComplete(progress *domain.Progress) bool {
	// 1. Hàng đợi viết lại phải rỗng
	if len(progress.PendingRewrites) > 0 {
		return false
	}
	volumes, err := t.store.Outline.LoadLayeredOutline()
	if err != nil || len(volumes) == 0 {
		return false
	}
	// 2. Không còn cung xương sống nào chờ mở rộng (vẫn còn nội dung cần viết theo kế hoạch)
	for i := range volumes {
		for j := range volumes[i].Arcs {
			if !volumes[i].Arcs[j].IsExpanded() {
				return false
			}
		}
	}
	// 3. Tất cả chương đã mở rộng phải được viết hoàn chỉnh
	expanded := len(domain.FlattenOutline(volumes))
	return expanded > 0 && len(progress.CompletedChapters) >= expanded
}

// layeredBookComplete dùng dữ liệu khách quan để phán định phân lớp dài có thực sự hoàn tất không,
// đối chiếu với các hạng mục định lượng trong danh sách kiểm tra hoàn tất của architect-long.md + dữ liệu cấu trúc.
// Ngoài cấu trúc hoàn chỉnh còn yêu cầu phục bút归零, tuyến dài thu lại — bất kỳ điều kiện nào chưa thỏa
// đều nhường lại kiến trúc sư tiếp tục expand_arc / append_volume, tuyệt đối không kết thúc khi câu chuyện chưa xong.
// Khi không có compass thì đánh giá thận trọng là chưa hoàn tất. Đây là phán định "cấp chất lượng" cho viết tiến về phía trước,
// nghiêm ngặt hơn layeredStructurallyComplete.
func (t *CommitChapterTool) layeredBookComplete(progress *domain.Progress) bool {
	if !t.layeredStructurallyComplete(progress) {
		return false
	}
	// 4. Phục bút hoạt động phải归零 (tất cả đã thực hiện lời hứa)
	if active, aerr := t.store.World.LoadActiveForeshadow(); aerr != nil || len(active) > 0 {
		return false
	}
	// 5. Các tuyến dài hoạt động trong compass phải được thu lại (không có compass / tuyến dài chưa xong đều trả lại kiến trúc sư phán quyết)
	compass, cerr := t.store.Outline.LoadCompass()
	if cerr != nil || compass == nil || len(compass.OpenThreads) > 0 {
		return false
	}
	return true
}

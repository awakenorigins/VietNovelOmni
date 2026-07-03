package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/voocel/agentcore/schema"
	"github.com/voocel/ainovel-cli/internal/domain"
	"github.com/voocel/ainovel-cli/internal/errs"
	"github.com/voocel/ainovel-cli/internal/store"
)

// DraftBeatTool (OmniNovel — beat-method) viết một beat quan trọng theo quy trình 3 bản:
//
//	3 bản độc lập (nội tâm / đối thoại / hành động) → chấm điểm → trộn tinh hoa thành bản cuối
//	→ cổng audit cơ học (em dash / tiếng Anh lọt / độ dài) → CHỈ KHI PASS mới append vào bản nháp.
//
// Khác draft_chapter(mode=append) ở chỗ: append qua draft_beat bắt buộc đi qua gate,
// và toàn bộ 3 bản + điểm + lý do trộn được lưu lại drafts/NN.beat-MM.json làm hồ sơ truy vết.
// Nguyên tắc sắt: tool chỉ trả sự thật; bản không đạt gate bị từ chối kèm lý do cơ học cụ thể,
// writer tự sửa rồi gọi lại — không có đường vòng nào ghi beat chưa qua gate xuống draft.
type DraftBeatTool struct {
	store *store.Store
}

func NewDraftBeatTool(store *store.Store) *DraftBeatTool {
	return &DraftBeatTool{store: store}
}

func (t *DraftBeatTool) Name() string { return "draft_beat" }
func (t *DraftBeatTool) Description() string {
	return "Viết một beat quan trọng (mở chương, cao trào, twist, kết) theo phương pháp 3 bản: " +
		"nộp đủ 3 bản độc lập (nội tâm/đối thoại/hành động) kèm điểm 0-100, cùng bản cuối đã trộn tinh hoa. " +
		"Tool chạy cổng audit cơ học trên bản cuối; PASS mới append vào bản nháp chương, FAIL trả lỗi để viết lại. " +
		"Hồ sơ 3 bản + điểm được lưu để truy vết."
}
func (t *DraftBeatTool) Label() string { return "Viết beat (3 bản)" }

// Công cụ ghi (append draft + ghi hồ sơ beat), cấm chạy đồng thời.
func (t *DraftBeatTool) ReadOnly(_ json.RawMessage) bool        { return false }
func (t *DraftBeatTool) ConcurrencySafe(_ json.RawMessage) bool { return false }

// beatFocuses là 3 hướng bắt buộc của beat-method.
var beatFocuses = []string{"noi_tam", "doi_thoai", "hanh_dong"}

func (t *DraftBeatTool) Schema() map[string]any {
	variantSchema := schema.Object(
		schema.Property("focus", schema.Enum("Hướng của bản này", "noi_tam", "doi_thoai", "hanh_dong")).Required(),
		schema.Property("content", schema.String("Nội dung bản viết (văn xuôi thuần)")).Required(),
		schema.Property("score", schema.Int("Tự chấm 0-100: sức sống, đúng nhịp, đúng nhân vật")).Required(),
		// note đánh dấu required để tương thích OpenAI strict tool calling —
		// chế độ strict yêu cầu tất cả properties đều có trong danh sách required.
		schema.Property("note", schema.String("Một câu: điểm mạnh/yếu của bản này (có thể để chuỗi rỗng)")).Required(),
	)
	return schema.Object(
		schema.Property("chapter", schema.Int("Số chương")).Required(),
		schema.Property("beat", schema.Int("Số thứ tự beat trong chương (1, 2, 3...)")).Required(),
		schema.Property("beat_goal", schema.String("Beat này phải đạt gì (một câu)")).Required(),
		schema.Property("variants", schema.Array("Đúng 3 bản độc lập, mỗi hướng một bản: noi_tam / doi_thoai / hanh_dong", variantSchema)).Required(),
		schema.Property("final_content", schema.String("Bản cuối đã trộn tinh hoa từ 3 bản — đây là phần sẽ được append vào bản nháp")).Required(),
		schema.Property("merge_rationale", schema.String("Trộn thế nào: lấy gì từ bản nào, vì sao")).Required(),
	)
}

func (t *DraftBeatTool) StrictSchema() bool { return true }

// beatVariant là một bản viết độc lập trong hồ sơ beat.
type beatVariant struct {
	Focus   string `json:"focus"`
	Content string `json:"content"`
	Score   int    `json:"score"`
	Note    string `json:"note,omitempty"`
}

// beatRecord là hồ sơ truy vết của một beat, lưu tại drafts/NN.beat-MM.json.
type beatRecord struct {
	Chapter        int           `json:"chapter"`
	Beat           int           `json:"beat"`
	BeatGoal       string        `json:"beat_goal"`
	Variants       []beatVariant `json:"variants"`
	FinalContent   string        `json:"final_content"`
	MergeRationale string        `json:"merge_rationale"`
	FinalWordCount int           `json:"final_word_count"`
	GatePassed     bool          `json:"gate_passed"`
	CreatedAt      string        `json:"created_at"`
}

// minBeatVariantRunes là độ dài tối thiểu của mỗi bản — chặn "bản làm phép" một hai câu cho có.
const minBeatVariantRunes = 120

// minBeatFinalRunes là độ dài tối thiểu của bản cuối.
const minBeatFinalRunes = 150

func (t *DraftBeatTool) Execute(_ context.Context, args json.RawMessage) (json.RawMessage, error) {
	var a struct {
		Chapter        int           `json:"chapter"`
		Beat           int           `json:"beat"`
		BeatGoal       string        `json:"beat_goal"`
		Variants       []beatVariant `json:"variants"`
		FinalContent   string        `json:"final_content"`
		MergeRationale string        `json:"merge_rationale"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fmt.Errorf("invalid args: %w: %w", errs.ErrToolArgs, err)
	}
	if a.Chapter <= 0 {
		return nil, fmt.Errorf("chapter must be > 0: %w", errs.ErrToolArgs)
	}
	if a.Beat <= 0 {
		return nil, fmt.Errorf("beat must be > 0: %w", errs.ErrToolArgs)
	}
	if strings.TrimSpace(a.BeatGoal) == "" {
		return nil, fmt.Errorf("beat_goal is required: %w", errs.ErrToolArgs)
	}
	if strings.TrimSpace(a.MergeRationale) == "" {
		return nil, fmt.Errorf("merge_rationale is required — phải nói rõ lấy gì từ bản nào: %w", errs.ErrToolArgs)
	}
	if err := validateBeatVariants(a.Variants); err != nil {
		return nil, err
	}

	final := strings.TrimSpace(a.FinalContent)
	if utf8.RuneCountInString(final) < minBeatFinalRunes {
		return nil, fmt.Errorf("final_content quá ngắn (%d ký tự, tối thiểu %d) — bản cuối phải là beat hoàn chỉnh, không phải tóm tắt: %w",
			utf8.RuneCountInString(final), minBeatFinalRunes, errs.ErrToolArgs)
	}

	// CỔNG MERGE-AUDIT (OmniForge Novel — audit merge/best) — chạy TRƯỚC gate prose.
	// Ép bản cuối phải thực sự kế thừa tinh hoa 3 bản, không cho trộn qua loa / clone một bản.
	if err := auditBeatMerge(a.Variants, final); err != nil {
		return nil, err
	}

	// Cổng chương giống draft_chapter: đúng luồng, đúng hàng đợi.
	if err := t.store.Progress.ValidateChapterWork(a.Chapter); err != nil {
		return nil, err
	}
	if t.store.Progress.IsChapterCompleted(a.Chapter) {
		progress, _ := t.store.Progress.Load()
		inRewriteQueue := progress != nil && slices.Contains(progress.PendingRewrites, a.Chapter)
		if !inRewriteQueue {
			return json.Marshal(map[string]any{
				"chapter": a.Chapter,
				"beat":    a.Beat,
				"skipped": true,
				"reason":  fmt.Sprintf("Chương %d đã hoàn thành, không thể append beat", a.Chapter),
			})
		}
	}

	// CỔNG AUDIT CƠ HỌC — chạy TRƯỚC khi ghi bất cứ thứ gì xuống draft.
	wordCount := utf8.RuneCountInString(final)
	if hard := proseHardErrors(final, wordCount); len(hard) > 0 {
		parts := make([]string, 0, len(hard))
		for _, v := range hard {
			parts = append(parts, fmt.Sprintf("%s(%q ×%d)", v.Rule, v.Target, v.Actual))
		}
		// Lưu hồ sơ beat FAIL để truy vết (không append draft).
		rec := beatRecord{
			Chapter: a.Chapter, Beat: a.Beat, BeatGoal: a.BeatGoal,
			Variants: a.Variants, FinalContent: final, MergeRationale: a.MergeRationale,
			FinalWordCount: wordCount, GatePassed: false,
			CreatedAt: time.Now().Format(time.RFC3339),
		}
		_ = t.store.Drafts.SaveBeatRecord(rec.Chapter, rec.Beat, rec)
		return nil, fmt.Errorf("beat %d chương %d KHÔNG QUA GATE: %s. Sửa bản cuối (0 em dash, 0 tiếng Anh lọt) rồi gọi draft_beat lại với cùng chapter/beat: %w",
			a.Beat, a.Chapter, strings.Join(parts, ", "), errs.ErrToolPrecondition)
	}

	if err := t.store.Progress.StartChapter(a.Chapter); err != nil {
		return nil, fmt.Errorf("mark chapter in progress: %w", err)
	}

	// PASS gate → append bản cuối vào draft + lưu hồ sơ.
	if err := t.store.Drafts.AppendDraft(a.Chapter, final); err != nil {
		return nil, fmt.Errorf("append beat to draft: %w", err)
	}
	rec := beatRecord{
		Chapter: a.Chapter, Beat: a.Beat, BeatGoal: a.BeatGoal,
		Variants: a.Variants, FinalContent: final, MergeRationale: a.MergeRationale,
		FinalWordCount: wordCount, GatePassed: true,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	if err := t.store.Drafts.SaveBeatRecord(rec.Chapter, rec.Beat, rec); err != nil {
		return nil, fmt.Errorf("save beat record: %w", err)
	}
	if _, err := t.store.Checkpoints.AppendArtifact(
		domain.ChapterScope(a.Chapter), "draft",
		fmt.Sprintf("drafts/%02d.draft.md", a.Chapter),
	); err != nil {
		return nil, fmt.Errorf("checkpoint draft: %w", err)
	}

	full, err := t.store.Drafts.LoadDraft(a.Chapter)
	if err != nil {
		return nil, fmt.Errorf("load draft after beat append: %w", err)
	}

	best := bestVariantFocus(a.Variants)
	return json.Marshal(map[string]any{
		"written":           true,
		"chapter":           a.Chapter,
		"beat":              a.Beat,
		"gate_passed":       true,
		"merge_audit":       "passed",
		"beat_word_count":   wordCount,
		"draft_word_count":  utf8.RuneCountInString(full),
		"best_variant":      best,
		"beat_record":       fmt.Sprintf("drafts/%02d.beat-%02d.json", a.Chapter, a.Beat),
		"next_step":         "Beat tiếp theo dùng draft_beat; hết beat thì read_chapter(source=draft) → check_consistency → commit_chapter",
	})
}

// validateBeatVariants bảo đảm đúng 3 bản, đủ 3 hướng khác nhau, điểm hợp lệ, nội dung đủ dài.
func validateBeatVariants(variants []beatVariant) error {
	if len(variants) != len(beatFocuses) {
		return fmt.Errorf("variants phải có đúng %d bản (noi_tam / doi_thoai / hanh_dong), nhận được %d: %w",
			len(beatFocuses), len(variants), errs.ErrToolArgs)
	}
	seen := make(map[string]struct{}, len(variants))
	for _, v := range variants {
		if !slices.Contains(beatFocuses, v.Focus) {
			return fmt.Errorf("focus không hợp lệ %q (phải là noi_tam / doi_thoai / hanh_dong): %w", v.Focus, errs.ErrToolArgs)
		}
		if _, dup := seen[v.Focus]; dup {
			return fmt.Errorf("focus trùng lặp %q — mỗi hướng đúng một bản: %w", v.Focus, errs.ErrToolArgs)
		}
		seen[v.Focus] = struct{}{}
		if v.Score < 0 || v.Score > 100 {
			return fmt.Errorf("score của bản %s phải trong 0-100, nhận %d: %w", v.Focus, v.Score, errs.ErrToolArgs)
		}
		if utf8.RuneCountInString(strings.TrimSpace(v.Content)) < minBeatVariantRunes {
			return fmt.Errorf("bản %s quá ngắn (%d ký tự, tối thiểu %d) — mỗi bản phải là một hướng viết thật, không phải phác một câu: %w",
				v.Focus, utf8.RuneCountInString(strings.TrimSpace(v.Content)), minBeatVariantRunes, errs.ErrToolArgs)
		}
	}
	return nil
}

// bestVariantFocus trả về focus của bản điểm cao nhất (chỉ để báo cáo).
func bestVariantFocus(variants []beatVariant) string {
	best, bestScore := "", -1
	for _, v := range variants {
		if v.Score > bestScore {
			best, bestScore = v.Focus, v.Score
		}
	}
	return best
}

// Ngưỡng cho merge-audit.
const (
	// dominantScoreGap: nếu bản cao nhất hơn bản nhì ≥ ngưỡng này → coi là "1 bản áp đảo",
	// bản cuối bắt buộc kế thừa xương sống bản đó.
	dominantScoreGap = 12
	// dominantMinOverlap: tỉ lệ từ khóa của bản áp đảo phải còn trong final (0..1).
	dominantMinOverlap = 0.30
	// mergeMinContributors: khi điểm sát nhau, final phải mang dấu vết ≥ số bản này.
	mergeMinContributors = 2
	// mergeContribOverlap: ngưỡng overlap coi như "final có dấu vết bản X" (0..1).
	mergeContribOverlap = 0.12
	// cloneMaxOverlap: nếu final trùng gần như nguyên một bản (≥ ngưỡng) thì bị coi là clone.
	cloneMaxOverlap = 0.85
)

// beatKeyword trích tập từ khóa (≥ 4 ký tự, đã hạ chữ thường) của một đoạn — dấu vân tay thô.
func beatKeywords(s string) map[string]struct{} {
	set := make(map[string]struct{})
	var b strings.Builder
	flush := func() {
		if b.Len() == 0 {
			return
		}
		w := b.String()
		b.Reset()
		if utf8.RuneCountInString(w) >= 4 {
			set[w] = struct{}{}
		}
	}
	for _, r := range strings.ToLower(s) {
		if r == ' ' || r == '\n' || r == '\t' || r == '.' || r == ',' || r == '!' ||
			r == '?' || r == ';' || r == ':' || r == '"' || r == '\'' || r == '(' ||
			r == ')' || r == '—' || r == '-' {
			flush()
			continue
		}
		b.WriteRune(r)
	}
	flush()
	return set
}

// overlapRatio: tỉ lệ từ khóa của src còn xuất hiện trong dst (|src ∩ dst| / |src|).
func overlapRatio(src, dst map[string]struct{}) float64 {
	if len(src) == 0 {
		return 0
	}
	hit := 0
	for k := range src {
		if _, ok := dst[k]; ok {
			hit++
		}
	}
	return float64(hit) / float64(len(src))
}

// auditBeatMerge ép bản cuối thực sự audit-merge/best từ 3 bản:
//   - Nếu 1 bản áp đảo (chênh điểm ≥ dominantScoreGap): final PHẢI kế thừa xương sống
//     bản đó (overlap ≥ dominantMinOverlap).
//   - Nếu điểm sát nhau: final PHẢI là merge thật — mang dấu vết ≥ mergeMinContributors bản,
//     và KHÔNG được clone nguyên một bản (overlap mọi bản < cloneMaxOverlap).
func auditBeatMerge(variants []beatVariant, final string) error {
	finalKW := beatKeywords(final)
	type vscan struct {
		focus string
		score int
		// inherit: tỉ lệ tinh hoa bản còn trong final (|variant ∩ final| / |variant|) — dùng cho áp đảo + đóng góp.
		inherit float64
		// covers: tỉ lệ final được phủ bởi riêng bản này (|final ∩ variant| / |final|) — dùng cho clone-check.
		covers float64
	}
	scans := make([]vscan, 0, len(variants))
	for _, v := range variants {
		vkw := beatKeywords(v.Content)
		scans = append(scans, vscan{
			focus:   v.Focus,
			score:   v.Score,
			inherit: overlapRatio(vkw, finalKW),
			covers:  overlapRatio(finalKW, vkw),
		})
	}

	// Tìm bản điểm cao nhất và nhì để xét áp đảo.
	top, second := -1, -1
	topIdx := -1
	for i, s := range scans {
		if s.score > top {
			second = top
			top = s.score
			topIdx = i
		} else if s.score > second {
			second = s.score
		}
	}

	// Clone-check: final không được được phủ gần toàn bộ bởi RIÊNG một bản
	// (|final ∩ v| / |final| ≥ ngưỡng) — tức final chỉ là bản đó viết lại, không trộn gì.
	for _, s := range scans {
		if s.covers >= cloneMaxOverlap {
			return fmt.Errorf("merge-audit: final giống gần nguyên bản %s (%.0f%% final đến từ riêng bản này) — phải trộn tinh hoa/nâng cấp, không bê nguyên một bản. Sửa final rồi gọi draft_beat lại: %w",
				s.focus, s.covers*100, errs.ErrToolPrecondition)
		}
	}

	if top-second >= dominantScoreGap && topIdx >= 0 {
		// 1 bản áp đảo → final phải kế thừa xương sống bản đó (tinh hoa bản đó còn trong final).
		if scans[topIdx].inherit < dominantMinOverlap {
			return fmt.Errorf("merge-audit: bản %s áp đảo (điểm %d, hơn bản nhì %d) nhưng final chỉ giữ %.0f%% tinh hoa bản đó (tối thiểu %.0f%%) — final phải kế thừa xương sống bản mạnh nhất. Sửa final rồi gọi lại: %w",
				scans[topIdx].focus, top, second, scans[topIdx].inherit*100, dominantMinOverlap*100, errs.ErrToolPrecondition)
		}
		return nil
	}

	// Điểm sát nhau → yêu cầu merge thật: ≥ mergeMinContributors bản có dấu vết trong final.
	contributors := 0
	for _, s := range scans {
		if s.inherit >= mergeContribOverlap {
			contributors++
		}
	}
	if contributors < mergeMinContributors {
		return fmt.Errorf("merge-audit: điểm 3 bản sát nhau nhưng final chỉ mang dấu vết %d/%d bản (cần ≥ %d) — phải trộn tinh hoa từ nhiều bản, không viết lại rời rạc. Sửa final rồi gọi lại: %w",
			contributors, len(scans), mergeMinContributors, errs.ErrToolPrecondition)
	}
	return nil
}

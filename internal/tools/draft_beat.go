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

// DraftBeatTool (OmniForge Novel — beat-method TỰ DO) viết một beat quan trọng:
//
//	Viết ≥2 bản độc lập (KHÔNG bó khung kiểu cố định — mỗi bản một hướng tự do do
//	người viết tự chọn) → tự audit chấm điểm từng bản → chọn lọc highlight hay nhất →
//	tổng hợp thành bản cuối (merge tinh hoa nhiều highlight) HOẶC lấy nguyên bản có
//	điểm vượt trội → cổng merge-audit + cổng prose cơ học → PASS mới append draft.
//
// Triết lý: "Văn là phải tự do." Engine KHÔNG ép các bản phải là nội tâm/đối thoại/hành động;
// nó chỉ ép đủ số bản độc lập, mỗi bản có nhãn hướng riêng (khác nhau), có tự audit, và bản cuối
// phải thật sự chắt lọc từ các bản chứ không bê nguyên/trộn qua loa.
//
// Toàn bộ các bản + điểm + highlight + lý do chốt được lưu drafts/NN.beat-MM.json để truy vết.
type DraftBeatTool struct {
	store *store.Store
}

func NewDraftBeatTool(store *store.Store) *DraftBeatTool {
	return &DraftBeatTool{store: store}
}

func (t *DraftBeatTool) Name() string { return "draft_beat" }
func (t *DraftBeatTool) Description() string {
	return "Viết một beat quan trọng (mở chương, cao trào, twist, kết) theo beat-method tự do: " +
		"nộp ≥2 bản độc lập (mỗi bản một hướng tự chọn, KHÔNG bó khung), tự chấm điểm 0-100 từng bản, " +
		"đánh dấu các câu/đoạn highlight hay nhất, rồi chốt bản cuối 400-600 chữ bằng cách gộp tinh hoa " +
		"HOẶC lấy nguyên bản điểm vượt trội. Tool audit: bản cuối phải chắt lọc thật từ các bản (không bê " +
		"nguyên/trộn qua loa), qua cổng prose cơ học (0 em dash, 0 tiếng Anh), PASS mới append vào nháp."
}
func (t *DraftBeatTool) Label() string { return "Viết beat (đa bản, tự do)" }

// Công cụ ghi (append draft + ghi hồ sơ beat), cấm chạy đồng thời.
func (t *DraftBeatTool) ReadOnly(_ json.RawMessage) bool        { return false }
func (t *DraftBeatTool) ConcurrencySafe(_ json.RawMessage) bool { return false }

func (t *DraftBeatTool) Schema() map[string]any {
	variantSchema := schema.Object(
		schema.Property("angle", schema.String("Nhãn hướng của bản này do người viết tự đặt (tự do: ví dụ 'nội tâm dồn nén', 'đối đầu nảy lửa', 'lạnh và tối giản', 'dòng ý thức'...). Các bản phải khác hướng nhau.")).Required(),
		schema.Property("content", schema.String("Nội dung bản viết (văn xuôi thuần)")).Required(),
		schema.Property("score", schema.Int("Tự audit 0-100: sức sống, đúng nhịp, đúng nhân vật, độ ám ảnh")).Required(),
		schema.Property("highlights", schema.Array("Các câu/đoạn hay nhất trích nguyên văn từ bản này (có thể rỗng nếu bản yếu)", schema.String("Một câu/đoạn highlight trích nguyên văn"))).Required(),
		schema.Property("note", schema.String("Một câu tự audit: điểm mạnh/yếu của bản này (có thể để chuỗi rỗng)")).Required(),
	)
	return schema.Object(
		schema.Property("chapter", schema.Int("Số chương")).Required(),
		schema.Property("beat", schema.Int("Số thứ tự beat trong chương (1, 2, 3...)")).Required(),
		schema.Property("beat_goal", schema.String("Beat này phải đạt gì (một câu)")).Required(),
		schema.Property("variants", schema.Array("≥2 bản độc lập, mỗi bản một hướng khác nhau do người viết tự chọn", variantSchema)).Required(),
		schema.Property("decision", schema.Enum("Cách chốt bản cuối", "merge", "pick_best")).Required(),
		schema.Property("final_content", schema.String("Bản cuối 400-600 chữ: nếu decision=merge thì gộp tinh hoa/highlight nhiều bản; nếu decision=pick_best thì lấy nguyên bản điểm vượt trội (có thể tinh chỉnh nhẹ)")).Required(),
		schema.Property("merge_rationale", schema.String("Chốt thế nào: nếu merge thì lấy highlight gì từ bản nào; nếu pick_best thì vì sao bản đó vượt trội")).Required(),
	)
}

func (t *DraftBeatTool) StrictSchema() bool { return true }

// beatVariant là một bản viết độc lập trong hồ sơ beat.
type beatVariant struct {
	Angle      string   `json:"angle"`
	Content    string   `json:"content"`
	Score      int      `json:"score"`
	Highlights []string `json:"highlights,omitempty"`
	Note       string   `json:"note,omitempty"`
}

// beatRecord là hồ sơ truy vết của một beat, lưu tại drafts/NN.beat-MM.json.
type beatRecord struct {
	Chapter        int           `json:"chapter"`
	Beat           int           `json:"beat"`
	BeatGoal       string        `json:"beat_goal"`
	Variants       []beatVariant `json:"variants"`
	Decision       string        `json:"decision"`
	FinalContent   string        `json:"final_content"`
	MergeRationale string        `json:"merge_rationale"`
	FinalWordCount int           `json:"final_word_count"`
	GatePassed     bool          `json:"gate_passed"`
	CreatedAt      string        `json:"created_at"`
}

// Ngưỡng về số bản + độ dài.
const (
	minBeatVariants     = 2   // ít nhất 2 bản độc lập (tự do, không ép đúng 3)
	minBeatVariantRunes = 120 // mỗi bản tối thiểu — chặn "bản làm phép" một hai câu
	// Chuẩn OmniForge: mỗi beat 400-600 CHỮ. Đếm theo TỪ tiếng Việt (khoảng trắng).
	minBeatFinalWords = 400
	maxBeatFinalWords = 600
	// Nới nhẹ biên để không phạt oan beat chốt/mở cực ngắn có chủ đích: cho 340..680.
	softMinBeatFinalWords = 340
	softMaxBeatFinalWords = 680
)

func (t *DraftBeatTool) Execute(_ context.Context, args json.RawMessage) (json.RawMessage, error) {
	var a struct {
		Chapter        int           `json:"chapter"`
		Beat           int           `json:"beat"`
		BeatGoal       string        `json:"beat_goal"`
		Variants       []beatVariant `json:"variants"`
		Decision       string        `json:"decision"`
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
		return nil, fmt.Errorf("merge_rationale is required — phải nói rõ chốt thế nào: %w", errs.ErrToolArgs)
	}
	a.Decision = strings.TrimSpace(strings.ToLower(a.Decision))
	if a.Decision != "merge" && a.Decision != "pick_best" {
		return nil, fmt.Errorf("decision phải là 'merge' hoặc 'pick_best', nhận %q: %w", a.Decision, errs.ErrToolArgs)
	}
	if err := validateBeatVariants(a.Variants); err != nil {
		return nil, err
	}

	final := strings.TrimSpace(a.FinalContent)

	// CHUẨN ĐỘ DÀI 400-600 CHỮ (OmniForge micro-beat). Đếm theo từ, nới biên mềm.
	words := countBeatWords(final)
	if words < softMinBeatFinalWords {
		return nil, fmt.Errorf("final_content quá ngắn (%d chữ) — mỗi beat chuẩn %d-%d chữ để đạt chất lượng, tối thiểu chấp nhận %d. Viết đầy hơn rồi gọi lại: %w",
			words, minBeatFinalWords, maxBeatFinalWords, softMinBeatFinalWords, errs.ErrToolArgs)
	}
	if words > softMaxBeatFinalWords {
		return nil, fmt.Errorf("final_content quá dài (%d chữ) — mỗi beat chuẩn %d-%d chữ (viết ngắn = nghĩ nhiều = prose sắc hơn), tối đa chấp nhận %d. Nén lại rồi gọi lại: %w",
			words, minBeatFinalWords, maxBeatFinalWords, softMaxBeatFinalWords, errs.ErrToolArgs)
	}

	// CỔNG MERGE-AUDIT — chạy TRƯỚC gate prose.
	if err := auditBeatMerge(a.Variants, final, a.Decision); err != nil {
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

	// CỔNG AUDIT CƠ HỌC PROSE — chạy TRƯỚC khi ghi bất cứ thứ gì xuống draft.
	wordCount := utf8.RuneCountInString(final)
	if hard := proseHardErrors(final, wordCount); len(hard) > 0 {
		parts := make([]string, 0, len(hard))
		for _, v := range hard {
			parts = append(parts, fmt.Sprintf("%s(%q ×%d)", v.Rule, v.Target, v.Actual))
		}
		rec := beatRecord{
			Chapter: a.Chapter, Beat: a.Beat, BeatGoal: a.BeatGoal,
			Variants: a.Variants, Decision: a.Decision, FinalContent: final, MergeRationale: a.MergeRationale,
			FinalWordCount: words, GatePassed: false,
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
		Variants: a.Variants, Decision: a.Decision, FinalContent: final, MergeRationale: a.MergeRationale,
		FinalWordCount: words, GatePassed: true,
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

	best := bestVariantAngle(a.Variants)
	return json.Marshal(map[string]any{
		"written":          true,
		"chapter":          a.Chapter,
		"beat":             a.Beat,
		"gate_passed":      true,
		"merge_audit":      "passed",
		"decision":         a.Decision,
		"beat_word_count":  words,
		"draft_word_count": utf8.RuneCountInString(full),
		"best_variant":     best,
		"beat_record":      fmt.Sprintf("drafts/%02d.beat-%02d.json", a.Chapter, a.Beat),
		"next_step":        "Beat tiếp theo dùng draft_beat; hết beat thì read_chapter(source=draft) → check_consistency → commit_chapter",
	})
}

// countBeatWords đếm số từ tiếng Việt (tách theo khoảng trắng).
func countBeatWords(s string) int {
	return len(strings.Fields(s))
}

// validateBeatVariants bảo đảm ≥2 bản, nhãn hướng khác nhau, điểm hợp lệ, nội dung đủ dài.
func validateBeatVariants(variants []beatVariant) error {
	if len(variants) < minBeatVariants {
		return fmt.Errorf("variants phải có ít nhất %d bản độc lập (mỗi bản một hướng tự chọn), nhận được %d: %w",
			minBeatVariants, len(variants), errs.ErrToolArgs)
	}
	seen := make(map[string]struct{}, len(variants))
	for i, v := range variants {
		angle := strings.TrimSpace(strings.ToLower(v.Angle))
		if angle == "" {
			return fmt.Errorf("bản #%d thiếu nhãn hướng (angle): %w", i+1, errs.ErrToolArgs)
		}
		if _, dup := seen[angle]; dup {
			return fmt.Errorf("nhãn hướng trùng lặp %q — mỗi bản phải một hướng khác nhau (văn tự do, không lặp góc): %w", v.Angle, errs.ErrToolArgs)
		}
		seen[angle] = struct{}{}
		if v.Score < 0 || v.Score > 100 {
			return fmt.Errorf("score của bản %q phải trong 0-100, nhận %d: %w", v.Angle, v.Score, errs.ErrToolArgs)
		}
		if utf8.RuneCountInString(strings.TrimSpace(v.Content)) < minBeatVariantRunes {
			return fmt.Errorf("bản %q quá ngắn (%d ký tự, tối thiểu %d) — mỗi bản phải là một hướng viết thật, không phải phác một câu: %w",
				v.Angle, utf8.RuneCountInString(strings.TrimSpace(v.Content)), minBeatVariantRunes, errs.ErrToolArgs)
		}
	}
	return nil
}

// bestVariantAngle trả về nhãn hướng của bản điểm cao nhất (chỉ để báo cáo).
func bestVariantAngle(variants []beatVariant) string {
	best, bestScore := "", -1
	for _, v := range variants {
		if v.Score > bestScore {
			best, bestScore = v.Angle, v.Score
		}
	}
	return best
}

// Ngưỡng cho merge-audit.
const (
	// pickBestScoreGap: pick_best chỉ hợp lệ khi bản chọn hơn bản nhì ≥ ngưỡng này (thật sự vượt trội).
	pickBestScoreGap = 10
	// pickBestMinOverlap: khi pick_best, final phải kế thừa xương sống bản đó (0..1).
	pickBestMinOverlap = 0.45
	// mergeMinContributors: khi merge, final phải mang dấu vết ≥ số bản này.
	mergeMinContributors = 2
	// mergeContribOverlap: ngưỡng overlap coi như "final có dấu vết bản X" (0..1).
	mergeContribOverlap = 0.12
	// cloneMaxOverlap: nếu final bị phủ gần toàn bộ bởi RIÊNG một bản (≥ ngưỡng) → clone (chỉ áp cho merge).
	cloneMaxOverlap = 0.85
)

// beatKeywords trích tập từ khóa (≥ 4 ký tự, đã hạ chữ thường) của một đoạn — dấu vân tay thô.
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

// auditBeatMerge ép bản cuối thật sự chắt lọc từ các bản theo decision:
//   - decision=pick_best: bản chọn phải thật vượt trội (chênh ≥ pickBestScoreGap) VÀ final kế thừa
//     xương sống bản đó (inherit ≥ pickBestMinOverlap). Cho phép final ~ bản đó (không tính clone).
//   - decision=merge: final phải mang dấu vết ≥ mergeMinContributors bản, KHÔNG được clone nguyên
//     một bản (covers < cloneMaxOverlap) — tức merge thật, không bê nguyên.
func auditBeatMerge(variants []beatVariant, final, decision string) error {
	finalKW := beatKeywords(final)
	type vscan struct {
		angle   string
		score   int
		inherit float64 // |variant ∩ final| / |variant| — tinh hoa bản còn trong final
		covers  float64 // |final ∩ variant| / |final| — final được phủ bởi riêng bản này
	}
	scans := make([]vscan, 0, len(variants))
	for _, v := range variants {
		vkw := beatKeywords(v.Content)
		scans = append(scans, vscan{
			angle:   v.Angle,
			score:   v.Score,
			inherit: overlapRatio(vkw, finalKW),
			covers:  overlapRatio(finalKW, vkw),
		})
	}

	// Tìm bản điểm cao nhất và nhì.
	top, second, topIdx := -1, -1, -1
	for i, s := range scans {
		if s.score > top {
			second = top
			top = s.score
			topIdx = i
		} else if s.score > second {
			second = s.score
		}
	}

	if decision == "pick_best" {
		if topIdx < 0 {
			return fmt.Errorf("merge-audit: không xác định được bản tốt nhất: %w", errs.ErrToolPrecondition)
		}
		if top-second < pickBestScoreGap {
			return fmt.Errorf("merge-audit: decision=pick_best nhưng bản %q chỉ hơn bản nhì %d điểm (cần ≥ %d để coi là vượt trội) — điểm sát nhau thì phải merge tinh hoa, đổi decision=merge. Sửa rồi gọi lại: %w",
				scans[topIdx].angle, top-second, pickBestScoreGap, errs.ErrToolPrecondition)
		}
		if scans[topIdx].inherit < pickBestMinOverlap {
			return fmt.Errorf("merge-audit: decision=pick_best chọn bản %q nhưng final chỉ giữ %.0f%% tinh hoa bản đó (tối thiểu %.0f%%) — pick_best nghĩa là lấy nguyên xương sống bản vượt trội. Sửa final rồi gọi lại: %w",
				scans[topIdx].angle, scans[topIdx].inherit*100, pickBestMinOverlap*100, errs.ErrToolPrecondition)
		}
		return nil
	}

	// decision == merge
	// Clone-check: final không được phủ gần toàn bộ bởi RIÊNG một bản.
	for _, s := range scans {
		if s.covers >= cloneMaxOverlap {
			return fmt.Errorf("merge-audit: decision=merge nhưng final giống gần nguyên bản %q (%.0f%% final đến từ riêng bản này) — merge phải chắt lọc từ nhiều bản, không bê nguyên. Nếu thật sự muốn lấy nguyên bản này thì đổi decision=pick_best; còn không sửa final trộn thêm highlight bản khác rồi gọi lại: %w",
				s.angle, s.covers*100, errs.ErrToolPrecondition)
		}
	}
	// Merge thật: ≥ mergeMinContributors bản có dấu vết trong final.
	contributors := 0
	for _, s := range scans {
		if s.inherit >= mergeContribOverlap {
			contributors++
		}
	}
	if contributors < mergeMinContributors {
		return fmt.Errorf("merge-audit: decision=merge nhưng final chỉ mang dấu vết %d/%d bản (cần ≥ %d) — phải gộp tinh hoa từ nhiều bản, không viết lại rời rạc. Sửa final rồi gọi lại: %w",
			contributors, len(scans), mergeMinContributors, errs.ErrToolPrecondition)
	}
	return nil
}

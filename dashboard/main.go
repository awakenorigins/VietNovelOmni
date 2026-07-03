// omni-dashboard: Web dashboard chỉ-đọc cho OmniNovel engine.
// Go stdlib only. Không framework. Read-only tuyệt đối.
package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

//go:embed templates/*.html
var tmplFS embed.FS

//go:embed static
var staticFS embed.FS

var (
	novelsDir string
	templates *template.Template
	slugRe    = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	numRe     = regexp.MustCompile(`(\d+)`)
)

// ---------- Models ----------

type ReviewInfo struct {
	Scores  map[string]float64 `json:"scores,omitempty"`
	Average float64            `json:"average,omitempty"`
	Verdict string             `json:"verdict,omitempty"`
	Found   bool               `json:"found"`
}

type AuditInfo struct {
	PassAll *bool           `json:"pass_all,omitempty"`
	Skills  map[string]bool `json:"skills,omitempty"`
	Found   bool            `json:"found"`
}

type ChapterInfo struct {
	Number int        `json:"number"`
	File   string     `json:"file"`
	Title  string     `json:"title,omitempty"`
	Words  int        `json:"words"`
	Status string     `json:"status"`
	Review ReviewInfo `json:"review"`
	Audit  AuditInfo  `json:"audit"`
}

type NovelSummary struct {
	Slug              string `json:"slug"`
	Title             string `json:"title"`
	Phase             string `json:"phase,omitempty"`
	Flow              string `json:"flow,omitempty"`
	Status            string `json:"status,omitempty"`
	CompletedChapters int    `json:"completed_chapters"`
	TotalChapters     int    `json:"total_chapters"`
	PendingRewrites   int    `json:"pending_rewrites"`
	TotalWords        int    `json:"total_words"`
	CurrentVolume     string   `json:"current_volume,omitempty"`
	CurrentArc        string   `json:"current_arc,omitempty"`
	Genres            []string `json:"genres,omitempty"`
	Author            string   `json:"author,omitempty"`
	Generating        bool     `json:"generating,omitempty"`
	LatestExport      string   `json:"latest_export,omitempty"`
}

type NovelDetail struct {
	NovelSummary
	Chapters        []ChapterInfo   `json:"chapters"`
	PendingRewrite  []string        `json:"pending_rewrite_list,omitempty"`
	ProgressRaw     json.RawMessage `json:"progress_raw,omitempty"`
	HasProgressFile bool            `json:"has_progress_file"`
}

type ChapterContent struct {
	Slug    string     `json:"slug"`
	Number  int        `json:"number"`
	File    string     `json:"file"`
	Words   int        `json:"words"`
	Content string     `json:"content"`
	Review  ReviewInfo `json:"review"`
	Audit   AuditInfo  `json:"audit"`
}

// ---------- Helpers ----------

func titleFromSlug(slug string) string {
	parts := strings.FieldsFunc(slug, func(r rune) bool { return r == '-' || r == '_' })
	for i, p := range parts {
		r := []rune(p)
		if len(r) > 0 {
			r[0] = unicode.ToUpper(r[0])
		}
		parts[i] = string(r)
	}
	return strings.Join(parts, " ")
}

func countWords(s string) int {
	return len(strings.Fields(s))
}

func chapterNumber(filename string) (int, bool) {
	m := numRe.FindString(filename)
	if m == "" {
		return 0, false
	}
	n, err := strconv.Atoi(m)
	if err != nil {
		return 0, false
	}
	return n, true
}

func readJSONMap(path string) (map[string]interface{}, json.RawMessage, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, nil, err
	}
	return m, json.RawMessage(b), nil
}

func asString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func asInt(v interface{}) (int, bool) {
	switch t := v.(type) {
	case float64:
		return int(t), true
	case string:
		if n, err := strconv.Atoi(t); err == nil {
			return n, true
		}
	}
	return 0, false
}

func listLen(v interface{}) (int, bool) {
	if arr, ok := v.([]interface{}); ok {
		return len(arr), true
	}
	return 0, false
}

func stringList(v interface{}) []string {
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, it := range arr {
		if s, ok := it.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// pickFirst returns the first present key's value from map.
func pickFirst(m map[string]interface{}, keys ...string) (interface{}, bool) {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			return v, true
		}
	}
	return nil, false
}

// ---------- Data loading ----------

// contentDir trả về thư mục chứa chapters/ thực tế của một novel:
// truyện gen bằng engine (headless, cwd=<novel>) có store tại <novel>/output/novel/,
// truyện cũ đặt chapters/ ngay gốc. Ưu tiên store engine nếu có.
func contentDir(dir string) string {
	nested := filepath.Join(dir, "output", "novel")
	if _, err := os.Stat(filepath.Join(nested, "chapters")); err == nil {
		return nested
	}
	return dir
}

func findChapterFiles(dir string) []string {
	entries, err := os.ReadDir(filepath.Join(dir, "chapters"))
	if err != nil {
		return nil
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext == ".md" || ext == ".txt" {
			files = append(files, name)
		}
	}
	sort.Slice(files, func(i, j int) bool {
		ni, oki := chapterNumber(files[i])
		nj, okj := chapterNumber(files[j])
		if oki && okj && ni != nj {
			return ni < nj
		}
		return files[i] < files[j]
	})
	return files
}

// loadReview reads reviews/NN.json for a chapter number, tolerant to naming.
func loadReview(dir string, num int) ReviewInfo {
	ri := ReviewInfo{}
	revDir := filepath.Join(dir, "reviews")
	entries, err := os.ReadDir(revDir)
	if err != nil {
		return ri
	}
	var match string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		if n, ok := chapterNumber(e.Name()); ok && n == num {
			match = e.Name()
			break
		}
	}
	if match == "" {
		return ri
	}
	m, _, err := readJSONMap(filepath.Join(revDir, match))
	if err != nil {
		return ri
	}
	ri.Found = true
	if v, ok := pickFirst(m, "verdict", "conclusion", "ket_luan"); ok {
		ri.Verdict = asString(v)
	}
	scores := map[string]float64{}
	// scores may be nested under "scores"/"dimensions" or flat numeric fields
	src := m
	if v, ok := pickFirst(m, "scores", "dimensions", "diem"); ok {
		if mm, ok2 := v.(map[string]interface{}); ok2 {
			src = mm
		}
	}
	for k, v := range src {
		switch t := v.(type) {
		case float64:
			if t >= 0 && t <= 100 && k != "chapter" && k != "number" && k != "ch" {
				scores[k] = t
			}
		case map[string]interface{}:
			if sv, ok := pickFirst(t, "score", "diem", "value"); ok {
				if f, ok2 := sv.(float64); ok2 && f >= 0 && f <= 100 {
					scores[k] = f
				}
			}
		}
	}
	if len(scores) > 0 {
		ri.Scores = scores
		var sum float64
		for _, v := range scores {
			sum += v
		}
		ri.Average = float64(int(sum/float64(len(scores))*10)) / 10
	}
	return ri
}

// loadAudit reads meta/skill-audit/chNN.json.
func loadAudit(dir string, num int) AuditInfo {
	ai := AuditInfo{}
	auditDir := filepath.Join(dir, "meta", "skill-audit")
	entries, err := os.ReadDir(auditDir)
	if err != nil {
		return ai
	}
	var match string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		if n, ok := chapterNumber(e.Name()); ok && n == num {
			match = e.Name()
			break
		}
	}
	if match == "" {
		return ai
	}
	m, _, err := readJSONMap(filepath.Join(auditDir, match))
	if err != nil {
		return ai
	}
	ai.Found = true
	if v, ok := pickFirst(m, "pass_all", "passAll", "pass"); ok {
		if b, ok2 := v.(bool); ok2 {
			ai.PassAll = &b
		}
	}
	skills := map[string]bool{}
	if v, ok := pickFirst(m, "skills", "checks", "results"); ok {
		switch t := v.(type) {
		case map[string]interface{}:
			for k, sv := range t {
				switch st := sv.(type) {
				case bool:
					skills[k] = st
				case map[string]interface{}:
					if pv, ok := pickFirst(st, "pass", "passed", "ok"); ok {
						if b, ok2 := pv.(bool); ok2 {
							skills[k] = b
						}
					}
				}
			}
		case []interface{}:
			for _, item := range t {
				if im, ok := item.(map[string]interface{}); ok {
					name := asString(firstOf(im, "skill", "name", "id"))
					if name == "" {
						continue
					}
					if pv, ok := pickFirst(im, "pass", "passed", "ok"); ok {
						if b, ok2 := pv.(bool); ok2 {
							skills[name] = b
						}
					}
				}
			}
		}
	}
	if len(skills) > 0 {
		ai.Skills = skills
	}
	return ai
}

func firstOf(m map[string]interface{}, keys ...string) interface{} {
	v, _ := pickFirst(m, keys...)
	return v
}

// loadSummary builds a NovelSummary from progress.json / monitor-state.json / chapters.
func loadSummary(slug string) (NovelSummary, map[string]interface{}, json.RawMessage) {
	dir := filepath.Join(novelsDir, slug)
	s := NovelSummary{Slug: slug, Title: titleFromSlug(slug)}

	var progress map[string]interface{}
	var raw json.RawMessage
	for _, cand := range []string{
		filepath.Join(dir, "progress.json"),
		filepath.Join(dir, "data", "progress.json"),
		filepath.Join(dir, "data", "monitor-state.json"),
	} {
		if m, r, err := readJSONMap(cand); err == nil {
			progress = m
			raw = r
			break
		}
	}

	cdir := contentDir(dir)
	files := findChapterFiles(cdir)
	s.TotalChapters = len(files)

	if progress != nil {
		if v, ok := pickFirst(progress, "title", "name"); ok {
			if t := asString(v); t != "" {
				s.Title = t
			}
		}
		if v, ok := pickFirst(progress, "phase"); ok {
			s.Phase = asString(v)
		}
		if v, ok := pickFirst(progress, "flow"); ok {
			s.Flow = asString(v)
		}
		if v, ok := pickFirst(progress, "status"); ok {
			s.Status = asString(v)
		}
		if v, ok := pickFirst(progress, "completed_chapters", "completedChapters"); ok {
			if n, isL := listLen(v); isL {
				s.CompletedChapters = n
			} else if n, isI := asInt(v); isI {
				s.CompletedChapters = n
			}
		}
		if v, ok := pickFirst(progress, "total_chapters", "totalChapters"); ok {
			if n, isI := asInt(v); isI && n > 0 {
				s.TotalChapters = n
			}
		}
		if v, ok := pickFirst(progress, "pending_rewrites", "pendingRewrites"); ok {
			if n, isL := listLen(v); isL {
				s.PendingRewrites = n
			} else if n, isI := asInt(v); isI {
				s.PendingRewrites = n
			}
		}
		if v, ok := pickFirst(progress, "total_words", "totalWords"); ok {
			if n, isI := asInt(v); isI {
				s.TotalWords = n
			}
		}
		if v, ok := pickFirst(progress, "current_volume", "currentVolume"); ok {
			s.CurrentVolume = fmt.Sprintf("%v", v)
		}
		if v, ok := pickFirst(progress, "current_arc", "currentArc"); ok {
			s.CurrentArc = fmt.Sprintf("%v", v)
		}
	}

	if s.CompletedChapters == 0 && s.TotalChapters > 0 && progress == nil {
		// Không có metadata: coi số chương hiện có là số đã viết.
		s.CompletedChapters = s.TotalChapters
	}
	if s.TotalWords == 0 {
		total := 0
		for _, f := range files {
			if b, err := os.ReadFile(filepath.Join(cdir, "chapters", f)); err == nil {
				total += countWords(string(b))
			}
		}
		s.TotalWords = total
	}
	return s, progress, raw
}

func completedSet(progress map[string]interface{}) map[int]bool {
	set := map[int]bool{}
	if progress == nil {
		return set
	}
	if v, ok := pickFirst(progress, "completed_chapters", "completedChapters"); ok {
		for _, name := range stringList(v) {
			if n, ok := chapterNumber(name); ok {
				set[n] = true
			}
		}
		if arr, ok := v.([]interface{}); ok {
			for _, it := range arr {
				if f, ok := it.(float64); ok {
					set[int(f)] = true
				}
			}
		}
	}
	return set
}

func pendingSet(progress map[string]interface{}) (map[int]bool, []string) {
	set := map[int]bool{}
	var names []string
	if progress == nil {
		return set, names
	}
	if v, ok := pickFirst(progress, "pending_rewrites", "pendingRewrites"); ok {
		names = stringList(v)
		for _, name := range names {
			if n, ok := chapterNumber(name); ok {
				set[n] = true
			}
		}
		if arr, ok := v.([]interface{}); ok {
			for _, it := range arr {
				if f, ok := it.(float64); ok {
					set[int(f)] = true
					names = append(names, strconv.Itoa(int(f)))
				}
			}
		}
	}
	return set, names
}

func loadDetail(slug string) (NovelDetail, error) {
	dir := filepath.Join(novelsDir, slug)
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return NovelDetail{}, os.ErrNotExist
	}
	summary, progress, raw := loadSummary(slug)
	d := NovelDetail{NovelSummary: summary, HasProgressFile: progress != nil, ProgressRaw: raw}

	done := completedSet(progress)
	pending, pendingNames := pendingSet(progress)
	d.PendingRewrite = pendingNames

	cdir := contentDir(dir)
	files := findChapterFiles(cdir)
	for _, f := range files {
		num, _ := chapterNumber(f)
		ci := ChapterInfo{Number: num, File: f}
		b, err := os.ReadFile(filepath.Join(cdir, "chapters", f))
		if err == nil {
			content := string(b)
			ci.Words = countWords(content)
			// Lấy dòng đầu làm tiêu đề nếu ngắn
			for _, line := range strings.SplitN(content, "\n", 5) {
				line = strings.TrimSpace(strings.TrimLeft(line, "# "))
				if line != "" {
					if len([]rune(line)) <= 80 {
						ci.Title = line
					}
					break
				}
			}
		}
		switch {
		case pending[num]:
			ci.Status = "cần viết lại"
		case progress != nil && done[num]:
			ci.Status = "hoàn thành"
		case progress != nil:
			ci.Status = "nháp"
		default:
			ci.Status = "hoàn thành"
		}
		ci.Review = loadReview(dir, num)
		ci.Audit = loadAudit(dir, num)
		d.Chapters = append(d.Chapters, ci)
	}
	return d, nil
}

func loadChapter(slug string, num int) (ChapterContent, error) {
	dir := contentDir(filepath.Join(novelsDir, slug))
	files := findChapterFiles(dir)
	for _, f := range files {
		if n, ok := chapterNumber(f); ok && n == num {
			b, err := os.ReadFile(filepath.Join(dir, "chapters", f))
			if err != nil {
				return ChapterContent{}, err
			}
			content := string(b)
			return ChapterContent{
				Slug: slug, Number: num, File: f,
				Words: countWords(content), Content: content,
				Review: loadReview(dir, num), Audit: loadAudit(dir, num),
			}, nil
		}
	}
	return ChapterContent{}, os.ErrNotExist
}

func listNovels() []NovelSummary {
	entries, err := os.ReadDir(novelsDir)
	if err != nil {
		return nil
	}
	var out []NovelSummary
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		s, _, _ := loadSummary(e.Name())
		if b, ok := loadBookJSON(e.Name()); ok {
			s.Genres = b.Genres
			s.Author = b.Author
			if b.Title != "" {
				s.Title = b.Title
			}
		}
		s.Generating = jobs.runningFor(e.Name()) != nil
		if exps := listExports(e.Name()); len(exps) > 0 {
			s.LatestExport = exps[0]
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out
}

// ---------- HTTP ----------

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func validSlug(slug string) bool { return slugRe.MatchString(slug) }

func handleAPINovels(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, listNovels())
}

func handleAPINovel(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if !validSlug(slug) {
		writeJSON(w, 400, map[string]string{"error": "slug không hợp lệ"})
		return
	}
	d, err := loadDetail(slug)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "không tìm thấy novel"})
		return
	}
	writeJSON(w, 200, d)
}

func handleAPIChapter(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	nStr := r.PathValue("n")
	if !validSlug(slug) {
		writeJSON(w, 400, map[string]string{"error": "slug không hợp lệ"})
		return
	}
	n, err := strconv.Atoi(nStr)
	if err != nil || n < 0 {
		writeJSON(w, 400, map[string]string{"error": "số chương không hợp lệ"})
		return
	}
	c, err := loadChapter(slug, n)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "không tìm thấy chương"})
		return
	}
	writeJSON(w, 200, c)
}

type albumInfo struct {
	Name  string
	Count int
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	all := listNovels()

	// Album theo thể loại: tập hợp thể loại có truyện
	genreSet := map[string]int{}
	for _, n := range all {
		for _, g := range n.Genres {
			genreSet[g]++
		}
	}
	var albums []albumInfo
	for g, c := range genreSet {
		albums = append(albums, albumInfo{g, c})
	}
	sort.Slice(albums, func(i, j int) bool { return albums[i].Name < albums[j].Name })

	// Filter theo album + search
	pickGenre := r.URL.Query().Get("genre")
	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	var novels []NovelSummary
	for _, n := range all {
		if pickGenre != "" && !slices.Contains(n.Genres, pickGenre) {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(n.Title), query) && !strings.Contains(strings.ToLower(n.Slug), query) {
			continue
		}
		novels = append(novels, n)
	}

	autoSt := autoMgr.snapshot()
	data := struct {
		Novels     []NovelSummary
		Albums     []albumInfo
		Picked     string
		Query      string
		Now        string
		AutoActive bool
		AutoTitle  string
	}{novels, albums, pickGenre, r.URL.Query().Get("q"), time.Now().Format("02/01/2006 15:04"), autoSt.Enabled, autoSt.CurrentTitle}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, "index.html", data); err != nil {
		log.Printf("template index: %v", err)
	}
}

func handleNovelPage(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if !validSlug(slug) {
		http.Error(w, "slug không hợp lệ", 400)
		return
	}
	d, err := loadDetail(slug)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	book, _ := loadBookJSON(slug)
	if book.Title != "" {
		d.Title = book.Title
	}
	job := jobs.runningFor(slug)
	data := struct {
		NovelDetail
		Book    BookJSON
		Job     *Job
		Exports []string
		Flash   flashData
	}{d, book, job, listExports(slug), flashData{Msg: r.URL.Query().Get("ok"), Err: r.URL.Query().Get("err")}}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, "novel.html", data); err != nil {
		log.Printf("template novel: %v", err)
	}
}

func handleChapterPage(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	nStr := r.PathValue("n")
	if !validSlug(slug) {
		http.Error(w, "slug không hợp lệ", 400)
		return
	}
	n, err := strconv.Atoi(nStr)
	if err != nil || n < 0 {
		http.Error(w, "số chương không hợp lệ", 400)
		return
	}
	c, errC := loadChapter(slug, n)
	if errC != nil {
		http.NotFound(w, r)
		return
	}
	data := struct {
		ChapterContent
		Title string
	}{c, titleFromSlug(slug)}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, "chapter.html", data); err != nil {
		log.Printf("template chapter: %v", err)
	}
}

func main() {
	addr := flag.String("addr", "127.0.0.1:8770", "địa chỉ bind")
	dir := flag.String("novels-dir", "/root/.openclaw/workspace/novels", "thư mục novels")
	flag.Parse()
	novelsDir = *dir

	funcs := template.FuncMap{
		"percent": func(a, b int) int {
			if b <= 0 {
				return 0
			}
			p := a * 100 / b
			if p > 100 {
				p = 100
			}
			return p
		},
		"deref": func(b *bool) bool {
			if b == nil {
				return false
			}
			return *b
		},
		"mkslice": func(items ...string) []string { return items },
	}
	var err error
	templates, err = template.New("").Funcs(funcs).ParseFS(tmplFS, "templates/*.html")
	if err != nil {
		log.Fatalf("parse templates: %v", err)
	}

	// OmniNovel App: khởi tạo app state (auth, engine config mặc định, job registry, book.json cho truyện cũ)
	initApp()

	mux := http.NewServeMux()
	// Static art assets (embedded)
	mux.Handle("GET /static/", http.FileServer(http.FS(staticFS)))
	// Viewer (đọc)
	mux.HandleFunc("GET /", handleIndex)
	mux.HandleFunc("GET /novel/{slug}", handleNovelPage)
	mux.HandleFunc("GET /novel/{slug}/chapter/{n}", handleChapterPage)
	mux.HandleFunc("GET /api/novels", handleAPINovels)
	mux.HandleFunc("GET /api/novels/{slug}", handleAPINovel)
	mux.HandleFunc("GET /api/novels/{slug}/chapters/{n}", handleAPIChapter)
	mux.HandleFunc("GET /api/jobs", handleAPIJobs)
	mux.HandleFunc("GET /jobs", handleJobsPage)
	mux.HandleFunc("GET /jobs/{id}/log", handleJobLog)
	mux.HandleFunc("GET /download/{slug}/{file}", handleDownload)
	mux.HandleFunc("GET /library", handleLibraryPage)
	mux.HandleFunc("GET /api/library", handleAPILibrary)
	mux.HandleFunc("GET /library/{album}/{file}", handleLibraryDownload)
	mux.HandleFunc("GET /api/auto", handleAPIAuto)
	// App (ghi) — qua authGate
	mux.Handle("GET /create", authGate(http.HandlerFunc(handleCreatePage)))
	mux.Handle("POST /create", authGate(http.HandlerFunc(handleCreateSubmit)))
	mux.Handle("GET /settings", authGate(http.HandlerFunc(handleSettingsPageV2)))
	mux.Handle("POST /settings", authGate(http.HandlerFunc(handleSettingsSaveV2)))
	mux.Handle("POST /settings/test", authGate(http.HandlerFunc(handleSettingsTestV2)))
	mux.Handle("GET /auto", authGate(http.HandlerFunc(handleAutoPage)))
	mux.Handle("POST /auto/start", authGate(http.HandlerFunc(handleAutoStart)))
	mux.Handle("POST /auto/stop", authGate(http.HandlerFunc(handleAutoStop)))
	mux.Handle("POST /jobs/{id}/cancel", authGate(http.HandlerFunc(handleJobCancel)))
	mux.Handle("POST /novel/{slug}/export", authGate(http.HandlerFunc(handleExport)))

	srv := &http.Server{
		Addr:         *addr,
		Handler:      maxBody(mux),
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 10 * time.Minute, // export EPUB truyện dài có thể lâu
	}
	log.Printf("omni-novel app đang chạy tại http://%s — novels: %s", *addr, novelsDir)
	log.Fatal(srv.ListenAndServe())
}

// maxBody: authGate đã áp MaxBytesReader cho POST đi qua nó; wrapper này chỉ là lưới đỡ cuối
// cho mọi route còn lại (hiện không có POST nào ngoài authGate, giữ để an toàn tương lai).
func maxBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, 2<<20) // 2 MB trần tuyệt đối
		}
		next.ServeHTTP(w, r)
	})
}

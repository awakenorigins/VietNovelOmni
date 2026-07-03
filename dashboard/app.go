// app.go — phần "APP FULL" của omni-dashboard: cấu hình model, tạo truyện,
// job manager chạy engine nền, export EPUB, Basic Auth cho route ghi.
package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"omni-dashboard/seedcraft"
)

var (
	enginePath string // đường dẫn binary oum-novel
	appDir     string // novelsDir/.omni-app
)

const maxBodyBytes = 1 << 20 // 1MB cho form POST

// ---------- Genres ----------

var allGenres = []string{
	"Tiên hiệp", "Huyền huyễn", "Đô thị", "Trọng sinh", "Hệ thống",
	"Xuyên không", "Kiếm hiệp", "Ngôn tình", "Đam mỹ", "Trinh thám",
	"Kinh dị", "Khoa huyễn", "Lịch sử", "Quân sự", "Võng du",
	"Thể thao", "Đồng nhân", "Hài hước", "Chính kịch", "Slice-of-life",
	"Dã sử",
}

// ---------- Book meta (.omni-app/book.json trong novel dir) ----------

type BookJSON struct {
	Title         string   `json:"title"`
	Author        string   `json:"author,omitempty"`
	Series        string   `json:"series,omitempty"`
	Genres        []string `json:"genres,omitempty"`
	Seeding       string   `json:"seeding,omitempty"`
	Script        string   `json:"script,omitempty"`
	ChaptersGoal  int      `json:"chapters_goal,omitempty"`
	WordsPerCh    string   `json:"words_per_chapter,omitempty"`
	CreatedAt     string   `json:"created_at,omitempty"`
}

func bookJSONPath(slug string) string {
	return filepath.Join(novelsDir, slug, ".omni-app", "book.json")
}

func loadBookJSON(slug string) (BookJSON, bool) {
	var b BookJSON
	raw, err := os.ReadFile(bookJSONPath(slug))
	if err != nil {
		return b, false
	}
	if json.Unmarshal(raw, &b) != nil {
		return b, false
	}
	return b, true
}

func saveBookJSON(slug string, b BookJSON) error {
	dir := filepath.Dir(bookJSONPath(slug))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	raw, _ := json.MarshalIndent(b, "", "  ")
	return os.WriteFile(bookJSONPath(slug), raw, 0o644)
}

// ---------- Auth ----------

type authInfo struct {
	User     string `json:"user"`
	Password string `json:"password"`
}

var auth authInfo

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// initApp khởi tạo toàn bộ state app: thư mục .omni-app, auth, engine config mặc định,
// job registry (hồi sinh job cũ sau restart), đường dẫn engine, book.json cho 2 truyện cũ.
func initApp() {
	appDir = filepath.Join(novelsDir, ".omni-app")
	_ = os.MkdirAll(appDir, 0o755)
	if enginePath == "" {
		enginePath = "/root/.openclaw/workspace/skills/oumstudio-novel/engine/oum-novel"
	}
	initAuth()
	ensureEngineConfig()
	jobs.load()
	seedLegacyBooks()
	seedReg.load()
	autoMgr.load()
	autoMgr.resume()
}

// seedLegacyBooks gán thể loại/tác giả mặc định cho truyện có sẵn trước khi app ra đời (chỉ khi chưa có book.json).
func seedLegacyBooks() {
	defaults := map[string]BookJSON{
		"trong-sinh":            {Title: "Trọng Sinh", Author: "Tiểu Tâm", Genres: []string{"Trọng sinh", "Đô thị"}},
		"vua-duong-ngoai-truyen": {Title: "Vua Đường Ngoại Truyện", Author: "Tiểu Tâm", Genres: []string{"Lịch sử", "Dã sử"}},
	}
	for slug, b := range defaults {
		if _, err := os.Stat(filepath.Join(novelsDir, slug)); err != nil {
			continue
		}
		if _, ok := loadBookJSON(slug); ok {
			continue
		}
		b.CreatedAt = time.Now().Format(time.RFC3339)
		if err := saveBookJSON(slug, b); err != nil {
			log.Printf("seed book.json %s: %v", slug, err)
		}
	}
}

func initAuth() {
	path := filepath.Join(appDir, "auth.json")
	raw, err := os.ReadFile(path)
	if err == nil && json.Unmarshal(raw, &auth) == nil && auth.Password != "" {
		return
	}
	auth = authInfo{User: "admin", Password: randHex(8)}
	out, _ := json.MarshalIndent(auth, "", "  ")
	_ = os.WriteFile(path, out, 0o600)
	log.Printf("đã tạo Basic Auth mới (user=admin), password lưu tại %s", path)
}

func checkAuth(r *http.Request) bool {
	u, p, ok := r.BasicAuth()
	if !ok {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(u), []byte(auth.User)) == 1 &&
		subtle.ConstantTimeCompare([]byte(p), []byte(auth.Password)) == 1
}

// authGate: yêu cầu Basic Auth cho mọi request KHÔNG phải GET/HEAD, và cho /settings.
func authGate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		needAuth := r.Method != http.MethodGet && r.Method != http.MethodHead
		if strings.HasPrefix(r.URL.Path, "/settings") || strings.HasPrefix(r.URL.Path, "/auto") || strings.HasPrefix(r.URL.Path, "/create") {
			needAuth = true
		}
		if needAuth {
			if !checkAuth(r) {
				w.Header().Set("WWW-Authenticate", `Basic realm="VietNovelOmni"`)
				http.Error(w, "cần đăng nhập", http.StatusUnauthorized)
				return
			}
			if r.Method == http.MethodPost {
				r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
			}
		}
		next.ServeHTTP(w, r)
	})
}

// ---------- Model config (v2 ở providers.go) ----------

func modelConfigPath() string  { return filepath.Join(appDir, "model-config.json") }
func engineConfigPath() string { return filepath.Join(appDir, "engine-config.json") }

// ensureEngineConfig: nếu chưa có engine-config.json thì copy từ ~/.ainovel/config.json.
func ensureEngineConfig() {
	if _, err := os.Stat(engineConfigPath()); err == nil {
		return
	}
	home, _ := os.UserHomeDir()
	src := filepath.Join(home, ".ainovel", "config.json")
	raw, err := os.ReadFile(src)
	if err != nil {
		log.Printf("chưa có engine-config.json và không đọc được %s: %v", src, err)
		return
	}
	if err := os.WriteFile(engineConfigPath(), raw, 0o600); err != nil {
		log.Printf("ghi engine-config.json: %v", err)
		return
	}
	log.Printf("đã copy config engine mặc định từ %s", src)
}

func maskKey(k string) string {
	if k == "" {
		return ""
	}
	if len(k) <= 8 {
		return "••••••••"
	}
	return "••••••••" + k[len(k)-4:]
}

// ---------- Job manager ----------

type Job struct {
	ID        string    `json:"id"`
	Slug      string    `json:"slug"`
	PID       int       `json:"pid"`
	Status    string    `json:"status"` // running/done/failed/cancelled
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at,omitempty"`
	LogPath   string    `json:"log_path"`
	Note      string    `json:"note,omitempty"`
}

type jobRegistry struct {
	mu   sync.Mutex
	jobs map[string]*Job
}

var jobs = &jobRegistry{jobs: map[string]*Job{}}

func jobsPath() string { return filepath.Join(appDir, "jobs.json") }

func (jr *jobRegistry) persistLocked() {
	list := make([]*Job, 0, len(jr.jobs))
	for _, j := range jr.jobs {
		list = append(list, j)
	}
	sort.Slice(list, func(i, k int) bool { return list[i].StartedAt.After(list[k].StartedAt) })
	raw, _ := json.MarshalIndent(list, "", "  ")
	_ = os.WriteFile(jobsPath(), raw, 0o644)
}

func (jr *jobRegistry) load() {
	raw, err := os.ReadFile(jobsPath())
	if err != nil {
		return
	}
	var list []*Job
	if json.Unmarshal(raw, &list) != nil {
		return
	}
	jr.mu.Lock()
	defer jr.mu.Unlock()
	for _, j := range list {
		if j.Status == "running" {
			// kiểm tra pid còn sống (kill -0)
			if j.PID > 0 && syscall.Kill(j.PID, 0) == nil {
				// tiến trình còn — theo dõi lại bằng polling
				go watchOrphan(j)
			} else {
				j.Status = "failed"
				j.Note = "mất theo dõi sau khi dashboard khởi động lại"
				j.EndedAt = time.Now()
			}
		}
		jr.jobs[j.ID] = j
	}
	jr.persistLocked()
}

// watchOrphan theo dõi job mồ côi (dashboard restart) bằng cách poll kill -0.
func watchOrphan(j *Job) {
	for {
		time.Sleep(5 * time.Second)
		if syscall.Kill(j.PID, 0) != nil {
			jobs.mu.Lock()
			if j.Status == "running" {
				j.Status = "done"
				j.Note = "kết thúc (theo dõi gián tiếp sau restart)"
				j.EndedAt = time.Now()
			}
			jobs.persistLocked()
			jobs.mu.Unlock()
			return
		}
		jobs.mu.Lock()
		cancelled := j.Status == "cancelled"
		jobs.mu.Unlock()
		if cancelled {
			return
		}
	}
}

func (jr *jobRegistry) list() []*Job {
	jr.mu.Lock()
	defer jr.mu.Unlock()
	out := make([]*Job, 0, len(jr.jobs))
	for _, j := range jr.jobs {
		cp := *j
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, k int) bool { return out[i].StartedAt.After(out[k].StartedAt) })
	return out
}

func (jr *jobRegistry) runningFor(slug string) *Job {
	jr.mu.Lock()
	defer jr.mu.Unlock()
	for _, j := range jr.jobs {
		if j.Slug == slug && j.Status == "running" {
			cp := *j
			return &cp
		}
	}
	return nil
}

func (jr *jobRegistry) get(id string) *Job {
	jr.mu.Lock()
	defer jr.mu.Unlock()
	if j, ok := jr.jobs[id]; ok {
		cp := *j
		return &cp
	}
	return nil
}

func (jr *jobRegistry) cancel(id string) error {
	jr.mu.Lock()
	defer jr.mu.Unlock()
	j, ok := jr.jobs[id]
	if !ok {
		return fmt.Errorf("không tìm thấy job")
	}
	if j.Status != "running" {
		return fmt.Errorf("job không ở trạng thái running")
	}
	// SIGTERM cả process group (engine spawn con)
	_ = syscall.Kill(-j.PID, syscall.SIGTERM)
	_ = syscall.Kill(j.PID, syscall.SIGTERM)
	j.Status = "cancelled"
	j.EndedAt = time.Now()
	jr.persistLocked()
	return nil
}

// startGenJob spawn oum-novel --headless trong cwd = novel dir.
func startGenJob(slug, prompt string) (*Job, error) {
	if running := jobs.runningFor(slug); running != nil {
		return nil, fmt.Errorf("truyện này đang có job chạy (%s)", running.ID)
	}
	novelDir := filepath.Join(novelsDir, slug)
	metaDir := filepath.Join(novelDir, ".omni-app")
	if err := os.MkdirAll(metaDir, 0o755); err != nil {
		return nil, err
	}
	ensureEngineConfig()
	if _, err := os.Stat(engineConfigPath()); err != nil {
		return nil, fmt.Errorf("chưa có cấu hình model — vào /settings để cấu hình trước")
	}
	promptFile := filepath.Join(metaDir, "prompt.txt")
	if err := os.WriteFile(promptFile, []byte(prompt), 0o644); err != nil {
		return nil, err
	}
	logPath := filepath.Join(metaDir, "gen.log")
	rotateLog(logPath)
	logF, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(logF, "\n===== JOB START %s =====\n", time.Now().Format(time.RFC3339))

	cmd := exec.Command(enginePath, "--headless", "--prompt-file", promptFile, "--config", engineConfigPath())
	cmd.Dir = novelDir
	cmd.Stdout = logF
	cmd.Stderr = logF
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		logF.Close()
		return nil, err
	}
	j := &Job{
		ID:        randHex(4),
		Slug:      slug,
		PID:       cmd.Process.Pid,
		Status:    "running",
		StartedAt: time.Now(),
		LogPath:   logPath,
	}
	jobs.mu.Lock()
	jobs.jobs[j.ID] = j
	jobs.persistLocked()
	jobs.mu.Unlock()

	go func() {
		err := cmd.Wait()
		logF.Close()
		jobs.mu.Lock()
		defer jobs.mu.Unlock()
		if j.Status == "cancelled" {
			jobs.persistLocked()
			return
		}
		if err != nil {
			j.Status = "failed"
			j.Note = err.Error()
		} else {
			j.Status = "done"
		}
		j.EndedAt = time.Now()
		jobs.persistLocked()
	}()
	return j, nil
}

// rotateLog: gen.log > 10MB → truncate giữ tail 1MB.
func rotateLog(path string) {
	fi, err := os.Stat(path)
	if err != nil || fi.Size() <= 10<<20 {
		return
	}
	tail := tailFile(path, 1<<20)
	if err := os.WriteFile(path, []byte("===== LOG ROTATED "+time.Now().Format(time.RFC3339)+" =====\n"+tail), 0o644); err != nil {
		log.Printf("rotate log %s: %v", path, err)
	}
}

func tailFile(path string, maxBytes int64) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return ""
	}
	off := int64(0)
	if fi.Size() > maxBytes {
		off = fi.Size() - maxBytes
	}
	_, _ = f.Seek(off, io.SeekStart)
	b, _ := io.ReadAll(f)
	return string(b)
}

// ---------- Slug ----------

var viFold = map[rune]string{
	'à': "a", 'á': "a", 'ả': "a", 'ã': "a", 'ạ': "a", 'ă': "a", 'ằ': "a", 'ắ': "a", 'ẳ': "a", 'ẵ': "a", 'ặ': "a",
	'â': "a", 'ầ': "a", 'ấ': "a", 'ẩ': "a", 'ẫ': "a", 'ậ': "a",
	'è': "e", 'é': "e", 'ẻ': "e", 'ẽ': "e", 'ẹ': "e", 'ê': "e", 'ề': "e", 'ế': "e", 'ể': "e", 'ễ': "e", 'ệ': "e",
	'ì': "i", 'í': "i", 'ỉ': "i", 'ĩ': "i", 'ị': "i",
	'ò': "o", 'ó': "o", 'ỏ': "o", 'õ': "o", 'ọ': "o", 'ô': "o", 'ồ': "o", 'ố': "o", 'ổ': "o", 'ỗ': "o", 'ộ': "o",
	'ơ': "o", 'ờ': "o", 'ớ': "o", 'ở': "o", 'ỡ': "o", 'ợ': "o",
	'ù': "u", 'ú': "u", 'ủ': "u", 'ũ': "u", 'ụ': "u", 'ư': "u", 'ừ': "u", 'ứ': "u", 'ử': "u", 'ữ': "u", 'ự': "u",
	'ỳ': "y", 'ý': "y", 'ỷ': "y", 'ỹ': "y", 'ỵ': "y",
	'đ': "d",
}

func makeSlug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var sb strings.Builder
	for _, r := range s {
		if rep, ok := viFold[r]; ok {
			sb.WriteString(rep)
			continue
		}
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			sb.WriteRune(r)
		case r == ' ', r == '-', r == '_', r == '.':
			sb.WriteRune('-')
		}
	}
	out := sb.String()
	for strings.Contains(out, "--") {
		out = strings.ReplaceAll(out, "--", "-")
	}
	out = strings.Trim(out, "-")
	if out == "" {
		out = "truyen-" + randHex(3)
	}
	return out
}

// ---------- Export ----------

func exportsDir(slug string) string { return filepath.Join(novelsDir, slug, "exports") }

func listExports(slug string) []string {
	entries, err := os.ReadDir(exportsDir(slug))
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".epub") {
			out = append(out, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(out)))
	return out
}

func nextExportName(slug string) string {
	maxV := 0
	for _, name := range listExports(slug) {
		base := strings.TrimSuffix(name, ".epub")
		if i := strings.LastIndex(base, "-v"); i >= 0 {
			if n, err := strconv.Atoi(base[i+2:]); err == nil && n > maxV {
				maxV = n
			}
		}
	}
	return fmt.Sprintf("%s-v%d.epub", slug, maxV+1)
}

// hasEngineStore: novel có store format engine (meta/progress.json) không.
func hasEngineStore(slug string) (string, bool) {
	dir := filepath.Join(novelsDir, slug)
	for _, cand := range []string{
		filepath.Join(dir, "output", "novel"),
		dir,
	} {
		if _, err := os.Stat(filepath.Join(cand, "meta", "progress.json")); err == nil {
			return cand, true
		}
	}
	return "", false
}

type exportRequest struct {
	From, To  int
	Author    string
	Series    string
	Subtitle  string
	Preface   string
	Afterword string
	AuditGate string // off/warn/block
}

// runEngineExport gọi oum-novel --export với cwd sao cho store = output/novel.
func runEngineExport(slug string, req exportRequest) (string, string, error) {
	storeDir, ok := hasEngineStore(slug)
	if !ok {
		return "", "", fmt.Errorf("không có store engine")
	}
	// engine đọc store tại {cwd}/output/novel → cwd phải là cha của output/novel.
	// storeDir = <novel>/output/novel → cwd=<novel>; storeDir=<novel> → tạo symlink shim không cần: dùng meta-file? Không —
	// trường hợp storeDir=<novel> nghĩa là novel dir CHÍNH LÀ store; khi đó cwd cần chứa output/novel.
	// Xử lý: nếu storeDir != <novel>/output/novel thì dựng shim .omni-app/export-shim/output -> symlink.
	novelDir := filepath.Join(novelsDir, slug)
	cwd := novelDir
	if storeDir != filepath.Join(novelDir, "output", "novel") {
		shim := filepath.Join(novelDir, ".omni-app", "export-shim")
		if err := os.MkdirAll(filepath.Join(shim, "output"), 0o755); err != nil {
			return "", "", err
		}
		link := filepath.Join(shim, "output", "novel")
		_ = os.Remove(link)
		if err := os.Symlink(storeDir, link); err != nil {
			return "", "", err
		}
		cwd = shim
	}
	if err := os.MkdirAll(exportsDir(slug), 0o755); err != nil {
		return "", "", err
	}
	name := nextExportName(slug)
	outPath := filepath.Join(exportsDir(slug), name)

	// meta-file cho preface/afterword (struct BookMeta không có json tag →
	// json.Unmarshal khớp tên field không phân biệt hoa thường: Author, Preface, ...)
	metaFile := ""
	if req.Preface != "" || req.Afterword != "" {
		mf := filepath.Join(novelsDir, slug, ".omni-app", "book-meta.json")
		m := map[string]string{}
		if req.Preface != "" {
			m["Preface"] = req.Preface
		}
		if req.Afterword != "" {
			m["Afterword"] = req.Afterword
		}
		raw, _ := json.MarshalIndent(m, "", "  ")
		if err := os.WriteFile(mf, raw, 0o644); err == nil {
			metaFile = mf
		}
	}

	args := []string{"--export", outPath, "--overwrite"}
	if req.From > 0 {
		args = append(args, "--export-from", strconv.Itoa(req.From))
	}
	if req.To > 0 {
		args = append(args, "--export-to", strconv.Itoa(req.To))
	}
	gate := req.AuditGate
	if gate == "" {
		gate = "off"
	}
	args = append(args, "--audit-gate", gate)
	if metaFile != "" {
		args = append(args, "--meta-file", metaFile)
	}
	if req.Author != "" {
		args = append(args, "--author", req.Author)
	}
	if req.Series != "" {
		args = append(args, "--series", req.Series)
	}
	if req.Subtitle != "" {
		args = append(args, "--subtitle", req.Subtitle)
	}
	if _, err := os.Stat(engineConfigPath()); err == nil {
		args = append(args, "--config", engineConfigPath())
	}
	cmd := exec.Command(enginePath, args...)
	cmd.Dir = cwd
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", string(out), fmt.Errorf("engine export lỗi: %v", err)
	}
	return name, string(out), nil
}

// runFallbackExport tự đóng EPUB 3 bằng Go cho truyện không có store engine.
func runFallbackExport(slug string, req exportRequest) (string, error) {
	dir := filepath.Join(novelsDir, slug)
	cdir := contentDir(dir)
	files := findChapterFiles(cdir)
	if len(files) == 0 {
		return "", fmt.Errorf("không có chương nào")
	}
	book, _ := loadBookJSON(slug)
	title := book.Title
	if title == "" {
		title = titleFromSlug(slug)
	}
	author := req.Author
	if author == "" {
		author = book.Author
	}
	series := req.Series
	if series == "" {
		series = book.Series
	}

	var chapters []epubChapter
	seq := 0
	for _, f := range files {
		num, ok := chapterNumber(f)
		if !ok {
			seq++
			num = seq
		} else {
			seq = num
		}
		if req.From > 0 && num < req.From {
			continue
		}
		if req.To > 0 && num > req.To {
			continue
		}
		b, err := os.ReadFile(filepath.Join(cdir, "chapters", f))
		if err != nil {
			continue
		}
		content := string(b)
		chTitle := ""
		for _, line := range strings.SplitN(content, "\n", 5) {
			line = strings.TrimSpace(strings.TrimLeft(line, "# "))
			if line != "" {
				if len([]rune(line)) <= 80 {
					chTitle = line
				}
				break
			}
		}
		chapters = append(chapters, epubChapter{Num: num, Title: chTitle, Body: content})
	}
	if len(chapters) == 0 {
		return "", fmt.Errorf("không có chương trong phạm vi yêu cầu")
	}
	if err := os.MkdirAll(exportsDir(slug), 0o755); err != nil {
		return "", err
	}
	name := nextExportName(slug)
	meta := epubMeta{
		Title: title, Subtitle: req.Subtitle, Author: author, Series: series,
		Preface: req.Preface, Afterword: req.Afterword,
	}
	if err := buildEPUB(filepath.Join(exportsDir(slug), name), meta, chapters); err != nil {
		return "", err
	}
	return name, nil
}

// ---------- HTTP handlers (app) ----------

type flashData struct {
	Msg  string
	Err  string
}

func urlQ(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "&", "%26"), " ", "+")
}

func handleCreatePage(w http.ResponseWriter, r *http.Request) {
	data := struct {
		Genres []string
		Flash  flashData
	}{allGenres, flashData{Err: r.URL.Query().Get("err")}}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, "create.html", data); err != nil {
		log.Printf("template create: %v", err)
	}
}

func handleCreateSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/create?err="+urlQ("form không hợp lệ"), http.StatusSeeOther)
		return
	}
	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		http.Redirect(w, r, "/create?err="+urlQ("thiếu tên truyện"), http.StatusSeeOther)
		return
	}
	slug := strings.TrimSpace(r.FormValue("slug"))
	if slug == "" {
		slug = makeSlug(title)
	} else {
		slug = makeSlug(slug)
	}
	if !validSlug(slug) {
		http.Redirect(w, r, "/create?err="+urlQ("slug không hợp lệ"), http.StatusSeeOther)
		return
	}
	novelDir := filepath.Join(novelsDir, slug)
	if _, err := os.Stat(novelDir); err == nil {
		http.Redirect(w, r, "/create?err="+urlQ("slug đã tồn tại: "+slug), http.StatusSeeOther)
		return
	}
	seeding := strings.TrimSpace(r.FormValue("seeding"))
	script := strings.TrimSpace(r.FormValue("script"))
	if seeding == "" && script == "" {
		http.Redirect(w, r, "/create?err="+urlQ("cần ít nhất seeding hoặc kịch bản"), http.StatusSeeOther)
		return
	}
	// Chống trùng seeding: fingerprint từ nội dung seeding+script nhập tay
	seedFP := seedcraft.FingerprintText(seeding + "\n" + script)
	if seedReg.has(seedFP) {
		http.Redirect(w, r, "/create?err="+urlQ("seeding này đã được dùng trước đây (trùng fingerprint) — hãy đổi ý tưởng"), http.StatusSeeOther)
		return
	}
	genres := r.Form["genres"]
	// chỉ giữ genre hợp lệ
	valid := map[string]bool{}
	for _, g := range allGenres {
		valid[g] = true
	}
	var pickedGenres []string
	for _, g := range genres {
		if valid[g] {
			pickedGenres = append(pickedGenres, g)
		}
	}
	author := strings.TrimSpace(r.FormValue("author"))
	if author == "" {
		author = "Tiểu Tâm"
	}
	series := strings.TrimSpace(r.FormValue("series"))
	chapters, _ := strconv.Atoi(r.FormValue("chapters"))
	if chapters < 1 {
		chapters = 10
	}
	if chapters > 200 {
		chapters = 200
	}
	wordsPer := strings.TrimSpace(r.FormValue("words_per_chapter"))
	if wordsPer == "" {
		wordsPer = "2000-3000"
	}

	if err := os.MkdirAll(filepath.Join(novelDir, ".omni-app"), 0o755); err != nil {
		http.Redirect(w, r, "/create?err="+urlQ(err.Error()), http.StatusSeeOther)
		return
	}
	book := BookJSON{
		Title: title, Author: author, Series: series, Genres: pickedGenres,
		Seeding: seeding, Script: script, ChaptersGoal: chapters, WordsPerCh: wordsPer,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	if err := saveBookJSON(slug, book); err != nil {
		http.Redirect(w, r, "/create?err="+urlQ(err.Error()), http.StatusSeeOther)
		return
	}
	// Ghi registry TRƯỚC khi start job — đã dùng là không bao giờ dùng lại
	if err := seedReg.add(seedRecord{
		Fingerprint: seedFP, Title: title, Slug: slug, Genres: pickedGenres,
		Logline: firstLine(seeding, script), Source: "manual",
	}); err != nil {
		http.Redirect(w, r, "/create?err="+urlQ(err.Error()), http.StatusSeeOther)
		return
	}

	// Dựng prompt tổng hợp
	var pb strings.Builder
	fmt.Fprintf(&pb, "Viết tiểu thuyết tiếng Việt tựa đề: %q.\n", title)
	if len(pickedGenres) > 0 {
		fmt.Fprintf(&pb, "Thể loại: %s.\n", strings.Join(pickedGenres, " + "))
	}
	fmt.Fprintf(&pb, "Tác giả: %s.\n", author)
	if series != "" {
		fmt.Fprintf(&pb, "Thuộc bộ sách: %s.\n", series)
	}
	fmt.Fprintf(&pb, "Tổng số chương mục tiêu: %d chương. Độ dài mỗi chương: khoảng %s từ.\n", chapters, wordsPer)
	if seeding != "" {
		fmt.Fprintf(&pb, "\nÝ tưởng / seeding:\n%s\n", seeding)
	}
	if script != "" {
		fmt.Fprintf(&pb, "\nKịch bản / outline chi tiết (bám sát):\n%s\n", script)
	}

	job, err := startGenJob(slug, pb.String())
	if err != nil {
		http.Redirect(w, r, "/create?err="+urlQ(err.Error()), http.StatusSeeOther)
		return
	}
	log.Printf("job %s bắt đầu gen %q (pid %d)", job.ID, slug, job.PID)
	http.Redirect(w, r, "/novel/"+slug, http.StatusSeeOther)
}

func handleJobsPage(w http.ResponseWriter, r *http.Request) {
	type row struct {
		*Job
		Dur string
	}
	var rows []row
	for _, j := range jobs.list() {
		end := j.EndedAt
		if end.IsZero() {
			end = time.Now()
		}
		rows = append(rows, row{j, end.Sub(j.StartedAt).Round(time.Second).String()})
	}
	data := struct {
		Jobs  []row
		Flash flashData
	}{rows, flashData{Msg: r.URL.Query().Get("ok"), Err: r.URL.Query().Get("err")}}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, "jobs.html", data); err != nil {
		log.Printf("template jobs: %v", err)
	}
}

func handleJobCancel(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := jobs.cancel(id); err != nil {
		http.Redirect(w, r, "/jobs?err="+urlQ(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/jobs?ok="+urlQ("đã gửi SIGTERM"), http.StatusSeeOther)
}

func handleJobLog(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	j := jobs.get(id)
	if j == nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(tailFile(j.LogPath, 64*1024)))
}

func handleAPIJobs(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, jobs.list())
}

func handleExport(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if !validSlug(slug) {
		http.Error(w, "slug không hợp lệ", 400)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/novel/"+slug+"?err="+urlQ("form không hợp lệ"), http.StatusSeeOther)
		return
	}
	from, _ := strconv.Atoi(r.FormValue("from"))
	to, _ := strconv.Atoi(r.FormValue("to"))
	req := exportRequest{
		From: from, To: to,
		Author:    strings.TrimSpace(r.FormValue("author")),
		Series:    strings.TrimSpace(r.FormValue("series")),
		Subtitle:  strings.TrimSpace(r.FormValue("subtitle")),
		Preface:   strings.TrimSpace(r.FormValue("preface")),
		Afterword: strings.TrimSpace(r.FormValue("afterword")),
		AuditGate: r.FormValue("audit_gate"),
	}
	switch req.AuditGate {
	case "off", "warn", "block":
	default:
		req.AuditGate = "off"
	}

	var name, engineOut string
	var err error
	if _, ok := hasEngineStore(slug); ok {
		name, engineOut, err = runEngineExport(slug, req)
		if err != nil {
			log.Printf("engine export %s: %v — thử fallback Go EPUB", slug, err)
			name, err = runFallbackExport(slug, req)
		}
	} else {
		name, err = runFallbackExport(slug, req)
	}
	_ = engineOut
	if err != nil {
		http.Redirect(w, r, "/novel/"+slug+"?err="+urlQ(err.Error()), http.StatusSeeOther)
		return
	}
	if errLib := copyToLibrary(slug, name); errLib != nil {
		log.Printf("copy library %s/%s: %v", slug, name, errLib)
	}
	http.Redirect(w, r, "/novel/"+slug+"?ok="+urlQ("đã xuất "+name+" (+ album)"), http.StatusSeeOther)
}

// firstLine lấy dòng đầu không rỗng làm logline tóm tắt.
func firstLine(candidates ...string) string {
	for _, c := range candidates {
		for _, l := range strings.Split(c, "\n") {
			l = strings.TrimSpace(l)
			if l != "" {
				if len([]rune(l)) > 160 {
					r := []rune(l)
					return string(r[:160]) + "…"
				}
				return l
			}
		}
	}
	return ""
}

func handleDownload(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	file := r.PathValue("file")
	if !validSlug(slug) {
		http.Error(w, "slug không hợp lệ", 400)
		return
	}
	// chống path traversal: chỉ nhận basename, chỉ .epub
	if file != filepath.Base(file) || strings.Contains(file, "..") || !strings.HasSuffix(file, ".epub") {
		http.Error(w, "tên file không hợp lệ", 400)
		return
	}
	path := filepath.Join(exportsDir(slug), file)
	// resolve và đảm bảo nằm trong exports dir
	abs, err := filepath.Abs(path)
	if err != nil || !strings.HasPrefix(abs, filepath.Clean(exportsDir(slug))+string(os.PathSeparator)) {
		http.Error(w, "đường dẫn không hợp lệ", 400)
		return
	}
	f, err := os.Open(abs)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "application/epub+zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+file+`"`)
	_, _ = io.Copy(w, f)
}

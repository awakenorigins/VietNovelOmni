// auto.go — chế độ AUTO: tự sinh seeding, tự tạo truyện, tự gen, tự export, lặp.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"omni-dashboard/seedcraft"
)

type autoStats struct {
	Done   int `json:"done"`
	Failed int `json:"failed"`
}

type autoState struct {
	Enabled       bool      `json:"enabled"`
	Mode          string    `json:"mode"` // count | infinite
	Remaining     int       `json:"remaining"`
	AllowedGenres []string  `json:"allowed_genres"`
	ChMin         int       `json:"ch_min"`
	ChMax         int       `json:"ch_max"`
	PauseSec      int       `json:"pause_sec"`
	AutoExport    bool      `json:"auto_export"`
	Author        string    `json:"author"`
	CurrentSlug   string    `json:"current_slug,omitempty"`
	CurrentTitle  string    `json:"current_title,omitempty"`
	StartedAt     time.Time `json:"started_at,omitempty"`
	Stats         autoStats `json:"stats"`
	Note          string    `json:"note,omitempty"`
}

type autoManager struct {
	mu      sync.Mutex
	st      autoState
	running bool // goroutine runner đang sống
}

var autoMgr = &autoManager{}

func autoStatePath() string { return filepath.Join(appDir, "auto-state.json") }

func (am *autoManager) persistLocked() {
	raw, _ := json.MarshalIndent(am.st, "", "  ")
	_ = os.WriteFile(autoStatePath(), raw, 0o644)
}

func (am *autoManager) load() {
	am.mu.Lock()
	defer am.mu.Unlock()
	raw, err := os.ReadFile(autoStatePath())
	if err != nil {
		return
	}
	_ = json.Unmarshal(raw, &am.st)
}

func (am *autoManager) snapshot() autoState {
	am.mu.Lock()
	defer am.mu.Unlock()
	return am.st
}

func (am *autoManager) isActive() bool {
	am.mu.Lock()
	defer am.mu.Unlock()
	return am.st.Enabled
}

// start cấu hình + bật runner. Trả lỗi nếu đang chạy.
func (am *autoManager) start(st autoState) error {
	am.mu.Lock()
	defer am.mu.Unlock()
	if am.st.Enabled {
		return fmt.Errorf("AUTO đang chạy — dừng trước khi cấu hình lại")
	}
	st.Enabled = true
	st.StartedAt = time.Now()
	st.Stats = autoStats{}
	st.Note = ""
	st.CurrentSlug = ""
	st.CurrentTitle = ""
	am.st = st
	am.persistLocked()
	am.ensureRunnerLocked()
	return nil
}

// stop dừng mềm (soft) hoặc khẩn (emergency = hủy job hiện tại).
func (am *autoManager) stop(emergency bool) {
	am.mu.Lock()
	am.st.Enabled = false
	if emergency {
		am.st.Note = "dừng khẩn cấp bởi người dùng " + time.Now().Format("02/01 15:04")
	} else {
		am.st.Note = "dừng bởi người dùng " + time.Now().Format("02/01 15:04")
	}
	slug := am.st.CurrentSlug
	am.persistLocked()
	am.mu.Unlock()
	if emergency && slug != "" {
		if j := jobs.runningFor(slug); j != nil {
			_ = jobs.cancel(j.ID)
		}
	}
}

func (am *autoManager) setNoteDisable(note string) {
	am.mu.Lock()
	am.st.Enabled = false
	am.st.Note = note
	am.persistLocked()
	am.mu.Unlock()
}

func (am *autoManager) ensureRunnerLocked() {
	if am.running {
		return
	}
	am.running = true
	go func() {
		defer func() {
			am.mu.Lock()
			am.running = false
			am.mu.Unlock()
		}()
		autoLoop()
	}()
}

// resume gọi lúc khởi động app: nếu state enabled thì chạy tiếp.
func (am *autoManager) resume() {
	am.mu.Lock()
	defer am.mu.Unlock()
	if am.st.Enabled {
		log.Printf("AUTO: resume sau restart (mode=%s remaining=%d)", am.st.Mode, am.st.Remaining)
		am.ensureRunnerLocked()
	}
}

// diskUsagePercent trả % dùng của filesystem chứa novelsDir.
func diskUsagePercent() (int, error) {
	var fs syscall.Statfs_t
	if err := syscall.Statfs(novelsDir, &fs); err != nil {
		return 0, err
	}
	if fs.Blocks == 0 {
		return 0, fmt.Errorf("statfs blocks=0")
	}
	used := fs.Blocks - fs.Bfree
	return int(used * 100 / fs.Blocks), nil
}

// anyJobRunning: có job gen nào đang chạy không (an toàn: tối đa 1 job cùng lúc).
func anyJobRunning() bool {
	for _, j := range jobs.list() {
		if j.Status == "running" {
			return true
		}
	}
	return false
}

// sleepInterruptible ngủ tối đa d, thoát sớm nếu AUTO bị tắt. Trả false nếu bị tắt.
func sleepInterruptible(d time.Duration) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if !autoMgr.isActive() {
			return false
		}
		time.Sleep(time.Second)
	}
	return autoMgr.isActive()
}

// autoLoop — vòng lặp chính của chế độ AUTO.
func autoLoop() {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	// Nếu resume mà truyện hiện tại còn job chạy → chờ nó xong trước.
	if st := autoMgr.snapshot(); st.CurrentSlug != "" {
		waitSlugIdle(st.CurrentSlug)
		finishCurrent(st.CurrentSlug)
	}
	for {
		st := autoMgr.snapshot()
		if !st.Enabled {
			return
		}
		if st.Mode == "count" && st.Remaining <= 0 {
			autoMgr.setNoteDisable(fmt.Sprintf("hoàn tất: đã gen %d truyện (%d lỗi)", st.Stats.Done, st.Stats.Failed))
			return
		}
		// Disk guard
		if pct, err := diskUsagePercent(); err == nil && pct > 90 {
			autoMgr.setNoteDisable(fmt.Sprintf("tự dừng: đĩa đã dùng %d%% (>90%%)", pct))
			return
		}
		// Tối đa 1 job gen cùng lúc — xếp hàng chờ
		for anyJobRunning() {
			if !sleepInterruptible(30 * time.Second) {
				return
			}
		}
		// 1. Sinh seed duy nhất
		spec, err := generateUniqueSeed(st.AllowedGenres, rng)
		if err != nil {
			autoMgr.setNoteDisable("tự dừng: " + err.Error())
			return
		}
		// 2. Slug duy nhất
		slug := makeSlug(spec.Title)
		if _, err := os.Stat(filepath.Join(novelsDir, slug)); err == nil {
			slug = slug + "-" + randHex(2)
		}
		// 3. Ghi registry TRƯỚC khi start job — đã dùng là không bao giờ dùng lại
		if err := seedReg.add(seedRecord{
			Fingerprint: spec.Fingerprint(), Title: spec.Title, Slug: slug,
			Genres: spec.Genres, Logline: spec.Logline, Source: "auto",
		}); err != nil {
			log.Printf("AUTO: registry add: %v — sinh lại", err)
			continue
		}
		// 4. Số chương ngẫu nhiên trong [min,max]
		chapters := st.ChMin
		if st.ChMax > st.ChMin {
			chapters += rng.Intn(st.ChMax - st.ChMin + 1)
		}
		author := st.Author
		if author == "" {
			author = "Tiểu Tâm"
		}
		// 5. Novel dir + book.json
		if err := os.MkdirAll(filepath.Join(novelsDir, slug, ".omni-app"), 0o755); err != nil {
			autoMgr.setNoteDisable("tự dừng: không tạo được thư mục truyện: " + err.Error())
			return
		}
		book := BookJSON{
			Title: spec.Title, Author: author, Genres: spec.Genres,
			Seeding: spec.Seeding, Script: "", ChaptersGoal: chapters,
			WordsPerCh: "2000-3000", CreatedAt: time.Now().Format(time.RFC3339),
		}
		if err := saveBookJSON(slug, book); err != nil {
			autoMgr.setNoteDisable("tự dừng: không ghi được book.json: " + err.Error())
			return
		}
		// 6. Prompt tổng hợp từ seed
		prompt := buildAutoPrompt(spec, author, chapters)
		job, err := startGenJob(slug, prompt)
		if err != nil {
			log.Printf("AUTO: start job %q: %v", slug, err)
			autoMgr.mu.Lock()
			autoMgr.st.Stats.Failed++
			if autoMgr.st.Mode == "count" {
				autoMgr.st.Remaining--
			}
			autoMgr.st.Note = "job không start được: " + err.Error()
			autoMgr.persistLocked()
			autoMgr.mu.Unlock()
			if !sleepInterruptible(time.Duration(st.PauseSec) * time.Second) {
				return
			}
			continue
		}
		log.Printf("AUTO: job %s gen %q (%d chương, thể loại %s)", job.ID, slug, chapters, strings.Join(spec.Genres, "+"))
		autoMgr.mu.Lock()
		autoMgr.st.CurrentSlug = slug
		autoMgr.st.CurrentTitle = spec.Title
		autoMgr.persistLocked()
		autoMgr.mu.Unlock()

		// 7. Chờ job xong — ticker 30s
		waitSlugIdle(slug)
		finishCurrent(slug)

		st = autoMgr.snapshot()
		if !st.Enabled {
			return
		}
		// 8. Nghỉ giữa 2 truyện
		if !sleepInterruptible(time.Duration(st.PauseSec) * time.Second) {
			return
		}
	}
}

// waitSlugIdle chờ tới khi slug không còn job chạy (poll 30s).
func waitSlugIdle(slug string) {
	tick := time.NewTicker(30 * time.Second)
	defer tick.Stop()
	for jobs.runningFor(slug) != nil {
		<-tick.C
	}
}

// finishCurrent xử lý sau khi job của slug kết thúc: cập nhật stats, auto-export, album.
func finishCurrent(slug string) {
	if slug == "" {
		return
	}
	// tìm job mới nhất của slug
	var last *Job
	for _, j := range jobs.list() {
		if j.Slug == slug {
			last = j
			break // list đã sort mới nhất trước
		}
	}
	ok := last != nil && last.Status == "done"
	autoMgr.mu.Lock()
	st := autoMgr.st
	if ok {
		autoMgr.st.Stats.Done++
	} else {
		autoMgr.st.Stats.Failed++
	}
	if autoMgr.st.Mode == "count" {
		autoMgr.st.Remaining--
	}
	autoMgr.st.CurrentSlug = ""
	autoMgr.st.CurrentTitle = ""
	autoMgr.persistLocked()
	autoMgr.mu.Unlock()

	if ok && st.AutoExport {
		book, _ := loadBookJSON(slug)
		req := exportRequest{Author: book.Author}
		var name string
		var err error
		if _, has := hasEngineStore(slug); has {
			name, _, err = runEngineExport(slug, req)
			if err != nil {
				log.Printf("AUTO: engine export %s: %v — fallback Go", slug, err)
				name, err = runFallbackExport(slug, req)
			}
		} else {
			name, err = runFallbackExport(slug, req)
		}
		if err != nil {
			log.Printf("AUTO: export %s lỗi: %v", slug, err)
			return
		}
		if err := copyToLibrary(slug, name); err != nil {
			log.Printf("AUTO: copy library %s: %v", slug, err)
		}
	}
}

// ---------- HTTP handlers ----------

func handleAutoPage(w http.ResponseWriter, r *http.Request) {
	st := autoMgr.snapshot()
	allowed := map[string]bool{}
	for _, g := range st.AllowedGenres {
		allowed[g] = true
	}
	if len(st.AllowedGenres) == 0 {
		for _, g := range allGenres {
			allowed[g] = true
		}
	}
	data := struct {
		State      autoState
		Genres     []string
		Allowed    map[string]bool
		SeedCount  int
		Flash      flashData
	}{st, allGenres, allowed, seedReg.count(), flashData{Msg: r.URL.Query().Get("ok"), Err: r.URL.Query().Get("err")}}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, "auto.html", data); err != nil {
		log.Printf("template auto: %v", err)
	}
}

func handleAutoStart(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/auto?err="+urlQ("form không hợp lệ"), http.StatusSeeOther)
		return
	}
	valid := map[string]bool{}
	for _, g := range allGenres {
		valid[g] = true
	}
	var genres []string
	for _, g := range r.Form["genres"] {
		if valid[g] {
			genres = append(genres, g)
		}
	}
	if len(genres) == 0 {
		genres = append([]string(nil), allGenres...)
	}
	chMin, _ := strconv.Atoi(r.FormValue("ch_min"))
	chMax, _ := strconv.Atoi(r.FormValue("ch_max"))
	if chMin < 1 {
		chMin = 5
	}
	if chMin > 200 {
		chMin = 200
	}
	if chMax < chMin {
		chMax = chMin
	}
	if chMax > 200 {
		chMax = 200
	}
	pause, _ := strconv.Atoi(r.FormValue("pause_sec"))
	if pause < 5 {
		pause = 60
	}
	if pause > 86400 {
		pause = 86400
	}
	mode := "count"
	remaining := 0
	if r.FormValue("infinite") == "on" {
		mode = "infinite"
	} else {
		remaining, _ = strconv.Atoi(r.FormValue("count"))
		if remaining < 1 {
			remaining = 1
		}
		if remaining > 1000 {
			remaining = 1000
		}
	}
	author := strings.TrimSpace(r.FormValue("author"))
	if len(author) > 80 {
		author = author[:80]
	}
	st := autoState{
		Mode: mode, Remaining: remaining, AllowedGenres: genres,
		ChMin: chMin, ChMax: chMax, PauseSec: pause,
		AutoExport: r.FormValue("auto_export") == "on",
		Author:     author,
	}
	if err := autoMgr.start(st); err != nil {
		http.Redirect(w, r, "/auto?err="+urlQ(err.Error()), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/auto?ok="+urlQ("AUTO đã bắt đầu"), http.StatusSeeOther)
}

func handleAutoStop(w http.ResponseWriter, r *http.Request) {
	emergency := r.FormValue("emergency") == "1"
	autoMgr.stop(emergency)
	msg := "đã dừng mềm — truyện đang gen sẽ chạy nốt"
	if emergency {
		msg = "đã dừng khẩn — hủy cả job đang chạy"
	}
	http.Redirect(w, r, "/auto?ok="+urlQ(msg), http.StatusSeeOther)
}

func handleAPIAuto(w http.ResponseWriter, r *http.Request) {
	st := autoMgr.snapshot()
	writeJSON(w, 200, st)
}

func buildAutoPrompt(spec seedcraft.SeedSpec, author string, chapters int) string {
	var pb strings.Builder
	fmt.Fprintf(&pb, "Viết tiểu thuyết tiếng Việt tựa đề: %q.\n", spec.Title)
	fmt.Fprintf(&pb, "Thể loại: %s.\n", strings.Join(spec.Genres, " + "))
	fmt.Fprintf(&pb, "Tác giả: %s.\n", author)
	fmt.Fprintf(&pb, "Tổng số chương mục tiêu: %d chương. Độ dài mỗi chương: khoảng 2000-3000 từ.\n", chapters)
	fmt.Fprintf(&pb, "\nLogline: %s\n", spec.Logline)
	fmt.Fprintf(&pb, "\nÝ tưởng / seeding:\n%s\n", spec.Seeding)
	fmt.Fprintf(&pb, "\nGhi chú cấu trúc & nhịp truyện (bám sát):\n%s\n", spec.StructureNote)
	return pb.String()
}

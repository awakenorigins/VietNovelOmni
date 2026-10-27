// library.go — album vật lý: copy EPUB vào .omni-app/library/<the-loai-chinh>/,
// trang /library + API /api/library + download an toàn.
package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func libraryDir() string { return filepath.Join(appDir, "library") }

// genreFolder: tên thư mục ASCII bỏ dấu cho thể loại chính.
func genreFolder(genre string) string {
	f := makeSlug(genre)
	if f == "" {
		f = "khac"
	}
	return f
}

// primaryGenre đọc thể loại chính từ book.json (phần tử đầu); trống → "khac".
func primaryGenre(slug string) string {
	book, ok := loadBookJSON(slug)
	if !ok || len(book.Genres) == 0 {
		return "Khác"
	}
	return book.Genres[0]
}

// copyToLibrary copy exports/<name> của slug vào library/<album>/<name>.
func copyToLibrary(slug, name string) error {
	if !validSlug(slug) || name != filepath.Base(name) || !strings.HasSuffix(name, ".epub") {
		return fmt.Errorf("tham số không hợp lệ")
	}
	src := filepath.Join(exportsDir(slug), name)
	album := genreFolder(primaryGenre(slug))
	dstDir := filepath.Join(libraryDir(), album)
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	dst := filepath.Join(dstDir, name)
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		os.Remove(dst)
		return err
	}
	log.Printf("library: đã xếp %s vào album %s", name, album)
	return nil
}

// ---------- Listing ----------

type libFile struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

type libAlbum struct {
	Folder    string    `json:"folder"`
	Files     []libFile `json:"files"`
	TotalSize int64     `json:"total_size"`
}

func listLibrary() []libAlbum {
	entries, err := os.ReadDir(libraryDir())
	if err != nil {
		return nil
	}
	var out []libAlbum
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		al := libAlbum{Folder: e.Name()}
		files, err := os.ReadDir(filepath.Join(libraryDir(), e.Name()))
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".epub") {
				continue
			}
			fi, err := f.Info()
			if err != nil {
				continue
			}
			al.Files = append(al.Files, libFile{Name: f.Name(), Size: fi.Size()})
			al.TotalSize += fi.Size()
		}
		sort.Slice(al.Files, func(i, j int) bool { return al.Files[i].Name < al.Files[j].Name })
		if len(al.Files) > 0 {
			out = append(out, al)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Folder < out[j].Folder })
	return out
}

func humanSize(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}

// ---------- HTTP ----------

func handleLibraryPage(w http.ResponseWriter, r *http.Request) {
	type albumView struct {
		Folder string
		Files  []libFile
		Total  string
	}
	var views []albumView
	for _, al := range listLibrary() {
		views = append(views, albumView{al.Folder, al.Files, humanSize(al.TotalSize)})
	}
	data := struct {
		Albums []albumView
	}{views}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := templates.ExecuteTemplate(w, "library.html", data); err != nil {
		log.Printf("template library: %v", err)
	}
}

func handleAPILibrary(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, listLibrary())
}

// handleLibraryDownload: GET /library/{album}/{file} — safe-join, chỉ .epub.
func handleLibraryDownload(w http.ResponseWriter, r *http.Request) {
	album := r.PathValue("album")
	file := r.PathValue("file")
	if !validSlug(album) {
		http.Error(w, "album không hợp lệ", 400)
		return
	}
	if file != filepath.Base(file) || strings.Contains(file, "..") || !strings.HasSuffix(file, ".epub") {
		http.Error(w, "tên file không hợp lệ", 400)
		return
	}
	path := filepath.Join(libraryDir(), album, file)
	abs, err := filepath.Abs(path)
	if err != nil || !strings.HasPrefix(abs, filepath.Clean(libraryDir())+string(os.PathSeparator)) {
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

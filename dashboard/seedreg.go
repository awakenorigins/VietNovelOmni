// seedreg.go — registry chống trùng seeding vĩnh viễn (append-only).
package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"

	"omni-dashboard/seedcraft"
)

type seedRecord struct {
	Fingerprint string   `json:"fingerprint"`
	Title       string   `json:"title"`
	Slug        string   `json:"slug"`
	Genres      []string `json:"genres,omitempty"`
	Logline     string   `json:"logline,omitempty"`
	Source      string   `json:"source"` // auto | manual
	UsedAt      string   `json:"used_at"`
}

type seedRegistry struct {
	mu   sync.Mutex
	byFP map[string]seedRecord
	list []seedRecord
}

var seedReg = &seedRegistry{byFP: map[string]seedRecord{}}

func seedRegPath() string { return filepath.Join(appDir, "seed-registry.json") }

func (sr *seedRegistry) load() {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	raw, err := os.ReadFile(seedRegPath())
	if err != nil {
		return
	}
	var list []seedRecord
	if json.Unmarshal(raw, &list) != nil {
		return
	}
	sr.list = list
	for _, rec := range list {
		sr.byFP[rec.Fingerprint] = rec
	}
}

func (sr *seedRegistry) has(fp string) bool {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	_, ok := sr.byFP[fp]
	return ok
}

// add ghi record vào registry (append-only) và persist NGAY — trước khi start job.
// Trả lỗi nếu fingerprint đã tồn tại.
func (sr *seedRegistry) add(rec seedRecord) error {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	if _, ok := sr.byFP[rec.Fingerprint]; ok {
		return fmt.Errorf("seeding trùng fingerprint với seed đã dùng (%s, %s)", sr.byFP[rec.Fingerprint].Title, sr.byFP[rec.Fingerprint].UsedAt)
	}
	rec.UsedAt = time.Now().Format(time.RFC3339)
	sr.byFP[rec.Fingerprint] = rec
	sr.list = append(sr.list, rec)
	raw, _ := json.MarshalIndent(sr.list, "", "  ")
	return os.WriteFile(seedRegPath(), raw, 0o644)
}

func (sr *seedRegistry) count() int {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	return len(sr.list)
}

// generateUniqueSeed sinh seed chưa từng dùng. Tối đa 50 lần với genres đã cho;
// vẫn trùng thì mở rộng random space (bỏ ràng buộc thể loại) thêm 50 lần nữa.
func generateUniqueSeed(allowedGenres []string, rng *rand.Rand) (seedcraft.SeedSpec, error) {
	pickGenres := func(pool []string) []string {
		if len(pool) == 0 {
			pool = allGenres
		}
		n := 1 + rng.Intn(2) // 1-2 thể loại
		if n > len(pool) {
			n = len(pool)
		}
		idx := rng.Perm(len(pool))[:n]
		out := make([]string, 0, n)
		for _, i := range idx {
			out = append(out, pool[i])
		}
		return out
	}
	for i := 0; i < 50; i++ {
		spec := seedcraft.GenerateSeed(pickGenres(allowedGenres), rng)
		if !seedReg.has(spec.Fingerprint()) {
			return spec, nil
		}
	}
	// mở rộng random space: mọi thể loại
	for i := 0; i < 50; i++ {
		spec := seedcraft.GenerateSeed(pickGenres(allGenres), rng)
		if !seedReg.has(spec.Fingerprint()) {
			return spec, nil
		}
	}
	return seedcraft.SeedSpec{}, fmt.Errorf("không sinh được seed mới sau 100 lần (registry %d seed) — kho tổ hợp cạn cho bộ thể loại này", seedReg.count())
}

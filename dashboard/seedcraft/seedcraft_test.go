package seedcraft

import (
	"math/rand"
	"strings"
	"testing"
)

func TestGenerateSeedUnique(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	seen := map[string]bool{}
	dup := 0
	for i := 0; i < 200; i++ {
		s := GenerateSeed([]string{"Tiên hiệp", "Trọng sinh"}, rng)
		fp := s.Fingerprint()
		if seen[fp] {
			dup++
		}
		seen[fp] = true
		words := len(strings.Fields(s.Seeding))
		if words < 100 || words > 500 {
			t.Errorf("seeding %d words ngoài khoảng: %d", i, words)
		}
		if s.Title == "" || s.Logline == "" || s.StructureNote == "" {
			t.Errorf("thiếu trường: %+v", s)
		}
	}
	if dup > 20 {
		t.Errorf("quá nhiều trùng: %d/200", dup)
	}
	t.Logf("unique %d/200, dup %d", len(seen), dup)
}

func TestFingerprintNormalize(t *testing.T) {
	a := FingerprintText("Tiên Hiệp  Báo Thù")
	b := FingerprintText("tien hiep bao thu")
	if a != b {
		t.Error("normalize phải bỏ dấu + lowercase + bỏ space")
	}
}

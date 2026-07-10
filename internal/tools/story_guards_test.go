package tools

import (
	"strings"
	"testing"

	"github.com/voocel/ainovel-cli/internal/domain"
)

func TestHookMonotonyWarning(t *testing.T) {
	cases := []struct {
		name    string
		history []string
		newHook string
		want    bool
	}{
		{"empty history", nil, "crisis", false},
		{"two in a row (below threshold)", []string{"crisis"}, "crisis", false},
		{"three in a row triggers", []string{"crisis", "crisis"}, "crisis", true},
		{"broken run resets", []string{"crisis", "mystery"}, "crisis", false},
		{"case-insensitive", []string{"Crisis", "CRISIS"}, "crisis", true},
		{"empty new hook ignored", []string{"crisis", "crisis"}, "", false},
		{"long run", []string{"emotion", "crisis", "crisis", "crisis"}, "crisis", true},
	}
	for _, c := range cases {
		got := hookMonotonyWarning(c.history, c.newHook)
		if (got != "") != c.want {
			t.Errorf("%s: hookMonotonyWarning(%v, %q) = %q, want warning=%v", c.name, c.history, c.newHook, got, c.want)
		}
	}
}

func TestFirstChapterHookIssues(t *testing.T) {
	weak := "# Chương 1\n\nBuổi sáng hôm ấy trời trong xanh, gió nhè nhẹ thổi qua hàng cây ven đường. " +
		strings.Repeat("Khung cảnh yên bình tĩnh lặng trải dài trước mắt với những mái nhà cũ kỹ nằm im lìm dưới nắng. ", 12)
	vs := firstChapterHookIssues(weak)
	var hasWeakOpen, hasStatic bool
	for _, v := range vs {
		if v.Rule == "ch1_weak_opening" {
			hasWeakOpen = true
		}
		if v.Rule == "ch1_static_opening" {
			hasStatic = true
		}
	}
	if !hasWeakOpen {
		t.Errorf("expected ch1_weak_opening for weather opening, got %+v", vs)
	}
	if !hasStatic {
		t.Errorf("expected ch1_static_opening for motionless opening, got %+v", vs)
	}

	strong := "# Chương 1\n\nLưỡi dao siết ngang cổ hắn trước khi kịp hét. \"Tiền đâu?\" Giọng nói phả sát tai, tanh mùi máu."
	if vs := firstChapterHookIssues(strong); len(vs) != 0 {
		t.Errorf("strong opening should pass, got %+v", vs)
	}
}

func TestPremiseVietnameseHeadings(t *testing.T) {
	premise := `# Đêm Dài Sắp Sáng

## Thể loại và tông điệu
Đô thị đen, hài đen lạnh.

## Định vị thể loại (độc giả mục tiêu, điểm tiêu dùng cốt lõi)
Độc giả web-novel Việt thích phản anh hùng.

## Xung đột cốt lõi
Tự do và quyền lực tỉ lệ nghịch.

## Mục tiêu nhân vật chính
Sống sót mà không quỳ.

## Hướng kết cục
Kết mở, trả giá thật.

## Vùng cấm viết
Không tẩy trắng, không deus ex machina.

## Điểm bán hàng khác biệt (ít nhất 3 điểm)
Giọng kể quái, hài đen, chi tiết đời thật.

## Điểm móc khác biệt: điểm độc đáo khiến độc giả muốn tiếp tục theo dõi
Mỗi chương một cú lật nhỏ.

## Cam kết thực hiện cốt lõi
Mỗi 3 chương một payoff.

## Động cơ truyện
Món nợ máu và cái giá của im lặng.

## Tuyến quan hệ/phát triển
Từ độc hành sang bầy đàn miễn cưỡng.

## Lộ trình nâng cấp
Đầu: mưu vặt. Giữa: thế lực. Cuối: đổi luật chơi.

## Bước ngoặt giữa chuyện
Chỗ dựa lớn nhất phản bội.

## Mệnh đề kết cục
Tự do có mua được bằng quyền lực không.
`
	structure := premiseStructure(premise, domain.PlanningTierLong)
	ready, _ := structure["template_ready"].(bool)
	if !ready {
		t.Fatalf("Vietnamese premise headings must be template_ready, got %+v", structure)
	}
}

func TestNormalizePremiseHeading(t *testing.T) {
	cases := map[string]string{
		"Định vị thể loại (độc giả mục tiêu)": "định vị thể loại",
		"Điểm móc khác biệt: điểm độc đáo":    "điểm móc khác biệt",
		"  Thể loại   và tông điệu  ":         "thể loại và tông điệu",
	}
	for in, want := range cases {
		if got := normalizePremiseHeading(in); got != want {
			t.Errorf("normalizePremiseHeading(%q) = %q, want %q", in, got, want)
		}
	}
}

package exp

import (
	"strings"
	"testing"

	"github.com/voocel/ainovel-cli/internal/domain"
)

func TestStripChapterTitleHeader(t *testing.T) {
	cases := []struct {
		name  string
		in    string
		title string
		want  string
	}{
		{"plain body untouched", "Anh nhìn ra ngoài cửa sổ.", "Đêm Mưa", "Anh nhìn ra ngoài cửa sổ."},
		{"strip h1 vietnamese title", "# Chương 1  Đêm Mưa\n\nAnh nhìn ra ngoài cửa sổ.", "Đêm Mưa", "Anh nhìn ra ngoài cửa sổ."},
		{"strip h2 with chapter token vi", "## Chương 2\n\nAnh nhìn ra ngoài cửa sổ.", "", "Anh nhìn ra ngoài cửa sổ."},
		{"strip h1 chinese title (backward compat)", "# 第 1 章  雨夜归人\n\n他望着窗外。", "雨夜归人", "他望着窗外。"},
		{"keep body even if no header", "Câu đầu tiên.\nCâu thứ hai.", "", "Câu đầu tiên.\nCâu thứ hai."},
		{"do not strip non-chapter heading", "# Khúc Dạo Đầu\nAnh nhìn ra ngoài cửa sổ.", "Làng Biên", "# Khúc Dạo Đầu\nAnh nhìn ra ngoài cửa sổ."},
		{"single line header only", "# Chương 1", "", ""},
		// writer ghi tên chương thuần túy vào dòng đầu dưới dạng tiêu đề → trùng với tiêu đề thống nhất của trình xuất, cần bóc bỏ
		{"strip h1 matching chapter title", "# Làng Biên\n\nTrời chưa sáng.", "Làng Biên", "Trời chưa sáng."},
		// Dòng đầu là h1 nhưng nội dung không khớp tiêu đề chương → xem là nội dung chính, giữ lại
		{"keep h1 not matching title", "# Tiểu mục khác\nNội dung.", "Làng Biên", "# Tiểu mục khác\nNội dung."},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := stripChapterTitleHeader(c.in, c.title)
			if got != c.want {
				t.Fatalf("stripChapterTitleHeader\nin   = %q\ntitle= %q\nwant = %q\ngot  = %q", c.in, c.title, c.want, got)
			}
		})
	}
}

func TestBuildTitleIndex(t *testing.T) {
	outline := []domain.OutlineEntry{
		{Chapter: 1, Title: "Đêm Mưa"},
		{Chapter: 2, Title: ""}, // tiêu đề rỗng cần được lọc bỏ
		{Chapter: 3, Title: "Bình Minh"},
	}
	idx := buildTitleIndex(outline)
	if got := idx[1]; got != "Đêm Mưa" {
		t.Errorf("ch1 title: got %q want Đêm Mưa", got)
	}
	if _, ok := idx[2]; ok {
		t.Errorf("ch2 should be absent (empty title)")
	}
	if got := idx[3]; got != "Bình Minh" {
		t.Errorf("ch3 title: got %q want Bình Minh", got)
	}
}

func TestBuildLocations(t *testing.T) {
	volumes := []domain.VolumeOutline{
		{Index: 1, Title: "Khởi Nguồn", Arcs: []domain.ArcOutline{
			{Index: 1, Title: "Thiếu niên xuất hiện", Chapters: []domain.OutlineEntry{{}, {}}}, // 2 chương
			{Index: 2, Title: "Tông môn thử thách", Chapters: []domain.OutlineEntry{{}}},        // 1 chương
		}},
		{Index: 2, Title: "Quật Khởi", Arcs: []domain.ArcOutline{
			{Index: 1, Title: "Trận đầu", Chapters: []domain.OutlineEntry{{}}},
		}},
	}
	locs := buildLocations(volumes)

	// Chỉ xác minh tư cách thuộc tập: cung không còn vào location, nhưng tầng cung vẫn tham gia tích lũy số chương toàn cục.
	if loc := locs[1]; !loc.IsFirstOfVolume || loc.VolumeIdx != 1 {
		t.Errorf("ch1 should be first of volume 1: %+v", loc)
	}
	if loc := locs[2]; loc.IsFirstOfVolume || loc.VolumeIdx != 1 {
		t.Errorf("ch2 should be volume 1, not first: %+v", loc)
	}
	// ch3 là chương đầu của cung 2, nhưng vẫn nằm trong tập 1 → không phải đầu tập.
	if loc := locs[3]; loc.IsFirstOfVolume || loc.VolumeIdx != 1 {
		t.Errorf("ch3 (arc 2, same volume) should not be first of volume: %+v", loc)
	}
	if loc := locs[4]; !loc.IsFirstOfVolume || loc.VolumeIdx != 2 {
		t.Errorf("ch4 should start volume 2: %+v", loc)
	}
}

func TestRenderTXT_TitleAndChapter(t *testing.T) {
	got := renderTXT(
		"Ánh Sáng",
		[]int{1, 2},
		chapterTitleIndex{1: "Đêm Mưa", 2: "Bình Minh"},
		nil,
		map[int]string{
			1: "# Chương 1 Đêm Mưa\n\nAnh nhìn ra ngoài cửa sổ.",
			2: "Cô đẩy cửa bước vào.",
		},
	)
	if !strings.HasPrefix(got, "《Ánh Sáng》\n\n") {
		t.Errorf("missing book title at start:\n%s", got)
	}
	// tiền đề không vào xuất: sau tên sách phải là chương trực tiếp, không kẹp bất kỳ tóm tắt tiền đề nào
	if !strings.Contains(got, "Chương 1  Đêm Mưa") {
		t.Errorf("missing ch1 header")
	}
	if !strings.Contains(got, "Anh nhìn ra ngoài cửa sổ.") {
		t.Errorf("missing ch1 body")
	}
	if strings.Contains(got, "# Chương 1") {
		t.Errorf("body markdown header not stripped:\n%s", got)
	}
	if !strings.Contains(got, "Chương 2  Bình Minh") {
		t.Errorf("missing ch2 header")
	}
}

func TestRenderTXT_EmptyNovelNameNoTitleLine(t *testing.T) {
	got := renderTXT(
		"",
		[]int{1},
		chapterTitleIndex{1: "Đêm Mưa"},
		nil,
		map[int]string{1: "Nội dung."},
	)
	if strings.Contains(got, "《") {
		t.Errorf("should not contain book title brackets: %s", got)
	}
	if !strings.HasPrefix(got, "Chương 1  Đêm Mưa") {
		t.Errorf("expect chapter header at very start: %s", got)
	}
}

// TestRenderTXT_LayeredVolume xác minh đề cương phân tầng chỉ chèn phân cách tập ở đầu tập, phân cách cung không bao giờ xuất hiện
// (issue #27: định dạng là "《Tên sách》→ phân cách tập → nội dung chương").
func TestRenderTXT_LayeredVolume(t *testing.T) {
	locs := map[int]chapterLocation{
		1: {VolumeIdx: 1, VolumeTitle: "Khởi Nguồn", IsFirstOfVolume: true},
		2: {VolumeIdx: 1, VolumeTitle: "Khởi Nguồn"},
	}
	got := renderTXT(
		"X", []int{1, 2},
		chapterTitleIndex{1: "A", 2: "B"},
		locs,
		map[int]string{1: "Nội dung một.", 2: "Nội dung hai."},
	)
	if !strings.Contains(got, "Tập 1  Khởi Nguồn") {
		t.Errorf("missing volume header: %s", got)
	}
	// tiêu đề tập chỉ xuất hiện một lần trước chương đầu tiên
	if strings.Count(got, "Tập 1") != 1 {
		t.Errorf("volume header should appear exactly once: %s", got)
	}
}

func TestRenderTXT_ChapterWithoutTitleFallsBackToNumberOnly(t *testing.T) {
	got := renderTXT(
		"", []int{5},
		chapterTitleIndex{}, // không có tiêu đề
		nil,
		map[int]string{5: "Nội dung."},
	)
	if !strings.Contains(got, "Chương 5\n\n") {
		t.Errorf("expect 'Chương 5' fallback header: %s", got)
	}
}

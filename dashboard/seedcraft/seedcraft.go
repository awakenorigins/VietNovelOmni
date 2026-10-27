// Package seedcraft — Seeding Mastery Engine: kho tri thức viết tiểu thuyết
// nén thành các bảng chọn + generator sinh seeding tiếng Việt đa dạng, chống trùng
// bằng fingerprint chuẩn hóa.
package seedcraft

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand"
	"sort"
	"strings"
)

// ---------- SeedSpec ----------

type SeedSpec struct {
	Genres        []string `json:"genres"`
	Title         string   `json:"title"`
	Logline       string   `json:"logline"`
	Seeding       string   `json:"seeding"`
	StructureNote string   `json:"structure_note"`
	Trope         string   `json:"trope"`
	Conflict      string   `json:"conflict"`
	Twist         string   `json:"twist"`
	Hook          string   `json:"hook"`
	Mood          string   `json:"mood"`
	Theme         string   `json:"theme"`
}

// Fingerprint: hash chuẩn hóa từ (genres + trope chính + xung đột + twist + logline).
func (s SeedSpec) Fingerprint() string {
	gs := append([]string(nil), s.Genres...)
	sort.Strings(gs)
	base := strings.Join(gs, "|") + "|" + s.Trope + "|" + s.Conflict + "|" + s.Twist + "|" + s.Logline
	return FingerprintText(base)
}

// FingerprintText: fingerprint cho văn bản tùy ý (dùng cho seeding nhập tay).
func FingerprintText(text string) string {
	h := sha256.Sum256([]byte(Normalize(text)))
	return hex.EncodeToString(h[:])
}

// Normalize: lowercase + bỏ dấu tiếng Việt + xóa mọi khoảng trắng/ký tự phân cách.
func Normalize(s string) string {
	s = strings.ToLower(s)
	var sb strings.Builder
	for _, r := range s {
		if rep, ok := viFold[r]; ok {
			sb.WriteRune(rep)
			continue
		}
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			sb.WriteRune(r)
		case r > 127: // chữ khác (Hán, emoji...) giữ nguyên để phân biệt
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

var viFold = map[rune]rune{
	'à': 'a', 'á': 'a', 'ả': 'a', 'ã': 'a', 'ạ': 'a', 'ă': 'a', 'ằ': 'a', 'ắ': 'a', 'ẳ': 'a', 'ẵ': 'a', 'ặ': 'a',
	'â': 'a', 'ầ': 'a', 'ấ': 'a', 'ẩ': 'a', 'ẫ': 'a', 'ậ': 'a',
	'è': 'e', 'é': 'e', 'ẻ': 'e', 'ẽ': 'e', 'ẹ': 'e', 'ê': 'e', 'ề': 'e', 'ế': 'e', 'ể': 'e', 'ễ': 'e', 'ệ': 'e',
	'ì': 'i', 'í': 'i', 'ỉ': 'i', 'ĩ': 'i', 'ị': 'i',
	'ò': 'o', 'ó': 'o', 'ỏ': 'o', 'õ': 'o', 'ọ': 'o', 'ô': 'o', 'ồ': 'o', 'ố': 'o', 'ổ': 'o', 'ỗ': 'o', 'ộ': 'o',
	'ơ': 'o', 'ờ': 'o', 'ớ': 'o', 'ở': 'o', 'ỡ': 'o', 'ợ': 'o',
	'ù': 'u', 'ú': 'u', 'ủ': 'u', 'ũ': 'u', 'ụ': 'u', 'ư': 'u', 'ừ': 'u', 'ứ': 'u', 'ử': 'u', 'ữ': 'u', 'ự': 'u',
	'ỳ': 'y', 'ý': 'y', 'ỷ': 'y', 'ỹ': 'y', 'ỵ': 'y',
	'đ': 'd',
}

// ---------- (a) CẤU TRÚC ----------

type structureDef struct{ Name, Note string }

var structures = []structureDef{
	{"Ba hồi (Three-Act)", "Hồi 1 thiết lập + biến cố khởi đầu (25%), hồi 2 leo thang qua midpoint đảo chiều (50%), hồi 3 khủng hoảng tối tăm rồi cao trào và dư âm (25%)."},
	{"Save the Cat 15 beats", "Opening image → theme stated → setup → catalyst → debate → break into two → B-story → fun and games → midpoint → bad guys close in → all is lost → dark night of the soul → break into three → finale → final image. Bám nhịp phần trăm của Snyder."},
	{"Hero's Journey", "Thế giới thường nhật → tiếng gọi phiêu lưu → từ chối → gặp người dẫn đường → vượt ngưỡng → thử thách/đồng minh/kẻ thù → hang sâu nhất → khổ hình → phần thưởng → đường về → tái sinh → trở về với thuốc tiên."},
	{"Kishōtenketsu (khởi-thừa-chuyển-hợp)", "Không dựa xung đột trực diện: khởi giới thiệu, thừa phát triển, CHUYỂN đưa vào yếu tố bất ngờ làm đổi cách hiểu toàn bộ, hợp gói lại ở tầng nghĩa mới. Hợp truyện đời thường, twist tinh tế."},
	{"7-point structure", "Hook → plot turn 1 → pinch 1 → midpoint (từ bị động sang chủ động) → pinch 2 (mất mát lớn) → plot turn 2 (mảnh ghép cuối) → resolution. Thiết kế ngược từ kết truyện."},
	{"Nested loops (vòng lặp lồng)", "Nhiều tuyến mở ra như vòng lồng nhau: mở A → mở B → mở C → đóng C → đóng B → đóng A. Câu chuyện trong câu chuyện, hồi ức lồng hiện tại, trả lời theo thứ tự ngược."},
}

// ---------- (b) HOOK MASTER ----------

type hookDef struct{ Name, Guide string }

var hooks = []hookDef{
	{"In medias res", "Mở màn giữa biến cố đang diễn ra, không giải thích; người đọc tự ráp bối cảnh qua hành động."},
	{"Câu hỏi treo", "Câu đầu tiên đặt một câu hỏi không thể bỏ qua, chỉ được trả lời trọn vẹn ở hồi cuối."},
	{"Nghịch lý nhân vật", "Giới thiệu nhân vật bằng một mâu thuẫn nội tại trớ trêu khiến người đọc phải hiểu cho bằng được."},
	{"Đồng hồ đếm ngược", "Ngay từ đầu đặt một hạn chót sống còn; mỗi chương siết thời gian còn lại."},
	{"Bí mật kể một nửa", "Người kể hé lộ nửa bí mật rồi ngừng — phần còn thiếu trở thành động cơ đọc tiếp."},
	{"Mở bằng hậu quả", "Chương đầu cho thấy hậu quả tàn khốc, rồi quay ngược kể lại con đường dẫn tới nó."},
}

// ---------- (c) MOOD/TONE ----------

var moods = []string{
	"u tối nhưng luôn le lói hy vọng, càng về cuối ánh sáng càng đắt giá",
	"hài đen — cười đấy nhưng dư vị chua chát, giễu mà thương",
	"bi tráng, hào hùng pha mất mát, đẹp như một khúc ca tế",
	"ấm áp xen lạnh lẽo — chương ấm nối chương buốt, tương phản nhiệt độ cảm xúc",
	"căng thẳng nghẹt thở, nhịp ngắn, thông tin nhỏ giọt",
	"hoài niệm man mác, thời gian như nước chảy qua kẽ tay",
	"lạnh lùng tỉnh táo, kể bằng con mắt quan sát không phán xét",
	"rực rỡ mà bất an — bề mặt lộng lẫy che một vết nứt lớn dần",
}

// ---------- (d) XUNG ĐỘT LÕI ----------

type conflictDef struct{ Name, Desc string }

var conflicts = []conflictDef{
	{"người vs người", "đối thủ cụ thể bằng xương thịt, mục tiêu loại trừ nhau, càng hiểu nhau càng nguy hiểm"},
	{"người vs xã hội", "luật lệ, định kiến, thể chế nghiền nát cá nhân; thắng một người dễ, thắng cả guồng máy mới khó"},
	{"người vs bản thân", "kẻ thù lớn nhất nằm trong gương: cám dỗ, vết thương cũ, con người mình có thể trở thành"},
	{"người vs định mệnh", "lời nguyền, tiên tri, vòng lặp — vùng vẫy trong bàn tay số phận và cái giá của việc cãi mệnh"},
	{"người vs công nghệ", "thứ con người tạo ra quay lại định nghĩa con người; tiện nghi đổi bằng tự do"},
	{"người vs thiên nhiên", "thiên tai, hoang dã, dịch bệnh — sự sống còn trần trụi lột mặt nạ văn minh"},
}

// ---------- (e) TROPE MATRIX theo thể loại ----------

var tropeMatrix = map[string][]string{
	"Tiên hiệp": {
		"linh căn phế vật ôm bí mật nghịch thiên cải mệnh",
		"lão quái ngàn năm trọng sinh trong thân xác thiếu niên vô danh",
		"đan sư ẩn danh giả làm tạp dịch trong tông môn thù địch",
		"tông môn bị diệt chỉ còn một truyền nhân mang theo tín vật cuối",
		"kẻ chuyên cướp cơ duyên của 'con cưng vận mệnh' và trả giá",
		"bia đá cổ khắc công pháp thất truyền chỉ hiện chữ khi trăng khuyết",
	},
	"Huyền huyễn": {
		"huyết mạch thượng cổ thức tỉnh kèm cơn đói không thể nói ra",
		"tranh đoạt dị hỏa khiến kẻ thắng phải mang theo ý thức kẻ thua",
		"trong cơ thể nhân vật chính có cả một thế giới đang tự tiến hóa",
		"ma đầu chuyển sinh làm đệ tử danh môn, bị chính đạo nuôi lớn",
		"khế ước thần thú nhưng thần thú mới là bên đặt điều khoản",
	},
	"Đô thị": {
		"cao thủ quy ẩn về thành phố sống đời thường, sóng gió tự tìm đến",
		"thần y giấu nghề chỉ ra tay khi món nợ ân tình bị gọi tên",
		"kẻ nghèo bỗng thừa kế tập đoàn kèm một điều kiện quái gở",
		"vạch mặt hào môn: hôn ước là nước cờ, tình cảm là biến số",
		"hệ thống chấm điểm cuộc sống — điểm cao đổi đặc quyền, điểm thấp mất ký ức",
	},
	"Trọng sinh": {
		"báo thù bằng ký ức tương lai, nhưng mỗi thay đổi làm tương lai lệch thêm",
		"trọng sinh để chuộc lỗi với người đã vì mình mà chết",
		"đầu tư đón đầu thời đại rồi phát hiện có kẻ khác cũng trọng sinh",
		"sống lại đúng ngày định mệnh, và ngày đó cứ lặp cho tới khi làm đúng",
		"đổi vai: sống lại trong thân xác kẻ thù không đội trời chung",
	},
	"Hệ thống": {
		"hệ thống giao nhiệm vụ oái oăm ngược đạo lý, chống lệnh thì trừ tuổi thọ",
		"hệ thống hỏng hóc chuyên phát thưởng nhầm — và nhầm lẫn có quy luật",
		"điểm thưởng đổi bằng ký ức: càng mạnh càng quên mình là ai",
		"hệ thống là linh hồn người dùng trước, đang tìm cách cảnh báo",
		"nhiệm vụ cuối cùng của chuỗi nhiệm vụ là xóa bỏ chính hệ thống",
	},
	"Xuyên không": {
		"xuyên vào vai phản diện bị định sẵn kết cục thảm sau ba năm",
		"xuyên vào cuốn sách chính mình viết dở, lỗ hổng cốt truyện thành tai họa",
		"mang kiến thức hiện đại về cổ đại nhưng mỗi phát minh đều có giá",
		"xuyên qua lại hai thế giới, thời gian hai bên chảy lệch nhau",
		"xuyên nhầm vào đêm trước đại nạn mà sử sách chép sai ngày",
	},
	"Kiếm hiệp": {
		"mối thù diệt môn mười năm mài một thanh kiếm gãy",
		"bí kíp võ công ai luyện cũng chết — trừ kẻ không muốn sống",
		"kiếm khách quy ẩn bị ép tái xuất bằng chính người mình muốn bảo vệ",
		"ân oán hai đại thế gia và đứa con mang nửa dòng máu mỗi bên",
		"giang hồ dậy sóng bởi một bàn tay giấu mặt đứng sau mọi đại hội võ lâm",
	},
	"Ngôn tình": {
		"hợp đồng hôn nhân có điều khoản cấm yêu — và cả hai đều vi phạm",
		"oan gia ngõ hẹp: ghét nhau vì hiểu lầm, gần nhau vì công việc",
		"thanh mai trúc mã thất lạc, tái ngộ khi một người đã đổi tên đổi phận",
		"yêu nhau dưới thân phận giả, sợ nhất ngày sự thật đến trước lời tỏ tình",
		"một lá thư gửi nhầm địa chỉ khơi lại mối duyên tưởng đã chôn",
	},
	"Đam mỹ": {
		"đối thủ không đội trời chung bị buộc hợp tác trong một vụ sống còn",
		"trọng sinh trở về sửa sai với người từng bị mình đẩy vào tuyệt lộ",
		"thân phận giả che thân phận thật, lớp mặt nạ nào rơi trước",
		"vệ sĩ lạnh lùng và thiếu gia ương bướng — bảo vệ rồi mới hiểu vì sao",
		"tri kỷ tưởng đã chết trở về với gương mặt khác và mối hận cũ",
	},
	"Trinh thám": {
		"án mạng phòng kín nơi mọi nghi phạm đều có chứng cứ ngoại phạm hoàn hảo",
		"kẻ sát nhân bắt chước những vụ án trong tiểu thuyết chưa xuất bản",
		"thám tử mang quá khứ tội lỗi buộc phải điều tra vụ án chạm vào chính nó",
		"mọi nhân chứng đều nói dối — mỗi lời khai che một bí mật riêng",
		"vụ án đóng hồ sơ hai mươi năm bật mở vì một tấm ảnh cũ",
	},
	"Kinh dị": {
		"căn nhà lưu giữ ký ức của mọi người từng chết trong nó",
		"trò chơi gọi hồn làm sai một bước — và luật chơi không cho dừng",
		"ngôi làng thờ một thứ không tên, hễ gọi tên là nó nghe thấy",
		"kẻ trong gương làm mọi thứ giống hệt, chỉ chậm hơn nửa giây",
		"lời nguyền truyền qua cuộc gọi nhỡ lúc 3 giờ sáng",
	},
	"Khoa huyễn": {
		"AI thức tỉnh và học cách giả vờ chưa thức tỉnh",
		"thuộc địa ngoài hành tinh mất liên lạc, đội cứu hộ tìm thấy thành phố vẫn sáng đèn",
		"du hành thời gian một chiều — về được nhưng không ai tin, ở lại thì biến mất",
		"ký ức cấy ghép được, và có kẻ đang sống bằng ký ức của người khác",
		"vết nứt trong mô phỏng: những người 'lỗi' nhìn thấy nhau",
	},
	"Lịch sử": {
		"công thần bị nghi kỵ, chọn giữa thanh danh và mạng của ba quân",
		"mưu sĩ phò tá vị chúa yếu nhất thiên hạ vì một lời hứa cũ",
		"một trận đánh định mệnh và người lính biết trước kết cục từ giấc mộng",
		"hoàng tử bị đày nơi biên tái học lại cách làm người trước khi làm vua",
		"thương nhân dùng sổ sách và muối gạo xoay chuyển cả triều cục",
	},
	"Quân sự": {
		"đơn vị bị bỏ rơi sau phòng tuyến địch phải tự tìm đường về",
		"điệp viên hai mang không còn nhớ lòng trung đầu tiên đặt ở đâu",
		"cuộc rút lui tử thủ: một trung đội đổi thời gian cho một quân đoàn",
		"tân binh thành chỉ huy sau một đêm vì là người duy nhất còn đứng",
		"mệnh lệnh sai từ thượng tầng và cái giá của việc kháng lệnh đúng",
	},
	"Võng du": {
		"người chơi duy nhất mở ẩn nghề nghiệp bị cả server truy lùng",
		"NPC có trí khôn bắt đầu hỏi người chơi những câu không có trong kịch bản",
		"ranh giới game và hiện thực mờ dần — chết trong game, đau ngoài đời",
		"chiến đội hết thời gom những kẻ bị loại thi đấu lại từ giải rác",
		"một bug không ai sửa được trở thành năng lực độc nhất của kẻ yếu nhất",
	},
	"Thể thao": {
		"thiên tài chấn thương trở lại từ con số âm — cơ thể cũ, bản năng mới",
		"đội bét bảng lột xác dưới tay huấn luyện viên bị cả giới ruồng bỏ",
		"đối thủ định mệnh: thắng người đó mới được quyền thắng chính mình",
		"tài năng nở muộn tuổi ba mươi đánh cược tất cả cho một mùa giải",
		"trận cuối của huyền thoại — và bí mật y giấu suốt sự nghiệp",
	},
	"Đồng nhân": {
		"nhân vật quần chúng biết trước cốt truyện quyết sống thật xa vai chính",
		"cốt truyện gốc bắt đầu lệch từ chương một — kiến thức tủ thành giấy lộn",
		"phản diện được viết lại: đứng ở góc nhìn của kẻ thua toàn cuộc",
		"xuyên vào thế giới lai ghép nhiều tác phẩm, quy tắc chồng chéo nhau",
	},
	"Hài hước": {
		"một hiểu lầm nhỏ tuyết lăn thành chuỗi hiểu lầm cấp thành phố",
		"kẻ xui tận mạng bỗng gặp may — nhưng may mắn toàn đến sai thời điểm",
		"nghề nghiệp kỳ dị nhất trần đời và khách hàng còn kỳ dị hơn",
		"đại gia đình lắm chuyện họp mặt, mỗi người mang theo một bí mật dở khóc dở cười",
		"thần tiên thất nghiệp xuống trần làm đủ nghề để đủ KPI công đức",
	},
	"Chính kịch": {
		"gia sản chia ba, tình thân chia bảy — bản di chúc có hai phiên bản",
		"đám tang phơi bày bí mật ba thế hệ cùng chôn giấu",
		"lý tưởng thời trẻ va vào hiện thực trung niên, vỡ hay uốn",
		"hai thế hệ yêu nhau bằng hai ngôn ngữ không phiên dịch được",
		"cái giá của im lặng: người tốt đứng nhìn và mọi chuyện xảy ra",
	},
	"Slice-of-life": {
		"quán ăn khuya nơi mỗi vị khách để quên lại một câu chuyện",
		"một năm ở thị trấn ven biển, sống chậm để nghe được điều gì đó",
		"những bức thư viết mà không gửi cho người đã khuất",
		"người hàng xóm kỳ lạ và khu vườn chỉ nở hoa ban đêm",
		"mùa hè cuối cùng trước khi cả đám bạn tản ra bốn phương",
	},
	"Dã sử": {
		"dật sử về nhân vật bị chính sử chép vỏn vẹn một dòng",
		"báu vật tiền triều tái xuất kéo theo hậu duệ ba phe tranh đoạt",
		"hậu duệ vương triều mai danh ẩn tích giữa chợ đời",
		"kỳ án chốn cung đình mà lời giải nằm trong một bài đồng dao",
		"giai thoại dân gian kể ngược hoàn toàn với sự thật — và cả hai đều đúng một nửa",
	},
}

// tropeGeneric dùng khi thể loại không có trong matrix.
var tropeGeneric = []string{
	"kẻ vô danh giữ một bí mật đủ làm nghiêng thế cục",
	"món nợ ân tình cũ đến hạn phải trả bằng thứ quý nhất",
	"hai kẻ đối nghịch buộc phải chung thuyền qua cơn bão lớn",
	"một lời hứa thời trẻ dại trở thành xiềng xích lẫn la bàn",
}

// ---------- (f) NHÂN VẬT ----------

var charFlaws = []string{
	"kiêu ngạo để che một nỗi tự ti không dám gọi tên",
	"không tin bất kỳ ai, kể cả người đã cứu mình",
	"nói dối trơn tru như thở — kể cả khi không cần",
	"sợ mất mát tới mức không dám nhận điều tốt đẹp",
	"nóng vội, luôn hành động trước khi nghĩ hết nước cờ",
	"cầu toàn tới tê liệt, thà không làm còn hơn làm chưa hoàn hảo",
	"mềm lòng sai chỗ, cứ dung túng đúng kẻ không nên dung túng",
	"bị quá khứ ám tới mức sống như người mắc nợ thời gian",
}

var charDesires = []string{
	"được một lần công nhận bởi chính người đã ruồng bỏ mình",
	"trả cho xong món nợ ân tình đè nặng nửa đời",
	"tìm lại người thân thất lạc trong biến cố năm xưa",
	"một chốn bình yên không ai biết tên mình",
	"đứng lên đỉnh cao nhất để không ai còn dám định đoạt thay mình",
	"đào ra sự thật bị chôn dưới mười tầng dối trá",
	"chuộc lại lỗi lầm đã khiến người vô tội trả giá",
	"tự do — dù phải đổi bằng tất cả những gì đang có",
}

var charMisbeliefs = []string{
	"tin rằng phải đủ mạnh thì mới xứng đáng được yêu thương",
	"tin rằng mình không xứng đáng với bất cứ điều tốt đẹp nào",
	"tin rằng tin người là ngu dại, cô độc mới an toàn",
	"tin rằng quá khứ định nghĩa vĩnh viễn con người mình",
	"tin rằng yêu thương nghĩa là hy sinh đến cạn kiệt",
	"tin rằng cứu cánh biện minh cho mọi phương tiện",
}

var charNames = []string{
	"Lâm Dạ", "Trần Khước", "Hàn Thanh", "Tô Vãn", "Lục Trầm", "Kỷ Vô Ưu",
	"Bạch Quan Hà", "Diệp Cô Chu", "Thẩm Trường Ca", "Ninh Thất", "Cố Tuyết Y",
	"Mạc Ly", "Giang Hồi", "Vệ Không", "Đường Thính Vũ", "Triệu Bắc Minh",
	"Hứa Chiêu", "Ôn Ngọc", "Sở Hoài An", "Nguyễn Đông Ly",
}

// ---------- (g) TWIST bank ----------

var twists = []string{
	"đồng minh thân cận nhất chính là chủ mưu đứng sau tất cả",
	"mục tiêu nhân vật chính theo đuổi cả truyện hóa ra là cái bẫy được dựng cho chính mình",
	"nhân vật chính mới là nguyên nhân gốc của thảm kịch mở đầu",
	"người tưởng đã chết chưa từng chết — và im lặng là lựa chọn của họ",
	"một phần ký ức của nhân vật chính là ký ức được cấy",
	"kẻ thù truy đuổi bấy lâu thực ra đang cố bảo vệ nhân vật chính khỏi thứ tệ hơn",
	"lời tiên tri bị dịch sai một chữ, đổi trắng thay đen toàn cục",
	"hai phe đối đầu đều là quân cờ của thế lực thứ ba chưa lộ mặt",
	"mỗi lần dùng sức mạnh, người trả giá không phải mình mà là người thân",
	"danh tính thật của nhân vật chính là điều cả hai phe cùng che giấu",
	"chiến thắng đầu tiên năm xưa là một ván dàn xếp",
	"vật tầm thường mang theo từ chương một chính là chìa khóa của tất cả",
}

// ---------- (h) THEME bank ----------

var themes = []string{
	"nghiệp và quả — không món nợ nào tự xóa",
	"tự do đối đầu an toàn: cái lồng sơn son vẫn là lồng",
	"cái giá của sức mạnh — mạnh thêm một phần, người mất một phần",
	"danh tính và mặt nạ: đeo đủ lâu, mặt nạ mọc vào da",
	"tha thứ hay báo thù — vết thương nào cũng đòi được lên tiếng",
	"lòng trung và phản bội chỉ cách nhau một lần không được lựa chọn",
	"ký ức làm nên con người: quên đi có còn là mình",
	"công lý và luật lệ không phải lúc nào cũng cùng phe",
	"những hy sinh thầm lặng không ai chép vào sử",
	"tham vọng nuốt dần nhân tính từng miếng nhỏ",
}

// ---------- (i) WORLD seed ----------

type worldDef struct{ Place, Rule string }

var worlds = []worldDef{
	{"kinh thành cổ chìm trong mưa dầm cuối triều đại", "ban đêm giới nghiêm, chỉ chức dịch cầm đèn xanh được đi lại"},
	{"thành phố ven biển hiện đại mùa bão về", "cứ sau mỗi cơn bão, thành phố lại 'quên' một chuyện đáng lẽ phải nhớ"},
	{"tông môn cheo leo trên núi tuyết quanh năm không tan", "linh khí đang cạn dần theo từng mùa, ai cũng biết nhưng không ai nói"},
	{"trạm không gian ở rìa hệ mặt trời", "tín hiệu về trái đất trễ bốn giờ — mọi quyết định sống còn đều phải tự quyết"},
	{"thị trấn vùng cao nơi mùa đông đêm dài hơn ngày", "có một con đường mà bản đồ nào vẽ vào cũng bị lệch"},
	{"đại mạc thương lộ nối hai đế quốc", "nước quý hơn vàng, và lời hứa giữa sa mạc có giá bằng mạng"},
	{"học viện khép kín trên đảo, mỗi năm chỉ một chuyến tàu", "điểm số quy đổi ra đặc quyền, và bảng xếp hạng chưa từng sai — cho tới nay"},
	{"khu chung cư cũ sắp giải tỏa giữa lòng đô thị", "cư dân đồn rằng tầng thượng khóa kín có người ở, dù danh sách hộ khẩu không có ai"},
	{"chiến trường biên ải nơi hai đế quốc giằng co ba thập kỷ", "mùa đông hai bên tự ngừng bắn — một luật bất thành văn không ai dám phá"},
	{"làng chài quanh năm sương mù không thấy mặt trời", "thuyền ra khơi phải treo chuông; thuyền về mà chuông câm là điềm dữ"},
	{"đô thị ngầm nhiều tầng dưới lòng đất sau đại nạn", "càng ở tầng sâu càng nghèo, ánh sáng mặt trời bán theo phút"},
	{"vương quốc nổi trên biển mây, di chuyển theo mùa gió", "rơi xuống dưới mây là cấm kỵ — nhưng dưới mây có tiếng người vọng lên"},
}

// ---------- (k) PACING map ----------

var pacings = []string{
	"mở chậm rãi tích khí, midpoint bẻ lái, 30% cuối tăng tốc dồn dập tới cao trào kép",
	"vào truyện dồn dập ngay từ chương một, giữa truyện hạ nhịp đào tâm lý, kết bùng nổ ngắn và dứt khoát",
	"nhịp sóng: mỗi 3-4 chương một đỉnh nhỏ, đỉnh sau cao hơn đỉnh trước, cao trào cuối gộp mọi tuyến",
	"xen kẽ chương nhanh (hành động) và chương chậm (dư âm), tương phản nhịp làm cả hai sắc hơn",
	"nửa đầu cài cắm chậm mà chắc, từ midpoint trở đi mỗi chương lật một lá bài đã cài",
}

// ---------- Title generator ----------

var titleAdjs = []string{"Tàn", "Cô", "Vô Danh", "Trầm Mặc", "Bất Diệt", "Lạc", "U Minh", "Phù Sinh", "Cuối Cùng", "Nghịch"}
var titleNouns = []string{"Kiếm", "Đêm", "Tuyết", "Hồi Ức", "Vực Sâu", "Ánh Lửa", "Cửu Thiên", "Trường An", "Bến Mê", "Cô Thành", "Mộng", "Tàn Tinh", "Hải Đăng", "Phong Ấn", "Dư Âm", "Thiên Mệnh"}

var titlePatterns = []func(r *rand.Rand) string{
	func(r *rand.Rand) string { return pick(r, titleNouns) + " " + pick(r, titleAdjs) },
	func(r *rand.Rand) string { return "Kẻ Gác " + pick(r, titleNouns) },
	func(r *rand.Rand) string { return "Trở Về Từ " + pick(r, titleNouns) },
	func(r *rand.Rand) string { return pick(r, titleNouns) + " Dưới " + pick(r, titleNouns) },
	func(r *rand.Rand) string { return "Người Cuối Cùng Của " + pick(r, titleNouns) },
	func(r *rand.Rand) string { return pick(r, titleAdjs) + " " + pick(r, titleNouns) + " Ký" },
	func(r *rand.Rand) string { return "Đêm Trước " + pick(r, titleNouns) },
	func(r *rand.Rand) string { return pick(r, titleNouns) + " Không Ngủ" },
}

func pick[T any](r *rand.Rand, arr []T) T { return arr[r.Intn(len(arr))] }

// ---------- GenerateSeed ----------

// GenerateSeed sinh SeedSpec từ thể loại đã chọn. rng do caller cấp (seed thời gian).
func GenerateSeed(genres []string, rng *rand.Rand) SeedSpec {
	if len(genres) == 0 {
		genres = []string{"Huyền huyễn"}
	}
	// trope: gộp trope của mọi thể loại đã chọn, thể loại đầu trọng số gấp đôi
	var pool []string
	for i, g := range genres {
		ts, ok := tropeMatrix[g]
		if !ok {
			ts = tropeGeneric
		}
		pool = append(pool, ts...)
		if i == 0 {
			pool = append(pool, ts...) // trọng số x2 cho thể loại chính
		}
	}
	trope := pick(rng, pool)
	conflict := pick(rng, conflicts)
	twist := pick(rng, twists)
	hook := pick(rng, hooks)
	mood := pick(rng, moods)
	theme := pick(rng, themes)
	structure := pick(rng, structures)
	pacing := pick(rng, pacings)
	world := pick(rng, worlds)
	name := pick(rng, charNames)
	flaw := pick(rng, charFlaws)
	desire := pick(rng, charDesires)
	misbelief := pick(rng, charMisbeliefs)
	title := titlePatterns[rng.Intn(len(titlePatterns))](rng)

	logline := buildLogline(rng, name, trope, conflict.Name, desire)
	seeding := buildSeeding(rng, genres, name, trope, conflict, twist, hook, mood, theme, world, flaw, desire, misbelief)
	structNote := fmt.Sprintf("Cấu trúc: %s — %s Nhịp truyện: %s. Hook mở màn kiểu %q: %s",
		structure.Name, structure.Note, pacing, hook.Name, hook.Guide)

	return SeedSpec{
		Genres: genres, Title: title, Logline: logline, Seeding: seeding,
		StructureNote: structNote,
		Trope:         trope, Conflict: conflict.Name, Twist: twist,
		Hook: hook.Name, Mood: mood, Theme: theme,
	}
}

var loglineTpls = []string{
	"Khi %s, %s buộc phải đối mặt với xung đột %s để %s.",
	"%s — và %s, kẻ vốn chỉ muốn %s, bị cuốn vào cuộc chiến %s không đường lui.",
	"Giữa cục diện %q, %s phát hiện con đường duy nhất để %s phải đi xuyên qua chính điều mình sợ nhất.",
	"%s. %s đứng giữa lằn ranh %s, nơi mọi lựa chọn đều có giá — kể cả việc %s.",
}

func buildLogline(rng *rand.Rand, name, trope, conflictName, desire string) string {
	switch rng.Intn(4) {
	case 0:
		return fmt.Sprintf(loglineTpls[0], trope, name, conflictName, desire)
	case 1:
		return fmt.Sprintf(loglineTpls[1], upperFirst(trope), name, desire, conflictName)
	case 2:
		return fmt.Sprintf(loglineTpls[2], trope, name, desire)
	default:
		return fmt.Sprintf(loglineTpls[3], upperFirst(trope), name, conflictName, desire)
	}
}

func upperFirst(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	return strings.ToUpper(string(r[0])) + string(r[1:])
}

func buildSeeding(rng *rand.Rand, genres []string, name, trope string, conflict conflictDef,
	twist string, hook hookDef, mood, theme string, world worldDef, flaw, desire, misbelief string) string {

	var sb strings.Builder

	// 1. Bối cảnh + premise
	openTpls := []string{
		"Bối cảnh đặt tại %s. Quy tắc đặc biệt của thế giới này: %s. Trên nền đó, câu chuyện xoay quanh %s.",
		"Thế giới truyện: %s — nơi %s. Chính giữa vùng đất ấy, %s.",
		"Lấy bối cảnh %s. Điều khiến nơi này khác mọi nơi: %s. Và từ kẽ nứt đó nảy ra chuyện %s.",
	}
	fmt.Fprintf(&sb, pick(rng, openTpls), world.Place, world.Rule, trope)
	sb.WriteString("\n\n")

	// 2. Nhân vật chính
	charTpls := []string{
		"Nhân vật chính là %s — kẻ %s. Sâu bên trong, điều %s khao khát nhất là %s, nhưng %s, và chính niềm tin sai lệch ấy sẽ bị câu chuyện bóc trần từng lớp.",
		"%s là trung tâm câu chuyện: %s. Khao khát cháy bỏng của %s: %s. Rào cản lớn nhất không nằm ngoài kia — %s.",
		"Trung tâm là %s, người mang khuyết điểm chí mạng: %s. Điều %s muốn: %s. Điều trói chân: %s.",
	}
	fmt.Fprintf(&sb, pick(rng, charTpls), name, flaw, name, desire, misbelief)
	sb.WriteString("\n\n")

	// 3. Xung đột + stakes
	confTpls := []string{
		"Xung đột lõi thuộc dạng %s: %s. Cái giá đặt cược tăng dần theo từng hồi — thua không chỉ mất mục tiêu, mà mất luôn lý do để bước tiếp.",
		"Trục xung đột chính: %s (%s). Mỗi bước tiến gần khao khát lại đẩy nhân vật xa thêm khỏi con người mình muốn giữ.",
	}
	fmt.Fprintf(&sb, pick(rng, confTpls), conflict.Name, conflict.Desc)
	sb.WriteString("\n\n")

	// 4. Hook mở màn
	hookTpls := []string{
		"Chương mở màn dùng hook %q: %s",
		"Cách vào truyện: %s — %s",
	}
	fmt.Fprintf(&sb, pick(rng, hookTpls), hook.Name, hook.Guide)
	sb.WriteString("\n\n")

	// 5. Twist hứa hẹn
	twistTpls := []string{
		"Twist chốt được cài từ sớm và chỉ nổ ở hồi cuối: %s. Mọi manh mối phải công bằng — đọc lại lần hai thấy tất cả đã bày trước mắt.",
		"Lá bài giấu của truyện: %s. Cài ít nhất ba manh mối rải ở ba hồi, mỗi manh mối ngụy trang bằng một cảnh tưởng như vô thưởng vô phạt.",
	}
	fmt.Fprintf(&sb, pick(rng, twistTpls), twist)
	sb.WriteString("\n\n")

	// 6. Mood + theme
	moodTpls := []string{
		"Không khí chủ đạo: %s. Chủ đề ngầm chảy suốt truyện: %s. Thể loại: %s.",
		"Tông truyện: %s. Sợi chỉ đỏ xuyên suốt: %s. Khung thể loại: %s.",
	}
	fmt.Fprintf(&sb, pick(rng, moodTpls), mood, theme, strings.Join(genres, " + "))

	return sb.String()
}

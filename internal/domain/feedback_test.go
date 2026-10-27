package domain

import (
	"encoding/json"
	"testing"
)

// TestOutlineFeedback_Unmarshal xác minh UnmarshalJSON khoan dung nhận cả 3 dạng đầu vào,
// tránh InputValidationError làm hỏng commit_chapter khi model trả feedback dạng string.
func TestOutlineFeedback_Unmarshal(t *testing.T) {
	cases := []struct {
		name       string
		in         string
		deviation  string
		suggestion string
	}{
		{
			name:       "real object",
			in:         `{"deviation":"có lệch nhẹ","suggestion":"chương sau đẩy nhanh"}`,
			deviation:  "có lệch nhẹ",
			suggestion: "chương sau đẩy nhanh",
		},
		{
			name:       "json-encoded string of object",
			in:         `"{\"deviation\":\"lệch A\",\"suggestion\":\"sửa B\"}"`,
			deviation:  "lệch A",
			suggestion: "sửa B",
		},
		{
			name:       "plain prose string",
			in:         `"Chương bám sát đề cương, gợi ý đẩy tình cảm sớm hơn."`,
			suggestion: "Chương bám sát đề cương, gợi ý đẩy tình cảm sớm hơn.",
		},
		{
			name: "null",
			in:   `null`,
		},
		{
			name: "empty string",
			in:   `""`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var f OutlineFeedback
			if err := json.Unmarshal([]byte(c.in), &f); err != nil {
				t.Fatalf("unmarshal should not error, got: %v", err)
			}
			if f.Deviation != c.deviation {
				t.Errorf("deviation: got %q want %q", f.Deviation, c.deviation)
			}
			if f.Suggestion != c.suggestion {
				t.Errorf("suggestion: got %q want %q", f.Suggestion, c.suggestion)
			}
		})
	}
}

// TestOutlineFeedback_InStructContext xác minh feedback hoạt động khi là field con của commit args.
func TestOutlineFeedback_InStructContext(t *testing.T) {
	type commitArgs struct {
		Chapter  int              `json:"chapter"`
		Feedback *OutlineFeedback `json:"feedback"`
	}
	// Model trả feedback dạng JSON-encoded string — đây chính là ca gây lỗi thực tế.
	raw := `{"chapter":1,"feedback":"{\"deviation\":\"không lệch\",\"suggestion\":\"giữ nhịp\"}"}`
	var a commitArgs
	if err := json.Unmarshal([]byte(raw), &a); err != nil {
		t.Fatalf("commit args unmarshal should tolerate string feedback: %v", err)
	}
	if a.Feedback == nil || a.Feedback.Suggestion != "giữ nhịp" {
		t.Errorf("feedback not parsed correctly: %+v", a.Feedback)
	}
}

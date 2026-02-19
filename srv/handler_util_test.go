package srv

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTimeAgo(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name  string
		input *time.Time
		want  string
	}{
		{"nil", nil, ""},
		{"seconds", new(now.Add(-30 * time.Second)), "30 sec ago"},
		{"1 min", new(now.Add(-1 * time.Minute)), "1 min ago"},
		{"minutes", new(now.Add(-5 * time.Minute)), "5 min ago"},
		{"1 hr", new(now.Add(-1 * time.Hour)), "1 hr ago"},
		{"hours", new(now.Add(-3 * time.Hour)), "3 hr ago"},
		{"1 day", new(now.Add(-24 * time.Hour)), "1 day ago"},
		{"days", new(now.Add(-2 * 24 * time.Hour)), "2 days ago"},
		{"old", new(now.Add(-30 * 24 * time.Hour)), now.Add(-30 * 24 * time.Hour).Format("Jan 2, 2006")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := timeAgo(tt.input)
			if got != tt.want {
				t.Errorf("timeAgo = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatDate(t *testing.T) {
	d := time.Date(2026, 1, 15, 14, 30, 0, 0, time.UTC)
	got := formatDate(&d)
	want := "2026-01-15T14:30:00Z"
	if got != want {
		t.Errorf("formatDate = %q, want %q", got, want)
	}
	if formatDate(nil) != "" {
		t.Error("formatDate(nil) should be empty")
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input string
		max   int
		want  string
	}{
		{"short", 10, "short"},
		{"hello world this is long", 10, "hello worl..."},
		{"", 5, ""},
	}
	for _, tt := range tests {
		got := truncate(tt.input, tt.max)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.max, got, tt.want)
		}
	}
}

func TestStripHTML(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"<p>Hello <b>world</b></p>", "Hello world"},
		{"no tags here", "no tags here"},
		{"", ""},
		{"<img src='x'/>text", "text"},
	}
	for _, tt := range tests {
		got := stripHTML(tt.input)
		if got != tt.want {
			t.Errorf("stripHTML(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestDeref(t *testing.T) {
	s := "hello"
	var ns *string
	i := int64(42)
	var ni *int64
	now := time.Now()
	var nt *time.Time

	tests := []struct {
		name string
		in   any
		want any
	}{
		{"string ptr", &s, "hello"},
		{"nil string ptr", ns, ""},
		{"int64 ptr", &i, int64(42)},
		{"nil int64 ptr", ni, 0},
		{"time ptr", &now, now},
		{"nil time ptr", nt, nil},
		{"nil", nil, ""},
		{"plain string", "direct", "direct"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deref(tt.in)
			if got != tt.want {
				t.Errorf("deref(%v) = %v (%T), want %v (%T)", tt.in, got, got, tt.want, tt.want)
			}
		})
	}
}

// TestFaviconURL and TestFaviconDomain are in favicon_test.go

func TestConvertSteamNewsURL(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"https://store.steampowered.com/news/app/4115450", "https://store.steampowered.com/feeds/news/app/4115450"},
		{"https://store.steampowered.com/news/app/4115450/", "https://store.steampowered.com/feeds/news/app/4115450"},
		{"https://example.com/feed", "https://example.com/feed"},
		{"", ""},
	}
	for _, tt := range tests {
		got := convertSteamNewsURL(tt.input)
		if got != tt.want {
			t.Errorf("convertSteamNewsURL(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestStripLeadingImage(t *testing.T) {
	tests := []struct {
		name, content, imageURL, want string
	}{
		{"strips matching img", `<img src="https://example.com/img.jpg">Hello`, "https://example.com/img.jpg", "Hello"},
		{"keeps non-matching img", `<img src="https://other.com/img.jpg">Hello`, "https://example.com/img.jpg", `<img src="https://other.com/img.jpg">Hello`},
		{"no img tag", "Hello world", "https://example.com/img.jpg", "Hello world"},
		{"empty image url", `<img src="x">Hello`, "", `<img src="x">Hello`},
		{"empty content", "", "https://example.com/img.jpg", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripLeadingImage(tt.content, tt.imageURL)
			if got != tt.want {
				t.Errorf("stripLeadingImage = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSafeHTML(t *testing.T) {
	got := safeHTML("<b>bold</b>")
	if string(got) != "<b>bold</b>" {
		t.Errorf("safeHTML = %q", got)
	}
}

func TestParseCursor(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		wantOK   bool
		wantID   int64
		wantYear int // 0 = don't check
	}{
		{"valid", "?before_time=2026-01-15T14:30:00Z&before_id=42", true, 42, 2026},
		{"nano", "?before_time=2026-01-15T14:30:00.123456789Z&before_id=7", true, 7, 2026},
		{"missing time", "?before_id=42", false, 0, 0},
		{"missing id", "?before_time=2026-01-15T14:30:00Z", false, 0, 0},
		{"bad time", "?before_time=not-a-time&before_id=42", false, 0, 0},
		{"bad id", "?before_time=2026-01-15T14:30:00Z&before_id=abc", false, 0, 0},
		{"both empty", "", false, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/test"+tt.query, http.NoBody)
			bt, id, ok := parseCursor(r)
			if ok != tt.wantOK {
				t.Fatalf("parseCursor ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if id != tt.wantID {
				t.Errorf("before_id = %d, want %d", id, tt.wantID)
			}
			if bt == nil {
				t.Fatal("before_time is nil")
			}
			if tt.wantYear != 0 && bt.Year() != tt.wantYear {
				t.Errorf("year = %d, want %d", bt.Year(), tt.wantYear)
			}
		})
	}
}

func TestSortTime(t *testing.T) {
	pub := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
	fetch := time.Date(2026, 3, 16, 10, 0, 0, 0, time.UTC)

	// With published_at set, should use it
	got := sortTimeFunc(&pub, fetch)
	want := pub.Format(time.RFC3339Nano)
	if got != want {
		t.Errorf("sortTime(pub, fetch) = %q, want %q", got, want)
	}

	// With nil published_at, should fall back to fetched_at
	got = sortTimeFunc(nil, fetch)
	want = fetch.Format(time.RFC3339Nano)
	if got != want {
		t.Errorf("sortTime(nil, fetch) = %q, want %q", got, want)
	}
}

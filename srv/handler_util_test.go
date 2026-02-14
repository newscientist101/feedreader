package srv

import (
	"testing"
	"time"
)

func tp(t time.Time) *time.Time { return &t }

func TestTimeAgo(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name string
		input *time.Time
		want  string
	}{
		{"nil", nil, ""},
		{"seconds", tp(now.Add(-30 * time.Second)), "30 sec ago"},
		{"1 min", tp(now.Add(-1 * time.Minute)), "1 min ago"},
		{"minutes", tp(now.Add(-5 * time.Minute)), "5 min ago"},
		{"1 hr", tp(now.Add(-1 * time.Hour)), "1 hr ago"},
		{"hours", tp(now.Add(-3 * time.Hour)), "3 hr ago"},
		{"1 day", tp(now.Add(-24 * time.Hour)), "1 day ago"},
		{"days", tp(now.Add(-2 * 24 * time.Hour)), "2 days ago"},
		{"old", tp(now.Add(-30 * 24 * time.Hour)), now.Add(-30 * 24 * time.Hour).Format("Jan 2, 2006")},
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
		input  string
		max    int
		want   string
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

func TestFaviconURL(t *testing.T) {
	tests := []struct {
		name, siteURL, feedURL, wantContains string
	}{
		{"uses site url", "https://example.com", "https://feeds.example.com/rss", "domain=example.com"},
		{"falls back to feed url", "", "https://example.com/feed", "domain=example.com"},
		{"strips feeds subdomain", "", "https://feeds.example.com/rss", "domain=example.com"},
		{"strips rss subdomain", "", "https://rss.example.com/feed", "domain=example.com"},
		{"keeps normal subdomain", "", "https://blog.example.com/feed", "domain=blog.example.com"},
		{"empty urls", "", "", ""},
		{"invalid url", "", "not-a-url", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := faviconURL(tt.siteURL, tt.feedURL)
			if tt.wantContains == "" {
				if got != "" {
					t.Errorf("faviconURL = %q, want empty", got)
				}
			} else if !contains(got, tt.wantContains) {
				t.Errorf("faviconURL = %q, want to contain %q", got, tt.wantContains)
			}
		})
	}
}


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

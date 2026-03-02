package srv

import (
	"regexp"
	"testing"

	"github.com/newscientist101/feedreader/db/dbgen"
)

func TestAlertMatches_Keyword(t *testing.T) {
	tests := []struct {
		name       string
		pattern    string
		matchField string
		title      string
		summary    string
		want       bool
	}{
		{"title match", "golang", "both", "New Golang Release", "", true},
		{"summary match", "golang", "both", "", "We love golang", true},
		{"no match", "rust", "both", "New Golang Release", "We love golang", false},
		{"case insensitive", "GOLANG", "both", "new golang release", "", true},
		{"title-only field", "golang", "title", "", "golang in summary", false},
		{"title-only match", "golang", "title", "golang in title", "", true},
		{"summary-only field", "golang", "summary", "golang in title", "", false},
		{"summary-only match", "golang", "summary", "", "golang in summary", true},
		{"empty matchField defaults to both", "golang", "", "golang", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ca := &compiledAlert{
				alert: dbgen.NewsAlert{
					Pattern:    tc.pattern,
					MatchField: tc.matchField,
				},
			}
			if got := alertMatches(ca, tc.title, tc.summary); got != tc.want {
				t.Errorf("alertMatches() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestAlertMatches_Regex(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		title   string
		summary string
		want    bool
	}{
		{"regex title match", `go\d+\.\d+`, "go1.22 released", "", true},
		{"regex summary match", `go\d+\.\d+`, "", "upgrade to go1.22", true},
		{"regex no match", `go\d+\.\d+`, "golang released", "no version", false},
		{"regex case insensitive", `Go\d+`, "GO123", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ca := &compiledAlert{
				alert: dbgen.NewsAlert{
					Pattern:    tc.pattern,
					IsRegex:    1,
					MatchField: "both",
				},
				re: regexp.MustCompile("(?i)" + tc.pattern),
			}
			if got := alertMatches(ca, tc.title, tc.summary); got != tc.want {
				t.Errorf("alertMatches() = %v, want %v", got, tc.want)
			}
		})
	}
}

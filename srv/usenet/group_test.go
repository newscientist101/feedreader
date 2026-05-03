package usenet_test

import (
	"errors"
	"testing"

	"github.com/newscientist101/feedreader/srv/usenet"
)

func TestValidateGroupName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		// Valid text groups
		{"simple", "comp.lang.go", "comp.lang.go", false},
		{"with underscores", "alt.fan.dr_who", "alt.fan.dr_who", false},
		{"with hyphen", "alt.culture.us-england", "alt.culture.us-england", false},
		{"with plus", "gnu.emacs.help+info", "gnu.emacs.help+info", false},
		{"normalizes whitespace", "  Sci.Physics  ", "sci.physics", false},
		{"normalizes case", "Rec.Arts.Books", "rec.arts.books", false},
		{"single segment", "general", "general", false},
		{"digits in segment", "alt.fan.r2d2", "alt.fan.r2d2", false},

		// Invalid syntax
		{"empty", "", "", true},
		{"only spaces", "   ", "", true},
		{"leading dot", ".comp.lang.go", "", true},
		{"trailing dot", "comp.lang.go.", "", true},
		{"double dot", "comp..lang", "", true},
		{"invalid char space", "comp.lang go", "", true},
		{"invalid char at", "user@host", "", true},
		{"invalid char slash", "comp/lang", "", true},

		// Binary group rejections
		{"alt.binaries exact", "alt.binaries", "", true},
		{"alt.binaries.*", "alt.binaries.pictures", "", true},
		{"alt.binaries.deep", "alt.binaries.misc.erotica", "", true},
		{"segment binary", "comp.binary.misc", "", true},
		{"segment binaries", "comp.binaries.amiga", "", true},
		{"segment binary last", "alt.sources.binary", "", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := usenet.ValidateGroupName(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for input %q, got nil (result %q)", tc.input, got)
				}
				if !errors.Is(err, usenet.ErrInvalidGroupName) {
					t.Fatalf("expected ErrInvalidGroupName, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for input %q: %v", tc.input, err)
			}
			if got != tc.want {
				t.Fatalf("ValidateGroupName(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

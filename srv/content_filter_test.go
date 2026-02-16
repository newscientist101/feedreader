package srv

import (
	"testing"
)

// ---------------------------------------------------------------------------
// ApplyContentFilters
// ---------------------------------------------------------------------------

func TestApplyContentFilters_NilFilters(t *testing.T) {
	content := "<p>Hello</p>"
	got := ApplyContentFilters(content, nil)
	if got != content {
		t.Errorf("expected unchanged content, got %q", got)
	}
}

func TestApplyContentFilters_EmptyFilters(t *testing.T) {
	content := "<p>Hello</p>"
	empty := ""
	got := ApplyContentFilters(content, &empty)
	if got != content {
		t.Errorf("expected unchanged content, got %q", got)
	}
}

func TestApplyContentFilters_EmptyArray(t *testing.T) {
	content := "<p>Hello</p>"
	filters := "[]"
	got := ApplyContentFilters(content, &filters)
	if got != content {
		t.Errorf("expected unchanged content, got %q", got)
	}
}

func TestApplyContentFilters_RemoveByClass(t *testing.T) {
	content := `<p>Keep me</p><div class="ad">Remove me</div><p>Also keep</p>`
	filters := `[{"selector": ".ad"}]`
	got := ApplyContentFilters(content, &filters)

	if contains(got, "Remove me") {
		t.Errorf("expected .ad to be removed, got %q", got)
	}
	if !contains(got, "Keep me") || !contains(got, "Also keep") {
		t.Errorf("expected kept content to remain, got %q", got)
	}
}

func TestApplyContentFilters_RemoveByTag(t *testing.T) {
	content := `<p>Text</p><footer>Footer stuff</footer>`
	filters := `[{"selector": "footer"}]`
	got := ApplyContentFilters(content, &filters)

	if contains(got, "Footer stuff") {
		t.Errorf("expected footer removed, got %q", got)
	}
	if !contains(got, "Text") {
		t.Errorf("expected text to remain, got %q", got)
	}
}

func TestApplyContentFilters_MultipleSelectors(t *testing.T) {
	content := `<p>Content</p><div class="social">Share</div><nav>Nav</nav>`
	filters := `[{"selector": ".social"}, {"selector": "nav"}]`
	got := ApplyContentFilters(content, &filters)

	if contains(got, "Share") || contains(got, "Nav") {
		t.Errorf("expected both selectors removed, got %q", got)
	}
	if !contains(got, "Content") {
		t.Errorf("expected content to remain")
	}
}

func TestApplyContentFilters_InvalidJSON(t *testing.T) {
	content := "<p>Hello</p>"
	bad := "not json"
	got := ApplyContentFilters(content, &bad)
	// Should return original content on error
	if got != content {
		t.Errorf("expected unchanged content on bad JSON, got %q", got)
	}
}

func TestApplyContentFilters_EmptySelector(t *testing.T) {
	content := "<p>Hello</p>"
	filters := `[{"selector": ""}]`
	got := ApplyContentFilters(content, &filters)
	if !contains(got, "Hello") {
		t.Errorf("empty selector should be a no-op, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// SerializeFilters / ParseFilters roundtrip
// ---------------------------------------------------------------------------

func TestSerializeParseFilters_Roundtrip(t *testing.T) {
	selectors := []string{".ads", "footer", "#sidebar"}
	json := SerializeFilters(selectors)
	parsed := ParseFilters(json)

	if len(parsed) != len(selectors) {
		t.Fatalf("expected %d selectors, got %d", len(selectors), len(parsed))
	}
	for i, s := range selectors {
		if parsed[i] != s {
			t.Errorf("selector[%d] = %q, want %q", i, parsed[i], s)
		}
	}
}

func TestSerializeFilters_Empty(t *testing.T) {
	got := SerializeFilters(nil)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestParseFilters_Empty(t *testing.T) {
	got := ParseFilters("")
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestParseFilters_InvalidJSON(t *testing.T) {
	got := ParseFilters("not json")
	if got != nil {
		t.Errorf("expected nil on invalid JSON, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// ValidateSelector
// ---------------------------------------------------------------------------

func TestValidateSelector(t *testing.T) {
	tests := []struct {
		sel  string
		want bool
	}{
		{".class", true},
		{"#id", true},
		{"div > p", true},
		{"div.foo", true},
		{"[data-x]", true},
	}
	for _, tc := range tests {
		t.Run(tc.sel, func(t *testing.T) {
			got := ValidateSelector(tc.sel)
			if got != tc.want {
				t.Errorf("ValidateSelector(%q) = %v, want %v", tc.sel, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// helper
// ---------------------------------------------------------------------------

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || substr == "" ||
		(s != "" && substr != "" && containsStr(s, substr)))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

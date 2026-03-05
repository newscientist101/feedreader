package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseImports(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.js")

	content := `import { api } from './modules/api.js';
import { utils } from './utils';
from './icons.js';
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got := parseImports(path)
	want := []string{"api.js", "utils.js", "icons.js"}

	if len(got) != len(want) {
		t.Fatalf("got %d imports, want %d: %v", len(got), len(want), got)
	}
	for i, g := range got {
		if g != want[i] {
			t.Errorf("import[%d] = %q, want %q", i, g, want[i])
		}
	}
}

func TestMaxDepth(t *testing.T) {
	graph := map[string][]string{
		"app.js": {"a.js", "b.js"},
		"a.js":   {"c.js"},
		"b.js":   {"c.js"},
		"c.js":   {"d.js"},
		"d.js":   nil,
	}

	got := maxDepth("app.js", graph)
	if got != 3 {
		t.Errorf("maxDepth = %d, want 3", got)
	}
}

func TestMaxDepthCycle(t *testing.T) {
	graph := map[string][]string{
		"app.js": {"a.js"},
		"a.js":   {"b.js"},
		"b.js":   {"a.js"}, // cycle
	}

	got := maxDepth("app.js", graph)
	if got != 2 {
		t.Errorf("maxDepth = %d, want 2", got)
	}
}

func TestFmtSize(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{500_000, "500 KB"},
		{1_000_000, "1.0 MB"},
		{1_500_000, "1.5 MB"},
	}
	for _, tt := range tests {
		got := fmtSize(tt.input)
		if got != tt.want {
			t.Errorf("fmtSize(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

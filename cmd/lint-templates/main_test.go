package main

import (
	"strings"
	"testing"
)

func TestCheckHTMLString_Valid(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{
			"simple balanced",
			`{{define "content"}}<div><p>Hello</p></div>{{end}}`,
		},
		{
			"void elements",
			`{{define "content"}}<div><br><img src="x"><input type="text"></div>{{end}}`,
		},
		{
			"nested tags",
			`{{define "content"}}<div><ul><li>A</li><li>B</li></ul></div>{{end}}`,
		},
		{
			"template actions in attributes",
			`{{define "content"}}<div class="{{.Class}}"><a href="{{.URL}}">link</a></div>{{end}}`,
		},
		{
			"trimming whitespace actions",
			`{{- define "content" -}}<div>{{- .Foo -}}</div>{{- end -}}`,
		},
		{
			"full HTML document",
			"<!DOCTYPE html><html><head><title>T</title></head><body><p>Hi</p></body></html>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			problems := checkHTMLString("test.html", tt.src)
			if len(problems) > 0 {
				t.Errorf("expected no problems, got: %v", problems)
			}
		})
	}
}

func TestCheckHTMLString_UnclosedTag(t *testing.T) {
	src := `{{define "content"}}<div><span>Hello</div>{{end}}`
	problems := checkHTMLString("test.html", src)
	if len(problems) == 0 {
		t.Fatal("expected problems for unclosed <span>")
	}
	found := false
	for _, p := range problems {
		if strings.Contains(p, "unclosed <span>") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected unclosed <span> problem, got: %v", problems)
	}
}

func TestCheckHTMLString_MismatchedTags(t *testing.T) {
	src := `{{define "content"}}<div></span>{{end}}`
	problems := checkHTMLString("test.html", src)
	if len(problems) == 0 {
		t.Fatal("expected problems for mismatched tags")
	}
	found := false
	for _, p := range problems {
		if strings.Contains(p, "</span>") && strings.Contains(p, "without matching") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected mismatched tag problem, got: %v", problems)
	}
}

func TestCheckHTMLString_VoidClosingTag(t *testing.T) {
	src := `{{define "content"}}<div><br></br></div>{{end}}`
	problems := checkHTMLString("test.html", src)
	if len(problems) == 0 {
		t.Fatal("expected problem for </br>")
	}
	found := false
	for _, p := range problems {
		if strings.Contains(p, "void element </br>") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected void element problem, got: %v", problems)
	}
}

func TestCheckHTMLString_MismatchedDelimiters(t *testing.T) {
	// One extra {{ without matching }}
	src := "{{define \"content\"}}{{ <div></div>{{end}}"
	problems := checkHTMLString("test.html", src)
	if len(problems) == 0 {
		t.Fatal("expected problem for mismatched delimiters")
	}
	found := false
	for _, p := range problems {
		if strings.Contains(p, "mismatched template delimiters") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected delimiter mismatch problem, got: %v", problems)
	}
}

func TestCheckHTMLString_UnclosedAtEndOfFile(t *testing.T) {
	// In a partial (with {{define}}), the wrapper adds </body></html>,
	// so the unclosed <div> is reported as closed-by-</body>.
	src := `{{define "content"}}<div><p>text</p>{{end}}`
	problems := checkHTMLString("test.html", src)
	if len(problems) == 0 {
		t.Fatal("expected problem for unclosed <div>")
	}
	found := false
	for _, p := range problems {
		if strings.Contains(p, "unclosed <div>") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected unclosed <div> problem, got: %v", problems)
	}
}

func TestCheckHTMLString_UnclosedAtEndOfDocument(t *testing.T) {
	// A full document (<!DOCTYPE) doesn't get wrapped, so truly
	// unclosed tags are reported as "at end of file".
	src := "<!DOCTYPE html><html><body><div><p>text</p></body></html>"
	problems := checkHTMLString("test.html", src)
	if len(problems) == 0 {
		t.Fatal("expected problem for unclosed <div>")
	}
	found := false
	for _, p := range problems {
		if strings.Contains(p, "unclosed <div>") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected unclosed <div> problem, got: %v", problems)
	}
}

func TestCheckTagBalance_Clean(t *testing.T) {
	src := "<html><body><div><p>Hi</p></div></body></html>"
	problems := checkTagBalance("test.html", src)
	if len(problems) > 0 {
		t.Errorf("expected no problems, got: %v", problems)
	}
}

func TestCheckTagBalance_MultipleUnclosed(t *testing.T) {
	src := "<html><body><div><span><p>text</p></body></html>"
	problems := checkTagBalance("test.html", src)
	// Should detect unclosed <span> and <div> (closed by </body>)
	if len(problems) == 0 {
		t.Fatal("expected problems for multiple unclosed tags")
	}
	var hasSpan, hasDiv bool
	for _, p := range problems {
		if strings.Contains(p, "<span>") {
			hasSpan = true
		}
		if strings.Contains(p, "<div>") {
			hasDiv = true
		}
	}
	if !hasSpan || !hasDiv {
		t.Errorf("expected both <span> and <div> unclosed, got: %v", problems)
	}
}

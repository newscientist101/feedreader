package srv

import (
	"strings"
	"testing"
)

func TestSanitizeHTML_StripsScript(t *testing.T) {
	input := `<p>Hello</p><script>alert('xss')</script>`
	got := string(sanitizeHTML(input))
	if strings.Contains(got, "<script") {
		t.Errorf("expected script to be stripped, got: %s", got)
	}
	if !strings.Contains(got, "<p>Hello</p>") {
		t.Errorf("expected <p>Hello</p> to be preserved, got: %s", got)
	}
}

func TestSanitizeHTML_StripsEventHandlers(t *testing.T) {
	input := `<img src="pic.jpg" onerror="alert('xss')">`
	got := string(sanitizeHTML(input))
	if strings.Contains(got, "onerror") {
		t.Errorf("expected onerror to be stripped, got: %s", got)
	}
	if !strings.Contains(got, "<img") {
		t.Errorf("expected img tag to be preserved, got: %s", got)
	}
}

func TestSanitizeHTML_StripsJavascriptURL(t *testing.T) {
	input := `<a href="javascript:alert('xss')">click</a>`
	got := string(sanitizeHTML(input))
	if strings.Contains(got, "javascript:") {
		t.Errorf("expected javascript: URL to be stripped, got: %s", got)
	}
}

func TestSanitizeHTML_PreservesFormatting(t *testing.T) {
	input := `<h1>Title</h1><p>Text with <strong>bold</strong> and <em>italic</em></p><ul><li>Item</li></ul>`
	got := string(sanitizeHTML(input))
	for _, tag := range []string{"<h1>", "<p>", "<strong>", "<em>", "<ul>", "<li>"} {
		if !strings.Contains(got, tag) {
			t.Errorf("expected %s to be preserved, got: %s", tag, got)
		}
	}
}

func TestSanitizeHTML_AllowsImages(t *testing.T) {
	input := `<img src="https://example.com/img.jpg" alt="test" width="100" height="50">`
	got := string(sanitizeHTML(input))
	if !strings.Contains(got, `src="https://example.com/img.jpg"`) {
		t.Errorf("expected img src to be preserved, got: %s", got)
	}
	if !strings.Contains(got, `alt="test"`) {
		t.Errorf("expected alt to be preserved, got: %s", got)
	}
}

func TestSanitizeHTML_AllowsIframes(t *testing.T) {
	input := `<iframe src="https://www.youtube.com/embed/abc" allowfullscreen></iframe>`
	got := string(sanitizeHTML(input))
	if !strings.Contains(got, "<iframe") {
		t.Errorf("expected iframe to be preserved, got: %s", got)
	}
}

func TestSanitizeHTML_StripsFormElements(t *testing.T) {
	input := `<form action="/steal"><input type="text"><button>Submit</button></form>`
	got := string(sanitizeHTML(input))
	if strings.Contains(got, "<form") {
		t.Errorf("expected form to be stripped, got: %s", got)
	}
	if strings.Contains(got, "<input") {
		t.Errorf("expected input to be stripped, got: %s", got)
	}
}

// Command lint-templates validates Go html/template files.
//
// It checks that every non-base template parses successfully together with
// base.html and reports common HTML issues such as mismatched tags.
package main

import (
	"errors"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/net/html"
)

// stubFuncMap mirrors the FuncMap registered in srv/server.go so that
// templates parse without "function … not defined" errors.  The function
// bodies are irrelevant — only the names matter for parsing.
var stubFuncMap = template.FuncMap{
	"timeAgo":           func() string { return "" },
	"formatDate":        func() string { return "" },
	"truncate":          func() string { return "" },
	"stripHTML":         func() string { return "" },
	"previewText":       func() string { return "" },
	"deref":             func() string { return "" },
	"safeHTML":          func() template.HTML { return "" },
	"toJSON":            func() template.JS { return "" },
	"stripLeadingImage": func() string { return "" },
	"multiply":          func() int { return 0 },
	"faviconURL":        func() string { return "" },
	"staticPath":        func() string { return "" },
	"dict":              func() map[string]any { return nil },
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: lint-templates <templates-dir>\n")
		os.Exit(1)
	}
	dir := os.Args[1]

	basePath := filepath.Join(dir, "base.html")
	if _, err := os.Stat(basePath); err != nil {
		fmt.Fprintf(os.Stderr, "base.html not found in %s\n", dir)
		os.Exit(1)
	}

	files, err := filepath.Glob(filepath.Join(dir, "*.html"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "glob: %v\n", err)
		os.Exit(1)
	}
	sort.Strings(files)

	var problems []string

	for _, f := range files {
		name := filepath.Base(f)

		// Parse check: every template must parse with base.html.
		if name != "base.html" {
			_, parseErr := template.New("base.html").Funcs(stubFuncMap).ParseFiles(basePath, f)
			if parseErr != nil {
				problems = append(problems, fmt.Sprintf("%s: template parse error: %v", name, parseErr))
				continue // can't do HTML checks on broken template
			}
		}

		// HTML well-formedness checks on the raw source.
		problems = append(problems, checkHTML(f)...)
	}

	if len(problems) > 0 {
		fmt.Fprintf(os.Stderr, "Template lint found %d problem(s):\n", len(problems))
		for _, p := range problems {
			fmt.Fprintf(os.Stderr, "  %s\n", p)
		}
		os.Exit(1)
	}

	fmt.Printf("All %d template(s) OK\n", len(files))
}

// templateActionRe matches Go template actions: {{ ... }}
var templateActionRE = regexp.MustCompile(`\{\{-?\s*.*?\s*-?\}\}`)

// checkHTML performs HTML well-formedness checks on a template file.
// It strips template actions first so the HTML parser isn't confused by them.
func checkHTML(path string) []string {
	name := filepath.Base(path)
	raw, err := os.ReadFile(path)
	if err != nil {
		return []string{fmt.Sprintf("%s: read error: %v", name, err)}
	}
	src := string(raw)

	var problems []string

	// --- Check 1: Unbalanced template actions ({{ without }}) ---
	openCount := strings.Count(src, "{{")
	closeCount := strings.Count(src, "}}")
	if openCount != closeCount {
		problems = append(problems, fmt.Sprintf("%s: mismatched template delimiters: %d opening '{{' vs %d closing '}}'", name, openCount, closeCount))
	}

	// --- Check 2: HTML tag balance ---
	// Replace template actions with empty strings so the HTML parser sees
	// clean HTML.  Also replace template actions inside attribute values.
	cleaned := templateActionRE.ReplaceAllString(src, "")

	// For partial templates (those with {{define ...}}), wrap in a minimal
	// HTML document so the parser doesn't complain about fragments.
	if strings.Contains(src, "{{define") && !strings.Contains(src, "<!DOCTYPE") {
		cleaned = "<html><body>" + cleaned + "</body></html>"
	}

	tagProblems := checkTagBalance(name, cleaned)
	problems = append(problems, tagProblems...)

	return problems
}

// voidElements are HTML elements that must not have a closing tag.
var voidElements = map[string]bool{
	"area": true, "base": true, "br": true, "col": true,
	"embed": true, "hr": true, "img": true, "input": true,
	"link": true, "meta": true, "param": true, "source": true,
	"track": true, "wbr": true,
}

// checkTagBalance tokenizes the HTML and checks for mismatched open/close tags.
func checkTagBalance(name, src string) []string {
	var problems []string

	type stackEntry struct {
		tag string
	}
	var stack []stackEntry

	z := html.NewTokenizer(strings.NewReader(src))
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			if !errors.Is(z.Err(), io.EOF) {
				problems = append(problems, fmt.Sprintf("%s: HTML parse error: %v", name, z.Err()))
			}
			break
		}

		tn, _ := z.TagName()
		tag := strings.ToLower(string(tn))

		switch tt {
		case html.StartTagToken:
			if !voidElements[tag] {
				stack = append(stack, stackEntry{tag: tag})
			}

		case html.EndTagToken:
			if voidElements[tag] {
				problems = append(problems, fmt.Sprintf("%s: void element </%s> should not have a closing tag", name, tag))
				continue
			}
			// Find matching open tag (search from top of stack).
			found := -1
			for i := len(stack) - 1; i >= 0; i-- {
				if stack[i].tag == tag {
					found = i
					break
				}
			}
			if found == -1 {
				problems = append(problems, fmt.Sprintf("%s: closing </%s> without matching opening tag", name, tag))
			} else {
				// Report any unclosed tags between the match and the top.
				for i := len(stack) - 1; i > found; i-- {
					problems = append(problems, fmt.Sprintf("%s: unclosed <%s> (closed by </%s>)", name, stack[i].tag, tag))
				}
				stack = stack[:found]
			}
		}
	}

	// Ignore top-level structural tags we may have added for wrapping, and
	// also ignore tags that are commonly left "open" in partials because
	// they span across template boundaries (html, head, body).
	ignore := map[string]bool{"html": true, "head": true, "body": true}
	for _, e := range stack {
		if !ignore[e.tag] {
			problems = append(problems, fmt.Sprintf("%s: unclosed <%s> at end of file", name, e.tag))
		}
	}

	return problems
}

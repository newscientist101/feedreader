package srv

import (
	"html/template"
	"sync"

	"github.com/microcosm-cc/bluemonday"
)

var (
	sanitizePolicy     *bluemonday.Policy
	sanitizePolicyOnce sync.Once
)

// articleSanitizePolicy returns a shared bluemonday policy configured for
// RSS/Atom article content. It allows common formatting tags and attributes
// while stripping scripts, event handlers, and other dangerous content.
func articleSanitizePolicy() *bluemonday.Policy {
	sanitizePolicyOnce.Do(func() {
		p := bluemonday.UGCPolicy()

		// Allow additional tags common in RSS/Atom content.
		p.AllowElements("figure", "figcaption", "picture", "source",
			"video", "audio", "details", "summary", "mark", "time",
			"section", "article", "aside", "header", "footer", "nav",
			"main", "hgroup", "address")

		// Allow data attributes commonly used in feed content.
		p.AllowDataAttributes()

		// Allow srcset/sizes for responsive images.
		p.AllowAttrs("srcset", "sizes").OnElements("img", "source")

		// Allow media element attributes.
		p.AllowAttrs("controls", "preload", "loop", "muted", "poster").
			OnElements("video", "audio")
		p.AllowAttrs("src", "type").OnElements("source")

		// Allow iframes for embedded media (YouTube, etc.) with restrictions.
		p.AllowElements("iframe")
		p.AllowAttrs("src", "width", "height", "frameborder",
			"allow", "allowfullscreen", "loading", "title").
			OnElements("iframe")

		// Allow common styling attributes.
		p.AllowAttrs("class", "id", "lang", "dir", "title").Globally()
		p.AllowAttrs("datetime").OnElements("time")
		p.AllowAttrs("open").OnElements("details")
		p.AllowAttrs("width", "height").OnElements("img", "video", "iframe")

		// Allow inline styles for layout in feed content (e.g., text-align).
		p.AllowStyles("text-align", "margin", "padding", "color",
			"background-color", "font-weight", "font-style",
			"text-decoration", "display", "width", "max-width",
			"height", "max-height", "float", "clear",
			"border", "border-radius", "vertical-align").Globally()

		sanitizePolicy = p
	})
	return sanitizePolicy
}

// sanitizeHTML sanitizes untrusted HTML content (e.g., from RSS feeds)
// and returns it as a template.HTML value safe for rendering.
func sanitizeHTML(s string) template.HTML {
	return template.HTML(articleSanitizePolicy().Sanitize(s)) //nolint:gosec // output is sanitized
}

// Package sources provides a registry of feed source handlers.
//
// Each source knows how to match, normalize, and auto-name feeds for a
// specific provider (Steam, Reddit, HuggingFace, etc.). The registry is
// iterated by the feed creation handler so that provider-specific logic
// does not leak into the HTTP layer.
package sources

import "context"

// Source defines a pluggable feed source handler.
type Source interface {
	// Match reports whether this source handles the given URL/feedType pair.
	Match(url, feedType string) bool

	// NormalizeURL rewrites the URL if needed (e.g. Steam news page → RSS).
	// Return the URL unchanged when no rewriting is needed.
	NormalizeURL(ctx context.Context, url string) (string, error)

	// ResolveName derives a human-readable feed name.
	// scraperConfig is the raw JSON config string (may be empty).
	// Return "" if no name can be determined.
	ResolveName(ctx context.Context, url, scraperConfig string) string

	// FeedType returns the canonical feed_type to persist, or "" to keep
	// whatever the caller already has.
	FeedType() string
}

// Registry holds registered feed sources in priority order.
type Registry struct {
	sources []Source
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// Register appends a source. Sources are evaluated in registration order;
// the first match wins.
func (r *Registry) Register(s Source) {
	r.sources = append(r.sources, s)
}

// Lookup returns the first Source that matches url/feedType, or nil.
func (r *Registry) Lookup(url, feedType string) Source {
	for _, s := range r.sources {
		if s.Match(url, feedType) {
			return s
		}
	}
	return nil
}

// Resolve applies source-specific normalization and naming to a feed
// creation request. It returns the (possibly rewritten) URL, name, and
// feedType. If no source matches, the inputs are returned unchanged.
func (r *Registry) Resolve(ctx context.Context, url, name, feedType, scraperConfig string) (rURL, rName, rFeedType string) {
	src := r.Lookup(url, feedType)
	if src == nil {
		return url, name, feedType
	}

	// Normalize URL.
	if nu, err := src.NormalizeURL(ctx, url); err == nil {
		url = nu
	}

	// Auto-name (only if caller didn't supply one).
	if name == "" {
		name = src.ResolveName(ctx, url, scraperConfig)
	}

	// Override feed type if the source dictates one.
	if ft := src.FeedType(); ft != "" {
		feedType = ft
	}

	return url, name, feedType
}

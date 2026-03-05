package sources

// DefaultRegistry returns a registry pre-populated with all built-in feed
// sources. Sources are registered in priority order: more-specific matchers
// first.
func DefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register(SteamSource{})
	r.Register(RedditSource{})
	r.Register(HuggingFaceSource{})
	return r
}

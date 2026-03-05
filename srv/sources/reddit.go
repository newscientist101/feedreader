package sources

import (
	"context"
	"regexp"
)

var redditSubRe = regexp.MustCompile(`reddit\.com/r/([^/]+)`)

// RedditSource handles Reddit RSS feed URLs.
type RedditSource struct{}

func (RedditSource) Match(rawURL, feedType string) bool {
	if feedType != "rss" {
		return false
	}
	return redditSubRe.MatchString(rawURL)
}

func (RedditSource) NormalizeURL(_ context.Context, rawURL string) (string, error) {
	return rawURL, nil
}

func (RedditSource) ResolveName(_ context.Context, rawURL, _ string) string {
	if m := redditSubRe.FindStringSubmatch(rawURL); m != nil {
		return "r/" + m[1]
	}
	return ""
}

func (RedditSource) FeedType() string { return "" }

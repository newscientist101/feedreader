package sources

import (
	"context"
	"regexp"
	"strings"
)

var (
	// Matches github.com/{owner}/{repo}/releases.atom
	githubReleaseFeedRe = regexp.MustCompile(`github\.com/([^/]+)/([^/]+)/releases\.atom$`)

	// Matches github.com/{owner}/{repo} (with optional trailing slash or extra path)
	githubRepoRe = regexp.MustCompile(`^https?://github\.com/([^/]+)/([^/]+?)(?:\.git)?(?:/.*)?$`)
)

// GitHubSource handles GitHub Releases Atom feed URLs.
type GitHubSource struct{}

func (GitHubSource) Match(rawURL, feedType string) bool {
	if feedType != "github" {
		return false
	}
	return strings.Contains(rawURL, "github.com")
}

func (GitHubSource) NormalizeURL(_ context.Context, rawURL string) (string, error) {
	// Already a releases.atom feed URL.
	if githubReleaseFeedRe.MatchString(rawURL) {
		// Ensure https prefix.
		if !strings.HasPrefix(rawURL, "https://") {
			rawURL = "https://" + strings.TrimPrefix(rawURL, "http://")
		}
		return rawURL, nil
	}

	// Try to extract owner/repo from a generic GitHub URL.
	if m := githubRepoRe.FindStringSubmatch(rawURL); m != nil {
		return "https://github.com/" + m[1] + "/" + m[2] + "/releases.atom", nil
	}

	return rawURL, nil
}

func (GitHubSource) ResolveName(_ context.Context, rawURL, _ string) string {
	if m := githubReleaseFeedRe.FindStringSubmatch(rawURL); m != nil {
		return m[1] + "/" + m[2] + " releases"
	}
	if m := githubRepoRe.FindStringSubmatch(rawURL); m != nil {
		return m[1] + "/" + m[2] + " releases"
	}
	return ""
}

// FeedType returns "rss" — GitHub Atom feeds are handled by the standard
// RSS/Atom parser.
func (GitHubSource) FeedType() string { return "rss" }

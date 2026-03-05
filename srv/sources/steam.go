package sources

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"time"

	"github.com/newscientist101/feedreader/srv/safenet"
)

var (
	steamNewsURLRe = regexp.MustCompile(`^(https?://store\.steampowered\.com)/news/(app/\d+)/?.*$`)
	steamFeedAppRe = regexp.MustCompile(`store\.steampowered\.com/feeds/news/app/(\d+)`)
)

// steamClient is a safe HTTP client for Steam API requests.
var steamClient = safenet.NewSafeClient(10*time.Second, nil)

// SteamSource handles Steam store / news feed URLs.
type SteamSource struct{}

func (SteamSource) Match(rawURL, feedType string) bool {
	if feedType != "rss" {
		return false
	}
	return steamNewsURLRe.MatchString(rawURL) || steamFeedAppRe.MatchString(rawURL)
}

func (SteamSource) NormalizeURL(_ context.Context, rawURL string) (string, error) {
	if m := steamNewsURLRe.FindStringSubmatch(rawURL); m != nil {
		return m[1] + "/feeds/news/" + m[2], nil
	}
	return rawURL, nil
}

func (SteamSource) ResolveName(_ context.Context, rawURL, _ string) string {
	if m := steamFeedAppRe.FindStringSubmatch(rawURL); m != nil {
		return fetchSteamAppName(m[1])
	}
	return ""
}

func (SteamSource) FeedType() string { return "" }

// fetchSteamAppName gets the game name from the Steam store API.
// The appID is already validated by the steamFeedAppRe regex (digits only).
func fetchSteamAppName(appID string) string {
	resp, err := steamClient.Get("https://store.steampowered.com/api/appdetails?appids=" + url.QueryEscape(appID))
	if err != nil {
		return ""
	}
	defer func() { _ = resp.Body.Close() }()
	var result map[string]struct {
		Success bool `json:"success"`
		Data    struct {
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ""
	}
	if app, ok := result[appID]; ok && app.Success {
		return app.Data.Name
	}
	return ""
}

// SetSteamClient replaces the HTTP client used for Steam API requests (for testing).
func SetSteamClient(c *http.Client) {
	steamClient = c
}

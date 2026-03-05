// Package youtube provides a client for the YouTube Data API v3.
// It fetches playlist items via the playlistItems.list endpoint,
// converting them to feeds.FeedItem structs for the existing pipeline.
package youtube

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/newscientist101/feedreader/srv/feeds"
	"github.com/newscientist101/feedreader/srv/safenet"
)

// Client fetches playlist items from the YouTube Data API v3.
type Client struct {
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a Client with the given API key.
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:     apiKey,
		httpClient: safenet.NewSafeClient(15*time.Second, nil),
	}
}

// SetHTTPClient replaces the HTTP client (for testing).
func (c *Client) SetHTTPClient(hc *http.Client) {
	c.httpClient = hc
}

// playlistItemsResponse is the YouTube Data API v3 playlistItems.list response.
type playlistItemsResponse struct {
	NextPageToken string              `json:"nextPageToken"`
	PageInfo      playlistPageInfo    `json:"pageInfo"`
	Items         []playlistItemEntry `json:"items"`
	Error         *apiError           `json:"error"`
}

type playlistPageInfo struct {
	TotalResults int `json:"totalResults"`
}

type playlistItemEntry struct {
	Snippet playlistSnippet `json:"snippet"`
}

type playlistSnippet struct {
	PublishedAt            string           `json:"publishedAt"`
	Title                  string           `json:"title"`
	Description            string           `json:"description"`
	Thumbnails             thumbnailMap     `json:"thumbnails"`
	VideoOwnerChannelTitle string           `json:"videoOwnerChannelTitle"`
	Position               int              `json:"position"`
	ResourceID             playlistResource `json:"resourceId"`
}

type playlistResource struct {
	VideoID string `json:"videoId"`
}

type thumbnailMap struct {
	High    *thumbnail `json:"high"`
	Medium  *thumbnail `json:"medium"`
	Default *thumbnail `json:"default"`
}

type thumbnail struct {
	URL string `json:"url"`
}

type apiError struct {
	Code    int              `json:"code"`
	Message string           `json:"message"`
	Errors  []apiErrorDetail `json:"errors"`
}

type apiErrorDetail struct {
	Reason string `json:"reason"`
}

// FetchPlaylistItems fetches all items from a YouTube playlist.
// It paginates through the full playlist, returning items as FeedItems.
// maxPages limits the number of API pages fetched (0 = unlimited).
func (c *Client) FetchPlaylistItems(ctx context.Context, playlistID string, maxPages int) ([]feeds.FeedItem, int, error) {
	var allItems []feeds.FeedItem
	var totalResults int
	pageToken := ""
	page := 0

	for {
		page++
		if maxPages > 0 && page > maxPages {
			break
		}

		resp, err := c.fetchPage(ctx, playlistID, pageToken)
		if err != nil {
			return nil, 0, err
		}

		if resp.Error != nil {
			return nil, 0, classifyError(resp.Error)
		}

		totalResults = resp.PageInfo.TotalResults

		for i := range resp.Items {
			fi := convertItem(&resp.Items[i])
			allItems = append(allItems, fi)
		}

		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}

	return allItems, totalResults, nil
}

const apiBaseURL = "https://www.googleapis.com/youtube/v3/playlistItems"

func (c *Client) fetchPage(ctx context.Context, playlistID, pageToken string) (*playlistItemsResponse, error) {
	u, _ := url.Parse(apiBaseURL)
	q := u.Query()
	q.Set("part", "snippet")
	q.Set("playlistId", playlistID)
	q.Set("maxResults", "50")
	q.Set("key", c.apiKey)
	if pageToken != "" {
		q.Set("pageToken", pageToken)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("youtube API request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Try to parse error response for a better message.
		var errResp playlistItemsResponse
		if json.Unmarshal(body, &errResp) == nil && errResp.Error != nil {
			return nil, classifyError(errResp.Error)
		}
		return nil, fmt.Errorf("youtube API HTTP %d: %s", resp.StatusCode, body)
	}

	var result playlistItemsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

func convertItem(item *playlistItemEntry) feeds.FeedItem {
	s := item.Snippet
	videoURL := "https://www.youtube.com/watch?v=" + s.ResourceID.VideoID
	embedURL := "https://www.youtube.com/embed/" + s.ResourceID.VideoID

	// Build content with embedded player and description.
	content := fmt.Sprintf( //nolint:gocritic // sprintfQuotedString: %q would add Go-style quotes in HTML attribute
		`<iframe src="%s" allowfullscreen></iframe>`,
		html.EscapeString(embedURL),
	)
	if s.Description != "" {
		content += "\n<p>" + html.EscapeString(s.Description) + "</p>"
	}

	var pubTime *time.Time
	if t, err := time.Parse(time.RFC3339, s.PublishedAt); err == nil {
		pubTime = &t
	}

	return feeds.FeedItem{
		GUID:        "yt:video:" + s.ResourceID.VideoID,
		Title:       s.Title,
		URL:         videoURL,
		Author:      s.VideoOwnerChannelTitle,
		Content:     content,
		ImageURL:    bestThumbnail(s.Thumbnails),
		PublishedAt: pubTime,
	}
}

func bestThumbnail(t thumbnailMap) string {
	if t.High != nil && t.High.URL != "" {
		return t.High.URL
	}
	if t.Medium != nil && t.Medium.URL != "" {
		return t.Medium.URL
	}
	if t.Default != nil && t.Default.URL != "" {
		return t.Default.URL
	}
	return ""
}

// classifyError converts a YouTube API error into a descriptive Go error.
func classifyError(e *apiError) error {
	for _, detail := range e.Errors {
		switch detail.Reason {
		case "quotaExceeded", "dailyLimitExceeded":
			return fmt.Errorf("youtube API quota exceeded: %s", e.Message)
		case "keyInvalid":
			return fmt.Errorf("youtube API key invalid: %s", e.Message)
		case "playlistNotFound":
			return fmt.Errorf("youtube playlist not found: %s", e.Message)
		case "forbidden":
			return fmt.Errorf("youtube API access forbidden: %s", e.Message)
		}
	}
	return fmt.Errorf("youtube API error %d: %s", e.Code, e.Message)
}

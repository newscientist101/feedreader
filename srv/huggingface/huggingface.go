// Package huggingface provides a feed source for Hugging Face content
package huggingface

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const baseURL = "https://huggingface.co"

// Client is a Hugging Face API client
type Client struct {
	httpClient *http.Client
	token      string // optional API token for private content
	baseURL    string // API base URL (defaults to baseURL const)
}

// NewClient creates a new HuggingFace client
func NewClient(token string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		token:      token,
		baseURL:    baseURL,
	}
}

// FeedItem represents a generic feed item from HuggingFace
type FeedItem struct {
	GUID        string
	Title       string
	URL         string
	Author      string
	Summary     string
	PublishedAt *time.Time
	ImageURL    string
	Tags        []string
}

// FeedType represents the type of HuggingFace feed
type FeedType string

const (
	FeedTypeUserModels   FeedType = "user_models"
	FeedTypeOrgModels    FeedType = "org_models"
	FeedTypeCollection   FeedType = "collection"
	FeedTypeUserPosts    FeedType = "user_posts"
	FeedTypeOrgPosts     FeedType = "org_posts"
	FeedTypeDailyPapers  FeedType = "daily_papers"
	FeedTypeUserDatasets FeedType = "user_datasets"
	FeedTypeOrgDatasets  FeedType = "org_datasets"
	FeedTypeUserSpaces   FeedType = "user_spaces"
	FeedTypeOrgSpaces    FeedType = "org_spaces"
)

// FeedConfig represents the configuration for a HuggingFace feed
type FeedConfig struct {
	Type        FeedType `json:"type"`
	Identifier  string   `json:"identifier"`   // username, org name, or collection slug
	Limit       int      `json:"limit"`        // max items to fetch
	IncludeTags []string `json:"include_tags"` // optional: only include items with these tags
	ExcludeTags []string `json:"exclude_tags"` // optional: exclude items with these tags
}

// Fetch retrieves items based on the feed configuration
// GetFeedName returns a suggested name for the feed based on the config
func (c *Client) GetFeedName(ctx context.Context, config *FeedConfig) (string, error) {
	switch config.Type {
	case FeedTypeCollection:
		// Fetch collection info to get its title
		apiURL := fmt.Sprintf("%s/api/collections/%s", c.baseURL, config.Identifier)
		data, err := c.doRequest(ctx, apiURL)
		if err != nil {
			return "", err
		}
		var collection struct {
			Title string `json:"title"`
		}
		if err := json.Unmarshal(data, &collection); err != nil {
			return "", err
		}
		if collection.Title != "" {
			return collection.Title, nil
		}
		return config.Identifier, nil

	case FeedTypeDailyPapers:
		return "HuggingFace Daily Papers", nil

	case FeedTypeUserModels, FeedTypeOrgModels:
		return fmt.Sprintf("%s Models", config.Identifier), nil

	case FeedTypeUserDatasets, FeedTypeOrgDatasets:
		return fmt.Sprintf("%s Datasets", config.Identifier), nil

	case FeedTypeUserSpaces, FeedTypeOrgSpaces:
		return fmt.Sprintf("%s Spaces", config.Identifier), nil

	case FeedTypeUserPosts, FeedTypeOrgPosts:
		return fmt.Sprintf("%s Posts", config.Identifier), nil

	default:
		return config.Identifier, nil
	}
}

func (c *Client) Fetch(ctx context.Context, config *FeedConfig) ([]FeedItem, error) {
	if config.Limit == 0 {
		config.Limit = 50
	}

	switch config.Type {
	case FeedTypeUserModels, FeedTypeOrgModels:
		return c.fetchModels(ctx, config)
	case FeedTypeCollection:
		return c.fetchCollection(ctx, config)
	case FeedTypeUserPosts, FeedTypeOrgPosts:
		return c.fetchPosts(ctx, config)
	case FeedTypeDailyPapers:
		return c.fetchDailyPapers(ctx, config)
	case FeedTypeUserDatasets, FeedTypeOrgDatasets:
		return c.fetchDatasets(ctx, config)
	case FeedTypeUserSpaces, FeedTypeOrgSpaces:
		return c.fetchSpaces(ctx, config)
	default:
		return nil, fmt.Errorf("unknown feed type: %s", config.Type)
	}
}

func (c *Client) doRequest(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, http.NoBody)
	if err != nil {
		return nil, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	req.Header.Set("User-Agent", "FeedReader/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// Model represents a HuggingFace model
type Model struct {
	ID           string    `json:"id"`
	ModelID      string    `json:"modelId"`
	Author       string    `json:"author"`
	Likes        int       `json:"likes"`
	Downloads    int       `json:"downloads"`
	Tags         []string  `json:"tags"`
	PipelineTag  string    `json:"pipeline_tag"`
	CreatedAt    time.Time `json:"createdAt"`
	LastModified time.Time `json:"lastModified"`
}

func (c *Client) fetchModels(ctx context.Context, config *FeedConfig) ([]FeedItem, error) {
	apiURL := fmt.Sprintf("%s/api/models?author=%s&limit=%d&sort=lastModified&direction=-1",
		c.baseURL, url.QueryEscape(config.Identifier), config.Limit)

	data, err := c.doRequest(ctx, apiURL)
	if err != nil {
		return nil, fmt.Errorf("fetch models: %w", err)
	}

	var models []Model
	if err := json.Unmarshal(data, &models); err != nil {
		return nil, fmt.Errorf("parse models: %w", err)
	}

	items := make([]FeedItem, 0, len(models))
	for i := range models {
		if !c.matchesTags(models[i].Tags, config) {
			continue
		}

		pubTime := models[i].CreatedAt
		if !models[i].LastModified.IsZero() {
			pubTime = models[i].LastModified
		}

		summary := fmt.Sprintf("Downloads: %d | Likes: %d", models[i].Downloads, models[i].Likes)
		if models[i].PipelineTag != "" {
			summary = fmt.Sprintf("%s | %s", models[i].PipelineTag, summary)
		}

		items = append(items, FeedItem{
			GUID:        "hf:model:" + models[i].ID,
			Title:       models[i].ID,
			URL:         fmt.Sprintf("%s/%s", c.baseURL, models[i].ID),
			Author:      config.Identifier,
			Summary:     summary,
			PublishedAt: &pubTime,
			Tags:        models[i].Tags,
		})
	}

	return items, nil
}

// Collection represents a HuggingFace collection
type Collection struct {
	Slug        string           `json:"slug"`
	Title       string           `json:"title"`
	Description string           `json:"description"`
	Owner       CollectionOwner  `json:"owner"`
	Items       []CollectionItem `json:"items"`
	LastUpdated time.Time        `json:"lastUpdated"`
}

type CollectionOwner struct {
	Name     string `json:"name"`
	Fullname string `json:"fullname"`
	Type     string `json:"type"` // "user" or "org"
}

type CollectionItem struct {
	ID           string    `json:"id"`
	Type         string    `json:"type"`     // "model", "dataset", "space"
	RepoType     string    `json:"repoType"` // alternate field name
	Position     int       `json:"position"`
	Author       string    `json:"author"`
	Likes        int       `json:"likes"`
	Downloads    int       `json:"downloads"`
	PipelineTag  string    `json:"pipeline_tag"`
	LastModified time.Time `json:"lastModified"`
}

func (c *Client) fetchCollection(ctx context.Context, config *FeedConfig) ([]FeedItem, error) {
	// Collection slug format: username/collection-name-id or username/collection-name
	apiURL := fmt.Sprintf("%s/api/collections/%s", c.baseURL, config.Identifier)

	data, err := c.doRequest(ctx, apiURL)
	if err != nil {
		return nil, fmt.Errorf("fetch collection: %w", err)
	}

	var collection Collection
	if err := json.Unmarshal(data, &collection); err != nil {
		return nil, fmt.Errorf("parse collection: %w", err)
	}

	items := make([]FeedItem, 0, len(collection.Items))
	for i := range collection.Items {
		if len(items) >= config.Limit {
			break
		}

		// Use repoType if type is empty
		itemType := collection.Items[i].Type
		if itemType == "" {
			itemType = collection.Items[i].RepoType
		}

		var itemURL string
		switch itemType {
		case "model":
			itemURL = fmt.Sprintf("%s/%s", c.baseURL, collection.Items[i].ID)
		case "dataset":
			itemURL = fmt.Sprintf("%s/datasets/%s", c.baseURL, collection.Items[i].ID)
		case "space":
			itemURL = fmt.Sprintf("%s/spaces/%s", c.baseURL, collection.Items[i].ID)
		default:
			// Default to model if type is unknown
			itemURL = fmt.Sprintf("%s/%s", c.baseURL, collection.Items[i].ID)
		}

		// Build summary with stats
		summary := fmt.Sprintf("Part of collection: %s", collection.Title)
		if collection.Items[i].Downloads > 0 || collection.Items[i].Likes > 0 {
			summary = fmt.Sprintf("%s | Downloads: %d | Likes: %d", summary, collection.Items[i].Downloads, collection.Items[i].Likes)
		}
		if collection.Items[i].PipelineTag != "" {
			summary = fmt.Sprintf("%s | %s", collection.Items[i].PipelineTag, summary)
		}

		// Use item's lastModified if available, otherwise collection's
		pubTime := collection.LastUpdated
		if !collection.Items[i].LastModified.IsZero() {
			pubTime = collection.Items[i].LastModified
		}

		items = append(items, FeedItem{
			GUID:        fmt.Sprintf("hf:collection:%s:%s", collection.Slug, collection.Items[i].ID),
			Title:       collection.Items[i].ID,
			URL:         itemURL,
			Author:      collection.Items[i].Author,
			Summary:     summary,
			PublishedAt: &pubTime,
		})
	}

	return items, nil
}

// Post represents a HuggingFace social post/article
type Post struct {
	Slug        string        `json:"slug"`
	Content     []PostContent `json:"content"`
	Author      PostAuthor    `json:"author"`
	PublishedAt time.Time     `json:"publishedAt"`
	NumLikes    int           `json:"numLikes"`
	NumComments int           `json:"numComments"`
}

type PostContent struct {
	Type  string `json:"type"`
	Value string `json:"value"`
	Raw   string `json:"raw"`
}

type PostAuthor struct {
	ID       string `json:"_id"`
	Name     string `json:"name"`
	Fullname string `json:"fullname"`
	Type     string `json:"type"` // "user" or "org"
}

type PostsResponse struct {
	SocialPosts []Post `json:"socialPosts"`
}

func (c *Client) fetchPosts(ctx context.Context, config *FeedConfig) ([]FeedItem, error) {
	apiURL := fmt.Sprintf("%s/api/posts?author=%s&limit=%d",
		c.baseURL, url.QueryEscape(config.Identifier), config.Limit)

	data, err := c.doRequest(ctx, apiURL)
	if err != nil {
		return nil, fmt.Errorf("fetch posts: %w", err)
	}

	var resp PostsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse posts: %w", err)
	}

	items := make([]FeedItem, 0, len(resp.SocialPosts))
	for i := range resp.SocialPosts {
		// Extract title from first text content
		title := "Post"
		var summary strings.Builder
		for _, content := range resp.SocialPosts[i].Content {
			if content.Type == "text" && content.Value != "" {
				if title == "Post" {
					title = truncateString(content.Value, 100)
				}
				summary.WriteString(content.Value)
				summary.WriteString(" ")
			}
		}

		pubTime := resp.SocialPosts[i].PublishedAt

		// Post URL format: /posts/{username}/{slug}
		authorName := resp.SocialPosts[i].Author.Name
		if authorName == "" {
			authorName = config.Identifier
		}

		items = append(items, FeedItem{
			GUID:        "hf:post:" + resp.SocialPosts[i].Slug,
			Title:       title,
			URL:         fmt.Sprintf("%s/posts/%s/%s", c.baseURL, authorName, resp.SocialPosts[i].Slug),
			Author:      authorName,
			Summary:     truncateString(summary.String(), 300),
			PublishedAt: &pubTime,
		})
	}

	return items, nil
}

// Paper represents a HuggingFace daily paper
type Paper struct {
	Paper struct {
		ID      string `json:"id"`
		Title   string `json:"title"`
		Summary string `json:"summary"`
		Authors []struct {
			Name string `json:"name"`
		} `json:"authors"`
	} `json:"paper"`
	Title       string    `json:"title"`
	Summary     string    `json:"summary"`
	Thumbnail   string    `json:"thumbnail"`
	PublishedAt time.Time `json:"publishedAt"`
	SubmittedBy struct {
		Name     string `json:"name"`
		Fullname string `json:"fullname"`
	} `json:"submittedBy"`
}

func (c *Client) fetchDailyPapers(ctx context.Context, config *FeedConfig) ([]FeedItem, error) {
	apiURL := fmt.Sprintf("%s/api/daily_papers?limit=%d", c.baseURL, config.Limit)

	data, err := c.doRequest(ctx, apiURL)
	if err != nil {
		return nil, fmt.Errorf("fetch papers: %w", err)
	}

	var papers []Paper
	if err := json.Unmarshal(data, &papers); err != nil {
		return nil, fmt.Errorf("parse papers: %w", err)
	}

	items := make([]FeedItem, 0, len(papers))
	for i := range papers {
		authors := make([]string, len(papers[i].Paper.Authors))
		for j, a := range papers[i].Paper.Authors {
			authors[j] = a.Name
		}

		pubTime := papers[i].PublishedAt
		items = append(items, FeedItem{
			GUID:        "hf:paper:" + papers[i].Paper.ID,
			Title:       papers[i].Title,
			URL:         fmt.Sprintf("%s/papers/%s", c.baseURL, papers[i].Paper.ID),
			Author:      strings.Join(authors, ", "),
			Summary:     truncateString(papers[i].Summary, 500),
			PublishedAt: &pubTime,
			ImageURL:    papers[i].Thumbnail,
		})
	}

	return items, nil
}

// Dataset represents a HuggingFace dataset
type Dataset struct {
	ID           string    `json:"id"`
	Author       string    `json:"author"`
	Likes        int       `json:"likes"`
	Downloads    int       `json:"downloads"`
	Tags         []string  `json:"tags"`
	CreatedAt    time.Time `json:"createdAt"`
	LastModified time.Time `json:"lastModified"`
}

func (c *Client) fetchDatasets(ctx context.Context, config *FeedConfig) ([]FeedItem, error) {
	apiURL := fmt.Sprintf("%s/api/datasets?author=%s&limit=%d&sort=lastModified&direction=-1",
		c.baseURL, url.QueryEscape(config.Identifier), config.Limit)

	data, err := c.doRequest(ctx, apiURL)
	if err != nil {
		return nil, fmt.Errorf("fetch datasets: %w", err)
	}

	var datasets []Dataset
	if err := json.Unmarshal(data, &datasets); err != nil {
		return nil, fmt.Errorf("parse datasets: %w", err)
	}

	items := make([]FeedItem, 0, len(datasets))
	for _, d := range datasets {
		if !c.matchesTags(d.Tags, config) {
			continue
		}

		pubTime := d.CreatedAt
		if !d.LastModified.IsZero() {
			pubTime = d.LastModified
		}

		items = append(items, FeedItem{
			GUID:        "hf:dataset:" + d.ID,
			Title:       d.ID,
			URL:         fmt.Sprintf("%s/datasets/%s", c.baseURL, d.ID),
			Author:      config.Identifier,
			Summary:     fmt.Sprintf("Downloads: %d | Likes: %d", d.Downloads, d.Likes),
			PublishedAt: &pubTime,
			Tags:        d.Tags,
		})
	}

	return items, nil
}

// Space represents a HuggingFace space
type Space struct {
	ID           string    `json:"id"`
	Author       string    `json:"author"`
	Likes        int       `json:"likes"`
	Tags         []string  `json:"tags"`
	CreatedAt    time.Time `json:"createdAt"`
	LastModified time.Time `json:"lastModified"`
	SDK          string    `json:"sdk"`
}

func (c *Client) fetchSpaces(ctx context.Context, config *FeedConfig) ([]FeedItem, error) {
	apiURL := fmt.Sprintf("%s/api/spaces?author=%s&limit=%d&sort=lastModified&direction=-1",
		c.baseURL, url.QueryEscape(config.Identifier), config.Limit)

	data, err := c.doRequest(ctx, apiURL)
	if err != nil {
		return nil, fmt.Errorf("fetch spaces: %w", err)
	}

	var spaces []Space
	if err := json.Unmarshal(data, &spaces); err != nil {
		return nil, fmt.Errorf("parse spaces: %w", err)
	}

	items := make([]FeedItem, 0, len(spaces))
	for i := range spaces {
		if !c.matchesTags(spaces[i].Tags, config) {
			continue
		}

		pubTime := spaces[i].CreatedAt
		if !spaces[i].LastModified.IsZero() {
			pubTime = spaces[i].LastModified
		}

		summary := fmt.Sprintf("Likes: %d", spaces[i].Likes)
		if spaces[i].SDK != "" {
			summary = fmt.Sprintf("SDK: %s | %s", spaces[i].SDK, summary)
		}

		items = append(items, FeedItem{
			GUID:        "hf:space:" + spaces[i].ID,
			Title:       spaces[i].ID,
			URL:         fmt.Sprintf("%s/spaces/%s", c.baseURL, spaces[i].ID),
			Author:      config.Identifier,
			Summary:     summary,
			PublishedAt: &pubTime,
			Tags:        spaces[i].Tags,
		})
	}

	return items, nil
}

func (c *Client) matchesTags(itemTags []string, config *FeedConfig) bool {
	// Check include tags (must have at least one)
	if len(config.IncludeTags) > 0 {
		hasInclude := false
		for _, includeTag := range config.IncludeTags {
			for _, tag := range itemTags {
				if strings.EqualFold(tag, includeTag) {
					hasInclude = true
					break
				}
			}
			if hasInclude {
				break
			}
		}
		if !hasInclude {
			return false
		}
	}

	// Check exclude tags (must not have any)
	for _, excludeTag := range config.ExcludeTags {
		for _, tag := range itemTags {
			if strings.EqualFold(tag, excludeTag) {
				return false
			}
		}
	}

	return true
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

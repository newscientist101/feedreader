package huggingface

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestClient(serverURL string) *Client {
	return &Client{
		httpClient: &http.Client{},
		baseURL:    serverURL,
	}
}

func newTestClientWithToken(serverURL, token string) *Client {
	c := newTestClient(serverURL)
	c.token = token
	return c
}

// --- TestTruncateString ---

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"short", "hello", 10, "hello"},
		{"exact", "hello", 5, "hello"},
		{"truncated", "hello world", 8, "hello..."},
		{"min truncation", "abcd", 3, "..."},
		{"empty", "", 5, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateString(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateString(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

// --- TestMatchesTags ---

func TestMatchesTags(t *testing.T) {
	c := &Client{}
	tests := []struct {
		name   string
		tags   []string
		config FeedConfig
		want   bool
	}{
		{"no filters", []string{"a", "b"}, FeedConfig{}, true},
		{"include match", []string{"a", "b"}, FeedConfig{IncludeTags: []string{"a"}}, true},
		{"include no match", []string{"a", "b"}, FeedConfig{IncludeTags: []string{"c"}}, false},
		{"include case insensitive", []string{"PyTorch"}, FeedConfig{IncludeTags: []string{"pytorch"}}, true},
		{"exclude match", []string{"a", "b"}, FeedConfig{ExcludeTags: []string{"b"}}, false},
		{"exclude no match", []string{"a", "b"}, FeedConfig{ExcludeTags: []string{"c"}}, true},
		{"exclude case insensitive", []string{"PyTorch"}, FeedConfig{ExcludeTags: []string{"pytorch"}}, false},
		{"include and exclude both pass", []string{"a", "b"}, FeedConfig{IncludeTags: []string{"a"}, ExcludeTags: []string{"c"}}, true},
		{"include passes exclude fails", []string{"a", "b"}, FeedConfig{IncludeTags: []string{"a"}, ExcludeTags: []string{"b"}}, false},
		{"empty tags include required", []string{}, FeedConfig{IncludeTags: []string{"a"}}, false},
		{"empty tags no filters", []string{}, FeedConfig{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.matchesTags(tt.tags, tt.config)
			if got != tt.want {
				t.Errorf("matchesTags(%v, ...) = %v, want %v", tt.tags, got, tt.want)
			}
		})
	}
}

// --- TestFetch_UnknownType ---

func TestFetch_UnknownType(t *testing.T) {
	c := newTestClient("http://unused")
	_, err := c.Fetch(context.Background(), FeedConfig{Type: "bogus"})
	if err == nil || !strings.Contains(err.Error(), "unknown feed type") {
		t.Fatalf("expected unknown feed type error, got: %v", err)
	}
}

// --- TestFetch_DefaultLimit ---

func TestFetch_DefaultLimit(t *testing.T) {
	var capturedURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL.String()
		w.Write([]byte("[]"))
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	c.Fetch(context.Background(), FeedConfig{Type: FeedTypeUserModels, Identifier: "testuser"})
	if !strings.Contains(capturedURL, "limit=50") {
		t.Errorf("expected default limit=50 in URL, got: %s", capturedURL)
	}
}

// --- TestFetch_APIError ---

func TestFetch_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	_, err := c.Fetch(context.Background(), FeedConfig{Type: FeedTypeUserModels, Identifier: "x"})
	if err == nil || !strings.Contains(err.Error(), "status 500") {
		t.Fatalf("expected status 500 error, got: %v", err)
	}
}

// --- TestAuthToken ---

func TestAuthToken(t *testing.T) {
	var gotAuth string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Write([]byte("[]"))
	}))
	defer ts.Close()

	c := newTestClientWithToken(ts.URL, "hf_secret123")
	c.Fetch(context.Background(), FeedConfig{Type: FeedTypeUserModels, Identifier: "u"})
	if gotAuth != "Bearer hf_secret123" {
		t.Errorf("expected 'Bearer hf_secret123', got %q", gotAuth)
	}
}

// --- TestDoRequest_UserAgent ---

func TestDoRequest_UserAgent(t *testing.T) {
	var gotUA string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.Write([]byte("[]"))
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	c.Fetch(context.Background(), FeedConfig{Type: FeedTypeUserModels, Identifier: "u"})
	if gotUA != "FeedReader/1.0" {
		t.Errorf("expected User-Agent 'FeedReader/1.0', got %q", gotUA)
	}
}

// --- TestFetchModels ---

func TestFetchModels(t *testing.T) {
	models := []Model{
		{
			ID: "alice/llama-finetune", ModelID: "alice/llama-finetune", Author: "alice",
			Likes: 42, Downloads: 1000, Tags: []string{"pytorch", "llm"},
			PipelineTag:  "text-generation",
			CreatedAt:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			LastModified: time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			ID: "alice/bert-small", ModelID: "alice/bert-small", Author: "alice",
			Likes: 5, Downloads: 200, Tags: []string{"pytorch"},
			CreatedAt: time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
		},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(models)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	items, err := c.Fetch(context.Background(), FeedConfig{Type: FeedTypeUserModels, Identifier: "alice", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	// First model
	item := items[0]
	if item.GUID != "hf:model:alice/llama-finetune" {
		t.Errorf("GUID = %q", item.GUID)
	}
	if item.Title != "alice/llama-finetune" {
		t.Errorf("Title = %q", item.Title)
	}
	if item.URL != ts.URL+"/alice/llama-finetune" {
		t.Errorf("URL = %q", item.URL)
	}
	if item.Author != "alice" {
		t.Errorf("Author = %q", item.Author)
	}
	expectedSummary := "text-generation | Downloads: 1000 | Likes: 42"
	if item.Summary != expectedSummary {
		t.Errorf("Summary = %q, want %q", item.Summary, expectedSummary)
	}
	if item.PublishedAt == nil || !item.PublishedAt.Equal(time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("PublishedAt = %v, want LastModified date", item.PublishedAt)
	}

	// Second model (no pipeline tag, no LastModified → uses CreatedAt)
	item2 := items[1]
	expectedSummary2 := "Downloads: 200 | Likes: 5"
	if item2.Summary != expectedSummary2 {
		t.Errorf("Summary = %q, want %q", item2.Summary, expectedSummary2)
	}
	if item2.PublishedAt == nil || !item2.PublishedAt.Equal(time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("PublishedAt should be CreatedAt when LastModified is zero")
	}
}

// --- TestFetchModels_WithTagFilters ---

func TestFetchModels_WithTagFilters(t *testing.T) {
	models := []Model{
		{ID: "u/m1", Tags: []string{"pytorch", "llm"}, CreatedAt: time.Now()},
		{ID: "u/m2", Tags: []string{"tensorflow"}, CreatedAt: time.Now()},
		{ID: "u/m3", Tags: []string{"pytorch", "deprecated"}, CreatedAt: time.Now()},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(models)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)

	// Include only pytorch
	items, err := c.Fetch(context.Background(), FeedConfig{
		Type: FeedTypeUserModels, Identifier: "u", Limit: 10,
		IncludeTags: []string{"pytorch"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("include pytorch: expected 2 items, got %d", len(items))
	}

	// Exclude deprecated
	items, err = c.Fetch(context.Background(), FeedConfig{
		Type: FeedTypeUserModels, Identifier: "u", Limit: 10,
		ExcludeTags: []string{"deprecated"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("exclude deprecated: expected 2 items, got %d", len(items))
	}

	// Include pytorch AND exclude deprecated
	items, err = c.Fetch(context.Background(), FeedConfig{
		Type: FeedTypeUserModels, Identifier: "u", Limit: 10,
		IncludeTags: []string{"pytorch"}, ExcludeTags: []string{"deprecated"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("include pytorch exclude deprecated: expected 1 item, got %d", len(items))
	}
	if items[0].GUID != "hf:model:u/m1" {
		t.Errorf("expected m1, got %s", items[0].GUID)
	}
}

// --- TestFetchCollection ---

func TestFetchCollection(t *testing.T) {
	collection := Collection{
		Slug:        "bob/my-collection-abc123",
		Title:       "My Collection",
		Owner:       CollectionOwner{Name: "bob"},
		LastUpdated: time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC),
		Items: []CollectionItem{
			{ID: "bob/model1", Type: "model", Likes: 10, Downloads: 500,
				LastModified: time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC)},
			{ID: "bob/dataset1", Type: "dataset", Likes: 3},
			{ID: "bob/space1", Type: "space"},
			{ID: "bob/unknown1", Type: ""},
		},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(collection)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	items, err := c.Fetch(context.Background(), FeedConfig{Type: FeedTypeCollection, Identifier: "bob/my-collection-abc123", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 4 {
		t.Fatalf("expected 4 items, got %d", len(items))
	}

	// Model URL
	if items[0].URL != ts.URL+"/bob/model1" {
		t.Errorf("model URL = %q", items[0].URL)
	}
	// Dataset URL
	if items[1].URL != ts.URL+"/datasets/bob/dataset1" {
		t.Errorf("dataset URL = %q", items[1].URL)
	}
	// Space URL
	if items[2].URL != ts.URL+"/spaces/bob/space1" {
		t.Errorf("space URL = %q", items[2].URL)
	}
	// Unknown type defaults to model URL
	if items[3].URL != ts.URL+"/bob/unknown1" {
		t.Errorf("unknown type URL = %q", items[3].URL)
	}

	// GUID format
	if items[0].GUID != "hf:collection:bob/my-collection-abc123:bob/model1" {
		t.Errorf("GUID = %q", items[0].GUID)
	}

	// Summary contains collection title
	if !strings.Contains(items[0].Summary, "My Collection") {
		t.Errorf("summary should contain collection title: %q", items[0].Summary)
	}
	// Summary with stats
	if !strings.Contains(items[0].Summary, "Downloads: 500") {
		t.Errorf("summary should contain downloads: %q", items[0].Summary)
	}

	// PublishedAt uses item LastModified when available
	if !items[0].PublishedAt.Equal(time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("item with LastModified should use it, got %v", items[0].PublishedAt)
	}
	// Falls back to collection LastUpdated
	if !items[1].PublishedAt.Equal(time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("item without LastModified should use collection date, got %v", items[1].PublishedAt)
	}
}

// --- TestFetchPosts ---

func TestFetchPosts(t *testing.T) {
	resp := PostsResponse{
		SocialPosts: []Post{
			{
				Slug: "post-abc123",
				Content: []PostContent{
					{Type: "text", Value: "This is the title of my post"},
					{Type: "text", Value: "And some more content"},
					{Type: "image", Value: "img.png"},
				},
				Author:      PostAuthor{Name: "carol", Fullname: "Carol D"},
				PublishedAt: time.Date(2024, 4, 1, 12, 0, 0, 0, time.UTC),
				NumLikes:    7,
			},
			{
				Slug:        "post-no-text",
				Content:     []PostContent{{Type: "image", Value: "pic.jpg"}},
				Author:      PostAuthor{},
				PublishedAt: time.Date(2024, 4, 2, 0, 0, 0, 0, time.UTC),
			},
		},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	items, err := c.Fetch(context.Background(), FeedConfig{Type: FeedTypeUserPosts, Identifier: "carol", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2, got %d", len(items))
	}

	// Title from first text content
	if items[0].Title != "This is the title of my post" {
		t.Errorf("Title = %q", items[0].Title)
	}
	if items[0].GUID != "hf:post:post-abc123" {
		t.Errorf("GUID = %q", items[0].GUID)
	}
	if items[0].URL != ts.URL+"/posts/carol/post-abc123" {
		t.Errorf("URL = %q", items[0].URL)
	}

	// Post with no text content: title defaults to "Post", author falls back to config Identifier
	if items[1].Title != "Post" {
		t.Errorf("expected default title 'Post', got %q", items[1].Title)
	}
	if items[1].Author != "carol" {
		t.Errorf("expected author fallback to identifier 'carol', got %q", items[1].Author)
	}
}

// --- TestFetchDailyPapers ---

func TestFetchDailyPapers(t *testing.T) {
	papers := []Paper{
		{
			Paper: struct {
				ID      string `json:"id"`
				Title   string `json:"title"`
				Summary string `json:"summary"`
				Authors []struct {
					Name string `json:"name"`
				} `json:"authors"`
			}{
				ID:    "2401.12345",
				Title: "A Great Paper",
				Authors: []struct {
					Name string `json:"name"`
				}{{Name: "Alice"}, {Name: "Bob"}},
			},
			Title:       "A Great Paper",
			Summary:     "This paper explores...",
			Thumbnail:   "https://img.example.com/thumb.png",
			PublishedAt: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/api/daily_papers") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(papers)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	items, err := c.Fetch(context.Background(), FeedConfig{Type: FeedTypeDailyPapers, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1, got %d", len(items))
	}

	item := items[0]
	if item.GUID != "hf:paper:2401.12345" {
		t.Errorf("GUID = %q", item.GUID)
	}
	if item.URL != ts.URL+"/papers/2401.12345" {
		t.Errorf("URL = %q", item.URL)
	}
	if item.Author != "Alice, Bob" {
		t.Errorf("Author = %q, want joined names", item.Author)
	}
	if item.ImageURL != "https://img.example.com/thumb.png" {
		t.Errorf("ImageURL = %q", item.ImageURL)
	}
	if item.Summary != "This paper explores..." {
		t.Errorf("Summary = %q", item.Summary)
	}
}

// --- TestFetchDatasets ---

func TestFetchDatasets(t *testing.T) {
	datasets := []Dataset{
		{
			ID: "dan/my-dataset", Author: "dan", Likes: 8, Downloads: 3000,
			Tags:         []string{"text", "en"},
			CreatedAt:    time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
			LastModified: time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC),
		},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(datasets)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	items, err := c.Fetch(context.Background(), FeedConfig{Type: FeedTypeUserDatasets, Identifier: "dan", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1, got %d", len(items))
	}
	item := items[0]
	if item.GUID != "hf:dataset:dan/my-dataset" {
		t.Errorf("GUID = %q", item.GUID)
	}
	if item.URL != ts.URL+"/datasets/dan/my-dataset" {
		t.Errorf("URL = %q", item.URL)
	}
	if item.Summary != "Downloads: 3000 | Likes: 8" {
		t.Errorf("Summary = %q", item.Summary)
	}
	if !item.PublishedAt.Equal(time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("PublishedAt should be LastModified")
	}
}

// --- TestFetchSpaces ---

func TestFetchSpaces(t *testing.T) {
	spaces := []Space{
		{
			ID: "eve/my-space", Author: "eve", Likes: 15,
			Tags:         []string{"gradio"},
			SDK:          "gradio",
			CreatedAt:    time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC),
			LastModified: time.Date(2024, 8, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			ID: "eve/plain-space", Author: "eve", Likes: 2,
			Tags:      []string{},
			CreatedAt: time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC),
		},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(spaces)
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	items, err := c.Fetch(context.Background(), FeedConfig{Type: FeedTypeUserSpaces, Identifier: "eve", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2, got %d", len(items))
	}

	// Space with SDK
	if items[0].Summary != "SDK: gradio | Likes: 15" {
		t.Errorf("Summary = %q", items[0].Summary)
	}
	if items[0].URL != ts.URL+"/spaces/eve/my-space" {
		t.Errorf("URL = %q", items[0].URL)
	}
	if items[0].GUID != "hf:space:eve/my-space" {
		t.Errorf("GUID = %q", items[0].GUID)
	}

	// Space without SDK
	if items[1].Summary != "Likes: 2" {
		t.Errorf("Summary without SDK = %q", items[1].Summary)
	}
}

// --- TestGetFeedName ---

func TestGetFeedName(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"title": "Awesome Models"})
	}))
	defer ts.Close()

	c := newTestClient(ts.URL)
	ctx := context.Background()

	tests := []struct {
		name   string
		config FeedConfig
		want   string
	}{
		{"collection", FeedConfig{Type: FeedTypeCollection, Identifier: "user/coll-123"}, "Awesome Models"},
		{"daily papers", FeedConfig{Type: FeedTypeDailyPapers}, "HuggingFace Daily Papers"},
		{"user models", FeedConfig{Type: FeedTypeUserModels, Identifier: "alice"}, "alice Models"},
		{"org models", FeedConfig{Type: FeedTypeOrgModels, Identifier: "myorg"}, "myorg Models"},
		{"user datasets", FeedConfig{Type: FeedTypeUserDatasets, Identifier: "bob"}, "bob Datasets"},
		{"org datasets", FeedConfig{Type: FeedTypeOrgDatasets, Identifier: "org1"}, "org1 Datasets"},
		{"user spaces", FeedConfig{Type: FeedTypeUserSpaces, Identifier: "carol"}, "carol Spaces"},
		{"org spaces", FeedConfig{Type: FeedTypeOrgSpaces, Identifier: "org2"}, "org2 Spaces"},
		{"user posts", FeedConfig{Type: FeedTypeUserPosts, Identifier: "dave"}, "dave Posts"},
		{"org posts", FeedConfig{Type: FeedTypeOrgPosts, Identifier: "org3"}, "org3 Posts"},
		{"unknown", FeedConfig{Type: "whatever", Identifier: "fallback-id"}, "fallback-id"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := c.GetFeedName(ctx, tt.config)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("GetFeedName() = %q, want %q", got, tt.want)
			}
		})
	}
}

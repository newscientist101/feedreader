package srv

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"srv.exe.dev/db"
	"srv.exe.dev/db/dbgen"
	"srv.exe.dev/srv/feeds"
	"srv.exe.dev/srv/huggingface"
	"srv.exe.dev/srv/opml"
	"srv.exe.dev/srv/scrapers"
)

type Server struct {
	DB               *sql.DB
	Hostname         string
	TemplatesDir     string
	StaticDir        string
	Fetcher          *feeds.Fetcher
	ScraperRunner    *scrapers.Runner
	RetentionManager *RetentionManager
	ShelleyGenerator *ShelleyScraperGenerator
}

func New(dbPath, hostname string) (*Server, error) {
	_, thisFile, _, _ := runtime.Caller(0)
	baseDir := filepath.Dir(thisFile)
	srv := &Server{
		Hostname:      hostname,
		TemplatesDir:  filepath.Join(baseDir, "templates"),
		StaticDir:     filepath.Join(baseDir, "static"),
		ScraperRunner: scrapers.NewRunner(),
	}
	if err := srv.setUpDatabase(dbPath); err != nil {
		return nil, err
	}
	srv.Fetcher = feeds.NewFetcher(srv.DB, srv.ScraperRunner)
	return srv, nil
}

func (s *Server) setUpDatabase(dbPath string) error {
	wdb, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open db: %w", err)
	}
	s.DB = wdb
	if err := db.RunMigrations(wdb); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}
	return nil
}

func (s *Server) Serve(addr string) error {
	// Start background fetcher
	s.Fetcher.Start(5 * time.Minute)
	defer s.Fetcher.Stop()

	// Start retention manager (30 day retention)
	s.RetentionManager = NewRetentionManager(s, 30)
	s.RetentionManager.Start()
	defer s.RetentionManager.Stop()

	// Initialize Shelley scraper generator
	s.ShelleyGenerator = NewShelleyScraperGenerator()

	mux := http.NewServeMux()

	// Pages
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("GET /feeds", s.handleFeeds)
	mux.HandleFunc("GET /starred", s.handleStarred)
	mux.HandleFunc("GET /feed/{id}", s.handleFeedArticles)
	mux.HandleFunc("GET /article/{id}", s.handleArticle)
	mux.HandleFunc("GET /scrapers", s.handleScrapers)

	// API endpoints
	mux.HandleFunc("GET /api/counts", s.apiGetCounts)
	mux.HandleFunc("GET /api/feeds/{id}/status", s.apiGetFeedStatus)
	mux.HandleFunc("POST /api/feeds", s.apiCreateFeed)
	mux.HandleFunc("DELETE /api/feeds/{id}", s.apiDeleteFeed)
	mux.HandleFunc("PUT /api/feeds/{id}", s.apiUpdateFeed)
	mux.HandleFunc("GET /api/feeds/{id}", s.apiGetFeed)
	mux.HandleFunc("POST /api/feeds/{id}/refresh", s.apiRefreshFeed)
	mux.HandleFunc("POST /api/articles/{id}/read", s.apiMarkRead)
	mux.HandleFunc("POST /api/articles/{id}/unread", s.apiMarkUnread)
	mux.HandleFunc("POST /api/articles/{id}/star", s.apiToggleStar)
	mux.HandleFunc("POST /api/feeds/{id}/read-all", s.apiMarkFeedRead)
	mux.HandleFunc("POST /api/articles/read-all", s.apiMarkAllRead)
	mux.HandleFunc("GET /api/scrapers/{id}", s.apiGetScraper)
	mux.HandleFunc("POST /api/scrapers", s.apiCreateScraper)
	mux.HandleFunc("PUT /api/scrapers/{id}", s.apiUpdateScraper)
	mux.HandleFunc("DELETE /api/scrapers/{id}", s.apiDeleteScraper)
	mux.HandleFunc("GET /api/search", s.apiSearch)

	// Category endpoints
	mux.HandleFunc("GET /category/{id}", s.handleCategoryArticles)
	mux.HandleFunc("GET /api/categories/{id}/articles", s.apiGetCategoryArticles)
	mux.HandleFunc("POST /api/categories", s.apiCreateCategory)
	mux.HandleFunc("POST /api/categories/reorder", s.apiReorderCategories)
	mux.HandleFunc("POST /api/categories/{id}/parent", s.apiSetCategoryParent)
	mux.HandleFunc("PUT /api/categories/{id}", s.apiUpdateCategory)
	mux.HandleFunc("DELETE /api/categories/{id}", s.apiDeleteCategory)
	mux.HandleFunc("POST /api/feeds/{id}/category", s.apiSetFeedCategory)
	mux.HandleFunc("POST /api/categories/{id}/read-all", s.apiMarkCategoryRead)

	// OPML endpoints
	mux.HandleFunc("GET /api/opml/export", s.apiExportOPML)
	mux.HandleFunc("POST /api/opml/import", s.apiImportOPML)

	// Retention/cleanup endpoints
	mux.HandleFunc("GET /api/retention/stats", s.apiRetentionStats)
	mux.HandleFunc("POST /api/retention/cleanup", s.apiRetentionCleanup)
	mux.HandleFunc("GET /settings", s.handleSettings)

	// AI scraper generation
	mux.HandleFunc("GET /api/ai/status", s.apiAIStatus)
	mux.HandleFunc("POST /api/ai/generate-scraper", s.apiGenerateScraper)

	// Exclusion rules endpoints
	mux.HandleFunc("GET /api/categories/{id}/exclusions", s.apiListExclusions)
	mux.HandleFunc("POST /api/categories/{id}/exclusions", s.apiCreateExclusion)
	mux.HandleFunc("DELETE /api/exclusions/{id}", s.apiDeleteExclusion)
	mux.HandleFunc("GET /category/{id}/settings", s.handleCategorySettings)

	// Static files
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(s.StaticDir))))

	// Wrap with auth middleware
	handler := s.AuthMiddleware(mux)

	slog.Info("starting server", "addr", addr)
	return http.ListenAndServe(addr, handler)
}

// Template helpers
func (s *Server) renderTemplate(w http.ResponseWriter, name string, data any) error {
	funcMap := template.FuncMap{
		"timeAgo":    timeAgo,
		"formatDate": formatDate,
		"truncate":   truncate,
		"stripHTML":  stripHTML,
		"deref":      deref,
		"safeHTML":          safeHTML,
		"stripLeadingImage": stripLeadingImage,
		"multiply":   func(a, b int) int { return a * b },
	}
	path := filepath.Join(s.TemplatesDir, name)
	basePath := filepath.Join(s.TemplatesDir, "base.html")
	tmpl, err := template.New("base.html").Funcs(funcMap).ParseFiles(basePath, path)
	if err != nil {
		return fmt.Errorf("parse template %q: %w", name, err)
	}
	if err := tmpl.Execute(w, data); err != nil {
		return fmt.Errorf("execute template %q: %w", name, err)
	}
	return nil
}

func timeAgo(t *time.Time) string {
	if t == nil {
		return ""
	}
	diff := time.Since(*t)
	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		m := int(diff.Minutes())
		if m == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", m)
	case diff < 24*time.Hour:
		h := int(diff.Hours())
		if h == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", h)
	case diff < 7*24*time.Hour:
		d := int(diff.Hours() / 24)
		if d == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", d)
	default:
		return t.Format("Jan 2, 2006")
	}
}

func formatDate(t *time.Time) string {
	if t == nil {
		return ""
	}
	// Return ISO 8601 format for JavaScript to parse
	return t.UTC().Format(time.RFC3339)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func stripHTML(s string) string {
	var result strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
		} else if r == '>' {
			inTag = false
		} else if !inTag {
			result.WriteRune(r)
		}
	}
	return strings.TrimSpace(result.String())
}

// Page Handlers
// getCommonData returns data shared across all pages
func (s *Server) getCommonData(ctx context.Context) map[string]any {
	q := dbgen.New(s.DB)
	user := GetUser(ctx)
	if user == nil {
		return map[string]any{}
	}
	userID := user.ID

	feeds, _ := q.ListFeeds(ctx, &userID)
	unreadCount, _ := q.GetUnreadCount(ctx, &userID)
	starredCount, _ := q.GetStarredCount(ctx, &userID)
	categories, _ := q.ListCategories(ctx, &userID)

	feedCounts := make(map[int64]int64)
	for _, feed := range feeds {
		count, _ := q.GetFeedUnreadCount(ctx, feed.ID)
		feedCounts[feed.ID] = count
	}

	catCounts := make(map[int64]int64)
	for _, cat := range categories {
		count, _ := q.GetCategoryUnreadCount(ctx, cat.ID)
		catCounts[cat.ID] = count
	}

	// Get feed-to-category mapping
	feedCategories := make(map[int64]int64)
	for _, feed := range feeds {
		cats, _ := q.GetFeedCategories(ctx, feed.ID)
		if len(cats) > 0 {
			feedCategories[feed.ID] = cats[0].ID
		}
	}

	// Build category tree for hierarchical display
	categoryTree := BuildCategoryTree(categories)
	flatCategories := FlattenCategoryTree(categoryTree)

	return map[string]any{
		"Feeds":           feeds,
		"FeedCounts":      feedCounts,
		"Categories":      categories,
		"CategoryTree":    categoryTree,
		"FlatCategories":  flatCategories,
		"CategoryCounts":  catCounts,
		"FeedCategories":  feedCategories,
		"UnreadCount":     unreadCount,
		"StarredCount":    starredCount,
		"User":            user,
	}
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := dbgen.New(s.DB)
	user := GetUser(ctx)

	userID := user.ID
	articles, _ := q.ListUnreadArticles(ctx, dbgen.ListUnreadArticlesParams{UserID: &userID, Limit: 50, Offset: 0})

	data := s.getCommonData(ctx)
	data["Title"] = "All Unread"
	data["Articles"] = articles
	data["ActiveView"] = "unread"

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.renderTemplate(w, "index.html", data); err != nil {
		slog.Warn("render template", "error", err)
		http.Error(w, "Internal Server Error", 500)
	}
}

func (s *Server) handleFeeds(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := dbgen.New(s.DB)
	user := GetUser(ctx)

	scraperModules, _ := q.ListScraperModules(ctx, &user.ID)

	data := s.getCommonData(ctx)
	data["Title"] = "Manage Feeds"
	data["ScraperModules"] = scraperModules
	data["ActiveView"] = "feeds"

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.renderTemplate(w, "feeds.html", data); err != nil {
		slog.Warn("render template", "error", err)
		http.Error(w, "Internal Server Error", 500)
	}
}

func (s *Server) handleStarred(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := dbgen.New(s.DB)

	articles, _ := q.ListStarredArticles(ctx, dbgen.ListStarredArticlesParams{Limit: 50, Offset: 0})

	data := s.getCommonData(ctx)
	data["Title"] = "Starred"
	data["Articles"] = articles
	data["ActiveView"] = "starred"

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.renderTemplate(w, "index.html", data); err != nil {
		slog.Warn("render template", "error", err)
		http.Error(w, "Internal Server Error", 500)
	}
}

func (s *Server) handleFeedArticles(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := dbgen.New(s.DB)
	user := GetUser(ctx)

	feedID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid feed ID", 400)
		return
	}

	feed, err := q.GetFeed(ctx, dbgen.GetFeedParams{ID: feedID, UserID: &user.ID})
	if err != nil {
		http.Error(w, "Feed not found", 404)
		return
	}

	articles, _ := q.ListArticlesByFeed(ctx, dbgen.ListArticlesByFeedParams{FeedID: feedID, UserID: &user.ID, Limit: 100, Offset: 0})

	// Apply exclusion filters based on feed's category
	filteredArticles := s.FilterArticlesByFeed(ctx, articles, feedID, user.ID)

	data := s.getCommonData(ctx)
	data["Title"] = feed.Name
	data["Articles"] = filteredArticles
	data["ActiveFeed"] = feedID
	data["CurrentFeed"] = feed

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.renderTemplate(w, "index.html", data); err != nil {
		slog.Warn("render template", "error", err)
		http.Error(w, "Internal Server Error", 500)
	}
}

func (s *Server) handleArticle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := dbgen.New(s.DB)
	user := GetUser(ctx)

	articleID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid article ID", 400)
		return
	}

	article, err := q.GetArticle(ctx, dbgen.GetArticleParams{ID: articleID, UserID: &user.ID})
	if err != nil {
		http.Error(w, "Article not found", 404)
		return
	}

	// Mark as read
	q.MarkArticleRead(ctx, dbgen.MarkArticleReadParams{ID: articleID, UserID: &user.ID})

	feed, _ := q.GetFeed(ctx, dbgen.GetFeedParams{ID: article.FeedID, UserID: &user.ID})

	// Apply content filters if configured
	if article.Content != nil && feed.ContentFilters != nil {
		filtered := ApplyContentFilters(*article.Content, feed.ContentFilters)
		article.Content = &filtered
	}
	if article.Summary != nil && feed.ContentFilters != nil {
		filtered := ApplyContentFilters(*article.Summary, feed.ContentFilters)
		article.Summary = &filtered
	}

	data := s.getCommonData(ctx)
	data["Title"] = article.Title
	data["Article"] = article
	data["Feed"] = feed

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.renderTemplate(w, "article.html", data); err != nil {
		slog.Warn("render template", "error", err)
		http.Error(w, "Internal Server Error", 500)
	}
}

func (s *Server) handleScrapers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := dbgen.New(s.DB)
	user := GetUser(ctx)

	scraperModules, _ := q.ListScraperModules(ctx, &user.ID)

	data := s.getCommonData(ctx)
	data["Title"] = "Scraper Modules"
	data["ScraperModules"] = scraperModules
	data["ActiveView"] = "scrapers"

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.renderTemplate(w, "scrapers.html", data); err != nil {
		slog.Warn("render template", "error", err)
		http.Error(w, "Internal Server Error", 500)
	}
}

// API Handlers
func (s *Server) apiCreateFeed(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := dbgen.New(s.DB)
	user := GetUser(ctx)

	var req struct {
		Name          string `json:"name"`
		URL           string `json:"url"`
		FeedType      string `json:"feedType"`
		ScraperModule string `json:"scraperModule"`
		ScraperConfig string `json:"scraperConfig"`
		Interval      int64  `json:"interval"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request", 400)
		return
	}

	if req.URL == "" {
		jsonError(w, "URL is required", 400)
		return
	}

	if req.FeedType == "" {
		req.FeedType = "rss"
	}

	// Auto-convert Steam news URLs to RSS feed URLs
	// e.g. https://store.steampowered.com/news/app/4115450 -> https://store.steampowered.com/feeds/news/app/4115450
	if req.FeedType == "rss" {
		req.URL = convertSteamNewsURL(req.URL)
	}

	// Auto-generate name for Steam feeds
	if req.Name == "" && req.FeedType == "rss" {
		steamAppRe := regexp.MustCompile(`store\.steampowered\.com/feeds/news/app/(\d+)`)
		if m := steamAppRe.FindStringSubmatch(req.URL); m != nil {
			if name := fetchSteamAppName(m[1]); name != "" {
				req.Name = name
			}
		}
	}

	// Auto-generate name for HuggingFace feeds
	if req.Name == "" && req.FeedType == "huggingface" && req.ScraperConfig != "" {
		var hfConfig huggingface.FeedConfig
		if err := json.Unmarshal([]byte(req.ScraperConfig), &hfConfig); err == nil {
			hfClient := huggingface.NewClient("")
			if name, err := hfClient.GetFeedName(ctx, hfConfig); err == nil && name != "" {
				req.Name = name
			}
		}
	}

	if req.Name == "" {
		req.Name = req.URL
	}
	if req.Interval == 0 {
		req.Interval = 60
	}

	var scraperModule, scraperConfig *string
	if req.ScraperModule != "" {
		scraperModule = &req.ScraperModule
	}
	if req.ScraperConfig != "" {
		scraperConfig = &req.ScraperConfig
	}

	feed, err := q.CreateFeed(ctx, dbgen.CreateFeedParams{
		Name:                 req.Name,
		Url:                  req.URL,
		FeedType:             req.FeedType,
		ScraperModule:        scraperModule,
		ScraperConfig:        scraperConfig,
		FetchIntervalMinutes: &req.Interval,
		UserID:               &user.ID,
	})
	if err != nil {
		jsonError(w, "Failed to create feed: "+err.Error(), 500)
		return
	}

	// Trigger immediate fetch
	go s.Fetcher.FetchFeed(context.Background(), feed)

	jsonResponse(w, feed)
}

func (s *Server) apiDeleteFeed(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := dbgen.New(s.DB)
	user := GetUser(ctx)

	feedID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid feed ID", 400)
		return
	}

	if err := q.DeleteFeed(ctx, dbgen.DeleteFeedParams{ID: feedID, UserID: &user.ID}); err != nil {
		jsonError(w, "Failed to delete feed", 500)
		return
	}

	jsonResponse(w, map[string]string{"status": "ok"})
}

func (s *Server) apiGetFeed(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetUser(ctx)
	q := dbgen.New(s.DB)

	feedID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid feed ID", 400)
		return
	}

	feed, err := q.GetFeed(ctx, dbgen.GetFeedParams{ID: feedID, UserID: &user.ID})
	if err != nil {
		jsonError(w, "Feed not found", 404)
		return
	}

	jsonResponse(w, feed)
}

func (s *Server) apiUpdateFeed(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetUser(ctx)
	q := dbgen.New(s.DB)

	feedID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid feed ID", 400)
		return
	}

	// Verify feed exists and belongs to user
	feed, err := q.GetFeed(ctx, dbgen.GetFeedParams{ID: feedID, UserID: &user.ID})
	if err != nil {
		jsonError(w, "Feed not found", 404)
		return
	}

	var req struct {
		Name                 string  `json:"name"`
		URL                  string  `json:"url"`
		FeedType             string  `json:"feed_type"`
		ScraperModule        *string `json:"scraper_module"`
		ScraperConfig        *string `json:"scraper_config"`
		FetchIntervalMinutes *int64  `json:"fetch_interval_minutes"`
		ContentFilters       *string `json:"content_filters"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request body", 400)
		return
	}

	// Use existing values if not provided
	name := req.Name
	if name == "" {
		name = feed.Name
	}
	url := req.URL
	if url == "" {
		url = feed.Url
	}
	feedType := req.FeedType
	if feedType == "" {
		feedType = feed.FeedType
	}
	scraperModule := req.ScraperModule
	if scraperModule == nil {
		scraperModule = feed.ScraperModule
	}
	scraperConfig := req.ScraperConfig
	if scraperConfig == nil {
		scraperConfig = feed.ScraperConfig
	}
	interval := req.FetchIntervalMinutes
	if interval == nil {
		interval = feed.FetchIntervalMinutes
	}
	contentFilters := req.ContentFilters
	if contentFilters == nil {
		contentFilters = feed.ContentFilters
	}

	if err := q.UpdateFeed(ctx, dbgen.UpdateFeedParams{
		Name:                 name,
		Url:                  url,
		FeedType:             feedType,
		ScraperModule:        scraperModule,
		ScraperConfig:        scraperConfig,
		FetchIntervalMinutes: interval,
		ContentFilters:       contentFilters,
		ID:                   feedID,
		UserID:               &user.ID,
	}); err != nil {
		jsonError(w, "Failed to update feed", 500)
		return
	}

	jsonResponse(w, map[string]string{"status": "ok"})
}

func (s *Server) apiRefreshFeed(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetUser(ctx)
	q := dbgen.New(s.DB)

	feedID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid feed ID", 400)
		return
	}

	feed, err := q.GetFeed(ctx, dbgen.GetFeedParams{ID: feedID, UserID: &user.ID})
	if err != nil {
		jsonError(w, "Feed not found", 404)
		return
	}

	go func() {
		slog.Info("starting manual feed refresh", "feed_id", feed.ID, "name", feed.Name)
		if err := s.Fetcher.FetchFeed(context.Background(), feed); err != nil {
			slog.Warn("manual feed refresh failed", "feed_id", feed.ID, "error", err)
		} else {
			slog.Info("manual feed refresh completed", "feed_id", feed.ID)
		}
	}()

	jsonResponse(w, map[string]string{"status": "refreshing"})
}

func (s *Server) apiGetCounts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetUser(ctx)
	q := dbgen.New(s.DB)
	userID := user.ID

	feeds, _ := q.ListFeeds(ctx, &userID)
	categories, _ := q.ListCategories(ctx, &userID)

	unreadCount, _ := q.GetUnreadCount(ctx, &userID)
	starredCount, _ := q.GetStarredCount(ctx, &userID)

	feedCounts := make(map[int64]int64)
	feedErrors := make(map[int64]string)
	for _, feed := range feeds {
		count, _ := q.GetFeedUnreadCount(ctx, feed.ID)
		feedCounts[feed.ID] = count
		if feed.LastError != nil && *feed.LastError != "" {
			feedErrors[feed.ID] = *feed.LastError
		}
	}

	catCounts := make(map[int64]int64)
	for _, cat := range categories {
		count, _ := q.GetCategoryUnreadCount(ctx, cat.ID)
		catCounts[cat.ID] = count
	}

	jsonResponse(w, map[string]any{
		"unread":     unreadCount,
		"starred":    starredCount,
		"feeds":      feedCounts,
		"categories": catCounts,
		"feedErrors": feedErrors,
	})
}

func (s *Server) apiGetFeedStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetUser(ctx)
	q := dbgen.New(s.DB)

	feedID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid feed ID", 400)
		return
	}

	feed, err := q.GetFeed(ctx, dbgen.GetFeedParams{ID: feedID, UserID: &user.ID})
	if err != nil {
		jsonError(w, "Feed not found", 404)
		return
	}

	var lastFetched, lastError string
	if feed.LastFetchedAt != nil {
		lastFetched = feed.LastFetchedAt.Format(time.RFC3339)
	}
	if feed.LastError != nil {
		lastError = *feed.LastError
	}

	jsonResponse(w, map[string]any{
		"id":           feed.ID,
		"name":         feed.Name,
		"lastFetched": lastFetched,
		"lastError":    lastError,
	})
}

func (s *Server) apiMarkRead(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetUser(ctx)
	q := dbgen.New(s.DB)

	articleID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid article ID", 400)
		return
	}

	if err := q.MarkArticleRead(ctx, dbgen.MarkArticleReadParams{ID: articleID, UserID: &user.ID}); err != nil {
		jsonError(w, "Failed to mark read", 500)
		return
	}

	jsonResponse(w, map[string]string{"status": "ok"})
}

func (s *Server) apiMarkUnread(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetUser(ctx)
	q := dbgen.New(s.DB)

	articleID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid article ID", 400)
		return
	}

	if err := q.MarkArticleUnread(ctx, dbgen.MarkArticleUnreadParams{ID: articleID, UserID: &user.ID}); err != nil {
		jsonError(w, "Failed to mark unread", 500)
		return
	}

	jsonResponse(w, map[string]string{"status": "ok"})
}

func (s *Server) apiToggleStar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetUser(ctx)
	q := dbgen.New(s.DB)

	articleID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid article ID", 400)
		return
	}

	if err := q.ToggleArticleStar(ctx, dbgen.ToggleArticleStarParams{ID: articleID, UserID: &user.ID}); err != nil {
		jsonError(w, "Failed to toggle star", 500)
		return
	}

	jsonResponse(w, map[string]string{"status": "ok"})
}

func (s *Server) apiMarkAllRead(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetUser(ctx)
	q := dbgen.New(s.DB)

	age := r.URL.Query().Get("age")
	oneDay := "1"
	oneWeek := "7"
	var err error
	switch age {
	case "day":
		err = q.MarkAllArticlesReadOlderThan(ctx, dbgen.MarkAllArticlesReadOlderThanParams{Column1: &oneDay, UserID: &user.ID})
	case "week":
		err = q.MarkAllArticlesReadOlderThan(ctx, dbgen.MarkAllArticlesReadOlderThanParams{Column1: &oneWeek, UserID: &user.ID})
	default:
		err = q.MarkAllArticlesRead(ctx, &user.ID)
	}

	if err != nil {
		jsonError(w, "Failed to mark all read", 500)
		return
	}

	jsonResponse(w, map[string]string{"status": "ok"})
}

func (s *Server) apiMarkFeedRead(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetUser(ctx)
	q := dbgen.New(s.DB)

	feedID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid feed ID", 400)
		return
	}

	age := r.URL.Query().Get("age")
	oneDay := "1"
	oneWeek := "7"
	switch age {
	case "day":
		err = q.MarkFeedArticlesReadOlderThan(ctx, dbgen.MarkFeedArticlesReadOlderThanParams{
			FeedID:  feedID,
			Column2: &oneDay,
			UserID:  &user.ID,
		})
	case "week":
		err = q.MarkFeedArticlesReadOlderThan(ctx, dbgen.MarkFeedArticlesReadOlderThanParams{
			FeedID:  feedID,
			Column2: &oneWeek,
			UserID:  &user.ID,
		})
	default:
		err = q.MarkFeedRead(ctx, dbgen.MarkFeedReadParams{FeedID: feedID, UserID: &user.ID})
	}

	if err != nil {
		jsonError(w, "Failed to mark feed read", 500)
		return
	}

	jsonResponse(w, map[string]string{"status": "ok"})
}

func (s *Server) apiGetScraper(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetUser(ctx)
	q := dbgen.New(s.DB)

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid scraper ID", 400)
		return
	}

	scraper, err := q.GetScraperModule(ctx, dbgen.GetScraperModuleParams{ID: id, UserID: &user.ID})
	if err != nil {
		jsonError(w, "Scraper not found", 404)
		return
	}

	jsonResponse(w, scraper)
}

func (s *Server) apiCreateScraper(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := dbgen.New(s.DB)
	user := GetUser(ctx)

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Script      string `json:"script"`
		ScriptType  string `json:"scriptType"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request", 400)
		return
	}

	if req.Name == "" || req.Script == "" {
		jsonError(w, "Name and script are required", 400)
		return
	}

	if req.ScriptType == "" {
		req.ScriptType = "json"
	}

	var desc *string
	if req.Description != "" {
		desc = &req.Description
	}

	reqID := r.Header.Get("X-Request-Id")
	slog.Info("creating scraper module", "name", req.Name, "user_id", user.ID, "request_id", reqID, "remote", r.RemoteAddr, "user_agent", r.UserAgent())
	module, err := q.CreateScraperModule(ctx, dbgen.CreateScraperModuleParams{
		Name:        req.Name,
		Description: desc,
		Script:      req.Script,
		ScriptType:  req.ScriptType,
		UserID:      &user.ID,
	})
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			jsonError(w, "A scraper with that name already exists", 409)
		} else {
			jsonError(w, "Failed to create scraper: "+err.Error(), 500)
		}
		return
	}

	jsonResponse(w, module)
}

func (s *Server) apiUpdateScraper(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := dbgen.New(s.DB)

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid ID", 400)
		return
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Script      string `json:"script"`
		ScriptType  string `json:"scriptType"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request", 400)
		return
	}

	var desc *string
	if req.Description != "" {
		desc = &req.Description
	}

	if err := q.UpdateScraperModule(ctx, dbgen.UpdateScraperModuleParams{
		ID:          id,
		Name:        req.Name,
		Description: desc,
		Script:      req.Script,
		ScriptType:  req.ScriptType,
	}); err != nil {
		jsonError(w, "Failed to update scraper", 500)
		return
	}

	jsonResponse(w, map[string]string{"status": "ok"})
}

func (s *Server) apiDeleteScraper(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetUser(ctx)
	q := dbgen.New(s.DB)

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid ID", 400)
		return
	}

	if err := q.DeleteScraperModule(ctx, dbgen.DeleteScraperModuleParams{ID: id, UserID: &user.ID}); err != nil {
		jsonError(w, "Failed to delete scraper", 500)
		return
	}

	jsonResponse(w, map[string]string{"status": "ok"})
}

func (s *Server) apiSearch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetUser(ctx)
	q := dbgen.New(s.DB)

	query := r.URL.Query().Get("q")
	if query == "" {
		jsonResponse(w, []any{})
		return
	}

	articles, err := q.SearchArticles(ctx, dbgen.SearchArticlesParams{
		UserID:  &user.ID,
		Column2: &query,
		Column3: &query,
		Limit:   50,
		Offset:  0,
	})
	if err != nil {
		jsonError(w, "Search failed", 500)
		return
	}

	jsonResponse(w, articles)
}

func jsonResponse(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func deref(p any) any {
	if p == nil {
		return ""
	}
	switch v := p.(type) {
	case *string:
		if v == nil {
			return ""
		}
		return *v
	case *int64:
		if v == nil {
			return 0
		}
		return *v
	case *time.Time:
		if v == nil {
			return nil
		}
		return *v
	default:
		return p
	}
}

func safeHTML(s string) template.HTML {
	return template.HTML(s)
}

// fetchSteamAppName gets the game name from the Steam store API
func fetchSteamAppName(appID string) string {
	resp, err := http.Get("https://store.steampowered.com/api/appdetails?appids=" + appID)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
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

// convertSteamNewsURL converts Steam store news URLs to their RSS feed equivalents
func convertSteamNewsURL(url string) string {
	// Match https://store.steampowered.com/news/app/DIGITS with optional trailing slash/params
	steamNewsRe := regexp.MustCompile(`^(https?://store\.steampowered\.com)/news/(app/\d+)/?.*$`)
	if m := steamNewsRe.FindStringSubmatch(url); m != nil {
		return m[1] + "/feeds/news/" + m[2]
	}
	return url
}

// stripLeadingImage removes the first <img> tag from content if its src matches the given URL
func stripLeadingImage(content string, imageURL string) string {
	if imageURL == "" {
		return content
	}
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "<img") {
		return content
	}
	// Find the end of the first img tag
	end := strings.Index(trimmed, ">")
	if end == -1 {
		return content
	}
	imgTag := trimmed[:end+1]
	if strings.Contains(imgTag, imageURL) {
		return strings.TrimSpace(trimmed[end+1:])
	}
	return content
}

// Category handlers
func (s *Server) handleCategoryArticles(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetUser(ctx)
	q := dbgen.New(s.DB)

	catID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid category ID", 400)
		return
	}

	categories, _ := q.ListCategories(ctx, &user.ID)
	var category *dbgen.Category
	for _, c := range categories {
		if c.ID == catID {
			catCopy := c
			category = &catCopy
			break
		}
	}
	if category == nil {
		http.Error(w, "Category not found", 404)
		return
	}

	articles, _ := q.ListUnreadArticlesByCategory(ctx, dbgen.ListUnreadArticlesByCategoryParams{
		CategoryID: catID,
		UserID:     &user.ID,
		Limit:      100, // Fetch more to account for filtering
		Offset:     0,
	})

	// Apply exclusion filters
	filteredArticles := s.FilterArticlesByCategory(ctx, articles, catID, user.ID)

	data := s.getCommonData(ctx)
	data["Title"] = category.Name
	data["Articles"] = filteredArticles
	data["ActiveCategory"] = catID
	data["CurrentCategory"] = category

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.renderTemplate(w, "index.html", data); err != nil {
		slog.Warn("render template", "error", err)
		http.Error(w, "Internal Server Error", 500)
	}
}

func (s *Server) apiGetCategoryArticles(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetUser(ctx)
	q := dbgen.New(s.DB)

	catID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid category ID", 400)
		return
	}

	categories, _ := q.ListCategories(ctx, &user.ID)
	var category *dbgen.Category
	for _, c := range categories {
		if c.ID == catID {
			catCopy := c
			category = &catCopy
			break
		}
	}
	if category == nil {
		jsonError(w, "Category not found", 404)
		return
	}

	articles, _ := q.ListUnreadArticlesByCategory(ctx, dbgen.ListUnreadArticlesByCategoryParams{
		CategoryID: catID,
		UserID:     &user.ID,
		Limit:      100,
		Offset:     0,
	})

	// Apply exclusion filters
	filteredArticles := s.FilterArticlesByCategory(ctx, articles, catID, user.ID)

	jsonResponse(w, map[string]any{
		"category": category,
		"articles": filteredArticles,
	})
}

func (s *Server) apiReorderCategories(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetUser(ctx)
	q := dbgen.New(s.DB)

	var req struct {
		Order    []int64 `json:"order"`     // Category IDs in new order
		ParentID *int64  `json:"parent_id"` // Optional: only reorder within this parent
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request", 400)
		return
	}

	// Update sort_order for each category
	for i, catID := range req.Order {
		sortOrder := int64(i)
		q.UpdateCategorySortOrder(ctx, dbgen.UpdateCategorySortOrderParams{
			SortOrder: &sortOrder,
			ID:        catID,
			UserID:    &user.ID,
		})
	}

	jsonResponse(w, map[string]string{"status": "ok"})
}

func (s *Server) apiSetCategoryParent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetUser(ctx)
	q := dbgen.New(s.DB)

	catID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid category ID", 400)
		return
	}

	var req struct {
		ParentID  *int64 `json:"parent_id"`  // null = top level
		SortOrder int64  `json:"sort_order"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request", 400)
		return
	}

	// Prevent circular references
	if req.ParentID != nil && *req.ParentID == catID {
		jsonError(w, "Cannot set category as its own parent", 400)
		return
	}

	err = q.UpdateCategoryParent(ctx, dbgen.UpdateCategoryParentParams{
		ParentID:  req.ParentID,
		SortOrder: &req.SortOrder,
		ID:        catID,
		UserID:    &user.ID,
	})
	if err != nil {
		jsonError(w, "Failed to update category", 500)
		return
	}

	jsonResponse(w, map[string]string{"status": "ok"})
}

func (s *Server) apiCreateCategory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetUser(ctx)
	q := dbgen.New(s.DB)

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request", 400)
		return
	}

	if req.Name == "" {
		jsonError(w, "Name is required", 400)
		return
	}

	cat, err := q.CreateCategory(ctx, dbgen.CreateCategoryParams{Name: req.Name, UserID: &user.ID})
	if err != nil {
		jsonError(w, "Failed to create category: "+err.Error(), 500)
		return
	}

	jsonResponse(w, cat)
}

func (s *Server) apiUpdateCategory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetUser(ctx)
	q := dbgen.New(s.DB)

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid ID", 400)
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request", 400)
		return
	}

	if err := q.UpdateCategory(ctx, dbgen.UpdateCategoryParams{Name: req.Name, ID: id, UserID: &user.ID}); err != nil {
		jsonError(w, "Failed to update category", 500)
		return
	}

	jsonResponse(w, map[string]string{"status": "ok"})
}

func (s *Server) apiDeleteCategory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetUser(ctx)
	q := dbgen.New(s.DB)

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid ID", 400)
		return
	}

	if err := q.DeleteCategory(ctx, dbgen.DeleteCategoryParams{ID: id, UserID: &user.ID}); err != nil {
		jsonError(w, "Failed to delete category", 500)
		return
	}

	jsonResponse(w, map[string]string{"status": "ok"})
}

func (s *Server) apiSetFeedCategory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := dbgen.New(s.DB)

	feedID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid feed ID", 400)
		return
	}

	var req struct {
		CategoryID int64 `json:"categoryId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request", 400)
		return
	}

	// Clear existing categories for this feed
	if err := q.ClearFeedCategories(ctx, feedID); err != nil {
		slog.Warn("clear feed categories", "error", err)
	}

	// Add to new category if specified
	if req.CategoryID > 0 {
		if err := q.AddFeedToCategory(ctx, dbgen.AddFeedToCategoryParams{
			FeedID:     feedID,
			CategoryID: req.CategoryID,
		}); err != nil {
			jsonError(w, "Failed to set category", 500)
			return
		}
	}

	jsonResponse(w, map[string]string{"status": "ok"})
}

func (s *Server) apiMarkCategoryRead(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetUser(ctx)
	q := dbgen.New(s.DB)

	catID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid category ID", 400)
		return
	}

	age := r.URL.Query().Get("age")
	oneDay := "1"
	oneWeek := "7"
	switch age {
	case "day":
		err = q.MarkCategoryArticlesReadOlderThan(ctx, dbgen.MarkCategoryArticlesReadOlderThanParams{
			CategoryID: catID,
			UserID:     &user.ID,
			Column3:    &oneDay,
		})
	case "week":
		err = q.MarkCategoryArticlesReadOlderThan(ctx, dbgen.MarkCategoryArticlesReadOlderThanParams{
			CategoryID: catID,
			UserID:     &user.ID,
			Column3:    &oneWeek,
		})
	default:
		err = q.MarkCategoryRead(ctx, dbgen.MarkCategoryReadParams{CategoryID: catID, UserID: &user.ID})
	}

	if err != nil {
		jsonError(w, "Failed to mark category read", 500)
		return
	}

	jsonResponse(w, map[string]string{"status": "ok"})
}

// OPML handlers
func (s *Server) apiExportOPML(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetUser(ctx)
	q := dbgen.New(s.DB)

	feeds, err := q.ListFeeds(ctx, &user.ID)
	if err != nil {
		jsonError(w, "Failed to list feeds", 500)
		return
	}

	var exportFeeds []opml.ExportFeed
	for _, feed := range feeds {
		cats, _ := q.GetFeedCategories(ctx, feed.ID)
		catName := ""
		if len(cats) > 0 {
			catName = cats[0].Name
		}
		exportFeeds = append(exportFeeds, opml.ExportFeed{
			Name:     feed.Name,
			URL:      feed.Url,
			Category: catName,
		})
	}

	data, err := opml.Export(exportFeeds, "FeedReader Export")
	if err != nil {
		jsonError(w, "Failed to generate OPML", 500)
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("Content-Disposition", "attachment; filename=feedreader-export.opml")
	w.Write(data)
}

func (s *Server) apiImportOPML(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetUser(ctx)
	q := dbgen.New(s.DB)

	// Handle both multipart form and raw body
	var reader io.Reader
	if r.Header.Get("Content-Type") == "application/xml" || r.Header.Get("Content-Type") == "text/xml" {
		reader = r.Body
	} else {
		// Try multipart
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			jsonError(w, "Failed to parse form", 400)
			return
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			jsonError(w, "No file uploaded", 400)
			return
		}
		defer file.Close()
		reader = file
	}

	feeds, err := opml.Parse(reader)
	if err != nil {
		jsonError(w, "Failed to parse OPML: "+err.Error(), 400)
		return
	}

	var imported, skipped int
	var importedFeeds []dbgen.Feed
	for _, feed := range feeds {
		// Get or create category
		var catID int64
		if feed.Category != "" {
			cat, err := q.GetCategoryByName(ctx, dbgen.GetCategoryByNameParams{Name: feed.Category, UserID: &user.ID})
			if err != nil {
				// Create category
				cat, err = q.CreateCategory(ctx, dbgen.CreateCategoryParams{Name: feed.Category, UserID: &user.ID})
				if err != nil {
					slog.Warn("create category", "error", err, "name", feed.Category)
				}
			}
			if err == nil {
				catID = cat.ID
			}
		}

		// Check if feed already exists
		_, err := q.GetFeedByURL(ctx, dbgen.GetFeedByURLParams{Url: feed.URL, UserID: &user.ID})
		if err == nil {
			slog.Debug("import: feed already exists", "url", feed.URL)
			skipped++
			continue
		}

		// Create feed
		interval := int64(60)
		newFeed, err := q.CreateFeed(ctx, dbgen.CreateFeedParams{
			Name:                 feed.Name,
			Url:                  feed.URL,
			FeedType:             "rss",
			FetchIntervalMinutes: &interval,
			UserID:               &user.ID,
		})
		if err != nil {
			slog.Warn("import: create feed failed", "error", err, "url", feed.URL, "name", feed.Name)
			skipped++
			continue
		}

		// Assign to category
		if catID > 0 {
			q.AddFeedToCategory(ctx, dbgen.AddFeedToCategoryParams{
				FeedID:     newFeed.ID,
				CategoryID: catID,
			})
		}

		imported++
		importedFeeds = append(importedFeeds, newFeed)
	}

	// Queue fetches for all imported feeds
	// Run in background so we can return the response immediately
	go func() {
		for _, feed := range importedFeeds {
			s.Fetcher.FetchFeed(context.Background(), feed)
		}
	}()

	jsonResponse(w, map[string]any{
		"imported": imported,
		"skipped":  skipped,
		"total":    len(feeds),
	})
}

// Exclusion handlers
func (s *Server) apiListExclusions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetUser(ctx)
	q := dbgen.New(s.DB)

	catID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid category ID", 400)
		return
	}

	exclusions, err := q.ListExclusionsByCategory(ctx, dbgen.ListExclusionsByCategoryParams{CategoryID: catID, UserID: &user.ID})
	if err != nil {
		jsonError(w, "Failed to list exclusions", 500)
		return
	}

	jsonResponse(w, exclusions)
}

func (s *Server) apiCreateExclusion(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := dbgen.New(s.DB)

	catID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid category ID", 400)
		return
	}

	var req struct {
		Type    string `json:"type"`    // "author" or "keyword"
		Pattern string `json:"pattern"`
		IsRegex bool   `json:"isRegex"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request", 400)
		return
	}

	if req.Type != "author" && req.Type != "keyword" {
		jsonError(w, "Type must be 'author' or 'keyword'", 400)
		return
	}
	if req.Pattern == "" {
		jsonError(w, "Pattern is required", 400)
		return
	}

	var isRegex int64
	if req.IsRegex {
		isRegex = 1
	}

	exclusion, err := q.CreateExclusion(ctx, dbgen.CreateExclusionParams{
		CategoryID:    catID,
		ExclusionType: req.Type,
		Pattern:       req.Pattern,
		IsRegex:       &isRegex,
	})
	if err != nil {
		jsonError(w, "Failed to create exclusion: "+err.Error(), 500)
		return
	}

	jsonResponse(w, exclusion)
}

func (s *Server) apiDeleteExclusion(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetUser(ctx)
	q := dbgen.New(s.DB)

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid ID", 400)
		return
	}

	if err := q.DeleteExclusion(ctx, dbgen.DeleteExclusionParams{ID: id, UserID: &user.ID}); err != nil {
		jsonError(w, "Failed to delete exclusion", 500)
		return
	}

	jsonResponse(w, map[string]string{"status": "ok"})
}

func (s *Server) handleCategorySettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetUser(ctx)
	q := dbgen.New(s.DB)

	catID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid category ID", 400)
		return
	}

	categories, _ := q.ListCategories(ctx, &user.ID)
	var category *dbgen.Category
	for _, c := range categories {
		if c.ID == catID {
			catCopy := c
			category = &catCopy
			break
		}
	}
	if category == nil {
		http.Error(w, "Category not found", 404)
		return
	}

	exclusions, _ := q.ListExclusionsByCategory(ctx, dbgen.ListExclusionsByCategoryParams{CategoryID: catID, UserID: &user.ID})

	data := s.getCommonData(ctx)
	data["Title"] = category.Name + " Settings"
	data["CurrentCategory"] = category
	data["Exclusions"] = exclusions
	data["ActiveView"] = "settings"

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.renderTemplate(w, "category_settings.html", data); err != nil {
		slog.Warn("render template", "error", err)
		http.Error(w, "Internal Server Error", 500)
	}
}

// Retention API handlers
func (s *Server) apiRetentionStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	stats, err := s.RetentionManager.GetStats(ctx)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	
	jsonResponse(w, stats)
}

func (s *Server) apiRetentionCleanup(w http.ResponseWriter, r *http.Request) {
	deleted, err := s.RetentionManager.RunCleanupNow()
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	
	jsonResponse(w, map[string]any{
		"deleted": deleted,
		"message": "Cleanup completed",
	})
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	stats, _ := s.RetentionManager.GetStats(ctx)
	
	data := s.getCommonData(ctx)
	data["Title"] = "Settings"
	data["RetentionStats"] = stats
	
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.renderTemplate(w, "settings.html", data); err != nil {
		slog.Warn("render template", "error", err)
		http.Error(w, "Internal Server Error", 500)
	}
}

// AI Scraper API handlers
func (s *Server) apiAIStatus(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, map[string]any{
		"available": s.ShelleyGenerator.IsAvailable(),
	})
}

func (s *Server) apiGenerateScraper(w http.ResponseWriter, r *http.Request) {
	if !s.ShelleyGenerator.IsAvailable() {
		jsonError(w, "Shelley is not available. Make sure the Shelley service is running.", 503)
		return
	}

	var req GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request body", 400)
		return
	}

	if req.URL == "" {
		jsonError(w, "URL is required", 400)
		return
	}
	if req.Description == "" {
		jsonError(w, "Description is required", 400)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	resp, err := s.ShelleyGenerator.Generate(ctx, req)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}

	jsonResponse(w, resp)
}

package srv

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"srv.exe.dev/db"
	"srv.exe.dev/db/dbgen"
	"srv.exe.dev/srv/feeds"
	"srv.exe.dev/srv/scrapers"
)

type Server struct {
	DB            *sql.DB
	Hostname      string
	TemplatesDir  string
	StaticDir     string
	Fetcher       *feeds.Fetcher
	ScraperRunner *scrapers.Runner
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

	mux := http.NewServeMux()

	// Pages
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("GET /feeds", s.handleFeeds)
	mux.HandleFunc("GET /starred", s.handleStarred)
	mux.HandleFunc("GET /feed/{id}", s.handleFeedArticles)
	mux.HandleFunc("GET /article/{id}", s.handleArticle)
	mux.HandleFunc("GET /scrapers", s.handleScrapers)

	// API endpoints
	mux.HandleFunc("POST /api/feeds", s.apiCreateFeed)
	mux.HandleFunc("DELETE /api/feeds/{id}", s.apiDeleteFeed)
	mux.HandleFunc("POST /api/feeds/{id}/refresh", s.apiRefreshFeed)
	mux.HandleFunc("POST /api/articles/{id}/read", s.apiMarkRead)
	mux.HandleFunc("POST /api/articles/{id}/unread", s.apiMarkUnread)
	mux.HandleFunc("POST /api/articles/{id}/star", s.apiToggleStar)
	mux.HandleFunc("POST /api/feeds/{id}/read-all", s.apiMarkFeedRead)
	mux.HandleFunc("POST /api/scrapers", s.apiCreateScraper)
	mux.HandleFunc("PUT /api/scrapers/{id}", s.apiUpdateScraper)
	mux.HandleFunc("DELETE /api/scrapers/{id}", s.apiDeleteScraper)
	mux.HandleFunc("GET /api/search", s.apiSearch)

	// Static files
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(s.StaticDir))))

	slog.Info("starting server", "addr", addr)
	return http.ListenAndServe(addr, mux)
}

// Template helpers
func (s *Server) renderTemplate(w http.ResponseWriter, name string, data any) error {
	funcMap := template.FuncMap{
		"timeAgo":   timeAgo,
		"truncate":  truncate,
		"stripHTML": stripHTML,
		"deref":     deref,
		"safeHTML":  safeHTML,
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
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := dbgen.New(s.DB)

	articles, _ := q.ListUnreadArticles(ctx, dbgen.ListUnreadArticlesParams{Limit: 50, Offset: 0})
	feeds, _ := q.ListFeeds(ctx)
	unreadCount, _ := q.GetUnreadCount(ctx)
	starredCount, _ := q.GetStarredCount(ctx)

	// Get unread counts per feed
	feedCounts := make(map[int64]int64)
	for _, feed := range feeds {
		count, _ := q.GetFeedUnreadCount(ctx, feed.ID)
		feedCounts[feed.ID] = count
	}

	data := map[string]any{
		"Title":        "All Unread",
		"Articles":     articles,
		"Feeds":        feeds,
		"FeedCounts":   feedCounts,
		"UnreadCount":  unreadCount,
		"StarredCount": starredCount,
		"ActiveView":   "unread",
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.renderTemplate(w, "index.html", data); err != nil {
		slog.Warn("render template", "error", err)
		http.Error(w, "Internal Server Error", 500)
	}
}

func (s *Server) handleFeeds(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := dbgen.New(s.DB)

	feeds, _ := q.ListFeeds(ctx)
	unreadCount, _ := q.GetUnreadCount(ctx)
	starredCount, _ := q.GetStarredCount(ctx)
	scraperModules, _ := q.ListScraperModules(ctx)

	feedCounts := make(map[int64]int64)
	for _, feed := range feeds {
		count, _ := q.GetFeedUnreadCount(ctx, feed.ID)
		feedCounts[feed.ID] = count
	}

	data := map[string]any{
		"Title":          "Manage Feeds",
		"Feeds":          feeds,
		"FeedCounts":     feedCounts,
		"UnreadCount":    unreadCount,
		"StarredCount":   starredCount,
		"ScraperModules": scraperModules,
		"ActiveView":     "feeds",
	}

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
	feeds, _ := q.ListFeeds(ctx)
	unreadCount, _ := q.GetUnreadCount(ctx)
	starredCount, _ := q.GetStarredCount(ctx)

	feedCounts := make(map[int64]int64)
	for _, feed := range feeds {
		count, _ := q.GetFeedUnreadCount(ctx, feed.ID)
		feedCounts[feed.ID] = count
	}

	data := map[string]any{
		"Title":        "Starred",
		"Articles":     articles,
		"Feeds":        feeds,
		"FeedCounts":   feedCounts,
		"UnreadCount":  unreadCount,
		"StarredCount": starredCount,
		"ActiveView":   "starred",
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.renderTemplate(w, "index.html", data); err != nil {
		slog.Warn("render template", "error", err)
		http.Error(w, "Internal Server Error", 500)
	}
}

func (s *Server) handleFeedArticles(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := dbgen.New(s.DB)

	feedID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid feed ID", 400)
		return
	}

	feed, err := q.GetFeed(ctx, feedID)
	if err != nil {
		http.Error(w, "Feed not found", 404)
		return
	}

	articles, _ := q.ListArticlesByFeed(ctx, dbgen.ListArticlesByFeedParams{FeedID: feedID, Limit: 50, Offset: 0})
	feeds, _ := q.ListFeeds(ctx)
	unreadCount, _ := q.GetUnreadCount(ctx)
	starredCount, _ := q.GetStarredCount(ctx)

	feedCounts := make(map[int64]int64)
	for _, f := range feeds {
		count, _ := q.GetFeedUnreadCount(ctx, f.ID)
		feedCounts[f.ID] = count
	}

	data := map[string]any{
		"Title":        feed.Name,
		"Articles":     articles,
		"Feeds":        feeds,
		"FeedCounts":   feedCounts,
		"UnreadCount":  unreadCount,
		"StarredCount": starredCount,
		"ActiveFeed":   feedID,
		"CurrentFeed":  feed,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.renderTemplate(w, "index.html", data); err != nil {
		slog.Warn("render template", "error", err)
		http.Error(w, "Internal Server Error", 500)
	}
}

func (s *Server) handleArticle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := dbgen.New(s.DB)

	articleID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid article ID", 400)
		return
	}

	article, err := q.GetArticle(ctx, articleID)
	if err != nil {
		http.Error(w, "Article not found", 404)
		return
	}

	// Mark as read
	q.MarkArticleRead(ctx, articleID)

	feed, _ := q.GetFeed(ctx, article.FeedID)
	feeds, _ := q.ListFeeds(ctx)
	unreadCount, _ := q.GetUnreadCount(ctx)
	starredCount, _ := q.GetStarredCount(ctx)

	feedCounts := make(map[int64]int64)
	for _, f := range feeds {
		count, _ := q.GetFeedUnreadCount(ctx, f.ID)
		feedCounts[f.ID] = count
	}

	data := map[string]any{
		"Title":        article.Title,
		"Article":      article,
		"Feed":         feed,
		"Feeds":        feeds,
		"FeedCounts":   feedCounts,
		"UnreadCount":  unreadCount,
		"StarredCount": starredCount,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.renderTemplate(w, "article.html", data); err != nil {
		slog.Warn("render template", "error", err)
		http.Error(w, "Internal Server Error", 500)
	}
}

func (s *Server) handleScrapers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := dbgen.New(s.DB)

	scraperModules, _ := q.ListScraperModules(ctx)
	feeds, _ := q.ListFeeds(ctx)
	unreadCount, _ := q.GetUnreadCount(ctx)
	starredCount, _ := q.GetStarredCount(ctx)

	feedCounts := make(map[int64]int64)
	for _, f := range feeds {
		count, _ := q.GetFeedUnreadCount(ctx, f.ID)
		feedCounts[f.ID] = count
	}

	data := map[string]any{
		"Title":          "Scraper Modules",
		"ScraperModules": scraperModules,
		"Feeds":          feeds,
		"FeedCounts":     feedCounts,
		"UnreadCount":    unreadCount,
		"StarredCount":   starredCount,
		"ActiveView":     "scrapers",
	}

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

	if req.Name == "" {
		req.Name = req.URL
	}
	if req.FeedType == "" {
		req.FeedType = "rss"
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

	feedID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid feed ID", 400)
		return
	}

	if err := q.DeleteFeed(ctx, feedID); err != nil {
		jsonError(w, "Failed to delete feed", 500)
		return
	}

	jsonResponse(w, map[string]string{"status": "ok"})
}

func (s *Server) apiRefreshFeed(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := dbgen.New(s.DB)

	feedID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid feed ID", 400)
		return
	}

	feed, err := q.GetFeed(ctx, feedID)
	if err != nil {
		jsonError(w, "Feed not found", 404)
		return
	}

	go s.Fetcher.FetchFeed(context.Background(), feed)

	jsonResponse(w, map[string]string{"status": "refreshing"})
}

func (s *Server) apiMarkRead(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := dbgen.New(s.DB)

	articleID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid article ID", 400)
		return
	}

	if err := q.MarkArticleRead(ctx, articleID); err != nil {
		jsonError(w, "Failed to mark read", 500)
		return
	}

	jsonResponse(w, map[string]string{"status": "ok"})
}

func (s *Server) apiMarkUnread(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := dbgen.New(s.DB)

	articleID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid article ID", 400)
		return
	}

	if err := q.MarkArticleUnread(ctx, articleID); err != nil {
		jsonError(w, "Failed to mark unread", 500)
		return
	}

	jsonResponse(w, map[string]string{"status": "ok"})
}

func (s *Server) apiToggleStar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := dbgen.New(s.DB)

	articleID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid article ID", 400)
		return
	}

	if err := q.ToggleArticleStar(ctx, articleID); err != nil {
		jsonError(w, "Failed to toggle star", 500)
		return
	}

	jsonResponse(w, map[string]string{"status": "ok"})
}

func (s *Server) apiMarkFeedRead(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := dbgen.New(s.DB)

	feedID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid feed ID", 400)
		return
	}

	if err := q.MarkFeedRead(ctx, feedID); err != nil {
		jsonError(w, "Failed to mark feed read", 500)
		return
	}

	jsonResponse(w, map[string]string{"status": "ok"})
}

func (s *Server) apiCreateScraper(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := dbgen.New(s.DB)

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

	module, err := q.CreateScraperModule(ctx, dbgen.CreateScraperModuleParams{
		Name:        req.Name,
		Description: desc,
		Script:      req.Script,
		ScriptType:  req.ScriptType,
	})
	if err != nil {
		jsonError(w, "Failed to create scraper: "+err.Error(), 500)
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
	q := dbgen.New(s.DB)

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid ID", 400)
		return
	}

	if err := q.DeleteScraperModule(ctx, id); err != nil {
		jsonError(w, "Failed to delete scraper", 500)
		return
	}

	jsonResponse(w, map[string]string{"status": "ok"})
}

func (s *Server) apiSearch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := dbgen.New(s.DB)

	query := r.URL.Query().Get("q")
	if query == "" {
		jsonResponse(w, []any{})
		return
	}

	articles, err := q.SearchArticles(ctx, dbgen.SearchArticlesParams{
		Column1: &query,
		Column2: &query,
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

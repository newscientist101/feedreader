package srv

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/newscientist101/feedreader/db"
	"github.com/newscientist101/feedreader/db/dbgen"
	"github.com/newscientist101/feedreader/srv/email"
	"github.com/newscientist101/feedreader/srv/feeds"
	"github.com/newscientist101/feedreader/srv/opml"
	"github.com/newscientist101/feedreader/srv/safenet"
	"github.com/newscientist101/feedreader/srv/scrapers"
	"github.com/newscientist101/feedreader/srv/sources"
)

type Server struct {
	DB               *sql.DB
	Hostname         string
	TemplatesDir     string
	StaticDir        string
	StaticHashes     map[string]string // filename -> short hash for cache busting
	Fetcher          *feeds.Fetcher
	ScraperRunner    *scrapers.Runner
	RetentionManager *RetentionManager
	ShelleyGenerator *ShelleyScraperGenerator
	EmailWatcher     *email.Watcher
	FaviconBaseURL   string       // upstream favicon service; default Google S2
	FaviconClient    *http.Client // HTTP client for favicon fetches; defaults to safe client

	CountsCache *CountsCache // per-user article count cache
	Sources     *sources.Registry

	// templateCache holds pre-parsed templates keyed by page name.
	// Populated by initTemplates(); nil disables caching (re-parse each request).
	templateCache map[string]*template.Template

	// bgCtx is the context for background goroutines (feed fetches, etc.).
	// Cancelled by Close() to ensure clean shutdown.
	bgCtx    context.Context
	bgCancel context.CancelFunc
}

func New(dbPath, hostname string) (*Server, error) {
	_, thisFile, _, _ := runtime.Caller(0)
	baseDir := filepath.Dir(thisFile)
	ctx, cancel := context.WithCancel(context.Background())
	srv := &Server{
		Hostname:      hostname,
		TemplatesDir:  filepath.Join(baseDir, "templates"),
		StaticDir:     filepath.Join(baseDir, "static"),
		ScraperRunner: scrapers.NewRunner(),
		CountsCache:   NewCountsCache(30 * time.Second),
		Sources:       sources.DefaultRegistry(),
		bgCtx:         ctx,
		bgCancel:      cancel,
	}
	if err := srv.setUpDatabase(dbPath); err != nil {
		cancel()
		return nil, err
	}
	srv.Fetcher = feeds.NewFetcher(srv.DB, srv.ScraperRunner)
	srv.Fetcher.OnFeedFetched = func(ctx context.Context, feedID int64) {
		srv.MarkExcludedArticlesReadForFeed(ctx, feedID)
		srv.EvaluateAlertsForFeed(ctx, feedID)
		// Invalidate counts cache for the feed owner since new articles arrived.
		if ownerID, err := dbgen.New(srv.DB).GetFeedOwner(ctx, feedID); err == nil && ownerID != nil {
			srv.CountsCache.Invalidate(*ownerID)
		}
	}
	srv.StaticHashes = hashStaticFiles(srv.StaticDir)
	if err := srv.initTemplates(); err != nil {
		cancel()
		return nil, fmt.Errorf("init templates: %w", err)
	}
	return srv, nil
}

// Close cancels background goroutines. Safe to call multiple times.
func (s *Server) Close() {
	if s.bgCancel != nil {
		s.bgCancel()
	}
}

// hashStaticFiles computes short SHA-256 hashes for static files for cache busting.
func hashStaticFiles(dir string) map[string]string {
	hashes := make(map[string]string)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		rel, _ := filepath.Rel(dir, path)
		hashes[rel] = hex.EncodeToString(sum[:4]) // 8 hex chars
		return nil
	})
	if err != nil {
		slog.Warn("failed to walk static dir", "error", err)
	}
	slog.Info("static file hashes computed", "count", len(hashes))
	return hashes
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
	defer s.Close()

	// Start background fetcher
	s.Fetcher.Start(5 * time.Minute)
	defer s.Fetcher.Stop()

	// Start retention manager (per-user configurable retention)
	s.RetentionManager = NewRetentionManager(s)
	s.RetentionManager.Start()
	defer s.RetentionManager.Stop()

	// Start email newsletter watcher
	s.EmailWatcher = email.NewWatcher(s.DB, s.Hostname)
	s.EmailWatcher.Start(10 * time.Second)
	defer s.EmailWatcher.Stop()

	// Initialize Shelley scraper generator
	s.ShelleyGenerator = NewShelleyScraperGenerator()

	handler := s.Handler()

	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MB
	}
	slog.Info("starting server", "addr", addr)
	return srv.ListenAndServe()
}

// Handler builds the full HTTP handler with routing, auth middleware, and gzip.
// Extracted from Serve so integration tests can use it without ListenAndServe.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Pages
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("GET /feeds", s.handleFeeds)
	mux.HandleFunc("GET /starred", s.handleStarred)
	mux.HandleFunc("GET /queue", s.handleQueue)
	mux.HandleFunc("GET /history", s.handleHistory)
	mux.HandleFunc("GET /alerts", s.handleAlerts)
	mux.HandleFunc("GET /alerts/{id}", s.handleAlertDetail)
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
	mux.HandleFunc("GET /api/feeds/{id}/articles", s.apiGetFeedArticles)
	mux.HandleFunc("POST /api/feeds/{id}/refresh", s.apiRefreshFeed)
	mux.HandleFunc("POST /api/articles/{id}/read", s.apiMarkRead)
	mux.HandleFunc("POST /api/articles/batch-read", s.apiBatchMarkRead)
	mux.HandleFunc("POST /api/articles/{id}/unread", s.apiMarkUnread)
	mux.HandleFunc("POST /api/articles/{id}/star", s.apiToggleStar)
	mux.HandleFunc("POST /api/articles/{id}/queue", s.apiToggleQueue)
	mux.HandleFunc("DELETE /api/articles/{id}/queue", s.apiRemoveFromQueue)
	mux.HandleFunc("GET /api/queue", s.apiListQueue)
	mux.HandleFunc("GET /api/articles/unread", s.apiGetUnreadArticles)
	mux.HandleFunc("GET /api/articles/starred", s.apiGetStarredArticles)
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
	mux.HandleFunc("GET /api/settings", s.apiGetSettings)
	mux.HandleFunc("PUT /api/settings", s.apiUpdateSettings)
	mux.HandleFunc("GET /settings", s.handleSettings)

	// Newsletter email
	mux.HandleFunc("POST /api/newsletter/generate-address", s.apiGenerateNewsletterAddress)
	mux.HandleFunc("GET /api/newsletter/address", s.apiGetNewsletterAddress)

	// AI scraper generation
	mux.HandleFunc("GET /api/ai/status", s.apiAIStatus)
	mux.HandleFunc("GET /api/favicon", s.apiFavicon)
	mux.HandleFunc("POST /api/ai/generate-scraper", s.apiGenerateScraper)

	// Exclusion rules endpoints
	mux.HandleFunc("GET /api/categories/{id}/exclusions", s.apiListExclusions)
	mux.HandleFunc("POST /api/categories/{id}/exclusions", s.apiCreateExclusion)
	mux.HandleFunc("DELETE /api/exclusions/{id}", s.apiDeleteExclusion)
	mux.HandleFunc("GET /category/{id}/settings", s.handleCategorySettings)

	// News alerts endpoints
	mux.HandleFunc("POST /api/alerts", s.apiCreateAlert)
	mux.HandleFunc("GET /api/alerts", s.apiListAlerts)
	mux.HandleFunc("GET /api/alerts/{id}", s.apiGetAlert)
	mux.HandleFunc("PUT /api/alerts/{id}", s.apiUpdateAlert)
	mux.HandleFunc("DELETE /api/alerts/{id}", s.apiDeleteAlert)
	mux.HandleFunc("POST /api/alerts/{id}/dismiss", s.apiDismissAllForAlert)
	mux.HandleFunc("POST /api/article-alerts/{id}/dismiss", s.apiDismissArticleAlert)
	mux.HandleFunc("POST /api/article-alerts/{id}/undismiss", s.apiUndismissArticleAlert)

	// Static files – serve with long cache lifetime (files are cache-busted via ?v=hash)
	staticFS := http.FileServer(http.Dir(s.StaticDir))
	mux.Handle("/static/", http.StripPrefix("/static/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("v") != "" {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=3600")
		}
		staticFS.ServeHTTP(w, r)
	})))

	// Serve service worker from root path so it can control all routes
	mux.HandleFunc("GET /sw.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Service-Worker-Allowed", "/")
		http.ServeFile(w, r, filepath.Join(s.StaticDir, "sw.js"))
	})

	// Wrap with auth middleware, security headers, gzip compression, logging,
	// CSRF protection, and per-user rate limiting.
	// Order (outermost first): gzip → logging → security → bodyLimit → auth → csrf → rateLimit → mux
	rl := newRateLimiter()
	return gzipMiddleware(loggingMiddleware(securityHeaders(bodyLimitMiddleware(s.AuthMiddleware(csrfMiddleware(rateLimitMiddleware(rl)(mux)))))))
}

// templateFuncMap returns the shared template function map.
// All closures capture s, which is safe since StaticHashes is immutable after init.
func (s *Server) templateFuncMap() template.FuncMap {
	return template.FuncMap{
		"timeAgo":     timeAgo,
		"formatDate":  formatDate,
		"truncate":    truncate,
		"previewText": previewText,
		"deref":       deref,
		"safeHTML":    sanitizeHTML,
		"toJSON": func(v any) template.JS {
			b, _ := json.Marshal(v)
			return template.JS(b)
		},
		"faviconURL": faviconURL,
		"staticPath": func(name string) string {
			if h, ok := s.StaticHashes[name]; ok {
				return "/static/" + name + "?v=" + h
			}
			return "/static/" + name
		},
		"moduleImportMap": func() template.HTML {
			imports := make(map[string]string)
			for name, hash := range s.StaticHashes {
				if strings.HasPrefix(name, "modules/") && strings.HasSuffix(name, ".js") && !strings.HasSuffix(name, ".test.js") && !strings.Contains(name, "__mocks__") {
					imports["/static/"+name] = "/static/" + name + "?v=" + hash
				}
			}
			data := map[string]any{"imports": imports}
			b, _ := json.Marshal(data)
			return template.HTML(b)
		},
		"modulePreloadTags": func() template.HTML {
			var tags []string
			for name, hash := range s.StaticHashes {
				if strings.HasPrefix(name, "modules/") && strings.HasSuffix(name, ".js") && !strings.HasSuffix(name, ".test.js") && !strings.Contains(name, "__mocks__") {
					tags = append(tags, fmt.Sprintf(`<link rel="modulepreload" href="/static/%s?v=%s">`, name, hash))
				}
			}
			sort.Strings(tags)
			return template.HTML(strings.Join(tags, "\n    "))
		},
		"add":      func(a, b int64) int64 { return a + b },
		"sortTime": sortTimeFunc,
		"dict": func(pairs ...any) map[string]any {
			m := make(map[string]any, len(pairs)/2)
			for i := 0; i+1 < len(pairs); i += 2 {
				if k, ok := pairs[i].(string); ok {
					m[k] = pairs[i+1]
				}
			}
			return m
		},
	}
}

// initTemplates pre-parses all page templates with base.html.
// Call after StaticHashes is set.
func (s *Server) initTemplates() error {
	funcMap := s.templateFuncMap()
	basePath := filepath.Join(s.TemplatesDir, "base.html")
	files, err := filepath.Glob(filepath.Join(s.TemplatesDir, "*.html"))
	if err != nil {
		return fmt.Errorf("glob templates: %w", err)
	}
	s.templateCache = make(map[string]*template.Template, len(files))
	for _, path := range files {
		name := filepath.Base(path)
		if name == "base.html" {
			continue
		}
		tmpl, err := template.New("base.html").Funcs(funcMap).ParseFiles(basePath, path)
		if err != nil {
			return fmt.Errorf("parse template %q: %w", name, err)
		}
		s.templateCache[name] = tmpl
	}
	return nil
}

// Template helpers
func (s *Server) renderTemplate(w http.ResponseWriter, r *http.Request, name string, data any) error {
	var tmpl *template.Template
	if s.templateCache != nil {
		var ok bool
		tmpl, ok = s.templateCache[name]
		if !ok {
			return fmt.Errorf("template %q not in cache", name)
		}
	} else {
		// Fallback: parse on the fly (used when initTemplates was not called)
		funcMap := s.templateFuncMap()
		path := filepath.Join(s.TemplatesDir, name)
		basePath := filepath.Join(s.TemplatesDir, "base.html")
		var err error
		tmpl, err = template.New("base.html").Funcs(funcMap).ParseFiles(basePath, path)
		if err != nil {
			return fmt.Errorf("parse template %q: %w", name, err)
		}
	}

	// Buffer the rendered output to compute an ETag.
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("execute template %q: %w", name, err)
	}

	// Compute weak ETag from content hash.
	h := sha256.Sum256(buf.Bytes())
	etag := `W/"` + hex.EncodeToString(h[:8]) + `"`
	w.Header().Set("ETag", etag)

	// Check If-None-Match for conditional response.
	if match := r.Header.Get("If-None-Match"); match != "" {
		if strings.Contains(match, etag) {
			w.WriteHeader(http.StatusNotModified)
			return nil
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, writeErr := w.Write(buf.Bytes())
	return writeErr
}

func timeAgo(v any) string {
	t := toTimePtr(v)
	if t == nil {
		return ""
	}
	diff := time.Since(*t)
	switch {
	case diff < time.Minute:
		s := int(diff.Seconds())
		if s == 1 {
			return "1 sec ago"
		}
		return fmt.Sprintf("%d sec ago", s)
	case diff < time.Hour:
		m := int(diff.Minutes())
		if m == 1 {
			return "1 min ago"
		}
		return fmt.Sprintf("%d min ago", m)
	case diff < 24*time.Hour:
		h := int(diff.Hours())
		if h == 1 {
			return "1 hr ago"
		}
		return fmt.Sprintf("%d hr ago", h)
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

func formatDate(v any) string {
	t := toTimePtr(v)
	if t == nil {
		return ""
	}
	// Return ISO 8601 format for JavaScript to parse
	return t.UTC().Format(time.RFC3339)
}

// toTimePtr converts *time.Time or time.Time to *time.Time for template funcs.
func toTimePtr(v any) *time.Time {
	switch val := v.(type) {
	case *time.Time:
		return val
	case time.Time:
		return &val
	default:
		return nil
	}
}

// articlePageSize is the number of articles returned per page.
const articlePageSize = 50

// queueMaxArticles is the maximum number of queue articles loaded at once.
// Queue UI shows one article at a time and needs all IDs for the "N of M" counter.
const queueMaxArticles = 500

// previewTextLimit caps how much text goes into article preview DOM elements.
// Keep in sync with PREVIEW_TEXT_LIMIT in static/app.js.
const previewTextLimit = 500

// markReadAgeDay and markReadAgeWeek are the string values passed to the
// MarkXxxReadOlderThan DB queries for "older than N days" filtering.
var (
	markReadAgeDay  = "1"
	markReadAgeWeek = "7"
)

// parseOffset extracts a non-negative integer "offset" query parameter.
func parseOffset(r *http.Request) int64 {
	v, err := strconv.ParseInt(r.URL.Query().Get("offset"), 10, 64)
	if err != nil || v < 0 {
		return 0
	}
	return v
}

// parseCursor extracts cursor-based pagination params from query string.
// Returns (beforeTime, beforeID, hasCursor).
func parseCursor(r *http.Request) (*time.Time, int64, bool) {
	bt := r.URL.Query().Get("before_time")
	bi := r.URL.Query().Get("before_id")
	if bt == "" || bi == "" {
		return nil, 0, false
	}
	t, err := time.Parse(time.RFC3339Nano, bt)
	if err != nil {
		return nil, 0, false
	}
	id, err := strconv.ParseInt(bi, 10, 64)
	if err != nil {
		return nil, 0, false
	}
	return &t, id, true
}

// parseAfterCursor extracts forward cursor-based pagination params from query string.
// Returns (afterTime, afterID, hasCursor). Used for ASC-ordered views like queue.
func parseAfterCursor(r *http.Request) (*time.Time, int64, bool) {
	at := r.URL.Query().Get("after_time")
	ai := r.URL.Query().Get("after_id")
	if at == "" || ai == "" {
		return nil, 0, false
	}
	t, err := time.Parse(time.RFC3339Nano, at)
	if err != nil {
		return nil, 0, false
	}
	id, err := strconv.ParseInt(ai, 10, 64)
	if err != nil {
		return nil, 0, false
	}
	return &t, id, true
}

// sortTimeFunc returns the article sort key as an RFC3339Nano string.
// It mirrors the SQL: COALESCE(published_at, fetched_at).
func sortTimeFunc(publishedAt *time.Time, fetchedAt time.Time) string {
	if publishedAt != nil {
		return publishedAt.Format(time.RFC3339Nano)
	}
	return fetchedAt.Format(time.RFC3339Nano)
}

func previewText(s string) string {
	return truncate(stripHTML(s), previewTextLimit)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func stripHTML(s string) string {
	// Remove <style> and <script> blocks entirely before stripping tags.
	for _, tag := range []string{"style", "script"} {
		for {
			open := strings.Index(strings.ToLower(s), "<"+tag)
			if open < 0 {
				break
			}
			closeIdx := strings.Index(strings.ToLower(s[open:]), "</"+tag)
			if closeIdx < 0 {
				s = s[:open]
				break
			}
			end := strings.IndexByte(s[open+closeIdx:], '>')
			if end < 0 {
				s = s[:open]
				break
			}
			s = s[:open] + s[open+closeIdx+end+1:]
		}
	}

	var result strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			result.WriteRune(r)
		}
	}
	return strings.TrimSpace(html.UnescapeString(result.String()))
}

// articleCounts holds the aggregated unread/starred/queue counts plus per-feed and per-category breakdowns.
type articleCounts struct {
	Unread     int64
	Starred    int64
	Queue      int64
	Alerts     int64
	FeedCounts map[int64]int64
	CatCounts  map[int64]int64
}

// getArticleCounts fetches all count data for a user in batch.
// Results are cached per-user and invalidated by state-changing operations.
func (s *Server) getArticleCounts(ctx context.Context, userID int64) articleCounts {
	if cached, ok := s.CountsCache.Get(userID); ok {
		return cached
	}

	q := dbgen.New(s.DB)
	unreadCount, _ := q.GetUnreadCount(ctx, &userID)
	starredCount, _ := q.GetStarredCount(ctx, &userID)
	queueCount, _ := q.GetQueueCount(ctx, userID)
	alertsCount, _ := q.CountUndismissedAlerts(ctx, userID)

	feedCounts := make(map[int64]int64)
	feedCountRows, _ := q.GetAllFeedUnreadCounts(ctx, &userID)
	for _, row := range feedCountRows {
		feedCounts[row.FeedID] = row.Count
	}

	catCounts := make(map[int64]int64)
	catCountRows, _ := q.GetAllCategoryUnreadCounts(ctx, &userID)
	for _, row := range catCountRows {
		catCounts[row.CategoryID] = row.Count
	}

	counts := articleCounts{
		Unread:     unreadCount,
		Starred:    starredCount,
		Queue:      queueCount,
		Alerts:     alertsCount,
		FeedCounts: feedCounts,
		CatCounts:  catCounts,
	}
	s.CountsCache.Set(userID, counts)
	return counts
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

	feedList, _ := q.ListFeeds(ctx, &userID)
	categories, _ := q.ListCategories(ctx, &userID)
	counts := s.getArticleCounts(ctx, userID)

	// Get feed-to-category mapping
	feedCategories := make(map[int64]int64)
	mappings, _ := q.ListFeedCategoryMappings(ctx, &userID)
	for _, m := range mappings {
		feedCategories[m.FeedID] = m.CategoryID
	}

	// Build feeds-by-category lookup for O(1) template access
	feedsByCategory := make(map[int64][]dbgen.Feed)
	for i := range feedList {
		catID := feedCategories[feedList[i].ID]
		feedsByCategory[catID] = append(feedsByCategory[catID], feedList[i])
	}

	// Build category tree for hierarchical display
	categoryTree := BuildCategoryTree(categories)
	flatCategories := FlattenCategoryTree(categoryTree)

	// Load user settings
	settingsRows, _ := q.GetUserSettings(ctx, userID)
	settings := make(map[string]string)
	for _, row := range settingsRows {
		settings[row.Key] = row.Value
	}

	return map[string]any{
		"Feeds":           feedList,
		"FeedCounts":      counts.FeedCounts,
		"Categories":      categories,
		"CategoryTree":    categoryTree,
		"FlatCategories":  flatCategories,
		"CategoryCounts":  counts.CatCounts,
		"FeedCategories":  feedCategories,
		"FeedsByCategory": feedsByCategory,
		"UnreadCount":     counts.Unread,
		"StarredCount":    counts.Starred,
		"QueueCount":      counts.Queue,
		"AlertsCount":     counts.Alerts,
		"User":            user,
		"Settings":        settings,
	}
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := dbgen.New(s.DB)
	user := GetUser(ctx)

	userID := user.ID
	articles, _ := q.ListUnreadArticles(ctx, dbgen.ListUnreadArticlesParams{UserID: &userID, Limit: articlePageSize * 2, Offset: 0})

	// Apply folder exclusion filters
	articles = s.FilterAllUnreadArticles(ctx, articles, userID)
	if len(articles) > articlePageSize {
		articles = articles[:articlePageSize]
	}

	data := s.getCommonData(ctx)
	data["Title"] = "All Unread"
	data["Articles"] = articles
	data["ActiveView"] = "unread"

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.renderTemplate(w, r, "index.html", data); err != nil {
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
	if err := s.renderTemplate(w, r, "feeds.html", data); err != nil {
		slog.Warn("render template", "error", err)
		http.Error(w, "Internal Server Error", 500)
	}
}

func (s *Server) handleStarred(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := dbgen.New(s.DB)
	user := GetUser(ctx)

	articles, _ := q.ListStarredArticles(ctx, dbgen.ListStarredArticlesParams{UserID: &user.ID, Limit: articlePageSize, Offset: 0})

	data := s.getCommonData(ctx)
	data["Title"] = "Starred"
	data["Articles"] = articles
	data["ActiveView"] = "starred"

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.renderTemplate(w, r, "index.html", data); err != nil {
		slog.Warn("render template", "error", err)
		http.Error(w, "Internal Server Error", 500)
	}
}

func (s *Server) handleQueue(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := dbgen.New(s.DB)
	user := GetUser(ctx)

	articles, _ := q.ListQueueArticles(ctx, dbgen.ListQueueArticlesParams{
		UserID: user.ID,
		Limit:  queueMaxArticles,
		Offset: 0,
	})

	data := s.getCommonData(ctx)
	data["Title"] = "Queue"
	data["ActiveView"] = "queue"
	data["QueueArticles"] = articles

	// If there are articles, load the first one fully (with content filters)
	if len(articles) > 0 {
		first := articles[0]
		feed, _ := q.GetFeed(ctx, dbgen.GetFeedParams{ID: first.FeedID, UserID: &user.ID})
		if first.Content != nil && feed.ContentFilters != nil {
			filtered := ApplyContentFilters(*first.Content, feed.ContentFilters)
			first.Content = &filtered
		}
		if first.Summary != nil && feed.ContentFilters != nil {
			filtered := ApplyContentFilters(*first.Summary, feed.ContentFilters)
			first.Summary = &filtered
		}
		data["Article"] = first
		data["Feed"] = feed
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.renderTemplate(w, r, "queue.html", data); err != nil {
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

	articles, _ := q.ListArticlesByFeed(ctx, dbgen.ListArticlesByFeedParams{FeedID: feedID, UserID: &user.ID, Limit: articlePageSize * 2, Offset: 0})

	// Apply exclusion filters based on feed's category
	filteredArticles := s.FilterArticlesByFeed(ctx, articles, feedID, user.ID)
	if len(filteredArticles) > articlePageSize {
		filteredArticles = filteredArticles[:articlePageSize]
	}

	data := s.getCommonData(ctx)
	data["Title"] = feed.Name
	data["Articles"] = filteredArticles
	data["ActiveFeed"] = feedID
	data["CurrentFeed"] = feed

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.renderTemplate(w, r, "index.html", data); err != nil {
		slog.Warn("render template", "error", err)
		http.Error(w, "Internal Server Error", 500)
	}
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := dbgen.New(s.DB)
	user := GetUser(ctx)

	offset := parseOffset(r)
	articles, _ := q.ListHistoryArticles(ctx, dbgen.ListHistoryArticlesParams{
		UserID: user.ID,
		Limit:  articlePageSize,
		Offset: offset,
	})

	data := s.getCommonData(ctx)
	data["Title"] = "History"
	data["ActiveView"] = "history"
	data["HistoryArticles"] = articles
	data["Offset"] = offset
	data["HasMore"] = len(articles) == articlePageSize

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.renderTemplate(w, r, "history.html", data); err != nil {
		slog.Warn("render template", "error", err)
		http.Error(w, "Internal Server Error", 500)
	}
}

func (s *Server) handleAlerts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := dbgen.New(s.DB)
	user := GetUser(ctx)

	offset := parseOffset(r)
	matches, _ := q.ListAlertArticlesGrouped(ctx, dbgen.ListAlertArticlesGroupedParams{
		UserID: user.ID,
		Limit:  articlePageSize,
		Offset: offset,
	})

	alerts, _ := q.ListAlertsByUser(ctx, user.ID)

	// Group matches by alert ID for template rendering.
	type alertGroup struct {
		AlertID   int64
		AlertName string
		Matches   []dbgen.ListAlertArticlesGroupedRow
	}
	var groups []alertGroup
	groupMap := make(map[int64]int) // alertID -> index in groups
	for i := range matches {
		idx, ok := groupMap[matches[i].AlertID]
		if !ok {
			idx = len(groups)
			groupMap[matches[i].AlertID] = idx
			groups = append(groups, alertGroup{AlertID: matches[i].AlertID, AlertName: matches[i].AlertName})
		}
		groups[idx].Matches = append(groups[idx].Matches, matches[i])
	}

	data := s.getCommonData(ctx)
	data["Title"] = "Alerts"
	data["ActiveView"] = "alerts"
	data["AlertGroups"] = groups
	data["Alerts"] = alerts
	data["Offset"] = offset
	data["HasMore"] = len(matches) == articlePageSize

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.renderTemplate(w, r, "alerts.html", data); err != nil {
		slog.Warn("render template", "error", err)
		http.Error(w, "Internal Server Error", 500)
	}
}

func (s *Server) handleAlertDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := dbgen.New(s.DB)
	user := GetUser(ctx)

	alertID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid alert ID", 400)
		return
	}

	alert, err := q.GetAlert(ctx, dbgen.GetAlertParams{ID: alertID, UserID: user.ID})
	if err != nil {
		http.Error(w, "Alert not found", 404)
		return
	}

	offset := parseOffset(r)
	matches, _ := q.ListAlertArticles(ctx, dbgen.ListAlertArticlesParams{
		AlertID: alertID,
		UserID:  user.ID,
		Limit:   articlePageSize,
		Offset:  offset,
	})

	data := s.getCommonData(ctx)
	data["Title"] = alert.Name + " — Alert"
	data["ActiveView"] = "alerts"
	data["Alert"] = alert
	data["AlertMatches"] = matches
	data["Offset"] = offset
	data["HasMore"] = len(matches) == articlePageSize

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.renderTemplate(w, r, "alert_detail.html", data); err != nil {
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
	if err := q.MarkArticleRead(ctx, dbgen.MarkArticleReadParams{ID: articleID, UserID: &user.ID}); err != nil {
		slog.Warn("failed to mark article read", "error", err)
	}
	s.CountsCache.Invalidate(user.ID)

	// Add to history
	if err := q.AddToHistory(ctx, dbgen.AddToHistoryParams{UserID: user.ID, ArticleID: articleID}); err != nil {
		slog.Warn("failed to add article to history", "error", err)
	}

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
	if err := s.renderTemplate(w, r, "article.html", data); err != nil {
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
	if err := s.renderTemplate(w, r, "scrapers.html", data); err != nil {
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
		CategoryID    int64  `json:"categoryId"`
		PlaylistOrder string `json:"playlistOrder"` // "top" (default/RSS) or "bottom" (YouTube Data API)
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request", 400)
		return
	}

	if req.URL == "" {
		jsonError(w, "URL is required", 400)
		return
	}

	// Validate URL to prevent SSRF (block private IPs, non-HTTP schemes, etc.)
	if err := safenet.ValidateURL(req.URL); err != nil {
		jsonError(w, "Invalid feed URL: "+err.Error(), 400)
		return
	}

	if req.FeedType == "" {
		req.FeedType = "rss"
	}

	// Apply feed-source-specific URL normalization and auto-naming.
	req.URL, req.Name, req.FeedType = s.Sources.Resolve(ctx, req.URL, req.Name, req.FeedType, req.ScraperConfig)

	// YouTube "add to bottom" playlists use the Data API instead of RSS.
	if req.PlaylistOrder == "bottom" && req.FeedType == "rss" {
		if u, err := url.Parse(req.URL); err == nil {
			if pid := u.Query().Get("playlist_id"); pid != "" {
				req.FeedType = "youtube-playlist"
				cfg := feeds.YouTubePlaylistConfig{PlaylistID: pid}
				cfgBytes, _ := json.Marshal(cfg)
				req.ScraperConfig = string(cfgBytes)
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

	var skipRetention int64
	if req.FeedType == "youtube-playlist" {
		skipRetention = 1
	}

	feed, err := q.CreateFeed(ctx, dbgen.CreateFeedParams{
		Name:                 req.Name,
		Url:                  req.URL,
		FeedType:             req.FeedType,
		ScraperModule:        scraperModule,
		ScraperConfig:        scraperConfig,
		FetchIntervalMinutes: &req.Interval,
		UserID:               &user.ID,
		SkipRetention:        skipRetention,
	})
	if err != nil {
		jsonError(w, "Failed to create feed: "+err.Error(), 500)
		return
	}

	// Set category if specified
	if req.CategoryID > 0 {
		err = q.AddFeedToCategory(ctx, dbgen.AddFeedToCategoryParams{
			FeedID:     feed.ID,
			CategoryID: req.CategoryID,
		})
		if err != nil {
			slog.Warn("failed to set feed category", "error", err)
		}
	}

	// Trigger immediate fetch
	go func() {
		if err := s.Fetcher.FetchFeed(s.bgCtx, &feed); err != nil {
			if s.bgCtx.Err() == nil {
				slog.Warn("background feed fetch failed", "error", err, "feed_id", feed.ID)
			}
		}
	}()

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
	s.CountsCache.Invalidate(user.ID)

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
		SkipRetention        *bool   `json:"skip_retention"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request body", 400)
		return
	}

	// Validate new URL if provided
	if req.URL != "" {
		if err := safenet.ValidateURL(req.URL); err != nil {
			jsonError(w, "Invalid feed URL: "+err.Error(), 400)
			return
		}
	}

	// Use existing values if not provided
	name := req.Name
	if name == "" {
		name = feed.Name
	}
	reqURL := req.URL
	if reqURL == "" {
		reqURL = feed.Url
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
	skipRetention := feed.SkipRetention
	if req.SkipRetention != nil {
		if *req.SkipRetention {
			skipRetention = 1
		} else {
			skipRetention = 0
		}
	}

	if err := q.UpdateFeed(ctx, dbgen.UpdateFeedParams{
		Name:                 name,
		Url:                  reqURL,
		FeedType:             feedType,
		ScraperModule:        scraperModule,
		ScraperConfig:        scraperConfig,
		FetchIntervalMinutes: interval,
		ContentFilters:       contentFilters,
		SkipRetention:        skipRetention,
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
		if err := s.Fetcher.FetchFeed(s.bgCtx, &feed); err != nil {
			if s.bgCtx.Err() == nil {
				slog.Warn("manual feed refresh failed", "feed_id", feed.ID, "error", err)
			}
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

	counts := s.getArticleCounts(ctx, userID)

	feedList, _ := q.ListFeeds(ctx, &userID)
	feedErrors := make(map[int64]string)
	for i := range feedList {
		if feedList[i].LastError != nil && *feedList[i].LastError != "" {
			feedErrors[feedList[i].ID] = *feedList[i].LastError
		}
	}

	jsonResponse(w, map[string]any{
		"unread":     counts.Unread,
		"starred":    counts.Starred,
		"queue":      counts.Queue,
		"alerts":     counts.Alerts,
		"feeds":      counts.FeedCounts,
		"categories": counts.CatCounts,
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
		"id":          feed.ID,
		"name":        feed.Name,
		"lastFetched": lastFetched,
		"lastError":   lastError,
	})
}

func (s *Server) apiGetUnreadArticles(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetUser(ctx)
	q := dbgen.New(s.DB)

	userID := user.ID
	includeRead := r.URL.Query().Get("include_read") == "1"
	beforeTime, beforeID, hasCursor := parseCursor(r)

	if includeRead {
		var allArticles []dbgen.ListArticlesRow
		if hasCursor {
			cursorRows, _ := q.ListArticlesCursor(ctx, dbgen.ListArticlesCursorParams{
				UserID: &userID, BeforeTime: beforeTime, BeforeTimeEq: beforeTime, BeforeID: beforeID, Limit: articlePageSize * 2,
			})
			for i := range cursorRows {
				allArticles = append(allArticles, dbgen.ListArticlesRow(cursorRows[i]))
			}
		} else {
			allArticles, _ = q.ListArticles(ctx, dbgen.ListArticlesParams{
				UserID: &userID, Limit: articlePageSize * 2, Offset: parseOffset(r),
			})
		}
		allArticles = s.FilterAllArticles(ctx, allArticles, userID)
		if len(allArticles) > articlePageSize {
			allArticles = allArticles[:articlePageSize]
		}
		jsonResponse(w, map[string]any{"articles": allArticles})
		return
	}

	var articles []dbgen.ListUnreadArticlesRow
	if hasCursor {
		cursorRows, _ := q.ListUnreadArticlesCursor(ctx, dbgen.ListUnreadArticlesCursorParams{
			UserID: &userID, BeforeTime: beforeTime, BeforeTimeEq: beforeTime, BeforeID: beforeID, Limit: articlePageSize * 2,
		})
		for i := range cursorRows {
			articles = append(articles, dbgen.ListUnreadArticlesRow(cursorRows[i]))
		}
	} else {
		articles, _ = q.ListUnreadArticles(ctx, dbgen.ListUnreadArticlesParams{
			UserID: &userID, Limit: articlePageSize * 2, Offset: parseOffset(r),
		})
	}

	articles = s.FilterAllUnreadArticles(ctx, articles, userID)
	if len(articles) > articlePageSize {
		articles = articles[:articlePageSize]
	}

	jsonResponse(w, map[string]any{"articles": articles})
}

func (s *Server) apiGetStarredArticles(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetUser(ctx)
	q := dbgen.New(s.DB)

	beforeTime, beforeID, hasCursor := parseCursor(r)

	var articles []dbgen.ListStarredArticlesRow
	if hasCursor {
		cursorRows, _ := q.ListStarredArticlesCursor(ctx, dbgen.ListStarredArticlesCursorParams{
			UserID: &user.ID, BeforeTime: beforeTime, BeforeTimeEq: beforeTime, BeforeID: beforeID, Limit: articlePageSize,
		})
		for i := range cursorRows {
			articles = append(articles, dbgen.ListStarredArticlesRow(cursorRows[i]))
		}
	} else {
		articles, _ = q.ListStarredArticles(ctx, dbgen.ListStarredArticlesParams{
			UserID: &user.ID, Limit: articlePageSize, Offset: 0,
		})
	}

	jsonResponse(w, map[string]any{"articles": articles})
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
		slog.Error("failed to mark article read", "article_id", articleID, "user_id", user.ID, "error", err)
		jsonError(w, "Failed to mark read", 500)
		return
	}
	s.CountsCache.Invalidate(user.ID)

	jsonResponse(w, map[string]string{"status": "ok"})
}

func (s *Server) apiBatchMarkRead(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetUser(ctx)
	q := dbgen.New(s.DB)

	var req struct {
		IDs []int64 `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request body", 400)
		return
	}
	if len(req.IDs) == 0 {
		jsonResponse(w, map[string]string{"status": "ok"})
		return
	}
	if len(req.IDs) > 200 {
		jsonError(w, "Too many IDs (max 200)", 400)
		return
	}

	for _, id := range req.IDs {
		if err := q.MarkArticleRead(ctx, dbgen.MarkArticleReadParams{ID: id, UserID: &user.ID}); err != nil {
			slog.Error("failed to mark article read (batch)", "article_id", id, "user_id", user.ID, "error", err)
		}
	}
	s.CountsCache.Invalidate(user.ID)

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
	s.CountsCache.Invalidate(user.ID)

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
	s.CountsCache.Invalidate(user.ID)

	jsonResponse(w, map[string]string{"status": "ok"})
}

func (s *Server) apiToggleQueue(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetUser(ctx)
	q := dbgen.New(s.DB)

	articleID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid article ID", 400)
		return
	}

	queued, _ := q.IsArticleQueued(ctx, dbgen.IsArticleQueuedParams{UserID: user.ID, ArticleID: articleID})
	if queued > 0 {
		if err := q.RemoveFromQueue(ctx, dbgen.RemoveFromQueueParams{UserID: user.ID, ArticleID: articleID}); err != nil {
			jsonError(w, "Failed to remove from queue", 500)
			return
		}
		s.CountsCache.Invalidate(user.ID)
		jsonResponse(w, map[string]any{"status": "ok", "queued": false})
	} else {
		if err := q.AddToQueue(ctx, dbgen.AddToQueueParams{UserID: user.ID, ArticleID: articleID}); err != nil {
			jsonError(w, "Failed to add to queue", 500)
			return
		}
		s.CountsCache.Invalidate(user.ID)
		jsonResponse(w, map[string]any{"status": "ok", "queued": true})
	}
}

func (s *Server) apiRemoveFromQueue(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetUser(ctx)
	q := dbgen.New(s.DB)

	articleID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid article ID", 400)
		return
	}

	if err := q.RemoveFromQueue(ctx, dbgen.RemoveFromQueueParams{UserID: user.ID, ArticleID: articleID}); err != nil {
		jsonError(w, "Failed to remove from queue", 500)
		return
	}
	s.CountsCache.Invalidate(user.ID)

	jsonResponse(w, map[string]string{"status": "ok"})
}

func (s *Server) apiListQueue(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetUser(ctx)
	q := dbgen.New(s.DB)

	afterTime, afterID, hasCursor := parseAfterCursor(r)

	var articles []dbgen.ListQueueArticlesRow
	var err error

	if hasCursor {
		cursorRows, cursorErr := q.ListQueueArticlesCursor(ctx, dbgen.ListQueueArticlesCursorParams{
			UserID: user.ID, AfterTime: afterTime, AfterTimeEq: afterTime, AfterID: afterID, Limit: articlePageSize,
		})
		err = cursorErr
		for i := range cursorRows {
			articles = append(articles, dbgen.ListQueueArticlesRow(cursorRows[i]))
		}
	} else {
		articles, err = q.ListQueueArticles(ctx, dbgen.ListQueueArticlesParams{
			UserID: user.ID,
			Limit:  queueMaxArticles,
			Offset: 0,
		})
	}
	if err != nil {
		jsonError(w, "Failed to list queue", 500)
		return
	}

	jsonResponse(w, articles)
}

func (s *Server) apiMarkAllRead(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetUser(ctx)
	q := dbgen.New(s.DB)

	age := r.URL.Query().Get("age")
	var err error
	switch age {
	case "day":
		err = q.MarkAllArticlesReadOlderThan(ctx, dbgen.MarkAllArticlesReadOlderThanParams{Column1: &markReadAgeDay, UserID: &user.ID})
	case "week":
		err = q.MarkAllArticlesReadOlderThan(ctx, dbgen.MarkAllArticlesReadOlderThanParams{Column1: &markReadAgeWeek, UserID: &user.ID})
	default:
		err = q.MarkAllArticlesRead(ctx, &user.ID)
	}

	if err != nil {
		jsonError(w, "Failed to mark all read", 500)
		return
	}
	s.CountsCache.Invalidate(user.ID)

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
	switch age {
	case "day":
		err = q.MarkFeedArticlesReadOlderThan(ctx, dbgen.MarkFeedArticlesReadOlderThanParams{
			FeedID:  feedID,
			Column2: &markReadAgeDay,
			UserID:  &user.ID,
		})
	case "week":
		err = q.MarkFeedArticlesReadOlderThan(ctx, dbgen.MarkFeedArticlesReadOlderThanParams{
			FeedID:  feedID,
			Column2: &markReadAgeWeek,
			UserID:  &user.ID,
		})
	default:
		err = q.MarkFeedRead(ctx, dbgen.MarkFeedReadParams{FeedID: feedID, UserID: &user.ID})
	}

	if err != nil {
		jsonError(w, "Failed to mark feed read", 500)
		return
	}
	s.CountsCache.Invalidate(user.ID)

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
	user := GetUser(ctx)

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

	if req.ScriptType == "" {
		req.ScriptType = "json"
	}
	if err := q.UpdateScraperModule(ctx, dbgen.UpdateScraperModuleParams{
		ID:          id,
		Name:        req.Name,
		Description: desc,
		Script:      req.Script,
		ScriptType:  req.ScriptType,
		UserID:      &user.ID,
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

	feedIDStr := r.URL.Query().Get("feed_id")
	categoryIDStr := r.URL.Query().Get("category_id")

	var result any
	var err error

	switch {
	case feedIDStr != "":
		feedID, convErr := strconv.ParseInt(feedIDStr, 10, 64)
		if convErr != nil {
			jsonError(w, "Invalid feed_id", 400)
			return
		}
		result, err = q.SearchArticlesByFeed(ctx, dbgen.SearchArticlesByFeedParams{
			FeedID:  feedID,
			UserID:  &user.ID,
			Column3: &query,
			Column4: &query,
			Limit:   50,
			Offset:  0,
		})
	case categoryIDStr != "":
		categoryID, convErr := strconv.ParseInt(categoryIDStr, 10, 64)
		if convErr != nil {
			jsonError(w, "Invalid category_id", 400)
			return
		}
		result, err = q.SearchArticlesByCategory(ctx, dbgen.SearchArticlesByCategoryParams{
			CategoryID: categoryID,
			UserID:     &user.ID,
			Column3:    &query,
			Column4:    &query,
			Limit:      50,
			Offset:     0,
		})
	default:
		result, err = q.SearchArticles(ctx, dbgen.SearchArticlesParams{
			UserID:  &user.ID,
			Column2: &query,
			Column3: &query,
			Limit:   50,
			Offset:  0,
		})
	}

	if err != nil {
		jsonError(w, "Search failed", 500)
		return
	}

	jsonResponse(w, result)
}

func jsonResponse(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Warn("jsonResponse encode error", "error", err)
	}
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": msg}); err != nil {
		slog.Warn("jsonError encode error", "error", err)
	}
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

// faviconDomain extracts the domain to use for a favicon lookup.
func faviconDomain(siteURL, feedURL string) string {
	src := siteURL
	if src == "" {
		src = feedURL
	}
	u, err := url.Parse(src)
	if err != nil || u.Host == "" {
		return ""
	}
	host := u.Host
	parts := strings.Split(host, ".")
	if len(parts) > 2 {
		sub := strings.ToLower(parts[0])
		if sub == "feeds" || sub == "feed" || sub == "rss" {
			host = strings.Join(parts[1:], ".")
		}
	}
	return host
}

// faviconURL returns a local proxy URL for the favicon.
func faviconURL(siteURL, feedURL string) string {
	host := faviconDomain(siteURL, feedURL)
	if host == "" {
		return ""
	}
	return "/api/favicon?domain=" + url.QueryEscape(host)
}

// fallbackFavicon is the app's own SVG icon, served when a feed favicon cannot be fetched.
//
//go:embed static/icons/icon.svg
var fallbackFavicon []byte

// defaultFaviconClient is used when Server.FaviconClient is nil.
var defaultFaviconClient = safenet.NewSafeClient(4*time.Second, nil)

func (s *Server) serveFallbackFavicon(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write(fallbackFavicon)
}

func (s *Server) apiFavicon(w http.ResponseWriter, r *http.Request) {
	domain := r.URL.Query().Get("domain")
	if domain == "" {
		s.serveFallbackFavicon(w)
		return
	}

	baseURL := s.FaviconBaseURL
	if baseURL == "" {
		baseURL = "https://www.google.com/s2/favicons"
	}
	upstream := baseURL + "?domain=" + url.QueryEscape(domain) + "&sz=32"
	client := s.FaviconClient
	if client == nil {
		client = defaultFaviconClient
	}
	resp, err := client.Get(upstream)
	if err != nil || resp.StatusCode != 200 {
		if resp != nil {
			_ = resp.Body.Close()
		}
		s.serveFallbackFavicon(w)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = "image/png"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "public, max-age=604800")
	_, _ = io.Copy(w, resp.Body)
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

	categoryVal, err := q.GetCategory(ctx, dbgen.GetCategoryParams{ID: catID, UserID: &user.ID})
	if err != nil {
		http.Error(w, "Category not found", 404)
		return
	}
	category := &categoryVal

	articles, _ := q.ListUnreadArticlesByCategory(ctx, dbgen.ListUnreadArticlesByCategoryParams{
		CategoryID: catID,
		UserID:     &user.ID,
		Limit:      articlePageSize * 2,
		Offset:     0,
	})

	// Apply exclusion filters
	filteredArticles := s.FilterArticlesByCategory(ctx, articles, catID, user.ID)
	if len(filteredArticles) > articlePageSize {
		filteredArticles = filteredArticles[:articlePageSize]
	}

	data := s.getCommonData(ctx)
	data["Title"] = category.Name
	data["Articles"] = filteredArticles
	data["ActiveCategory"] = catID
	data["CurrentCategory"] = category

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.renderTemplate(w, r, "index.html", data); err != nil {
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

	categoryVal, err := q.GetCategory(ctx, dbgen.GetCategoryParams{ID: catID, UserID: &user.ID})
	if err != nil {
		jsonError(w, "Category not found", 404)
		return
	}
	category := &categoryVal

	includeRead := r.URL.Query().Get("include_read") == "1"
	beforeTime, beforeID, hasCursor := parseCursor(r)

	if includeRead {
		var allArticles []dbgen.ListArticlesByCategoryRow
		if hasCursor {
			cursorRows, _ := q.ListArticlesByCategoryCursor(ctx, dbgen.ListArticlesByCategoryCursorParams{
				CategoryID: catID, UserID: &user.ID, BeforeTime: beforeTime, BeforeTimeEq: beforeTime, BeforeID: beforeID, Limit: articlePageSize * 2,
			})
			for i := range cursorRows {
				allArticles = append(allArticles, dbgen.ListArticlesByCategoryRow(cursorRows[i]))
			}
		} else {
			allArticles, _ = q.ListArticlesByCategory(ctx, dbgen.ListArticlesByCategoryParams{
				CategoryID: catID, UserID: &user.ID, Limit: articlePageSize * 2, Offset: parseOffset(r),
			})
		}
		filteredAll := s.FilterArticlesByCategoryAll(ctx, allArticles, catID, user.ID)
		if len(filteredAll) > articlePageSize {
			filteredAll = filteredAll[:articlePageSize]
		}
		jsonResponse(w, map[string]any{
			"category": category,
			"articles": filteredAll,
		})
		return
	}

	var articles []dbgen.ListUnreadArticlesByCategoryRow
	if hasCursor {
		cursorRows, _ := q.ListUnreadArticlesByCategoryCursor(ctx, dbgen.ListUnreadArticlesByCategoryCursorParams{
			CategoryID: catID, UserID: &user.ID, BeforeTime: beforeTime, BeforeTimeEq: beforeTime, BeforeID: beforeID, Limit: articlePageSize * 2,
		})
		for i := range cursorRows {
			articles = append(articles, dbgen.ListUnreadArticlesByCategoryRow(cursorRows[i]))
		}
	} else {
		articles, _ = q.ListUnreadArticlesByCategory(ctx, dbgen.ListUnreadArticlesByCategoryParams{
			CategoryID: catID, UserID: &user.ID, Limit: articlePageSize * 2, Offset: parseOffset(r),
		})
	}

	// Apply exclusion filters
	filteredArticles := s.FilterArticlesByCategory(ctx, articles, catID, user.ID)
	if len(filteredArticles) > articlePageSize {
		filteredArticles = filteredArticles[:articlePageSize]
	}

	jsonResponse(w, map[string]any{
		"category": category,
		"articles": filteredArticles,
	})
}

func (s *Server) apiGetFeedArticles(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := dbgen.New(s.DB)
	user := GetUser(ctx)

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

	var articles []dbgen.ListArticlesByFeedRow
	beforeTime, beforeID, hasCursor := parseCursor(r)
	if hasCursor {
		cursorRows, _ := q.ListArticlesByFeedCursor(ctx, dbgen.ListArticlesByFeedCursorParams{
			FeedID: feedID, UserID: &user.ID, BeforeTime: beforeTime, BeforeTimeEq: beforeTime, BeforeID: beforeID, Limit: articlePageSize * 2,
		})
		for i := range cursorRows {
			articles = append(articles, dbgen.ListArticlesByFeedRow(cursorRows[i]))
		}
	} else {
		articles, _ = q.ListArticlesByFeed(ctx, dbgen.ListArticlesByFeedParams{
			FeedID: feedID, UserID: &user.ID, Limit: articlePageSize * 2, Offset: parseOffset(r),
		})
	}

	// Apply exclusion filters based on feed's category
	filteredArticles := s.FilterArticlesByFeed(ctx, articles, feedID, user.ID)
	if len(filteredArticles) > articlePageSize {
		filteredArticles = filteredArticles[:articlePageSize]
	}

	jsonResponse(w, map[string]any{
		"feed":     feed,
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
		if err := q.UpdateCategorySortOrder(ctx, dbgen.UpdateCategorySortOrderParams{
			SortOrder: &sortOrder,
			ID:        catID,
			UserID:    &user.ID,
		}); err != nil {
			slog.Warn("failed to update category sort order", "error", err, "category_id", catID)
		}
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
		ParentID  *int64 `json:"parent_id"` // null = top level
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
	user := GetUser(ctx)
	q := dbgen.New(s.DB)

	feedID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid feed ID", 400)
		return
	}

	// Verify feed belongs to user
	if _, err := q.GetFeed(ctx, dbgen.GetFeedParams{ID: feedID, UserID: &user.ID}); err != nil {
		jsonError(w, "Feed not found", 404)
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
	switch age {
	case "day":
		err = q.MarkCategoryArticlesReadOlderThan(ctx, dbgen.MarkCategoryArticlesReadOlderThanParams{
			CategoryID: catID,
			UserID:     &user.ID,
			Column3:    &markReadAgeDay,
		})
	case "week":
		err = q.MarkCategoryArticlesReadOlderThan(ctx, dbgen.MarkCategoryArticlesReadOlderThanParams{
			CategoryID: catID,
			UserID:     &user.ID,
			Column3:    &markReadAgeWeek,
		})
	default:
		err = q.MarkCategoryRead(ctx, dbgen.MarkCategoryReadParams{CategoryID: catID, UserID: &user.ID})
	}

	if err != nil {
		jsonError(w, "Failed to mark category read", 500)
		return
	}
	s.CountsCache.Invalidate(user.ID)

	jsonResponse(w, map[string]string{"status": "ok"})
}

// OPML handlers
func (s *Server) apiExportOPML(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetUser(ctx)
	q := dbgen.New(s.DB)

	feedList, err := q.ListFeeds(ctx, &user.ID)
	if err != nil {
		jsonError(w, "Failed to list feeds", 500)
		return
	}

	var exportFeeds []opml.ExportFeed
	for i := range feedList {
		cats, _ := q.GetFeedCategories(ctx, feedList[i].ID)
		catName := ""
		if len(cats) > 0 {
			catName = cats[0].Name
		}
		exportFeeds = append(exportFeeds, opml.ExportFeed{
			Name:     feedList[i].Name,
			URL:      feedList[i].Url,
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
	if _, err := w.Write(data); err != nil {
		slog.Error("failed to write OPML export", "error", err)
	}
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
		defer func() { _ = file.Close() }()
		reader = file
	}

	opmlFeeds, err := opml.Parse(reader)
	if err != nil {
		jsonError(w, "Failed to parse OPML: "+err.Error(), 400)
		return
	}

	var imported, skipped int
	var importedFeeds []dbgen.Feed
	for _, feed := range opmlFeeds {
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
			SkipRetention:        0,
		})
		if err != nil {
			slog.Warn("import: create feed failed", "error", err, "url", feed.URL, "name", feed.Name)
			skipped++
			continue
		}

		// Assign to category
		if catID > 0 {
			if err := q.AddFeedToCategory(ctx, dbgen.AddFeedToCategoryParams{
				FeedID:     newFeed.ID,
				CategoryID: catID,
			}); err != nil {
				slog.Warn("import: failed to add feed to category", "error", err, "feed_id", newFeed.ID, "category_id", catID)
			}
		}

		imported++
		importedFeeds = append(importedFeeds, newFeed)
	}

	// Queue fetches for all imported feeds
	// Run in background so we can return the response immediately
	go func() {
		for i := range importedFeeds {
			if s.bgCtx.Err() != nil {
				return
			}
			if err := s.Fetcher.FetchFeed(s.bgCtx, &importedFeeds[i]); err != nil {
				if s.bgCtx.Err() == nil {
					slog.Warn("import: background feed fetch failed", "error", err, "feed_id", importedFeeds[i].ID)
				}
			}
		}
	}()

	jsonResponse(w, map[string]any{
		"imported": imported,
		"skipped":  skipped,
		"total":    len(opmlFeeds),
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
	user := GetUser(ctx)
	q := dbgen.New(s.DB)

	catID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid category ID", 400)
		return
	}

	var req struct {
		Type    string `json:"type"` // "author" or "keyword"
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

	// Mark existing unread articles matching the new rule as read
	s.MarkExcludedArticlesReadForCategory(ctx, catID, user.ID)
	s.CountsCache.Invalidate(user.ID)

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

	categoryVal, err := q.GetCategory(ctx, dbgen.GetCategoryParams{ID: catID, UserID: &user.ID})
	if err != nil {
		http.Error(w, "Category not found", 404)
		return
	}
	category := &categoryVal

	exclusions, _ := q.ListExclusionsByCategory(ctx, dbgen.ListExclusionsByCategoryParams{CategoryID: catID, UserID: &user.ID})

	data := s.getCommonData(ctx)
	data["Title"] = category.Name + " Settings"
	data["CurrentCategory"] = category
	data["Exclusions"] = exclusions
	data["ActiveView"] = "settings"

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.renderTemplate(w, r, "category_settings.html", data); err != nil {
		slog.Warn("render template", "error", err)
		http.Error(w, "Internal Server Error", 500)
	}
}

// Retention API handlers
func (s *Server) apiRetentionStats(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r.Context())

	stats, err := s.RetentionManager.GetStats(r.Context(), user.ID)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}

	jsonResponse(w, stats)
}

func (s *Server) apiRetentionCleanup(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r.Context())
	deleted, err := s.RetentionManager.RunCleanupNow(user.ID)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}

	jsonResponse(w, map[string]any{
		"deleted": deleted,
		"message": "Cleanup completed",
	})
}

// Valid setting keys and their allowed values (empty slice = any value)
var validSettings = map[string][]string{
	"autoMarkRead":      {"true", "false"},
	"hideReadArticles":  {"hide", "show"},
	"hideEmptyFeeds":    {"hide", "show"},
	"defaultFolderView": {"card", "list", "magazine", "expanded"},
	"defaultFeedView":   {"card", "list", "magazine", "expanded"},
	"defaultView":       {"card", "list", "magazine", "expanded"},
	"retentionDays":     {"7", "14", "30", "60", "90", "180", "365"},
	"youtubeApiKey":     {},
}

func (s *Server) apiGetSettings(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r.Context())
	q := dbgen.New(s.DB)
	rows, err := q.GetUserSettings(r.Context(), user.ID)
	if err != nil {
		jsonError(w, "Failed to get settings", 500)
		return
	}
	settings := make(map[string]string)
	for _, row := range rows {
		settings[row.Key] = row.Value
	}
	jsonResponse(w, settings)
}

func (s *Server) apiUpdateSettings(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r.Context())
	var body map[string]string
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "Invalid request body", 400)
		return
	}
	q := dbgen.New(s.DB)
	for key, value := range body {
		allowed, ok := validSettings[key]
		if !ok {
			jsonError(w, "Unknown setting: "+key, 400)
			return
		}
		if len(allowed) > 0 {
			valid := slices.Contains(allowed, value)
			if !valid {
				jsonError(w, "Invalid value for "+key, 400)
				return
			}
		}
		if err := q.SetUserSetting(r.Context(), dbgen.SetUserSettingParams{
			UserID: user.ID,
			Key:    key,
			Value:  value,
		}); err != nil {
			jsonError(w, "Failed to save setting", 500)
			return
		}
	}
	jsonResponse(w, map[string]string{"status": "ok"})
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetUser(ctx)

	stats, _ := s.RetentionManager.GetStats(ctx, user.ID)

	data := s.getCommonData(ctx)
	data["Title"] = "Settings"
	data["RetentionStats"] = stats

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.renderTemplate(w, r, "settings.html", data); err != nil {
		slog.Warn("render template", "error", err)
		http.Error(w, "Internal Server Error", 500)
	}
}

// Newsletter API handlers

func (s *Server) apiGetNewsletterAddress(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r.Context())
	q := dbgen.New(s.DB)

	token, err := q.GetNewsletterToken(r.Context(), user.ID)
	if err != nil {
		jsonResponse(w, map[string]any{"address": nil})
		return
	}

	addr := email.EmailAddress(token, s.Hostname)
	jsonResponse(w, map[string]any{"address": addr})
}

func (s *Server) apiGenerateNewsletterAddress(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r.Context())
	q := dbgen.New(s.DB)

	// Check if token already exists
	if token, err := q.GetNewsletterToken(r.Context(), user.ID); err == nil {
		addr := email.EmailAddress(token, s.Hostname)
		jsonResponse(w, map[string]any{"address": addr})
		return
	}

	token, err := email.GenerateToken()
	if err != nil {
		jsonError(w, "Failed to generate token", 500)
		return
	}

	if err := q.SetNewsletterToken(r.Context(), dbgen.SetNewsletterTokenParams{
		UserID: user.ID,
		Value:  token,
	}); err != nil {
		jsonError(w, "Failed to save token", 500)
		return
	}

	addr := email.EmailAddress(token, s.Hostname)
	slog.Info("generated newsletter address", "user_id", user.ID, "address", addr)
	jsonResponse(w, map[string]any{"address": addr})
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

	ctx, cancel := context.WithTimeout(r.Context(), aiScraperTimeout)
	defer cancel()

	resp, err := s.ShelleyGenerator.Generate(ctx, req)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}

	jsonResponse(w, resp)
}

// ---------------------------------------------------------------------------
// News Alerts API handlers
// ---------------------------------------------------------------------------

func (s *Server) apiCreateAlert(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetUser(ctx)
	q := dbgen.New(s.DB)

	var req struct {
		Name       string `json:"name"`
		Pattern    string `json:"pattern"`
		IsRegex    bool   `json:"is_regex"`
		MatchField string `json:"match_field"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request", 400)
		return
	}
	if req.Name == "" {
		jsonError(w, "Name is required", 400)
		return
	}
	if req.Pattern == "" {
		jsonError(w, "Pattern is required", 400)
		return
	}
	if req.MatchField == "" {
		req.MatchField = "title_and_content"
	}
	if req.MatchField != "title" && req.MatchField != "content" && req.MatchField != "title_and_content" {
		jsonError(w, "match_field must be 'title', 'content', or 'title_and_content'", 400)
		return
	}
	if req.IsRegex {
		if _, err := regexp.Compile("(?i)" + req.Pattern); err != nil {
			jsonError(w, "Invalid regex pattern: "+err.Error(), 400)
			return
		}
	}

	var isRegex int64
	if req.IsRegex {
		isRegex = 1
	}

	alert, err := q.CreateAlert(ctx, dbgen.CreateAlertParams{
		UserID:     user.ID,
		Name:       req.Name,
		Pattern:    req.Pattern,
		IsRegex:    isRegex,
		MatchField: req.MatchField,
	})
	if err != nil {
		jsonError(w, "Failed to create alert: "+err.Error(), 500)
		return
	}

	jsonResponse(w, alert)
}

func (s *Server) apiListAlerts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetUser(ctx)
	q := dbgen.New(s.DB)

	alerts, err := q.ListAlertsByUser(ctx, user.ID)
	if err != nil {
		jsonError(w, "Failed to list alerts", 500)
		return
	}

	jsonResponse(w, alerts)
}

func (s *Server) apiGetAlert(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetUser(ctx)
	q := dbgen.New(s.DB)

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid alert ID", 400)
		return
	}

	alert, err := q.GetAlert(ctx, dbgen.GetAlertParams{ID: id, UserID: user.ID})
	if err != nil {
		jsonError(w, "Alert not found", 404)
		return
	}

	// Also fetch matched articles.
	matches, err := q.ListAlertArticles(ctx, dbgen.ListAlertArticlesParams{
		AlertID: id,
		UserID:  user.ID,
		Limit:   50,
		Offset:  0,
	})
	if err != nil {
		jsonError(w, "Failed to list alert articles", 500)
		return
	}

	jsonResponse(w, map[string]any{
		"alert":   alert,
		"matches": matches,
	})
}

func (s *Server) apiUpdateAlert(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetUser(ctx)
	q := dbgen.New(s.DB)

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid alert ID", 400)
		return
	}

	var req struct {
		Name       string `json:"name"`
		Pattern    string `json:"pattern"`
		IsRegex    bool   `json:"is_regex"`
		MatchField string `json:"match_field"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request", 400)
		return
	}
	if req.Name == "" {
		jsonError(w, "Name is required", 400)
		return
	}
	if req.Pattern == "" {
		jsonError(w, "Pattern is required", 400)
		return
	}
	if req.MatchField == "" {
		req.MatchField = "title_and_content"
	}
	if req.MatchField != "title" && req.MatchField != "content" && req.MatchField != "title_and_content" {
		jsonError(w, "match_field must be 'title', 'content', or 'title_and_content'", 400)
		return
	}
	if req.IsRegex {
		if _, err := regexp.Compile("(?i)" + req.Pattern); err != nil {
			jsonError(w, "Invalid regex pattern: "+err.Error(), 400)
			return
		}
	}

	var isRegex int64
	if req.IsRegex {
		isRegex = 1
	}

	alert, err := q.UpdateAlert(ctx, dbgen.UpdateAlertParams{
		ID:         id,
		UserID:     user.ID,
		Name:       req.Name,
		Pattern:    req.Pattern,
		IsRegex:    isRegex,
		MatchField: req.MatchField,
	})
	if err != nil {
		jsonError(w, "Failed to update alert", 500)
		return
	}

	jsonResponse(w, alert)
}

func (s *Server) apiDeleteAlert(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetUser(ctx)
	q := dbgen.New(s.DB)

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid alert ID", 400)
		return
	}

	if err := q.DeleteAlert(ctx, dbgen.DeleteAlertParams{ID: id, UserID: user.ID}); err != nil {
		jsonError(w, "Failed to delete alert", 500)
		return
	}
	s.CountsCache.Invalidate(user.ID)

	jsonResponse(w, map[string]string{"status": "ok"})
}

func (s *Server) apiDismissAllForAlert(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetUser(ctx)
	q := dbgen.New(s.DB)

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid alert ID", 400)
		return
	}

	if err := q.DismissAllForAlert(ctx, dbgen.DismissAllForAlertParams{AlertID: id, UserID: user.ID}); err != nil {
		jsonError(w, "Failed to dismiss alerts", 500)
		return
	}
	s.CountsCache.Invalidate(user.ID)

	jsonResponse(w, map[string]string{"status": "ok"})
}

func (s *Server) apiDismissArticleAlert(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetUser(ctx)
	q := dbgen.New(s.DB)

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid article-alert ID", 400)
		return
	}

	if err := q.DismissArticleAlert(ctx, dbgen.DismissArticleAlertParams{ID: id, UserID: user.ID}); err != nil {
		jsonError(w, "Failed to dismiss article alert", 500)
		return
	}
	s.CountsCache.Invalidate(user.ID)

	jsonResponse(w, map[string]string{"status": "ok"})
}

func (s *Server) apiUndismissArticleAlert(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetUser(ctx)
	q := dbgen.New(s.DB)

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		jsonError(w, "Invalid article-alert ID", 400)
		return
	}

	if err := q.UndismissArticleAlert(ctx, dbgen.UndismissArticleAlertParams{ID: id, UserID: user.ID}); err != nil {
		jsonError(w, "Failed to undismiss article alert", 500)
		return
	}
	s.CountsCache.Invalidate(user.ID)

	jsonResponse(w, map[string]string{"status": "ok"})
}

// gzipResponseWriter wraps http.ResponseWriter to compress responses.
type gzipResponseWriter struct {
	http.ResponseWriter
	gz *gzip.Writer
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	return w.gz.Write(b)
}

// gzipMiddleware compresses text responses (HTML, CSS, JS, JSON).
func gzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}
		gz, err := gzip.NewWriterLevel(w, gzip.BestSpeed)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		defer func() { _ = gz.Close() }()
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Del("Content-Length")
		next.ServeHTTP(&gzipResponseWriter{ResponseWriter: w, gz: gz}, r)
	})
}

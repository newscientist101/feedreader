package srv

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/newscientist101/feedreader/db"
	"github.com/newscientist101/feedreader/db/dbgen"
	"github.com/newscientist101/feedreader/srv/scrapers"
)

// benchServer creates a Server backed by an in-memory SQLite DB,
// equivalent to testServer but accepting *testing.B.
func benchServer(b *testing.B) *Server {
	b.Helper()
	schema := getSchema(&testing.T{}) //nolint:all // getSchema needs *testing.T but is safe to call with zero value
	sqlDB, err := db.Open(":memory:")
	if err != nil {
		b.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	b.Cleanup(func() {
		cancel()
		sqlDB.Close()
	})
	if _, err := sqlDB.Exec(schema); err != nil {
		b.Fatal(err)
	}
	s := &Server{
		DB:               sqlDB,
		Hostname:         "test",
		ScraperRunner:    scrapers.NewRunner(),
		StaticHashes:     map[string]string{},
		ShelleyGenerator: NewShelleyScraperGenerator(),
		bgCtx:            ctx,
		bgCancel:         cancel,
	}
	s.RetentionManager = &RetentionManager{server: s, retentionDays: 30}
	return s
}

// benchDB creates a Server with an in-memory DB seeded with realistic data
// for benchmarking. Returns the server, a user context, and useful IDs.
func benchDB(b *testing.B) (s *Server, ctx context.Context, userID, feedID, categoryID int64) {
	b.Helper()
	s = benchServer(b)

	q := dbgen.New(s.DB)
	baseCtx := context.Background()

	// Create user
	dbUser, err := q.GetOrCreateUser(baseCtx, dbgen.GetOrCreateUserParams{
		ExternalID: "bench-user",
		Email:      "bench@test.com",
	})
	if err != nil {
		b.Fatal(err)
	}
	userID = dbUser.ID
	user := &User{ID: userID, ExternalID: dbUser.ExternalID, Email: dbUser.Email}
	ctx = context.WithValue(baseCtx, userContextKey, user)

	// Create 25 categories
	catIDs := make([]int64, 25)
	for i := range catIDs {
		cat, err := q.CreateCategory(baseCtx, dbgen.CreateCategoryParams{
			Name:   fmt.Sprintf("Category %d", i),
			UserID: &userID,
		})
		if err != nil {
			b.Fatal(err)
		}
		catIDs[i] = cat.ID
	}
	categoryID = catIDs[0]

	// Create 200 feeds, each in a category, with 3 articles each (600 total)
	interval := int64(60)
	for i := range 200 {
		feed, err := q.CreateFeed(baseCtx, dbgen.CreateFeedParams{
			Name:                 fmt.Sprintf("Feed %d", i),
			Url:                  fmt.Sprintf("https://example.com/feed/%d", i),
			FeedType:             "rss",
			FetchIntervalMinutes: &interval,
			UserID:               &userID,
		})
		if err != nil {
			b.Fatal(err)
		}
		if i == 0 {
			feedID = feed.ID
		}

		if err := q.AddFeedToCategory(baseCtx, dbgen.AddFeedToCategoryParams{
			FeedID:     feed.ID,
			CategoryID: catIDs[i%len(catIDs)],
		}); err != nil {
			b.Fatal(err)
		}

		for j := range 3 {
			title := fmt.Sprintf("Article %d-%d", i, j)
			url := fmt.Sprintf("https://example.com/feed/%d/article/%d", i, j)
			content := "<p>Test content for benchmarking. This is a realistic paragraph of text that simulates real article content.</p>"
			_, err := q.CreateArticle(baseCtx, dbgen.CreateArticleParams{
				FeedID:  feed.ID,
				Title:   title,
				Url:     &url,
				Content: &content,
				Guid:    url,
			})
			if err != nil {
				b.Fatal(err)
			}
		}
	}

	return s, ctx, userID, feedID, categoryID
}

// --- Count queries ---

func BenchmarkGetUnreadCount(b *testing.B) {
	s, ctx, userID, _, _ := benchDB(b)
	q := dbgen.New(s.DB)

	b.ResetTimer()
	for range b.N {
		_, _ = q.GetUnreadCount(ctx, &userID)
	}
}

func BenchmarkGetStarredCount(b *testing.B) {
	s, ctx, userID, _, _ := benchDB(b)
	q := dbgen.New(s.DB)

	b.ResetTimer()
	for range b.N {
		_, _ = q.GetStarredCount(ctx, &userID)
	}
}

func BenchmarkGetAllFeedUnreadCounts(b *testing.B) {
	s, ctx, userID, _, _ := benchDB(b)
	q := dbgen.New(s.DB)

	b.ResetTimer()
	for range b.N {
		_, _ = q.GetAllFeedUnreadCounts(ctx, &userID)
	}
}

func BenchmarkGetAllCategoryUnreadCounts(b *testing.B) {
	s, ctx, userID, _, _ := benchDB(b)
	q := dbgen.New(s.DB)

	b.ResetTimer()
	for range b.N {
		_, _ = q.GetAllCategoryUnreadCounts(ctx, &userID)
	}
}

func BenchmarkGetArticleCounts(b *testing.B) {
	s, ctx, userID, _, _ := benchDB(b)
	// Disable caching so we measure actual query cost
	s.CountsCache.Invalidate(userID)

	b.ResetTimer()
	for range b.N {
		s.CountsCache.Invalidate(userID)
		_ = s.getArticleCounts(ctx, userID)
	}
}

// --- Article listing queries ---

func BenchmarkListUnreadArticles(b *testing.B) {
	s, ctx, userID, _, _ := benchDB(b)
	q := dbgen.New(s.DB)

	b.ResetTimer()
	for range b.N {
		_, _ = q.ListUnreadArticles(ctx, dbgen.ListUnreadArticlesParams{
			UserID: &userID,
			Limit:  50,
			Offset: 0,
		})
	}
}

func BenchmarkListArticlesByCategory(b *testing.B) {
	s, ctx, userID, _, categoryID := benchDB(b)
	q := dbgen.New(s.DB)

	b.ResetTimer()
	for range b.N {
		_, _ = q.ListArticlesByCategory(ctx, dbgen.ListArticlesByCategoryParams{
			CategoryID: categoryID,
			UserID:     &userID,
			Limit:      50,
			Offset:     0,
		})
	}
}

func BenchmarkListUnreadArticlesByCategory(b *testing.B) {
	s, ctx, userID, _, categoryID := benchDB(b)
	q := dbgen.New(s.DB)

	b.ResetTimer()
	for range b.N {
		_, _ = q.ListUnreadArticlesByCategory(ctx, dbgen.ListUnreadArticlesByCategoryParams{
			CategoryID: categoryID,
			UserID:     &userID,
			Limit:      50,
			Offset:     0,
		})
	}
}

func BenchmarkListArticlesByFeed(b *testing.B) {
	s, ctx, userID, feedID, _ := benchDB(b)
	q := dbgen.New(s.DB)

	b.ResetTimer()
	for range b.N {
		_, _ = q.ListArticlesByFeed(ctx, dbgen.ListArticlesByFeedParams{
			FeedID: feedID,
			UserID: &userID,
			Limit:  50,
			Offset: 0,
		})
	}
}

// --- Cursor-based pagination queries ---

func BenchmarkListArticlesCursor(b *testing.B) {
	s, ctx, userID, _, _ := benchDB(b)
	q := dbgen.New(s.DB)

	farFuture := time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)

	b.ResetTimer()
	for range b.N {
		_, _ = q.ListArticlesCursor(ctx, dbgen.ListArticlesCursorParams{
			UserID:       &userID,
			BeforeTime:   &farFuture,
			BeforeTimeEq: &farFuture,
			BeforeID:     999999,
			Limit:        50,
		})
	}
}

func BenchmarkListUnreadArticlesCursor(b *testing.B) {
	s, ctx, userID, _, _ := benchDB(b)
	q := dbgen.New(s.DB)
	farFuture := time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)

	b.ResetTimer()
	for range b.N {
		_, _ = q.ListUnreadArticlesCursor(ctx, dbgen.ListUnreadArticlesCursorParams{
			UserID:       &userID,
			BeforeTime:   &farFuture,
			BeforeTimeEq: &farFuture,
			BeforeID:     999999,
			Limit:        50,
		})
	}
}

func BenchmarkListArticlesByCategoryCursor(b *testing.B) {
	s, ctx, userID, _, categoryID := benchDB(b)
	q := dbgen.New(s.DB)
	farFuture := time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)

	b.ResetTimer()
	for range b.N {
		_, _ = q.ListArticlesByCategoryCursor(ctx, dbgen.ListArticlesByCategoryCursorParams{
			CategoryID:   categoryID,
			UserID:       &userID,
			BeforeTime:   &farFuture,
			BeforeTimeEq: &farFuture,
			BeforeID:     999999,
			Limit:        50,
		})
	}
}

// --- Search queries ---

func BenchmarkSearchArticles(b *testing.B) {
	s, ctx, userID, _, _ := benchDB(b)
	q := dbgen.New(s.DB)

	searchTerm := "Article"

	b.ResetTimer()
	for range b.N {
		_, _ = q.SearchArticles(ctx, dbgen.SearchArticlesParams{
			UserID:  &userID,
			Column2: &searchTerm,
			Column3: &searchTerm,
			Limit:   50,
			Offset:  0,
		})
	}
}

func BenchmarkSearchArticlesByCategory(b *testing.B) {
	s, ctx, userID, _, categoryID := benchDB(b)
	q := dbgen.New(s.DB)
	searchTerm := "Article"

	b.ResetTimer()
	for range b.N {
		_, _ = q.SearchArticlesByCategory(ctx, dbgen.SearchArticlesByCategoryParams{
			CategoryID: categoryID,
			UserID:     &userID,
			Column3:    &searchTerm,
			Column4:    &searchTerm,
			Limit:      50,
			Offset:     0,
		})
	}
}

// --- Feed listing queries ---

func BenchmarkListFeeds(b *testing.B) {
	s, ctx, userID, _, _ := benchDB(b)
	q := dbgen.New(s.DB)

	b.ResetTimer()
	for range b.N {
		_, _ = q.ListFeeds(ctx, &userID)
	}
}

func BenchmarkListCategories(b *testing.B) {
	s, ctx, userID, _, _ := benchDB(b)
	q := dbgen.New(s.DB)

	b.ResetTimer()
	for range b.N {
		_, _ = q.ListCategories(ctx, &userID)
	}
}

func BenchmarkListFeedCategoryMappings(b *testing.B) {
	s, ctx, userID, _, _ := benchDB(b)
	q := dbgen.New(s.DB)

	b.ResetTimer()
	for range b.N {
		_, _ = q.ListFeedCategoryMappings(ctx, &userID)
	}
}

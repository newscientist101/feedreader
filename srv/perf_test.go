package srv

import (
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"srv.exe.dev/db/dbgen"
	"srv.exe.dev/srv/feeds"
)

// drainClose fully reads and closes the response body to avoid broken pipe
// errors from the gzip middleware.
func drainClose(resp *http.Response) {
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
}

// Performance thresholds. These are generous multiples of measured baselines
// to avoid flaky failures while still catching major regressions.
// Baselines (in-memory SQLite, 200 feeds, 25 categories, 500 articles):
//
//	/article/{id}  ~5ms
//	/              ~30ms
//	/api/counts    ~5ms
var perfThresholds = map[string]time.Duration{
	"/article/1":  50 * time.Millisecond,
	"/":           50 * time.Millisecond,
	"/category/1": 50 * time.Millisecond,
	"/api/counts": 20 * time.Millisecond,
}

// seedPerfData populates the database with enough feeds, categories, and
// articles to surface N+1 query regressions.
func seedPerfData(t *testing.T, s *Server, userID int64) (articleID, categoryID int64) {
	t.Helper()
	q := dbgen.New(s.DB)
	ctx := t
	_ = ctx

	// Create 25 categories
	catIDs := make([]int64, 25)
	for i := range catIDs {
		cat, err := q.CreateCategory(t.Context(), dbgen.CreateCategoryParams{
			Name:   fmt.Sprintf("Category %d", i),
			UserID: &userID,
		})
		if err != nil {
			t.Fatal(err)
		}
		catIDs[i] = cat.ID
	}

	// Create 200 feeds, each assigned to a category, with a few articles each
	interval := int64(60)
	for i := range 200 {
		feed, err := q.CreateFeed(t.Context(), dbgen.CreateFeedParams{
			Name:                 fmt.Sprintf("Feed %d", i),
			Url:                  fmt.Sprintf("https://example.com/feed/%d", i),
			FeedType:             "rss",
			FetchIntervalMinutes: &interval,
			UserID:               &userID,
		})
		if err != nil {
			t.Fatal(err)
		}

		if err := q.AddFeedToCategory(t.Context(), dbgen.AddFeedToCategoryParams{
			FeedID:     feed.ID,
			CategoryID: catIDs[i%len(catIDs)],
		}); err != nil {
			t.Fatal(err)
		}

		// 2-3 articles per feed
		for j := range 3 {
			title := fmt.Sprintf("Article %d-%d", i, j)
			url := fmt.Sprintf("https://example.com/feed/%d/article/%d", i, j)
			content := "<p>Test content for performance benchmarking.</p>"
			article, err := q.CreateArticle(t.Context(), dbgen.CreateArticleParams{
				FeedID:  feed.ID,
				Title:   title,
				Url:     &url,
				Content: &content,
				Guid:    url,
			})
			if err != nil {
				t.Fatal(err)
			}
			if articleID == 0 {
				articleID = article.ID
			}
		}
	}

	return articleID, catIDs[0]
}

func TestPerformance(t *testing.T) {
	ts, s := integrationServer(t)
	s.Fetcher = feeds.NewFetcher(s.DB, s.ScraperRunner)

	// Trigger user auto-creation via auth middleware
	resp := authGet(t, ts, "/api/counts")
	drainClose(resp)

	// Look up the auto-created user
	q := dbgen.New(s.DB)
	dbUser, err := q.GetOrCreateUser(t.Context(), dbgen.GetOrCreateUserParams{
		ExternalID: "integ-user",
		Email:      "integ@test.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	articleID, categoryID := seedPerfData(t, s, dbUser.ID)

	articlePath := fmt.Sprintf("/article/%d", articleID)
	categoryPath := fmt.Sprintf("/category/%d", categoryID)
	endpoints := []string{
		articlePath,
		"/",
		categoryPath,
		"/api/counts",
	}

	// Warm up: one request per endpoint to prime caches/templates
	for _, ep := range endpoints {
		resp := authGet(t, ts, ep)
		drainClose(resp)
	}

	for _, ep := range endpoints {
		// Use the generic threshold key (normalize dynamic IDs)
		thresholdKey := ep
		switch ep {
		case articlePath:
			thresholdKey = "/article/1"
		case categoryPath:
			thresholdKey = "/category/1"
		}
		threshold := perfThresholds[thresholdKey]

		// Take median of 5 runs
		durations := make([]time.Duration, 5)
		for i := range durations {
			start := time.Now()
			resp := authGet(t, ts, ep)
			durations[i] = time.Since(start)
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("%s returned %d", ep, resp.StatusCode)
			}
			drainClose(resp)
		}

		// Sort and take median
		for i := range durations {
			for j := i + 1; j < len(durations); j++ {
				if durations[j] < durations[i] {
					durations[i], durations[j] = durations[j], durations[i]
				}
			}
		}
		median := durations[len(durations)/2]

		t.Logf("%-20s median=%v  threshold=%v  [%v]", ep, median, threshold, durations)
		if median > threshold {
			t.Errorf("%s too slow: median %v exceeds threshold %v", ep, median, threshold)
		}
	}
}

package srv

import (
	"testing"
	"time"

	"github.com/newscientist101/feedreader/db/dbgen"
)

// TestGetArticleCounts_FastPath verifies that users without exclusion rules use
// the direct SQL counts path and get correct counts.
func TestGetArticleCounts_FastPath(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	s.CountsCache = NewCountsCache(30 * time.Second)
	ctx, user := testUser(t, s)
	feed := createFeed(t, s, user.ID, "feed1", "http://feed1.test/rss")

	// Create a few unread articles
	createArticle(t, s, feed.ID, "Article 1", "g1")
	createArticle(t, s, feed.ID, "Article 2", "g2")

	counts := s.getArticleCounts(ctx, user.ID)
	if counts.Unread != 2 {
		t.Errorf("expected 2 unread, got %d", counts.Unread)
	}
}

// TestGetArticleCounts_WithRules verifies that users with at least one
// exclusion rule are routed through getFilteredArticleCounts and that
// matching articles are excluded from counts.
func TestGetArticleCounts_WithRules(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	s.CountsCache = NewCountsCache(30 * time.Second)
	ctx, user := testUser(t, s)
	q := dbgen.New(s.DB)
	feed := createFeed(t, s, user.ID, "feed1", "http://feed1.test/rss")
	createArticle(t, s, feed.ID, "Normal article", "g1")
	createArticle(t, s, feed.ID, "spam article should be excluded", "g2")

	// Create a category, assign feed, add exclusion rule matching "spam".
	cat, err := q.CreateCategory(ctx, dbgen.CreateCategoryParams{Name: "Tech", UserID: &user.ID})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.AddFeedToCategory(ctx, dbgen.AddFeedToCategoryParams{FeedID: feed.ID, CategoryID: cat.ID}); err != nil {
		t.Fatal(err)
	}
	var zero int64
	_, err = q.CreateExclusion(ctx, dbgen.CreateExclusionParams{
		CategoryID:    cat.ID,
		ExclusionType: "keyword",
		Pattern:       "spam",
		IsRegex:       &zero,
	})
	if err != nil {
		t.Fatal(err)
	}

	counts := s.getArticleCounts(ctx, user.ID)
	// Exclusion rule removes 1 article, so unread should be 1.
	if counts.Unread != 1 {
		t.Errorf("expected 1 unread, got %d", counts.Unread)
	}
	// Feed count also drops by 1.
	if counts.FeedCounts[feed.ID] != 1 {
		t.Errorf("expected feedCounts[%d]=1, got %d", feed.ID, counts.FeedCounts[feed.ID])
	}
	// Category count drops by 1.
	if counts.CatCounts[cat.ID] != 1 {
		t.Errorf("expected catCounts[%d]=1, got %d", cat.ID, counts.CatCounts[cat.ID])
	}
}

// TestGetArticleCounts_FilteredFeedNoCat verifies that when a feed has an
// exclusion rule match but the feed is in NO category, FeedCounts decrements
// correctly but CatCounts is unaffected (no panic, no negative).
func TestGetArticleCounts_FilteredFeedNoCat(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	s.CountsCache = NewCountsCache(30 * time.Second)
	ctx, user := testUser(t, s)
	q := dbgen.New(s.DB)

	// Feed NOT added to any category.
	feed := createFeed(t, s, user.ID, "uncategorized", "http://uncat.test/rss")
	createArticle(t, s, feed.ID, "Normal article", "g1")
	createArticle(t, s, feed.ID, "spam excluded article", "g2")

	// Create a category with an exclusion rule, but do NOT add the feed to it.
	cat, err := q.CreateCategory(ctx, dbgen.CreateCategoryParams{Name: "Cat", UserID: &user.ID})
	if err != nil {
		t.Fatal(err)
	}
	var zero int64
	_, err = q.CreateExclusion(ctx, dbgen.CreateExclusionParams{
		CategoryID:    cat.ID,
		ExclusionType: "keyword",
		Pattern:       "spam",
		IsRegex:       &zero,
	})
	if err != nil {
		t.Fatal(err)
	}

	counts := s.getArticleCounts(ctx, user.ID)
	// Feed is not in the category, so exclusion rule does NOT apply.
	// Both articles remain unread.
	if counts.Unread != 2 {
		t.Errorf("expected 2 unread (feed not in cat), got %d", counts.Unread)
	}
	if counts.FeedCounts[feed.ID] != 2 {
		t.Errorf("expected feedCounts[%d]=2, got %d", feed.ID, counts.FeedCounts[feed.ID])
	}
	// CatCounts for the category should be 0 (feed not in it).
	if v := counts.CatCounts[cat.ID]; v != 0 {
		t.Errorf("expected catCounts[%d]=0, got %d", cat.ID, v)
	}
}

// TestGetArticleCounts_AlertsIgnoreFilter verifies that alertsCount is
// independent of exclusion rules: an alert on an excluded article still counts.
func TestGetArticleCounts_AlertsIgnoreFilter(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	s.CountsCache = NewCountsCache(30 * time.Second)
	ctx, user := testUser(t, s)
	q := dbgen.New(s.DB)

	feed := createFeed(t, s, user.ID, "feed1", "http://feed1.test/rss")
	// Article that will be excluded by the rule.
	article := createArticle(t, s, feed.ID, "spam excluded article", "g1")

	// Create category, add feed, add exclusion rule matching "spam".
	cat, err := q.CreateCategory(ctx, dbgen.CreateCategoryParams{Name: "Cat", UserID: &user.ID})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.AddFeedToCategory(ctx, dbgen.AddFeedToCategoryParams{FeedID: feed.ID, CategoryID: cat.ID}); err != nil {
		t.Fatal(err)
	}
	var zero int64
	_, err = q.CreateExclusion(ctx, dbgen.CreateExclusionParams{
		CategoryID:    cat.ID,
		ExclusionType: "keyword",
		Pattern:       "spam",
		IsRegex:       &zero,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create an alert and fire it on the excluded article.
	alert, err := q.CreateAlert(ctx, dbgen.CreateAlertParams{
		UserID:     user.ID,
		Name:       "test alert",
		Pattern:    "spam",
		IsRegex:    0,
		MatchField: "title",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.InsertArticleAlert(ctx, dbgen.InsertArticleAlertParams{
		ArticleID: article.ID,
		AlertID:   alert.ID,
	}); err != nil {
		t.Fatal(err)
	}

	counts := s.getArticleCounts(ctx, user.ID)
	// Unread should be 0 (excluded by rule).
	if counts.Unread != 0 {
		t.Errorf("expected 0 unread (excluded), got %d", counts.Unread)
	}
	// Alerts should still be 1 (alert fires on excluded articles).
	if counts.Alerts != 1 {
		t.Errorf("expected alerts=1, got %d", counts.Alerts)
	}
}

// TestGetArticleCounts_CacheHit verifies that a cached result is returned
// without hitting the DB again.
func TestGetArticleCounts_CacheHit(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	s.CountsCache = NewCountsCache(30 * time.Second)
	ctx, user := testUser(t, s)
	feed := createFeed(t, s, user.ID, "feed1", "http://feed1.test/rss")
	createArticle(t, s, feed.ID, "Article 1", "g1")

	// First call populates the cache.
	counts1 := s.getArticleCounts(ctx, user.ID)
	if counts1.Unread != 1 {
		t.Fatalf("expected 1 unread, got %d", counts1.Unread)
	}

	// Prime the cache with a different value to prove the second call uses it.
	s.CountsCache.Set(user.ID, articleCounts{Unread: 42})
	counts2 := s.getArticleCounts(ctx, user.ID)
	if counts2.Unread != 42 {
		t.Errorf("expected 42 (cached), got %d", counts2.Unread)
	}

	// After invalidation a fresh count is returned.
	s.CountsCache.Invalidate(user.ID)
	counts3 := s.getArticleCounts(ctx, user.ID)
	if counts3.Unread != 1 {
		t.Errorf("expected 1 after invalidation, got %d", counts3.Unread)
	}
}

// TestGetArticleCounts_UserIsolation verifies that counts are isolated
// per user: exclusion rules for user A don't affect user B's fast path.
// Two separate servers (and thus DBs) are used to avoid shared state from
// the single-user testUser helper.
func TestGetArticleCounts_UserIsolation(t *testing.T) {
	t.Parallel()

	// Server A: user has an exclusion rule → filtered path
	sA := testServer(t)
	sA.CountsCache = NewCountsCache(30 * time.Second)
	ctxA, userA := testUser(t, sA)
	qA := dbgen.New(sA.DB)
	feedA := createFeed(t, sA, userA.ID, "feedA", "http://feeda.test/rss")
	createArticle(t, sA, feedA.ID, "A article 1", "gA1")

	catA, _ := qA.CreateCategory(ctxA, dbgen.CreateCategoryParams{Name: "CatA", UserID: &userA.ID})
	var zero int64
	_, _ = qA.CreateExclusion(ctxA, dbgen.CreateExclusionParams{
		CategoryID:    catA.ID,
		ExclusionType: "keyword",
		Pattern:       "spam",
		IsRegex:       &zero,
	})

	countsA := sA.getArticleCounts(ctxA, userA.ID)
	if countsA.Unread != 1 {
		t.Errorf("userA: expected 1 unread, got %d", countsA.Unread)
	}

	// Server B: user has no exclusion rules → fast path
	sB := testServer(t)
	sB.CountsCache = NewCountsCache(30 * time.Second)
	ctxB, userB := testUser(t, sB)
	feedB := createFeed(t, sB, userB.ID, "feedB", "http://feedb.test/rss")
	createArticle(t, sB, feedB.ID, "B article 1", "gB1")
	createArticle(t, sB, feedB.ID, "B article 2", "gB2")

	countsB := sB.getArticleCounts(ctxB, userB.ID)
	if countsB.Unread != 2 {
		t.Errorf("userB: expected 2 unread, got %d", countsB.Unread)
	}
}

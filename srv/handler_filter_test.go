package srv

import (
	"context"
	"testing"

	"srv.exe.dev/db/dbgen"
)

func TestFilterArticles_NoCategory(t *testing.T) {
	s := testServer(t)
	ctx, user := testUser(t, s)

	articles := []dbgen.ListUnreadArticlesRow{{Title: "test"}}
	got := s.FilterArticles(ctx, articles, 0, user.ID)
	if len(got) != 1 {
		t.Fatalf("expected 1 article, got %d", len(got))
	}
}

func TestFilterArticles_WithExclusions(t *testing.T) {
	s := testServer(t)
	ctx, user := testUser(t, s)
	q := dbgen.New(s.DB)

	cat, err := q.CreateCategory(ctx, dbgen.CreateCategoryParams{Name: "News", UserID: &user.ID})
	if err != nil {
		t.Fatal(err)
	}

	// Create a keyword exclusion
	var zero int64
	_, err = q.CreateExclusion(ctx, dbgen.CreateExclusionParams{
		CategoryID: cat.ID, ExclusionType: "keyword", Pattern: "sponsored", IsRegex: &zero,
	})
	if err != nil {
		t.Fatal(err)
	}

	sponsored := "Check out this sponsored post"
	articles := []dbgen.ListUnreadArticlesRow{
		{Title: "Good article"},
		{Title: "Sponsored content", Summary: &sponsored},
		{Title: "Another good one"},
	}

	got := s.FilterArticles(ctx, articles, cat.ID, user.ID)
	if len(got) != 2 {
		t.Fatalf("expected 2 articles after filtering, got %d", len(got))
	}
	if got[0].Title != "Good article" || got[1].Title != "Another good one" {
		t.Errorf("wrong articles returned: %v, %v", got[0].Title, got[1].Title)
	}
}

func TestFilterArticlesByCategory(t *testing.T) {
	s := testServer(t)
	ctx, user := testUser(t, s)
	q := dbgen.New(s.DB)

	cat, _ := q.CreateCategory(ctx, dbgen.CreateCategoryParams{Name: "Tech", UserID: &user.ID})
	var one int64 = 1
	q.CreateExclusion(ctx, dbgen.CreateExclusionParams{
		CategoryID: cat.ID, ExclusionType: "author", Pattern: "spammer", IsRegex: &one,
	})

	author := "spammer123"
	articles := []dbgen.ListUnreadArticlesByCategoryRow{
		{Title: "Good"},
		{Title: "Bad", Author: &author},
	}

	got := s.FilterArticlesByCategory(ctx, articles, cat.ID, user.ID)
	if len(got) != 1 {
		t.Fatalf("expected 1 article, got %d", len(got))
	}
	if got[0].Title != "Good" {
		t.Errorf("wrong article: %s", got[0].Title)
	}
}

func TestFilterArticlesByFeed_NoCategory(t *testing.T) {
	s := testServer(t)
	ctx, user := testUser(t, s)
	feed := createFeed(t, s, user.ID, "Test", "https://example.com/feed")

	// Feed has no category — should return all articles unfiltered
	articles := []dbgen.ListArticlesByFeedRow{{Title: "a"}, {Title: "b"}}
	got := s.FilterArticlesByFeed(context.Background(), articles, feed.ID, user.ID)
	if len(got) != 2 {
		t.Fatalf("expected 2 articles, got %d", len(got))
	}
	_ = ctx
}

func TestFilterArticlesByFeed_WithCategory(t *testing.T) {
	s := testServer(t)
	ctx, user := testUser(t, s)
	q := dbgen.New(s.DB)

	feed := createFeed(t, s, user.ID, "Test", "https://example.com/feed")
	cat, _ := q.CreateCategory(ctx, dbgen.CreateCategoryParams{Name: "Tech", UserID: &user.ID})
	q.AddFeedToCategory(ctx, dbgen.AddFeedToCategoryParams{FeedID: feed.ID, CategoryID: cat.ID})

	var zero int64
	q.CreateExclusion(ctx, dbgen.CreateExclusionParams{
		CategoryID: cat.ID, ExclusionType: "keyword", Pattern: "blocked", IsRegex: &zero,
	})

	articles := []dbgen.ListArticlesByFeedRow{
		{Title: "Normal"},
		{Title: "This is blocked content"},
	}

	got := s.FilterArticlesByFeed(ctx, articles, feed.ID, user.ID)
	if len(got) != 1 {
		t.Fatalf("expected 1 article, got %d", len(got))
	}
	if got[0].Title != "Normal" {
		t.Errorf("wrong article: %s", got[0].Title)
	}
}

// ---------------------------------------------------------------------------
// FilterAllUnreadArticles
// ---------------------------------------------------------------------------

func TestFilterAllUnreadArticles_NoExclusions(t *testing.T) {
	s := testServer(t)
	ctx, user := testUser(t, s)

	articles := []dbgen.ListUnreadArticlesRow{
		{Title: "Article A", FeedID: 1},
		{Title: "Article B", FeedID: 2},
	}
	got := s.FilterAllUnreadArticles(ctx, articles, user.ID)
	if len(got) != 2 {
		t.Fatalf("expected 2 articles, got %d", len(got))
	}
}

func TestFilterAllUnreadArticles_FiltersAcrossCategories(t *testing.T) {
	s := testServer(t)
	ctx, user := testUser(t, s)
	q := dbgen.New(s.DB)

	// Create two categories with different exclusion rules
	catNews, _ := q.CreateCategory(ctx, dbgen.CreateCategoryParams{Name: "News", UserID: &user.ID})
	catTech, _ := q.CreateCategory(ctx, dbgen.CreateCategoryParams{Name: "Tech", UserID: &user.ID})

	var zero int64
	q.CreateExclusion(ctx, dbgen.CreateExclusionParams{
		CategoryID: catNews.ID, ExclusionType: "keyword", Pattern: "sponsored", IsRegex: &zero,
	})
	q.CreateExclusion(ctx, dbgen.CreateExclusionParams{
		CategoryID: catTech.ID, ExclusionType: "author", Pattern: "spambot", IsRegex: &zero,
	})

	// Create feeds in each category
	feedNews := createFeed(t, s, user.ID, "BBC", "https://bbc.co.uk/feed")
	q.AddFeedToCategory(ctx, dbgen.AddFeedToCategoryParams{FeedID: feedNews.ID, CategoryID: catNews.ID})

	feedTech := createFeed(t, s, user.ID, "Hacker News", "https://hn.com/feed")
	q.AddFeedToCategory(ctx, dbgen.AddFeedToCategoryParams{FeedID: feedTech.ID, CategoryID: catTech.ID})

	spamAuthor := "spambot"
	articles := []dbgen.ListUnreadArticlesRow{
		{Title: "Normal news", FeedID: feedNews.ID},
		{Title: "Sponsored post", FeedID: feedNews.ID}, // keyword match in News
		{Title: "Good tech article", FeedID: feedTech.ID},
		{Title: "Spam tech article", FeedID: feedTech.ID, Author: &spamAuthor}, // author match in Tech
	}

	got := s.FilterAllUnreadArticles(ctx, articles, user.ID)
	if len(got) != 2 {
		t.Fatalf("expected 2 articles, got %d", len(got))
	}
	if got[0].Title != "Normal news" {
		t.Errorf("expected 'Normal news', got %q", got[0].Title)
	}
	if got[1].Title != "Good tech article" {
		t.Errorf("expected 'Good tech article', got %q", got[1].Title)
	}
}

func TestFilterAllUnreadArticles_UncategorizedFeedsUnfiltered(t *testing.T) {
	s := testServer(t)
	ctx, user := testUser(t, s)
	q := dbgen.New(s.DB)

	// Create a category with exclusions
	cat, _ := q.CreateCategory(ctx, dbgen.CreateCategoryParams{Name: "News", UserID: &user.ID})
	var zero int64
	q.CreateExclusion(ctx, dbgen.CreateExclusionParams{
		CategoryID: cat.ID, ExclusionType: "keyword", Pattern: "blocked", IsRegex: &zero,
	})

	// Create a feed NOT in any category
	feedNoCat := createFeed(t, s, user.ID, "NoCat", "https://nocat.com/feed")

	// Create a feed in the category
	feedInCat := createFeed(t, s, user.ID, "InCat", "https://incat.com/feed")
	q.AddFeedToCategory(ctx, dbgen.AddFeedToCategoryParams{FeedID: feedInCat.ID, CategoryID: cat.ID})

	articles := []dbgen.ListUnreadArticlesRow{
		{Title: "Blocked from uncategorized", FeedID: feedNoCat.ID}, // no category → not filtered
		{Title: "Blocked from categorized", FeedID: feedInCat.ID},   // keyword match → filtered
		{Title: "Normal from categorized", FeedID: feedInCat.ID},
	}

	got := s.FilterAllUnreadArticles(ctx, articles, user.ID)
	if len(got) != 2 {
		t.Fatalf("expected 2 articles, got %d", len(got))
	}
	if got[0].Title != "Blocked from uncategorized" {
		t.Errorf("expected uncategorized article to pass, got %q", got[0].Title)
	}
	if got[1].Title != "Normal from categorized" {
		t.Errorf("expected 'Normal from categorized', got %q", got[1].Title)
	}
}

func TestFilterAllUnreadArticles_RegexExclusion(t *testing.T) {
	s := testServer(t)
	ctx, user := testUser(t, s)
	q := dbgen.New(s.DB)

	cat, _ := q.CreateCategory(ctx, dbgen.CreateCategoryParams{Name: "Tech", UserID: &user.ID})
	var one int64 = 1
	q.CreateExclusion(ctx, dbgen.CreateExclusionParams{
		CategoryID: cat.ID, ExclusionType: "keyword", Pattern: `^\[AD\]`, IsRegex: &one,
	})

	feed := createFeed(t, s, user.ID, "Blog", "https://blog.com/feed")
	q.AddFeedToCategory(ctx, dbgen.AddFeedToCategoryParams{FeedID: feed.ID, CategoryID: cat.ID})

	articles := []dbgen.ListUnreadArticlesRow{
		{Title: "[AD] Buy now", FeedID: feed.ID},
		{Title: "Normal post", FeedID: feed.ID},
		{Title: "Not an [AD] in the middle", FeedID: feed.ID}, // regex anchored to start → not filtered
	}

	got := s.FilterAllUnreadArticles(ctx, articles, user.ID)
	if len(got) != 2 {
		t.Fatalf("expected 2 articles, got %d", len(got))
	}
	if got[0].Title != "Normal post" {
		t.Errorf("expected 'Normal post', got %q", got[0].Title)
	}
	if got[1].Title != "Not an [AD] in the middle" {
		t.Errorf("expected 'Not an [AD] in the middle', got %q", got[1].Title)
	}
}

func TestFilterAllUnreadArticles_FeedInMultipleCategories(t *testing.T) {
	s := testServer(t)
	ctx, user := testUser(t, s)
	q := dbgen.New(s.DB)

	// A feed in two categories — exclusion in either should filter
	cat1, _ := q.CreateCategory(ctx, dbgen.CreateCategoryParams{Name: "Cat1", UserID: &user.ID})
	cat2, _ := q.CreateCategory(ctx, dbgen.CreateCategoryParams{Name: "Cat2", UserID: &user.ID})

	var zero int64
	// Only cat2 has an exclusion
	q.CreateExclusion(ctx, dbgen.CreateExclusionParams{
		CategoryID: cat2.ID, ExclusionType: "keyword", Pattern: "clickbait", IsRegex: &zero,
	})

	feed := createFeed(t, s, user.ID, "Multi", "https://multi.com/feed")
	q.AddFeedToCategory(ctx, dbgen.AddFeedToCategoryParams{FeedID: feed.ID, CategoryID: cat1.ID})
	q.AddFeedToCategory(ctx, dbgen.AddFeedToCategoryParams{FeedID: feed.ID, CategoryID: cat2.ID})

	articles := []dbgen.ListUnreadArticlesRow{
		{Title: "Clickbait title", FeedID: feed.ID},
		{Title: "Serious article", FeedID: feed.ID},
	}

	got := s.FilterAllUnreadArticles(ctx, articles, user.ID)
	if len(got) != 1 {
		t.Fatalf("expected 1 article, got %d", len(got))
	}
	if got[0].Title != "Serious article" {
		t.Errorf("expected 'Serious article', got %q", got[0].Title)
	}
}

func TestFilterAllUnreadArticles_SummaryMatch(t *testing.T) {
	s := testServer(t)
	ctx, user := testUser(t, s)
	q := dbgen.New(s.DB)

	cat, _ := q.CreateCategory(ctx, dbgen.CreateCategoryParams{Name: "News", UserID: &user.ID})
	var zero int64
	q.CreateExclusion(ctx, dbgen.CreateExclusionParams{
		CategoryID: cat.ID, ExclusionType: "keyword", Pattern: "casino", IsRegex: &zero,
	})

	feed := createFeed(t, s, user.ID, "Feed", "https://feed.com/rss")
	q.AddFeedToCategory(ctx, dbgen.AddFeedToCategoryParams{FeedID: feed.ID, CategoryID: cat.ID})

	casinoSummary := "Visit our online casino for great deals"
	articles := []dbgen.ListUnreadArticlesRow{
		{Title: "Innocent title", FeedID: feed.ID, Summary: &casinoSummary}, // summary match
		{Title: "Clean article", FeedID: feed.ID},
	}

	got := s.FilterAllUnreadArticles(ctx, articles, user.ID)
	if len(got) != 1 {
		t.Fatalf("expected 1 article, got %d", len(got))
	}
	if got[0].Title != "Clean article" {
		t.Errorf("expected 'Clean article', got %q", got[0].Title)
	}
}

func TestFilterAllArticles_NoExclusions(t *testing.T) {
	s := testServer(t)
	ctx, user := testUser(t, s)

	articles := []dbgen.ListArticlesRow{
		{Title: "One"},
		{Title: "Two"},
	}
	got := s.FilterAllArticles(ctx, articles, user.ID)
	if len(got) != 2 {
		t.Fatalf("expected 2 articles, got %d", len(got))
	}
}

func TestFilterAllArticles_FiltersAcrossCategories(t *testing.T) {
	s := testServer(t)
	ctx, user := testUser(t, s)
	q := dbgen.New(s.DB)

	cat, _ := q.CreateCategory(ctx, dbgen.CreateCategoryParams{Name: "News", UserID: &user.ID})
	var zero int64
	q.CreateExclusion(ctx, dbgen.CreateExclusionParams{
		CategoryID: cat.ID, ExclusionType: "keyword", Pattern: "sponsored", IsRegex: &zero,
	})

	feed := createFeed(t, s, user.ID, "BBC", "https://bbc.co.uk/feed")
	q.AddFeedToCategory(ctx, dbgen.AddFeedToCategoryParams{FeedID: feed.ID, CategoryID: cat.ID})

	articles := []dbgen.ListArticlesRow{
		{Title: "Good article", FeedID: feed.ID},
		{Title: "Sponsored post", FeedID: feed.ID},
		{Title: "Another good one", FeedID: feed.ID},
	}

	got := s.FilterAllArticles(ctx, articles, user.ID)
	if len(got) != 2 {
		t.Fatalf("expected 2 articles after filtering, got %d", len(got))
	}
	if got[0].Title != "Good article" {
		t.Errorf("got[0] = %q, want 'Good article'", got[0].Title)
	}
	if got[1].Title != "Another good one" {
		t.Errorf("got[1] = %q, want 'Another good one'", got[1].Title)
	}
}

func TestFilterArticlesByCategoryAll(t *testing.T) {
	s := testServer(t)
	ctx, user := testUser(t, s)
	q := dbgen.New(s.DB)

	cat, _ := q.CreateCategory(ctx, dbgen.CreateCategoryParams{Name: "Tech", UserID: &user.ID})
	var zero int64
	q.CreateExclusion(ctx, dbgen.CreateExclusionParams{
		CategoryID: cat.ID, ExclusionType: "author", Pattern: "spammer", IsRegex: &zero,
	})

	spammer := "spammer"
	articles := []dbgen.ListArticlesByCategoryRow{
		{Title: "Good"},
		{Title: "Bad", Author: &spammer},
	}

	got := s.FilterArticlesByCategoryAll(ctx, articles, cat.ID, user.ID)
	if len(got) != 1 {
		t.Fatalf("expected 1 article, got %d", len(got))
	}
	if got[0].Title != "Good" {
		t.Errorf("got %q, want 'Good'", got[0].Title)
	}
}

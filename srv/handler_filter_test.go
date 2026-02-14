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

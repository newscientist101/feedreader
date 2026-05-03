package dbgen_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/newscientist101/feedreader/db"
	"github.com/newscientist101/feedreader/db/dbgen"

	_ "modernc.org/sqlite"
)

// setupTestDB opens an in-memory SQLite DB, runs migrations, and returns
// a *dbgen.Queries ready for use.
func setupTestDB(t *testing.T) (*sql.DB, *dbgen.Queries) {
	t.Helper()
	sqlDB, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	if err := db.RunMigrations(sqlDB); err != nil {
		t.Fatal(err)
	}
	return sqlDB, dbgen.New(sqlDB)
}

// createTestUser is a helper that creates a user and fails the test on error.
func createTestUser(t *testing.T, q *dbgen.Queries, extID, email string) dbgen.User {
	t.Helper()
	u, err := q.CreateUser(context.Background(), dbgen.CreateUserParams{
		ExternalID: extID,
		Email:      email,
	})
	if err != nil {
		t.Fatalf("CreateUser(%s): %v", extID, err)
	}
	return u
}

// createTestFeed is a helper that creates a feed owned by the given user.
func createTestFeed(t *testing.T, q *dbgen.Queries, name, url string, userID int64) dbgen.Feed {
	t.Helper()
	f, err := q.CreateFeed(context.Background(), dbgen.CreateFeedParams{
		Name:     name,
		Url:      url,
		FeedType: "rss",
		UserID:   &userID,
	})
	if err != nil {
		t.Fatalf("CreateFeed(%s): %v", name, err)
	}
	return f
}

// createTestArticle is a helper that creates an article in the given feed.
func createTestArticle(t *testing.T, q *dbgen.Queries, feedID int64, guid, title string) dbgen.Article {
	t.Helper()
	a, err := q.CreateArticle(context.Background(), dbgen.CreateArticleParams{
		FeedID:  feedID,
		Guid:    guid,
		Title:   title,
		Url:     new("https://example.com/" + guid),
		Content: new("Content of " + title),
	})
	if err != nil {
		t.Fatalf("CreateArticle(%s): %v", guid, err)
	}
	return a
}

// ---------------------------------------------------------------------------
// Users
// ---------------------------------------------------------------------------

func TestUsers_CreateAndGet(t *testing.T) {
	_, q := setupTestDB(t)
	ctx := context.Background()

	u := createTestUser(t, q, "ext-1", "alice@example.com")
	if u.ID == 0 {
		t.Fatal("expected non-zero user ID")
	}
	if u.ExternalID != "ext-1" {
		t.Fatalf("external_id = %q, want %q", u.ExternalID, "ext-1")
	}

	// GetUserByExternalID
	got, err := q.GetUserByExternalID(ctx, "ext-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != u.ID {
		t.Fatalf("GetUserByExternalID returned id=%d, want %d", got.ID, u.ID)
	}
}

func TestUsers_GetOrCreate(t *testing.T) {
	_, q := setupTestDB(t)
	ctx := context.Background()

	u1, err := q.GetOrCreateUser(ctx, dbgen.GetOrCreateUserParams{
		ExternalID: "ext-oc",
		Email:      "bob@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Second call should return the same user (upsert).
	u2, err := q.GetOrCreateUser(ctx, dbgen.GetOrCreateUserParams{
		ExternalID: "ext-oc",
		Email:      "bob-updated@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	if u2.ID != u1.ID {
		t.Fatalf("GetOrCreateUser returned different ID on second call: %d vs %d", u2.ID, u1.ID)
	}
	if u2.Email != "bob-updated@example.com" {
		t.Fatalf("email not updated: got %q", u2.Email)
	}
}

func TestUsers_UpdateLastSeen(t *testing.T) {
	_, q := setupTestDB(t)
	ctx := context.Background()

	u := createTestUser(t, q, "ext-ls", "ls@example.com")
	err := q.UpdateUserLastSeen(ctx, dbgen.UpdateUserLastSeenParams{
		Email: "ls-new@example.com",
		ID:    u.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := q.GetUserByExternalID(ctx, "ext-ls")
	if err != nil {
		t.Fatal(err)
	}
	if got.Email != "ls-new@example.com" {
		t.Fatalf("email after UpdateUserLastSeen = %q, want %q", got.Email, "ls-new@example.com")
	}
}

// ---------------------------------------------------------------------------
// Feeds & Categories
// ---------------------------------------------------------------------------

func TestFeeds_CRUD(t *testing.T) {
	_, q := setupTestDB(t)
	ctx := context.Background()

	u := createTestUser(t, q, "ext-f", "feed@example.com")

	// Create
	f := createTestFeed(t, q, "My Feed", "https://example.com/feed.xml", u.ID)
	if f.ID == 0 {
		t.Fatal("expected non-zero feed ID")
	}
	if f.Name != "My Feed" {
		t.Fatalf("feed name = %q", f.Name)
	}

	// GetFeed
	got, err := q.GetFeed(ctx, dbgen.GetFeedParams{ID: f.ID, UserID: &u.ID})
	if err != nil {
		t.Fatal(err)
	}
	if got.Url != "https://example.com/feed.xml" {
		t.Fatalf("feed url = %q", got.Url)
	}

	// GetFeedByURL
	got2, err := q.GetFeedByURL(ctx, dbgen.GetFeedByURLParams{Url: f.Url, UserID: &u.ID})
	if err != nil {
		t.Fatal(err)
	}
	if got2.ID != f.ID {
		t.Fatalf("GetFeedByURL id = %d, want %d", got2.ID, f.ID)
	}

	// ListFeeds
	feeds, err := q.ListFeeds(ctx, &u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(feeds) != 1 {
		t.Fatalf("ListFeeds count = %d, want 1", len(feeds))
	}

	// UpdateFeed
	err = q.UpdateFeed(ctx, dbgen.UpdateFeedParams{
		Name: "Renamed Feed", Url: f.Url, FeedType: "atom",
		ID: f.ID, UserID: &u.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	got3, _ := q.GetFeed(ctx, dbgen.GetFeedParams{ID: f.ID, UserID: &u.ID})
	if got3.Name != "Renamed Feed" {
		t.Fatalf("after update name = %q", got3.Name)
	}

	// UpdateFeedLastFetched
	now := time.Now().Truncate(time.Second)
	err = q.UpdateFeedLastFetched(ctx, dbgen.UpdateFeedLastFetchedParams{
		LastFetchedAt: &now, ID: f.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	// UpdateFeedSiteURL
	err = q.UpdateFeedSiteURL(ctx, dbgen.UpdateFeedSiteURLParams{
		SiteUrl: "https://example.com", ID: f.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	// GetFeedOwner
	owner, err := q.GetFeedOwner(ctx, f.ID)
	if err != nil {
		t.Fatal(err)
	}
	if owner == nil || *owner != u.ID {
		t.Fatalf("GetFeedOwner = %v, want %d", owner, u.ID)
	}

	// DeleteFeed
	err = q.DeleteFeed(ctx, dbgen.DeleteFeedParams{ID: f.ID, UserID: &u.ID})
	if err != nil {
		t.Fatal(err)
	}
	feeds, _ = q.ListFeeds(ctx, &u.ID)
	if len(feeds) != 0 {
		t.Fatalf("after delete: ListFeeds count = %d", len(feeds))
	}
}

func TestCategories_CRUD(t *testing.T) {
	_, q := setupTestDB(t)
	ctx := context.Background()

	u := createTestUser(t, q, "ext-c", "cat@example.com")

	// Create
	cat, err := q.CreateCategory(ctx, dbgen.CreateCategoryParams{Name: "Tech", UserID: &u.ID})
	if err != nil {
		t.Fatal(err)
	}
	if cat.Name != "Tech" {
		t.Fatalf("category name = %q", cat.Name)
	}

	// GetCategory
	got, err := q.GetCategory(ctx, dbgen.GetCategoryParams{ID: cat.ID, UserID: &u.ID})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Tech" {
		t.Fatalf("GetCategory name = %q", got.Name)
	}

	// GetCategoryByName
	got2, err := q.GetCategoryByName(ctx, dbgen.GetCategoryByNameParams{Name: "Tech", UserID: &u.ID})
	if err != nil {
		t.Fatal(err)
	}
	if got2.ID != cat.ID {
		t.Fatalf("GetCategoryByName id mismatch")
	}

	// ListCategories
	cats, err := q.ListCategories(ctx, &u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(cats) != 1 {
		t.Fatalf("ListCategories count = %d, want 1", len(cats))
	}

	// UpdateCategory
	err = q.UpdateCategory(ctx, dbgen.UpdateCategoryParams{Name: "Science", ID: cat.ID, UserID: &u.ID})
	if err != nil {
		t.Fatal(err)
	}
	got3, _ := q.GetCategory(ctx, dbgen.GetCategoryParams{ID: cat.ID, UserID: &u.ID})
	if got3.Name != "Science" {
		t.Fatalf("after update name = %q", got3.Name)
	}

	// UpdateCategorySortOrder
	err = q.UpdateCategorySortOrder(ctx, dbgen.UpdateCategorySortOrderParams{
		SortOrder: new(int64(5)), ID: cat.ID, UserID: &u.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	// DeleteCategory
	err = q.DeleteCategory(ctx, dbgen.DeleteCategoryParams{ID: cat.ID, UserID: &u.ID})
	if err != nil {
		t.Fatal(err)
	}
	cats, _ = q.ListCategories(ctx, &u.ID)
	if len(cats) != 0 {
		t.Fatalf("after delete: count = %d", len(cats))
	}
}

func TestFeedCategoryAssociation(t *testing.T) {
	_, q := setupTestDB(t)
	ctx := context.Background()

	u := createTestUser(t, q, "ext-fca", "fca@example.com")
	f := createTestFeed(t, q, "Feed A", "https://a.com/feed", u.ID)
	cat, _ := q.CreateCategory(ctx, dbgen.CreateCategoryParams{Name: "News", UserID: &u.ID})

	// AddFeedToCategory
	err := q.AddFeedToCategory(ctx, dbgen.AddFeedToCategoryParams{FeedID: f.ID, CategoryID: cat.ID})
	if err != nil {
		t.Fatal(err)
	}

	// GetFeedCategories
	cats, err := q.GetFeedCategories(ctx, f.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(cats) != 1 || cats[0].ID != cat.ID {
		t.Fatalf("GetFeedCategories = %v", cats)
	}

	// ListFeedsByCategory
	feeds, err := q.ListFeedsByCategory(ctx, dbgen.ListFeedsByCategoryParams{CategoryID: cat.ID, UserID: &u.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(feeds) != 1 {
		t.Fatalf("ListFeedsByCategory count = %d", len(feeds))
	}

	// ListUncategorizedFeeds should be empty
	uncats, err := q.ListUncategorizedFeeds(ctx, &u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(uncats) != 0 {
		t.Fatalf("ListUncategorizedFeeds = %d, want 0", len(uncats))
	}

	// RemoveFeedFromCategory
	err = q.RemoveFeedFromCategory(ctx, dbgen.RemoveFeedFromCategoryParams{FeedID: f.ID, CategoryID: cat.ID})
	if err != nil {
		t.Fatal(err)
	}
	uncats, _ = q.ListUncategorizedFeeds(ctx, &u.ID)
	if len(uncats) != 1 {
		t.Fatalf("after remove: ListUncategorizedFeeds = %d, want 1", len(uncats))
	}

	// ClearFeedCategories
	_ = q.AddFeedToCategory(ctx, dbgen.AddFeedToCategoryParams{FeedID: f.ID, CategoryID: cat.ID})
	err = q.ClearFeedCategories(ctx, f.ID)
	if err != nil {
		t.Fatal(err)
	}
	cats, _ = q.GetFeedCategories(ctx, f.ID)
	if len(cats) != 0 {
		t.Fatalf("after clear: categories count = %d", len(cats))
	}
}

func TestChildCategories(t *testing.T) {
	_, q := setupTestDB(t)
	ctx := context.Background()

	u := createTestUser(t, q, "ext-cc", "cc@example.com")
	parent, _ := q.CreateCategory(ctx, dbgen.CreateCategoryParams{Name: "Parent", UserID: &u.ID})
	child, _ := q.CreateCategory(ctx, dbgen.CreateCategoryParams{Name: "Child", UserID: &u.ID})

	err := q.UpdateCategoryParent(ctx, dbgen.UpdateCategoryParentParams{
		ParentID: &parent.ID, SortOrder: new(int64(1)),
		ID: child.ID, UserID: &u.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	children, err := q.GetChildCategories(ctx, dbgen.GetChildCategoriesParams{
		ParentID: &parent.ID, UserID: &u.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(children) != 1 || children[0].ID != child.ID {
		t.Fatalf("GetChildCategories = %v", children)
	}
}

// ---------------------------------------------------------------------------
// Articles
// ---------------------------------------------------------------------------

func TestArticles_CreateAndGet(t *testing.T) {
	_, q := setupTestDB(t)
	ctx := context.Background()

	u := createTestUser(t, q, "ext-a", "art@example.com")
	f := createTestFeed(t, q, "Blog", "https://blog.example.com/rss", u.ID)

	a := createTestArticle(t, q, f.ID, "guid-1", "First Post")
	if a.ID == 0 {
		t.Fatal("expected non-zero article ID")
	}
	if a.Title != "First Post" {
		t.Fatalf("article title = %q", a.Title)
	}

	// GetArticle
	got, err := q.GetArticle(ctx, dbgen.GetArticleParams{ID: a.ID, UserID: &u.ID})
	if err != nil {
		t.Fatal(err)
	}
	if got.Guid != "guid-1" {
		t.Fatalf("GetArticle guid = %q", got.Guid)
	}

	// GetArticleWithFeed
	awf, err := q.GetArticleWithFeed(ctx, dbgen.GetArticleWithFeedParams{ID: a.ID, UserID: &u.ID})
	if err != nil {
		t.Fatal(err)
	}
	if awf.FeedName != "Blog" {
		t.Fatalf("GetArticleWithFeed feed_name = %q", awf.FeedName)
	}

	// CreateArticle upsert: same feed+guid should update title
	a2, err := q.CreateArticle(ctx, dbgen.CreateArticleParams{
		FeedID: f.ID, Guid: "guid-1", Title: "Updated First Post",
		Url: new("https://example.com/guid-1"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if a2.ID != a.ID {
		t.Fatalf("upsert returned different ID: %d vs %d", a2.ID, a.ID)
	}
	if a2.Title != "Updated First Post" {
		t.Fatalf("upsert title = %q", a2.Title)
	}
}

func TestArticles_ListAndFilter(t *testing.T) {
	_, q := setupTestDB(t)
	ctx := context.Background()

	u := createTestUser(t, q, "ext-al", "al@example.com")
	f := createTestFeed(t, q, "Feed", "https://f.com/rss", u.ID)

	for i := range 5 {
		createTestArticle(t, q, f.ID, fmt.Sprintf("g-%d", i), fmt.Sprintf("Article %d", i))
	}

	// ListArticles
	rows, err := q.ListArticles(ctx, dbgen.ListArticlesParams{UserID: &u.ID, Limit: 10, Offset: 0})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 5 {
		t.Fatalf("ListArticles count = %d, want 5", len(rows))
	}

	// ListArticles with pagination
	rows, err = q.ListArticles(ctx, dbgen.ListArticlesParams{UserID: &u.ID, Limit: 2, Offset: 0})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("ListArticles(limit=2) count = %d", len(rows))
	}

	// ListArticlesByFeed
	byFeed, err := q.ListArticlesByFeed(ctx, dbgen.ListArticlesByFeedParams{
		FeedID: f.ID, UserID: &u.ID, Limit: 10, Offset: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(byFeed) != 5 {
		t.Fatalf("ListArticlesByFeed count = %d", len(byFeed))
	}
}

func TestArticles_ReadUnread(t *testing.T) {
	_, q := setupTestDB(t)
	ctx := context.Background()

	u := createTestUser(t, q, "ext-ru", "ru@example.com")
	f := createTestFeed(t, q, "Feed", "https://f.com/rss", u.ID)
	a := createTestArticle(t, q, f.ID, "g-r", "Readable")

	// Initially unread (is_read defaults to 0)
	unread, err := q.GetUnreadCount(ctx, &u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if unread != 1 {
		t.Fatalf("unread count = %d, want 1", unread)
	}

	// ListUnreadArticles
	unreadRows, err := q.ListUnreadArticles(ctx, dbgen.ListUnreadArticlesParams{
		UserID: &u.ID, Limit: 10, Offset: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(unreadRows) != 1 {
		t.Fatalf("ListUnreadArticles count = %d", len(unreadRows))
	}

	// MarkArticleRead
	err = q.MarkArticleRead(ctx, dbgen.MarkArticleReadParams{ID: a.ID, UserID: &u.ID})
	if err != nil {
		t.Fatal(err)
	}
	unread, _ = q.GetUnreadCount(ctx, &u.ID)
	if unread != 0 {
		t.Fatalf("after MarkArticleRead: unread = %d", unread)
	}

	// MarkArticleUnread
	err = q.MarkArticleUnread(ctx, dbgen.MarkArticleUnreadParams{ID: a.ID, UserID: &u.ID})
	if err != nil {
		t.Fatal(err)
	}
	unread, _ = q.GetUnreadCount(ctx, &u.ID)
	if unread != 1 {
		t.Fatalf("after MarkArticleUnread: unread = %d", unread)
	}

	// MarkAllArticlesRead
	err = q.MarkAllArticlesRead(ctx, &u.ID)
	if err != nil {
		t.Fatal(err)
	}
	unread, _ = q.GetUnreadCount(ctx, &u.ID)
	if unread != 0 {
		t.Fatalf("after MarkAllArticlesRead: unread = %d", unread)
	}
}

func TestArticles_Star(t *testing.T) {
	_, q := setupTestDB(t)
	ctx := context.Background()

	u := createTestUser(t, q, "ext-star", "star@example.com")
	f := createTestFeed(t, q, "Feed", "https://f.com/rss", u.ID)
	a := createTestArticle(t, q, f.ID, "g-s", "Starrable")

	// Initially not starred
	starred, err := q.GetStarredCount(ctx, &u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if starred != 0 {
		t.Fatalf("initial starred = %d", starred)
	}

	// ToggleArticleStar
	err = q.ToggleArticleStar(ctx, dbgen.ToggleArticleStarParams{ID: a.ID, UserID: &u.ID})
	if err != nil {
		t.Fatal(err)
	}
	starred, _ = q.GetStarredCount(ctx, &u.ID)
	if starred != 1 {
		t.Fatalf("after toggle star: count = %d", starred)
	}

	// ListStarredArticles
	starredRows, err := q.ListStarredArticles(ctx, dbgen.ListStarredArticlesParams{
		UserID: &u.ID, Limit: 10, Offset: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(starredRows) != 1 {
		t.Fatalf("ListStarredArticles count = %d", len(starredRows))
	}

	// Toggle again → unstar
	err = q.ToggleArticleStar(ctx, dbgen.ToggleArticleStarParams{ID: a.ID, UserID: &u.ID})
	if err != nil {
		t.Fatal(err)
	}
	starred, _ = q.GetStarredCount(ctx, &u.ID)
	if starred != 0 {
		t.Fatalf("after untoggle star: count = %d", starred)
	}
}

func TestArticles_MarkFeedAndCategoryRead(t *testing.T) {
	_, q := setupTestDB(t)
	ctx := context.Background()

	u := createTestUser(t, q, "ext-mfr", "mfr@example.com")
	f := createTestFeed(t, q, "Feed", "https://f.com/rss", u.ID)
	cat, _ := q.CreateCategory(ctx, dbgen.CreateCategoryParams{Name: "Cat", UserID: &u.ID})
	_ = q.AddFeedToCategory(ctx, dbgen.AddFeedToCategoryParams{FeedID: f.ID, CategoryID: cat.ID})

	createTestArticle(t, q, f.ID, "g1", "A1")
	createTestArticle(t, q, f.ID, "g2", "A2")

	// MarkFeedRead
	err := q.MarkFeedRead(ctx, dbgen.MarkFeedReadParams{FeedID: f.ID, UserID: &u.ID})
	if err != nil {
		t.Fatal(err)
	}
	unread, _ := q.GetFeedUnreadCount(ctx, f.ID)
	if unread != 0 {
		t.Fatalf("after MarkFeedRead: unread = %d", unread)
	}

	// Add more articles and test MarkCategoryRead
	createTestArticle(t, q, f.ID, "g3", "A3")
	err = q.MarkCategoryRead(ctx, dbgen.MarkCategoryReadParams{CategoryID: cat.ID, UserID: &u.ID})
	if err != nil {
		t.Fatal(err)
	}
	catUnread, _ := q.GetCategoryUnreadCount(ctx, cat.ID)
	if catUnread != 0 {
		t.Fatalf("after MarkCategoryRead: unread = %d", catUnread)
	}
}

func TestArticles_Search(t *testing.T) {
	_, q := setupTestDB(t)
	ctx := context.Background()

	u := createTestUser(t, q, "ext-search", "s@example.com")
	f := createTestFeed(t, q, "Feed", "https://f.com/rss", u.ID)
	createTestArticle(t, q, f.ID, "g-1", "Golang Tutorial")
	createTestArticle(t, q, f.ID, "g-2", "Python Guide")

	term := "Golang"
	results, err := q.SearchArticles(ctx, dbgen.SearchArticlesParams{
		UserID: &u.ID, Column2: &term, Column3: &term,
		Limit: 10, Offset: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("SearchArticles count = %d, want 1", len(results))
	}
	if results[0].Title != "Golang Tutorial" {
		t.Fatalf("search result title = %q", results[0].Title)
	}
}

func TestArticles_ListByCategory(t *testing.T) {
	_, q := setupTestDB(t)
	ctx := context.Background()

	u := createTestUser(t, q, "ext-lbc", "lbc@example.com")
	f := createTestFeed(t, q, "Feed", "https://f.com/rss", u.ID)
	cat, _ := q.CreateCategory(ctx, dbgen.CreateCategoryParams{Name: "Cat", UserID: &u.ID})
	_ = q.AddFeedToCategory(ctx, dbgen.AddFeedToCategoryParams{FeedID: f.ID, CategoryID: cat.ID})
	createTestArticle(t, q, f.ID, "gc-1", "Cat Article")

	rows, err := q.ListArticlesByCategory(ctx, dbgen.ListArticlesByCategoryParams{
		CategoryID: cat.ID, UserID: &u.ID, Limit: 10, Offset: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("ListArticlesByCategory count = %d", len(rows))
	}

	// ListUnreadArticlesByCategory
	unreadRows, err := q.ListUnreadArticlesByCategory(ctx, dbgen.ListUnreadArticlesByCategoryParams{
		CategoryID: cat.ID, UserID: &u.ID, Limit: 10, Offset: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(unreadRows) != 1 {
		t.Fatalf("ListUnreadArticlesByCategory count = %d", len(unreadRows))
	}
}

// ---------------------------------------------------------------------------
// Queue
// ---------------------------------------------------------------------------

func TestQueue_AddRemoveList(t *testing.T) {
	_, q := setupTestDB(t)
	ctx := context.Background()

	u := createTestUser(t, q, "ext-q", "q@example.com")
	f := createTestFeed(t, q, "Feed", "https://f.com/rss", u.ID)
	a := createTestArticle(t, q, f.ID, "gq-1", "Queued Article")

	// AddToQueue
	err := q.AddToQueue(ctx, dbgen.AddToQueueParams{UserID: u.ID, ArticleID: a.ID})
	if err != nil {
		t.Fatal(err)
	}

	// GetQueueCount
	count, err := q.GetQueueCount(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("GetQueueCount = %d, want 1", count)
	}

	// IsArticleQueued
	queued, err := q.IsArticleQueued(ctx, dbgen.IsArticleQueuedParams{UserID: u.ID, ArticleID: a.ID})
	if err != nil {
		t.Fatal(err)
	}
	if queued != 1 {
		t.Fatalf("IsArticleQueued = %d, want 1", queued)
	}

	// ListQueueArticles
	rows, err := q.ListQueueArticles(ctx, dbgen.ListQueueArticlesParams{
		UserID: u.ID, Limit: 10, Offset: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("ListQueueArticles count = %d", len(rows))
	}
	if rows[0].Title != "Queued Article" {
		t.Fatalf("queued article title = %q", rows[0].Title)
	}

	// AddToQueue is idempotent (INSERT OR IGNORE)
	err = q.AddToQueue(ctx, dbgen.AddToQueueParams{UserID: u.ID, ArticleID: a.ID})
	if err != nil {
		t.Fatal(err)
	}
	count, _ = q.GetQueueCount(ctx, u.ID)
	if count != 1 {
		t.Fatalf("after duplicate add: count = %d", count)
	}

	// RemoveFromQueue
	err = q.RemoveFromQueue(ctx, dbgen.RemoveFromQueueParams{UserID: u.ID, ArticleID: a.ID})
	if err != nil {
		t.Fatal(err)
	}
	count, _ = q.GetQueueCount(ctx, u.ID)
	if count != 0 {
		t.Fatalf("after remove: count = %d", count)
	}
}

// ---------------------------------------------------------------------------
// Scrapers
// ---------------------------------------------------------------------------

func TestScrapers_CRUD(t *testing.T) {
	_, q := setupTestDB(t)
	ctx := context.Background()

	u := createTestUser(t, q, "ext-sc", "sc@example.com")

	desc := "A test scraper"
	sm, err := q.CreateScraperModule(ctx, dbgen.CreateScraperModuleParams{
		Name: "test-scraper", Description: &desc,
		Script: "console.log('hello')", ScriptType: "javascript",
		UserID: &u.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if sm.Name != "test-scraper" {
		t.Fatalf("scraper name = %q", sm.Name)
	}

	// GetScraperModule
	got, err := q.GetScraperModule(ctx, dbgen.GetScraperModuleParams{ID: sm.ID, UserID: &u.ID})
	if err != nil {
		t.Fatal(err)
	}
	if got.Script != "console.log('hello')" {
		t.Fatalf("script = %q", got.Script)
	}

	// GetScraperModuleByName
	got2, err := q.GetScraperModuleByName(ctx, dbgen.GetScraperModuleByNameParams{Name: "test-scraper", UserID: &u.ID})
	if err != nil {
		t.Fatal(err)
	}
	if got2.ID != sm.ID {
		t.Fatalf("GetScraperModuleByName id mismatch")
	}

	// GetScraperModuleInternal (no user_id filter)
	got3, err := q.GetScraperModuleInternal(ctx, "test-scraper")
	if err != nil {
		t.Fatal(err)
	}
	if got3.ID != sm.ID {
		t.Fatalf("GetScraperModuleInternal id mismatch")
	}

	// ListScraperModules
	list, err := q.ListScraperModules(ctx, &u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("ListScraperModules count = %d", len(list))
	}

	// UpdateScraperModule
	err = q.UpdateScraperModule(ctx, dbgen.UpdateScraperModuleParams{
		Name: "test-scraper-v2", Description: &desc,
		Script: "console.log('v2')", ScriptType: "javascript",
		ID: sm.ID, UserID: &u.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	// DisableScraperModule
	err = q.DisableScraperModule(ctx, dbgen.DisableScraperModuleParams{ID: sm.ID, UserID: &u.ID})
	if err != nil {
		t.Fatal(err)
	}
	got4, _ := q.GetScraperModule(ctx, dbgen.GetScraperModuleParams{ID: sm.ID, UserID: &u.ID})
	if got4.Enabled == nil || *got4.Enabled != 0 {
		t.Fatalf("after disable: enabled = %v", got4.Enabled)
	}

	// EnableScraperModule
	err = q.EnableScraperModule(ctx, dbgen.EnableScraperModuleParams{ID: sm.ID, UserID: &u.ID})
	if err != nil {
		t.Fatal(err)
	}
	got5, _ := q.GetScraperModule(ctx, dbgen.GetScraperModuleParams{ID: sm.ID, UserID: &u.ID})
	if got5.Enabled == nil || *got5.Enabled != 1 {
		t.Fatalf("after enable: enabled = %v", got5.Enabled)
	}

	// DeleteScraperModule
	err = q.DeleteScraperModule(ctx, dbgen.DeleteScraperModuleParams{ID: sm.ID, UserID: &u.ID})
	if err != nil {
		t.Fatal(err)
	}
	list, _ = q.ListScraperModules(ctx, &u.ID)
	if len(list) != 0 {
		t.Fatalf("after delete: count = %d", len(list))
	}
}

// ---------------------------------------------------------------------------
// Exclusions & Category Settings
// ---------------------------------------------------------------------------

func TestExclusions_CRUD(t *testing.T) {
	_, q := setupTestDB(t)
	ctx := context.Background()

	u := createTestUser(t, q, "ext-ex", "ex@example.com")
	cat, _ := q.CreateCategory(ctx, dbgen.CreateCategoryParams{Name: "Filtered", UserID: &u.ID})

	// CreateExclusion
	excl, err := q.CreateExclusion(ctx, dbgen.CreateExclusionParams{
		CategoryID: cat.ID, ExclusionType: "title",
		Pattern: "spam", IsRegex: new(int64(0)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if excl.Pattern != "spam" {
		t.Fatalf("exclusion pattern = %q", excl.Pattern)
	}

	// GetExclusion
	got, err := q.GetExclusion(ctx, dbgen.GetExclusionParams{ID: excl.ID, UserID: &u.ID})
	if err != nil {
		t.Fatal(err)
	}
	if got.ExclusionType != "title" {
		t.Fatalf("exclusion type = %q", got.ExclusionType)
	}

	// ListExclusionsByCategory
	list, err := q.ListExclusionsByCategory(ctx, dbgen.ListExclusionsByCategoryParams{
		CategoryID: cat.ID, UserID: &u.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("ListExclusionsByCategory count = %d", len(list))
	}

	// ListAllExclusions
	all, err := q.ListAllExclusions(ctx, &u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("ListAllExclusions count = %d", len(all))
	}
	if all[0].CategoryName != "Filtered" {
		t.Fatalf("exclusion category_name = %q", all[0].CategoryName)
	}

	// UpdateExclusion
	err = q.UpdateExclusion(ctx, dbgen.UpdateExclusionParams{
		Pattern: "ads.*", IsRegex: new(int64(1)),
		ID: excl.ID, UserID: &u.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	got2, _ := q.GetExclusion(ctx, dbgen.GetExclusionParams{ID: excl.ID, UserID: &u.ID})
	if got2.Pattern != "ads.*" {
		t.Fatalf("after update: pattern = %q", got2.Pattern)
	}

	// DeleteExclusion
	err = q.DeleteExclusion(ctx, dbgen.DeleteExclusionParams{ID: excl.ID, UserID: &u.ID})
	if err != nil {
		t.Fatal(err)
	}
	list, _ = q.ListExclusionsByCategory(ctx, dbgen.ListExclusionsByCategoryParams{
		CategoryID: cat.ID, UserID: &u.ID,
	})
	if len(list) != 0 {
		t.Fatalf("after delete: count = %d", len(list))
	}
}

func TestExclusions_DeleteByCategory(t *testing.T) {
	_, q := setupTestDB(t)
	ctx := context.Background()

	u := createTestUser(t, q, "ext-exd", "exd@example.com")
	cat, _ := q.CreateCategory(ctx, dbgen.CreateCategoryParams{Name: "Bulk", UserID: &u.ID})

	for i := range 3 {
		_, err := q.CreateExclusion(ctx, dbgen.CreateExclusionParams{
			CategoryID: cat.ID, ExclusionType: "title",
			Pattern: fmt.Sprintf("pattern-%d", i), IsRegex: new(int64(0)),
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	err := q.DeleteExclusionsByCategory(ctx, cat.ID)
	if err != nil {
		t.Fatal(err)
	}
	list, _ := q.ListExclusionsByCategory(ctx, dbgen.ListExclusionsByCategoryParams{
		CategoryID: cat.ID, UserID: &u.ID,
	})
	if len(list) != 0 {
		t.Fatalf("after bulk delete: count = %d", len(list))
	}
}

func TestCategorySettings_CRUD(t *testing.T) {
	_, q := setupTestDB(t)
	ctx := context.Background()

	u := createTestUser(t, q, "ext-cs", "cs@example.com")
	cat, _ := q.CreateCategory(ctx, dbgen.CreateCategoryParams{Name: "Configured", UserID: &u.ID})

	// SetCategorySetting
	val := "dark"
	err := q.SetCategorySetting(ctx, dbgen.SetCategorySettingParams{
		CategoryID: cat.ID, SettingKey: "theme", SettingValue: &val,
	})
	if err != nil {
		t.Fatal(err)
	}

	// GetCategorySetting
	got, err := q.GetCategorySetting(ctx, dbgen.GetCategorySettingParams{
		CategoryID: cat.ID, SettingKey: "theme", UserID: &u.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.SettingValue == nil || *got.SettingValue != "dark" {
		t.Fatalf("setting value = %v", got.SettingValue)
	}

	// ListCategorySettings
	list, err := q.ListCategorySettings(ctx, dbgen.ListCategorySettingsParams{
		CategoryID: cat.ID, UserID: &u.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("ListCategorySettings count = %d", len(list))
	}

	// SetCategorySetting upsert
	val2 := "light"
	err = q.SetCategorySetting(ctx, dbgen.SetCategorySettingParams{
		CategoryID: cat.ID, SettingKey: "theme", SettingValue: &val2,
	})
	if err != nil {
		t.Fatal(err)
	}
	got2, _ := q.GetCategorySetting(ctx, dbgen.GetCategorySettingParams{
		CategoryID: cat.ID, SettingKey: "theme", UserID: &u.ID,
	})
	if got2.SettingValue == nil || *got2.SettingValue != "light" {
		t.Fatalf("after upsert: value = %v", got2.SettingValue)
	}

	// DeleteCategorySetting
	err = q.DeleteCategorySetting(ctx, dbgen.DeleteCategorySettingParams{
		CategoryID: cat.ID, SettingKey: "theme", UserID: &u.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	list, _ = q.ListCategorySettings(ctx, dbgen.ListCategorySettingsParams{
		CategoryID: cat.ID, UserID: &u.ID,
	})
	if len(list) != 0 {
		t.Fatalf("after delete: count = %d", len(list))
	}
}

// ---------------------------------------------------------------------------
// User Settings
// ---------------------------------------------------------------------------

func TestUserSettings_CRUD(t *testing.T) {
	_, q := setupTestDB(t)
	ctx := context.Background()

	u := createTestUser(t, q, "ext-us", "us@example.com")

	// SetUserSetting
	err := q.SetUserSetting(ctx, dbgen.SetUserSettingParams{
		UserID: u.ID, Key: "theme", Value: "dark",
	})
	if err != nil {
		t.Fatal(err)
	}

	// GetUserSetting
	val, err := q.GetUserSetting(ctx, dbgen.GetUserSettingParams{
		UserID: u.ID, Key: "theme",
	})
	if err != nil {
		t.Fatal(err)
	}
	if val != "dark" {
		t.Fatalf("GetUserSetting = %q", val)
	}

	// GetUserSettings (list all)
	rows, err := q.GetUserSettings(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("GetUserSettings count = %d", len(rows))
	}

	// Upsert
	err = q.SetUserSetting(ctx, dbgen.SetUserSettingParams{
		UserID: u.ID, Key: "theme", Value: "light",
	})
	if err != nil {
		t.Fatal(err)
	}
	val, _ = q.GetUserSetting(ctx, dbgen.GetUserSettingParams{UserID: u.ID, Key: "theme"})
	if val != "light" {
		t.Fatalf("after upsert: value = %q", val)
	}

	// DeleteUserSetting
	err = q.DeleteUserSetting(ctx, dbgen.DeleteUserSettingParams{
		UserID: u.ID, Key: "theme",
	})
	if err != nil {
		t.Fatal(err)
	}
	rows, _ = q.GetUserSettings(ctx, u.ID)
	if len(rows) != 0 {
		t.Fatalf("after delete: count = %d", len(rows))
	}
}

// ---------------------------------------------------------------------------
// Transactions
// ---------------------------------------------------------------------------

func TestWithTx(t *testing.T) {
	sqlDB, q := setupTestDB(t)
	ctx := context.Background()

	u := createTestUser(t, q, "ext-tx", "tx@example.com")

	// Start a transaction, create a feed, then roll back.
	tx, err := sqlDB.Begin()
	if err != nil {
		t.Fatal(err)
	}
	qtx := q.WithTx(tx)
	_, err = qtx.CreateFeed(ctx, dbgen.CreateFeedParams{
		Name: "Transient Feed", Url: "https://t.com/rss",
		FeedType: "rss", UserID: &u.ID,
	})
	if err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	_ = tx.Rollback()

	// Feed should not exist after rollback.
	feeds, err := q.ListFeeds(ctx, &u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(feeds) != 0 {
		t.Fatalf("after rollback: ListFeeds count = %d, want 0", len(feeds))
	}
}

// ---------------------------------------------------------------------------
// ListFeedsToFetch
// ---------------------------------------------------------------------------

func TestListFeedsToFetch(t *testing.T) {
	_, q := setupTestDB(t)
	ctx := context.Background()

	u := createTestUser(t, q, "ext-ltf", "ltf@example.com")
	_ = createTestFeed(t, q, "Never Fetched", "https://nf.com/rss", u.ID)

	// A feed with no last_fetched_at should appear.
	feeds, err := q.ListFeedsToFetch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(feeds) < 1 {
		t.Fatalf("ListFeedsToFetch count = %d, want >= 1", len(feeds))
	}
}

// ---------------------------------------------------------------------------
// Cursor-based pagination queries
// ---------------------------------------------------------------------------

func TestArticles_CursorPagination(t *testing.T) {
	_, q := setupTestDB(t)
	ctx := context.Background()

	u := createTestUser(t, q, "ext-cursor", "cursor@example.com")
	f := createTestFeed(t, q, "CursorFeed", "https://cursor.com/rss", u.ID)

	// Create articles with staggered published_at times
	now := time.Now().UTC().Truncate(time.Second)
	for i := range 5 {
		pub := now.Add(time.Duration(-i) * time.Hour)
		_, err := q.CreateArticle(ctx, dbgen.CreateArticleParams{
			FeedID:      f.ID,
			Guid:        fmt.Sprintf("cursor-guid-%d", i),
			Title:       fmt.Sprintf("Article %d", i),
			PublishedAt: &pub,
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	// ListArticlesCursor: get articles before the 3rd oldest (index 2 = now-2h)
	before := now.Add(-1 * time.Hour).Add(-time.Second) // between index 1 and 2
	rows, err := q.ListArticlesCursor(ctx, dbgen.ListArticlesCursorParams{
		UserID: &u.ID, BeforeTime: &before, BeforeTimeEq: &before, BeforeID: 999999, Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	// Should get articles 2, 3, 4 (published at now-2h, now-3h, now-4h)
	if len(rows) != 3 {
		t.Fatalf("ListArticlesCursor: got %d rows, want 3", len(rows))
	}

	// ListUnreadArticlesCursor: same thing but only unread
	// Mark article 3 as read
	q.MarkArticleRead(ctx, dbgen.MarkArticleReadParams{ID: rows[1].ID, UserID: &u.ID})

	unreadRows, err := q.ListUnreadArticlesCursor(ctx, dbgen.ListUnreadArticlesCursorParams{
		UserID: &u.ID, BeforeTime: &before, BeforeTimeEq: &before, BeforeID: 999999, Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(unreadRows) != 2 {
		t.Fatalf("ListUnreadArticlesCursor: got %d rows, want 2", len(unreadRows))
	}

	// ListArticlesByFeedCursor
	feedRows, err := q.ListArticlesByFeedCursor(ctx, dbgen.ListArticlesByFeedCursorParams{
		FeedID: f.ID, UserID: &u.ID, BeforeTime: &before, BeforeTimeEq: &before, BeforeID: 999999, Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(feedRows) != 3 {
		t.Fatalf("ListArticlesByFeedCursor: got %d rows, want 3", len(feedRows))
	}
}

func TestArticles_CursorPaginationByCategory(t *testing.T) {
	_, q := setupTestDB(t)
	ctx := context.Background()

	u := createTestUser(t, q, "ext-catcur", "catcur@example.com")
	f := createTestFeed(t, q, "CatCursorFeed", "https://catcursor.com/rss", u.ID)
	cat, err := q.CreateCategory(ctx, dbgen.CreateCategoryParams{Name: "CursorCat", UserID: &u.ID})
	if err != nil {
		t.Fatal(err)
	}
	q.AddFeedToCategory(ctx, dbgen.AddFeedToCategoryParams{FeedID: f.ID, CategoryID: cat.ID})

	now := time.Now().UTC().Truncate(time.Second)
	for i := range 4 {
		pub := now.Add(time.Duration(-i) * time.Hour)
		_, err := q.CreateArticle(ctx, dbgen.CreateArticleParams{
			FeedID:      f.ID,
			Guid:        fmt.Sprintf("catcur-guid-%d", i),
			Title:       fmt.Sprintf("CatArticle %d", i),
			PublishedAt: &pub,
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	before := now.Add(-1 * time.Hour).Add(-time.Second)

	// ListArticlesByCategoryCursor
	catRows, err := q.ListArticlesByCategoryCursor(ctx, dbgen.ListArticlesByCategoryCursorParams{
		CategoryID: cat.ID, UserID: &u.ID, BeforeTime: &before, BeforeTimeEq: &before, BeforeID: 999999, Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(catRows) != 2 {
		t.Fatalf("ListArticlesByCategoryCursor: got %d rows, want 2", len(catRows))
	}

	// ListUnreadArticlesByCategoryCursor
	unreadCatRows, err := q.ListUnreadArticlesByCategoryCursor(ctx, dbgen.ListUnreadArticlesByCategoryCursorParams{
		CategoryID: cat.ID, UserID: &u.ID, BeforeTime: &before, BeforeTimeEq: &before, BeforeID: 999999, Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(unreadCatRows) != 2 {
		t.Fatalf("ListUnreadArticlesByCategoryCursor: got %d rows, want 2", len(unreadCatRows))
	}

	// Mark one as read and verify unread cursor excludes it
	q.MarkArticleRead(ctx, dbgen.MarkArticleReadParams{ID: catRows[0].ID, UserID: &u.ID})
	unreadCatRows, err = q.ListUnreadArticlesByCategoryCursor(ctx, dbgen.ListUnreadArticlesByCategoryCursorParams{
		CategoryID: cat.ID, UserID: &u.ID, BeforeTime: &before, BeforeTimeEq: &before, BeforeID: 999999, Limit: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(unreadCatRows) != 1 {
		t.Fatalf("after mark-read: got %d rows, want 1", len(unreadCatRows))
	}
}

// ---------------------------------------------------------------------------
// NNTP Credentials
// ---------------------------------------------------------------------------

func TestNNTPCredentials_CRUD(t *testing.T) {
	_, q := setupTestDB(t)
	ctx := context.Background()

	u := createTestUser(t, q, "ext-nntp", "nntp@example.com")

	// UpsertNNTPCredentials — insert
	cred, err := q.UpsertNNTPCredentials(ctx, dbgen.UpsertNNTPCredentialsParams{
		UserID:      u.ID,
		Username:    "testuser",
		PasswordEnc: "hexencryptedblob",
		KeyVersion:  "v1",
	})
	if err != nil {
		t.Fatalf("UpsertNNTPCredentials (insert): %v", err)
	}
	if cred.UserID != u.ID {
		t.Fatalf("user_id = %d, want %d", cred.UserID, u.ID)
	}
	if cred.Username != "testuser" {
		t.Fatalf("username = %q", cred.Username)
	}
	if cred.PasswordEnc != "hexencryptedblob" {
		t.Fatalf("password_enc = %q", cred.PasswordEnc)
	}
	if cred.KeyVersion != "v1" {
		t.Fatalf("key_version = %q, want %q", cred.KeyVersion, "v1")
	}

	// GetNNTPCredentials
	got, err := q.GetNNTPCredentials(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetNNTPCredentials: %v", err)
	}
	if got.Username != "testuser" {
		t.Fatalf("GetNNTPCredentials username = %q", got.Username)
	}
	if got.KeyVersion != "v1" {
		t.Fatalf("GetNNTPCredentials key_version = %q, want %q", got.KeyVersion, "v1")
	}

	// UpsertNNTPCredentials — update (same user_id)
	updated, err := q.UpsertNNTPCredentials(ctx, dbgen.UpsertNNTPCredentialsParams{
		UserID:      u.ID,
		Username:    "newuser",
		PasswordEnc: "newhexblob",
		KeyVersion:  "v1",
	})
	if err != nil {
		t.Fatalf("UpsertNNTPCredentials (update): %v", err)
	}
	if updated.Username != "newuser" {
		t.Fatalf("after update: username = %q", updated.Username)
	}
	if updated.PasswordEnc != "newhexblob" {
		t.Fatalf("after update: password_enc = %q", updated.PasswordEnc)
	}
	if updated.KeyVersion != "v1" {
		t.Fatalf("after update: key_version = %q", updated.KeyVersion)
	}

	// Confirm only one row exists
	got2, err := q.GetNNTPCredentials(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetNNTPCredentials after update: %v", err)
	}
	if got2.Username != "newuser" {
		t.Fatalf("after upsert update: username = %q", got2.Username)
	}

	// DeleteNNTPCredentials
	err = q.DeleteNNTPCredentials(ctx, u.ID)
	if err != nil {
		t.Fatalf("DeleteNNTPCredentials: %v", err)
	}

	// GetNNTPCredentials after delete should return sql.ErrNoRows
	_, err = q.GetNNTPCredentials(ctx, u.ID)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetNNTPCredentials after delete: got %v, want sql.ErrNoRows", err)
	}
}

func TestNNTPCredentials_UserDeleteCascade(t *testing.T) {
	sqlDB, q := setupTestDB(t)
	ctx := context.Background()

	u := createTestUser(t, q, "ext-nntp-cascade", "nntp-cascade@example.com")

	// Insert credentials
	_, err := q.UpsertNNTPCredentials(ctx, dbgen.UpsertNNTPCredentialsParams{
		UserID:      u.ID,
		Username:    "cascadeuser",
		PasswordEnc: "encblob",
		KeyVersion:  "v1",
	})
	if err != nil {
		t.Fatalf("UpsertNNTPCredentials: %v", err)
	}

	// Credentials exist
	_, err = q.GetNNTPCredentials(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetNNTPCredentials before cascade: %v", err)
	}

	// Delete the user directly — credentials should cascade-delete via FK.
	_, err = sqlDB.ExecContext(ctx, "DELETE FROM users WHERE id = ?", u.ID)
	if err != nil {
		t.Fatalf("DELETE FROM users: %v", err)
	}

	// Credentials should be gone
	_, err = q.GetNNTPCredentials(ctx, u.ID)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetNNTPCredentials after user delete: got %v, want sql.ErrNoRows", err)
	}
}

// ---------------------------------------------------------------------------
// usenet_feed_state
// ---------------------------------------------------------------------------

func TestUsenetFeedState_CreateAndGet(t *testing.T) {
	_, q := setupTestDB(t)
	ctx := context.Background()

	u := createTestUser(t, q, "ext-usenet-state", "usenet-state@example.com")
	f := createTestFeed(t, q, "comp.lang.go", "nntp://news.eternal-september.org/comp.lang.go", u.ID)

	state, err := q.CreateUsenetFeedState(ctx, dbgen.CreateUsenetFeedStateParams{
		FeedID:    f.ID,
		Provider:  "eternal-september",
		GroupName: "comp.lang.go",
	})
	if err != nil {
		t.Fatalf("CreateUsenetFeedState: %v", err)
	}
	if state.FeedID != f.ID {
		t.Errorf("FeedID = %d, want %d", state.FeedID, f.ID)
	}
	if state.GroupName != "comp.lang.go" {
		t.Errorf("GroupName = %q, want %q", state.GroupName, "comp.lang.go")
	}
	if state.Provider != "eternal-september" {
		t.Errorf("Provider = %q, want %q", state.Provider, "eternal-september")
	}
	if state.HighWaterArticleNumber != 0 {
		t.Errorf("HighWaterArticleNumber = %d, want 0", state.HighWaterArticleNumber)
	}

	// GetUsenetFeedState
	got, err := q.GetUsenetFeedState(ctx, dbgen.GetUsenetFeedStateParams{
		FeedID: f.ID,
		UserID: &u.ID,
	})
	if err != nil {
		t.Fatalf("GetUsenetFeedState: %v", err)
	}
	if got.FeedID != f.ID {
		t.Errorf("GetUsenetFeedState FeedID = %d, want %d", got.FeedID, f.ID)
	}
}

func TestUsenetFeedState_ListFeeds(t *testing.T) {
	_, q := setupTestDB(t)
	ctx := context.Background()

	u := createTestUser(t, q, "ext-usenet-list", "usenet-list@example.com")
	f1 := createTestFeed(t, q, "comp.lang.go", "nntp://news.eternal-september.org/comp.lang.go", u.ID)
	f2 := createTestFeed(t, q, "rec.arts.sf", "nntp://news.eternal-september.org/rec.arts.sf", u.ID)

	_, err := q.CreateUsenetFeedState(ctx, dbgen.CreateUsenetFeedStateParams{
		FeedID: f1.ID, Provider: "eternal-september", GroupName: "comp.lang.go",
	})
	if err != nil {
		t.Fatalf("CreateUsenetFeedState f1: %v", err)
	}
	_, err = q.CreateUsenetFeedState(ctx, dbgen.CreateUsenetFeedStateParams{
		FeedID: f2.ID, Provider: "eternal-september", GroupName: "rec.arts.sf",
	})
	if err != nil {
		t.Fatalf("CreateUsenetFeedState f2: %v", err)
	}

	rows, err := q.ListUsenetFeeds(ctx, &u.ID)
	if err != nil {
		t.Fatalf("ListUsenetFeeds: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("ListUsenetFeeds: got %d rows, want 2", len(rows))
	}
}

func TestUsenetFeedState_UpdateHighWater(t *testing.T) {
	_, q := setupTestDB(t)
	ctx := context.Background()

	u := createTestUser(t, q, "ext-usenet-hw", "usenet-hw@example.com")
	f := createTestFeed(t, q, "comp.lang.go", "nntp://news.eternal-september.org/comp.lang.go", u.ID)

	_, err := q.CreateUsenetFeedState(ctx, dbgen.CreateUsenetFeedStateParams{
		FeedID: f.ID, Provider: "eternal-september", GroupName: "comp.lang.go",
	})
	if err != nil {
		t.Fatalf("CreateUsenetFeedState: %v", err)
	}

	err = q.UpdateUsenetHighWater(ctx, dbgen.UpdateUsenetHighWaterParams{
		FeedID:                 f.ID,
		HighWaterArticleNumber: 12345,
	})
	if err != nil {
		t.Fatalf("UpdateUsenetHighWater: %v", err)
	}

	state, err := q.GetUsenetFeedState(ctx, dbgen.GetUsenetFeedStateParams{
		FeedID: f.ID,
		UserID: &u.ID,
	})
	if err != nil {
		t.Fatalf("GetUsenetFeedState after update: %v", err)
	}
	if state.HighWaterArticleNumber != 12345 {
		t.Errorf("HighWaterArticleNumber = %d, want 12345", state.HighWaterArticleNumber)
	}
}

func TestUsenetFeedState_DeleteCascade(t *testing.T) {
	sqlDB, q := setupTestDB(t)
	ctx := context.Background()

	u := createTestUser(t, q, "ext-usenet-del", "usenet-del@example.com")
	f := createTestFeed(t, q, "alt.test", "nntp://news.eternal-september.org/alt.test", u.ID)

	_, err := q.CreateUsenetFeedState(ctx, dbgen.CreateUsenetFeedStateParams{
		FeedID: f.ID, Provider: "eternal-september", GroupName: "alt.test",
	})
	if err != nil {
		t.Fatalf("CreateUsenetFeedState: %v", err)
	}

	// Delete feed — usenet_feed_state should cascade-delete.
	_, err = sqlDB.ExecContext(ctx, "DELETE FROM feeds WHERE id = ?", f.ID)
	if err != nil {
		t.Fatalf("DELETE FROM feeds: %v", err)
	}

	_, err = q.GetUsenetFeedState(ctx, dbgen.GetUsenetFeedStateParams{
		FeedID: f.ID,
		UserID: &u.ID,
	})
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetUsenetFeedState after feed delete: got %v, want sql.ErrNoRows", err)
	}
}

func TestUsenetFeedState_DuplicatePrevention(t *testing.T) {
	_, q := setupTestDB(t)
	ctx := context.Background()

	u := createTestUser(t, q, "ext-usenet-dup", "usenet-dup@example.com")
	f := createTestFeed(t, q, "alt.test", "nntp://news.eternal-september.org/alt.test", u.ID)

	_, err := q.CreateUsenetFeedState(ctx, dbgen.CreateUsenetFeedStateParams{
		FeedID: f.ID, Provider: "eternal-september", GroupName: "alt.test",
	})
	if err != nil {
		t.Fatalf("CreateUsenetFeedState first: %v", err)
	}

	// Attempting to create again for the same feed_id should fail.
	_, err = q.CreateUsenetFeedState(ctx, dbgen.CreateUsenetFeedStateParams{
		FeedID: f.ID, Provider: "eternal-september", GroupName: "alt.test",
	})
	if err == nil {
		t.Error("expected error on duplicate CreateUsenetFeedState for same feed_id, got nil")
	}
}

func TestUsenetFeedState_GetByGroup(t *testing.T) {
	_, q := setupTestDB(t)
	ctx := context.Background()

	u := createTestUser(t, q, "ext-usenet-grp", "usenet-grp@example.com")
	f := createTestFeed(t, q, "alt.test", "nntp://news.eternal-september.org/alt.test", u.ID)

	_, err := q.CreateUsenetFeedState(ctx, dbgen.CreateUsenetFeedStateParams{
		FeedID: f.ID, Provider: "eternal-september", GroupName: "alt.test",
	})
	if err != nil {
		t.Fatalf("CreateUsenetFeedState: %v", err)
	}

	state, err := q.GetUsenetFeedStateByGroup(ctx, dbgen.GetUsenetFeedStateByGroupParams{
		Provider:  "eternal-september",
		GroupName: "alt.test",
		UserID:    &u.ID,
	})
	if err != nil {
		t.Fatalf("GetUsenetFeedStateByGroup: %v", err)
	}
	if state.FeedID != f.ID {
		t.Errorf("GetUsenetFeedStateByGroup FeedID = %d, want %d", state.FeedID, f.ID)
	}

	// Look up a non-existent group — should return sql.ErrNoRows.
	_, err = q.GetUsenetFeedStateByGroup(ctx, dbgen.GetUsenetFeedStateByGroupParams{
		Provider:  "eternal-september",
		GroupName: "no.such.group",
		UserID:    &u.ID,
	})
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("GetUsenetFeedStateByGroup unknown group: got %v, want sql.ErrNoRows", err)
	}
}

// ---------------------------------------------------------------------------
// UsenetArticleMeta
// ---------------------------------------------------------------------------

func TestUsenetArticleMeta_InsertAndGet(t *testing.T) {
	_, q := setupTestDB(t)
	ctx := context.Background()

	u := createTestUser(t, q, "ext-uam-1", "uam1@example.com")
	f := createTestFeed(t, q, "comp.lang.go", "nntp://news.eternal-september.org/comp.lang.go", u.ID)
	a := createTestArticle(t, q, f.ID, "<msg1@host>", "Test Article")

	// InsertUsenetArticleMeta
	meta, err := q.InsertUsenetArticleMeta(ctx, dbgen.InsertUsenetArticleMetaParams{
		ArticleID:        a.ID,
		FeedID:           f.ID,
		MessageID:        "<msg1@host>",
		ReferencesHeader: nil,
		ParentMessageID:  nil,
		RootMessageID:    "<msg1@host>",
		GroupName:        "comp.lang.go",
		ArticleNumber:    42,
	})
	if err != nil {
		t.Fatalf("InsertUsenetArticleMeta: %v", err)
	}
	if meta.ArticleID != a.ID {
		t.Errorf("ArticleID = %d, want %d", meta.ArticleID, a.ID)
	}
	if meta.MessageID != "<msg1@host>" {
		t.Errorf("MessageID = %q, want %q", meta.MessageID, "<msg1@host>")
	}

	// GetUsenetArticleMeta by article_id
	got, err := q.GetUsenetArticleMeta(ctx, a.ID)
	if err != nil {
		t.Fatalf("GetUsenetArticleMeta: %v", err)
	}
	if got.ArticleID != a.ID {
		t.Errorf("GetUsenetArticleMeta ArticleID = %d, want %d", got.ArticleID, a.ID)
	}
}

func TestUsenetArticleMeta_GetByMessageID(t *testing.T) {
	_, q := setupTestDB(t)
	ctx := context.Background()

	u := createTestUser(t, q, "ext-uam-2", "uam2@example.com")
	f := createTestFeed(t, q, "comp.lang.go", "nntp://news.eternal-september.org/comp.lang.go", u.ID)
	a := createTestArticle(t, q, f.ID, "<msg2@host>", "Test Article 2")

	_, err := q.InsertUsenetArticleMeta(ctx, dbgen.InsertUsenetArticleMetaParams{
		ArticleID:     a.ID,
		FeedID:        f.ID,
		MessageID:     "<msg2@host>",
		RootMessageID: "<msg2@host>",
		GroupName:     "comp.lang.go",
		ArticleNumber: 100,
	})
	if err != nil {
		t.Fatalf("InsertUsenetArticleMeta: %v", err)
	}

	got, err := q.GetUsenetArticleMetaByMessageID(ctx, dbgen.GetUsenetArticleMetaByMessageIDParams{
		FeedID:    f.ID,
		MessageID: "<msg2@host>",
	})
	if err != nil {
		t.Fatalf("GetUsenetArticleMetaByMessageID: %v", err)
	}
	if got.ArticleID != a.ID {
		t.Errorf("ArticleID = %d, want %d", got.ArticleID, a.ID)
	}

	// Non-existent message-id
	_, err = q.GetUsenetArticleMetaByMessageID(ctx, dbgen.GetUsenetArticleMetaByMessageIDParams{
		FeedID:    f.ID,
		MessageID: "<notexist@host>",
	})
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestUsenetArticleMeta_GetByArticleNumber(t *testing.T) {
	_, q := setupTestDB(t)
	ctx := context.Background()

	u := createTestUser(t, q, "ext-uam-3", "uam3@example.com")
	f := createTestFeed(t, q, "comp.lang.go", "nntp://news.eternal-september.org/comp.lang.go", u.ID)
	a := createTestArticle(t, q, f.ID, "<msg3@host>", "Test Article 3")

	_, err := q.InsertUsenetArticleMeta(ctx, dbgen.InsertUsenetArticleMetaParams{
		ArticleID:     a.ID,
		FeedID:        f.ID,
		MessageID:     "<msg3@host>",
		RootMessageID: "<msg3@host>",
		GroupName:     "comp.lang.go",
		ArticleNumber: 200,
	})
	if err != nil {
		t.Fatalf("InsertUsenetArticleMeta: %v", err)
	}

	got, err := q.GetUsenetArticleMetaByArticleNumber(ctx, dbgen.GetUsenetArticleMetaByArticleNumberParams{
		FeedID:        f.ID,
		ArticleNumber: 200,
	})
	if err != nil {
		t.Fatalf("GetUsenetArticleMetaByArticleNumber: %v", err)
	}
	if got.ArticleID != a.ID {
		t.Errorf("ArticleID = %d, want %d", got.ArticleID, a.ID)
	}
}

func TestUsenetArticleMeta_ListByThread(t *testing.T) {
	_, q := setupTestDB(t)
	ctx := context.Background()

	u := createTestUser(t, q, "ext-uam-4", "uam4@example.com")
	f := createTestFeed(t, q, "comp.lang.go", "nntp://news.eternal-september.org/comp.lang.go", u.ID)

	// Create three articles in the same thread.
	a1 := createTestArticle(t, q, f.ID, "<root@host>", "Root Post")
	a2 := createTestArticle(t, q, f.ID, "<reply1@host>", "Reply 1")
	a3 := createTestArticle(t, q, f.ID, "<reply2@host>", "Reply 2")

	for _, row := range []struct {
		artID   int64
		msgID   string
		parent  *string
		artNum  int64
		refsHdr *string
	}{
		{a1.ID, "<root@host>", nil, 300, nil},
		{a2.ID, "<reply1@host>", new("<root@host>"), 301, new("<root@host>")},
		{a3.ID, "<reply2@host>", new("<reply1@host>"), 302, new("<root@host> <reply1@host>")},
	} {
		_, err := q.InsertUsenetArticleMeta(ctx, dbgen.InsertUsenetArticleMetaParams{
			ArticleID:        row.artID,
			FeedID:           f.ID,
			MessageID:        row.msgID,
			ReferencesHeader: row.refsHdr,
			ParentMessageID:  row.parent,
			RootMessageID:    "<root@host>",
			GroupName:        "comp.lang.go",
			ArticleNumber:    row.artNum,
		})
		if err != nil {
			t.Fatalf("InsertUsenetArticleMeta %s: %v", row.msgID, err)
		}
	}

	rows, err := q.ListUsenetArticleMetaByThread(ctx, "<root@host>")
	if err != nil {
		t.Fatalf("ListUsenetArticleMetaByThread: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want 3", len(rows))
	}
	if rows[0].MessageID != "<root@host>" {
		t.Errorf("rows[0].MessageID = %q, want <root@host>", rows[0].MessageID)
	}
}

func TestUsenetArticleMeta_DuplicateMessageID(t *testing.T) {
	_, q := setupTestDB(t)
	ctx := context.Background()

	u := createTestUser(t, q, "ext-uam-5", "uam5@example.com")
	f := createTestFeed(t, q, "comp.lang.go", "nntp://news.eternal-september.org/comp.lang.go", u.ID)
	a1 := createTestArticle(t, q, f.ID, "<dup@host>", "Dup")
	a2 := createTestArticle(t, q, f.ID, "<dup2@host>", "Dup2")

	_, err := q.InsertUsenetArticleMeta(ctx, dbgen.InsertUsenetArticleMetaParams{
		ArticleID:     a1.ID,
		FeedID:        f.ID,
		MessageID:     "<dup@host>",
		RootMessageID: "<dup@host>",
		GroupName:     "comp.lang.go",
		ArticleNumber: 400,
	})
	if err != nil {
		t.Fatalf("InsertUsenetArticleMeta first: %v", err)
	}

	// Same feed_id + message_id — should fail UNIQUE constraint.
	_, err = q.InsertUsenetArticleMeta(ctx, dbgen.InsertUsenetArticleMetaParams{
		ArticleID:     a2.ID,
		FeedID:        f.ID,
		MessageID:     "<dup@host>",
		RootMessageID: "<dup@host>",
		GroupName:     "comp.lang.go",
		ArticleNumber: 401,
	})
	if err == nil {
		t.Error("expected UNIQUE constraint error for duplicate message_id, got nil")
	}
}

func TestUsenetArticleMeta_DuplicateArticleNumber(t *testing.T) {
	_, q := setupTestDB(t)
	ctx := context.Background()

	u := createTestUser(t, q, "ext-uam-6", "uam6@example.com")
	f := createTestFeed(t, q, "comp.lang.go", "nntp://news.eternal-september.org/comp.lang.go", u.ID)
	a1 := createTestArticle(t, q, f.ID, "<numdup1@host>", "NumDup1")
	a2 := createTestArticle(t, q, f.ID, "<numdup2@host>", "NumDup2")

	_, err := q.InsertUsenetArticleMeta(ctx, dbgen.InsertUsenetArticleMetaParams{
		ArticleID:     a1.ID,
		FeedID:        f.ID,
		MessageID:     "<numdup1@host>",
		RootMessageID: "<numdup1@host>",
		GroupName:     "comp.lang.go",
		ArticleNumber: 500,
	})
	if err != nil {
		t.Fatalf("InsertUsenetArticleMeta first: %v", err)
	}

	// Same feed_id + article_number — should fail UNIQUE constraint.
	_, err = q.InsertUsenetArticleMeta(ctx, dbgen.InsertUsenetArticleMetaParams{
		ArticleID:     a2.ID,
		FeedID:        f.ID,
		MessageID:     "<numdup2@host>",
		RootMessageID: "<numdup2@host>",
		GroupName:     "comp.lang.go",
		ArticleNumber: 500,
	})
	if err == nil {
		t.Error("expected UNIQUE constraint error for duplicate article_number, got nil")
	}
}

func TestUsenetArticleMeta_CascadeDeleteOnArticle(t *testing.T) {
	sqlDB, q := setupTestDB(t)
	ctx := context.Background()

	u := createTestUser(t, q, "ext-uam-7", "uam7@example.com")
	f := createTestFeed(t, q, "comp.lang.go", "nntp://news.eternal-september.org/comp.lang.go", u.ID)
	a := createTestArticle(t, q, f.ID, "<cascade@host>", "Cascade")

	_, err := q.InsertUsenetArticleMeta(ctx, dbgen.InsertUsenetArticleMetaParams{
		ArticleID:     a.ID,
		FeedID:        f.ID,
		MessageID:     "<cascade@host>",
		RootMessageID: "<cascade@host>",
		GroupName:     "comp.lang.go",
		ArticleNumber: 600,
	})
	if err != nil {
		t.Fatalf("InsertUsenetArticleMeta: %v", err)
	}

	// Delete the article — meta should cascade-delete.
	_, err = sqlDB.ExecContext(ctx, "DELETE FROM articles WHERE id = ?", a.ID)
	if err != nil {
		t.Fatalf("DELETE FROM articles: %v", err)
	}

	_, err = q.GetUsenetArticleMeta(ctx, a.ID)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("GetUsenetArticleMeta after article delete: got %v, want sql.ErrNoRows", err)
	}
}

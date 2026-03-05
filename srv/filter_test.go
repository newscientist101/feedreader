package srv

import (
	"context"
	"testing"

	"github.com/newscientist101/feedreader/db/dbgen"
)

// ---------------------------------------------------------------------------
// matchesPattern
// ---------------------------------------------------------------------------

func TestMatchesPattern_Substring(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		pattern string
		want    bool
	}{
		{"exact match", "hello", "hello", true},
		{"substring", "hello world", "world", true},
		{"case insensitive", "Hello World", "hello", true},
		{"no match", "hello", "xyz", false},
		{"empty text", "", "hello", false},
		{"empty pattern", "hello", "", false},
		{"both empty", "", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := matchesPattern(tc.text, tc.pattern, false)
			if got != tc.want {
				t.Errorf("matchesPattern(%q, %q, false) = %v, want %v", tc.text, tc.pattern, got, tc.want)
			}
		})
	}
}

func TestMatchesPattern_Regex(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		pattern string
		want    bool
	}{
		{"simple regex", "hello world", "^hello", true},
		{"case insensitive", "Hello World", "hello", true},
		{"word boundary", "the cat sat", "\\bcat\\b", true},
		{"no match", "hello", "^world", false},
		{"empty text", "", ".*", false},        // empty text returns false early
		{"empty pattern", "hello", "", false},  // empty pattern returns false early
		{"invalid regex", "hello", "[", false}, // compile error -> false
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := matchesPattern(tc.text, tc.pattern, true)
			if got != tc.want {
				t.Errorf("matchesPattern(%q, %q, true) = %v, want %v", tc.text, tc.pattern, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ptrToStr
// ---------------------------------------------------------------------------

func TestPtrToStr(t *testing.T) {
	if ptrToStr(nil) != "" {
		t.Error("nil should return empty string")
	}
	s := "hello"
	if ptrToStr(&s) != "hello" {
		t.Error("expected 'hello'")
	}
}

// ---------------------------------------------------------------------------
// shouldExclude
// ---------------------------------------------------------------------------

func makeExclusion(exclType, pattern string, isRegex bool) dbgen.CategoryExclusion {
	var r *int64
	if isRegex {
		one := int64(1)
		r = &one
	} else {
		zero := int64(0)
		r = &zero
	}
	return dbgen.CategoryExclusion{
		ExclusionType: exclType,
		Pattern:       pattern,
		IsRegex:       r,
	}
}

func TestShouldExclude(t *testing.T) {
	s := &Server{}

	exclusions := []dbgen.CategoryExclusion{
		makeExclusion("keyword", "sponsored", false),
		makeExclusion("author", "spambot", false),
		makeExclusion("keyword", `^\[AD\]`, true),
	}

	tests := []struct {
		name    string
		title   string
		summary string
		author  string
		want    bool
	}{
		{"keyword in title", "This is sponsored content", "", "", true},
		{"keyword in summary", "Normal title", "Sponsored link", "", true},
		{"author match", "Title", "", "SpamBot", true},
		{"regex match", "[AD] Buy now", "", "", true},
		{"no match", "Normal article", "Normal summary", "RealAuthor", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := s.shouldExclude(tc.title, tc.summary, tc.author, exclusions)
			if got != tc.want {
				t.Errorf("shouldExclude(%q, %q, %q) = %v, want %v", tc.title, tc.summary, tc.author, got, tc.want)
			}
		})
	}
}

func TestShouldExclude_TitleOnly(t *testing.T) {
	s := &Server{}

	exclusions := []dbgen.CategoryExclusion{
		makeExclusion("title", "sponsored", false),
	}

	tests := []struct {
		name    string
		title   string
		summary string
		want    bool
	}{
		{"match in title", "Sponsored post", "clean summary", true},
		{"match in summary only", "Clean title", "This is sponsored", false},
		{"match in both", "Sponsored post", "Also sponsored", true},
		{"no match", "Normal title", "Normal summary", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := s.shouldExclude(tc.title, tc.summary, "", exclusions)
			if got != tc.want {
				t.Errorf("shouldExclude(%q, %q, \"\") = %v, want %v", tc.title, tc.summary, got, tc.want)
			}
		})
	}
}

func TestShouldExclude_TitleOnlyRegex(t *testing.T) {
	s := &Server{}

	exclusions := []dbgen.CategoryExclusion{
		makeExclusion("title", `^\[MEGA\]`, true),
	}

	tests := []struct {
		name    string
		title   string
		summary string
		want    bool
	}{
		{"regex match in title", "[MEGA] Thread", "", true},
		{"regex in summary only", "Normal", "[MEGA] Thread", false},
		{"no match", "Normal title", "Normal summary", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := s.shouldExclude(tc.title, tc.summary, "", exclusions)
			if got != tc.want {
				t.Errorf("shouldExclude(%q, %q, \"\") = %v, want %v", tc.title, tc.summary, got, tc.want)
			}
		})
	}
}

// isRead returns the is_read value, defaulting to 0 for nil.
func isRead(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

// ---------------------------------------------------------------------------
// MarkExcludedArticlesReadForFeed
// ---------------------------------------------------------------------------

func TestMarkExcludedArticlesReadForFeed_NoCategory(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	_, user := testUser(t, s)

	feed := createFeed(t, s, user.ID, "Test", "https://example.com/feed")
	a := createArticle(t, s, feed.ID, "Hello", "g1")

	// Feed has no category — nothing should happen
	s.MarkExcludedArticlesReadForFeed(context.Background(), feed.ID)

	q := dbgen.New(s.DB)
	art, _ := q.GetArticle(context.Background(), dbgen.GetArticleParams{ID: a.ID, UserID: &user.ID})
	if isRead(art.IsRead) != 0 {
		t.Error("article should still be unread")
	}
}

func TestMarkExcludedArticlesReadForFeed_NoExclusions(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, user := testUser(t, s)
	q := dbgen.New(s.DB)

	feed := createFeed(t, s, user.ID, "Test", "https://example.com/feed")
	cat, _ := q.CreateCategory(ctx, dbgen.CreateCategoryParams{Name: "News", UserID: &user.ID})
	q.AddFeedToCategory(ctx, dbgen.AddFeedToCategoryParams{FeedID: feed.ID, CategoryID: cat.ID})
	a := createArticle(t, s, feed.ID, "Hello", "g1")

	// Category has no exclusions — nothing should happen
	s.MarkExcludedArticlesReadForFeed(ctx, feed.ID)

	art, _ := q.GetArticle(ctx, dbgen.GetArticleParams{ID: a.ID, UserID: &user.ID})
	if isRead(art.IsRead) != 0 {
		t.Error("article should still be unread")
	}
}

func TestMarkExcludedArticlesReadForFeed_MarksMatchingArticles(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, user := testUser(t, s)
	q := dbgen.New(s.DB)

	feed := createFeed(t, s, user.ID, "Test", "https://example.com/feed")
	cat, _ := q.CreateCategory(ctx, dbgen.CreateCategoryParams{Name: "News", UserID: &user.ID})
	q.AddFeedToCategory(ctx, dbgen.AddFeedToCategoryParams{FeedID: feed.ID, CategoryID: cat.ID})

	var zero int64
	q.CreateExclusion(ctx, dbgen.CreateExclusionParams{
		CategoryID: cat.ID, ExclusionType: "keyword", Pattern: "sponsored", IsRegex: &zero,
	})

	good := createArticle(t, s, feed.ID, "Good article", "g1")
	bad := createArticle(t, s, feed.ID, "Sponsored content", "g2")

	s.MarkExcludedArticlesReadForFeed(ctx, feed.ID)

	artGood, _ := q.GetArticle(ctx, dbgen.GetArticleParams{ID: good.ID, UserID: &user.ID})
	artBad, _ := q.GetArticle(ctx, dbgen.GetArticleParams{ID: bad.ID, UserID: &user.ID})

	if isRead(artGood.IsRead) != 0 {
		t.Error("non-matching article should still be unread")
	}
	if isRead(artBad.IsRead) != 1 {
		t.Error("matching article should be marked read")
	}
}

func TestMarkExcludedArticlesReadForFeed_SkipsAlreadyRead(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, user := testUser(t, s)
	q := dbgen.New(s.DB)

	feed := createFeed(t, s, user.ID, "Test", "https://example.com/feed")
	cat, _ := q.CreateCategory(ctx, dbgen.CreateCategoryParams{Name: "News", UserID: &user.ID})
	q.AddFeedToCategory(ctx, dbgen.AddFeedToCategoryParams{FeedID: feed.ID, CategoryID: cat.ID})

	var zero int64
	q.CreateExclusion(ctx, dbgen.CreateExclusionParams{
		CategoryID: cat.ID, ExclusionType: "keyword", Pattern: "sponsored", IsRegex: &zero,
	})

	// Create and mark as read already
	a := createArticle(t, s, feed.ID, "Sponsored old", "g1")
	q.MarkArticleRead(ctx, dbgen.MarkArticleReadParams{ID: a.ID, UserID: &user.ID})

	// Should not error or panic
	s.MarkExcludedArticlesReadForFeed(ctx, feed.ID)
}

func TestMarkExcludedArticlesReadForFeed_RegexPattern(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, user := testUser(t, s)
	q := dbgen.New(s.DB)

	feed := createFeed(t, s, user.ID, "Test", "https://example.com/feed")
	cat, _ := q.CreateCategory(ctx, dbgen.CreateCategoryParams{Name: "News", UserID: &user.ID})
	q.AddFeedToCategory(ctx, dbgen.AddFeedToCategoryParams{FeedID: feed.ID, CategoryID: cat.ID})

	var one int64 = 1
	q.CreateExclusion(ctx, dbgen.CreateExclusionParams{
		CategoryID: cat.ID, ExclusionType: "keyword", Pattern: `^\[AD\]`, IsRegex: &one,
	})

	matched := createArticle(t, s, feed.ID, "[AD] Buy stuff", "g1")
	notMatched := createArticle(t, s, feed.ID, "Something [AD] in middle", "g2")

	s.MarkExcludedArticlesReadForFeed(ctx, feed.ID)

	artMatched, _ := q.GetArticle(ctx, dbgen.GetArticleParams{ID: matched.ID, UserID: &user.ID})
	artNot, _ := q.GetArticle(ctx, dbgen.GetArticleParams{ID: notMatched.ID, UserID: &user.ID})

	if isRead(artMatched.IsRead) != 1 {
		t.Error("regex-matched article should be marked read")
	}
	if isRead(artNot.IsRead) != 0 {
		t.Error("non-matching article should still be unread")
	}
}

func TestMarkExcludedArticlesReadForFeed_AuthorExclusion(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, user := testUser(t, s)
	q := dbgen.New(s.DB)

	feed := createFeed(t, s, user.ID, "Test", "https://example.com/feed")
	cat, _ := q.CreateCategory(ctx, dbgen.CreateCategoryParams{Name: "News", UserID: &user.ID})
	q.AddFeedToCategory(ctx, dbgen.AddFeedToCategoryParams{FeedID: feed.ID, CategoryID: cat.ID})

	var zero int64
	q.CreateExclusion(ctx, dbgen.CreateExclusionParams{
		CategoryID: cat.ID, ExclusionType: "author", Pattern: "spambot", IsRegex: &zero,
	})

	// Create articles with authors
	spamAuthor := "SpamBot"
	goodAuthor := "Real Author"
	url1 := "https://example.com/g1"
	url2 := "https://example.com/g2"
	spamArt, _ := q.CreateArticle(ctx, dbgen.CreateArticleParams{
		FeedID: feed.ID, Title: "Spam post", Guid: "g1", Author: &spamAuthor, Url: &url1,
	})
	goodArt, _ := q.CreateArticle(ctx, dbgen.CreateArticleParams{
		FeedID: feed.ID, Title: "Good post", Guid: "g2", Author: &goodAuthor, Url: &url2,
	})

	s.MarkExcludedArticlesReadForFeed(ctx, feed.ID)

	spam, _ := q.GetArticle(ctx, dbgen.GetArticleParams{ID: spamArt.ID, UserID: &user.ID})
	good, _ := q.GetArticle(ctx, dbgen.GetArticleParams{ID: goodArt.ID, UserID: &user.ID})

	if isRead(spam.IsRead) != 1 {
		t.Error("spam author article should be marked read")
	}
	if isRead(good.IsRead) != 0 {
		t.Error("good author article should still be unread")
	}
}

func TestMarkExcludedArticlesReadForFeed_TitleOnlyExclusion(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, user := testUser(t, s)
	q := dbgen.New(s.DB)

	feed := createFeed(t, s, user.ID, "Test", "https://example.com/feed")
	cat, _ := q.CreateCategory(ctx, dbgen.CreateCategoryParams{Name: "News", UserID: &user.ID})
	q.AddFeedToCategory(ctx, dbgen.AddFeedToCategoryParams{FeedID: feed.ID, CategoryID: cat.ID})

	var zero int64
	q.CreateExclusion(ctx, dbgen.CreateExclusionParams{
		CategoryID: cat.ID, ExclusionType: "title", Pattern: "open thread", IsRegex: &zero,
	})

	// Title match — should be excluded
	titleMatch := createArticle(t, s, feed.ID, "Open Thread #42", "g1")
	// Summary-only match — should NOT be excluded
	summary := "This is an open thread discussion"
	url := "https://example.com/g2"
	summaryOnly, _ := q.CreateArticle(ctx, dbgen.CreateArticleParams{
		FeedID: feed.ID, Title: "Weekly Discussion", Guid: "g2", Summary: &summary, Url: &url,
	})
	// No match
	noMatch := createArticle(t, s, feed.ID, "Normal Article", "g3")

	s.MarkExcludedArticlesReadForFeed(ctx, feed.ID)

	for _, tc := range []struct {
		name string
		id   int64
		want int64
	}{
		{"title match", titleMatch.ID, 1},
		{"summary only match", summaryOnly.ID, 0},
		{"no match", noMatch.ID, 0},
	} {
		art, _ := q.GetArticle(ctx, dbgen.GetArticleParams{ID: tc.id, UserID: &user.ID})
		if isRead(art.IsRead) != tc.want {
			t.Errorf("%s: is_read = %d, want %d", tc.name, isRead(art.IsRead), tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// MarkExcludedArticlesReadForCategory
// ---------------------------------------------------------------------------

func TestMarkExcludedArticlesReadForCategory_MarksMatchingArticles(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, user := testUser(t, s)
	q := dbgen.New(s.DB)

	cat, _ := q.CreateCategory(ctx, dbgen.CreateCategoryParams{Name: "News", UserID: &user.ID})

	// Two feeds in the same category
	feed1 := createFeed(t, s, user.ID, "Feed1", "https://example.com/f1")
	feed2 := createFeed(t, s, user.ID, "Feed2", "https://example.com/f2")
	q.AddFeedToCategory(ctx, dbgen.AddFeedToCategoryParams{FeedID: feed1.ID, CategoryID: cat.ID})
	q.AddFeedToCategory(ctx, dbgen.AddFeedToCategoryParams{FeedID: feed2.ID, CategoryID: cat.ID})

	var zero int64
	q.CreateExclusion(ctx, dbgen.CreateExclusionParams{
		CategoryID: cat.ID, ExclusionType: "keyword", Pattern: "open thread", IsRegex: &zero,
	})

	good1 := createArticle(t, s, feed1.ID, "Good article", "g1")
	bad1 := createArticle(t, s, feed1.ID, "Open Thread", "g2")
	good2 := createArticle(t, s, feed2.ID, "Another good one", "g3")
	bad2 := createArticle(t, s, feed2.ID, "Weekly Open Thread", "g4")

	s.MarkExcludedArticlesReadForCategory(ctx, cat.ID, user.ID)

	for _, tc := range []struct {
		name string
		id   int64
		want int64
	}{
		{"good1", good1.ID, 0},
		{"bad1", bad1.ID, 1},
		{"good2", good2.ID, 0},
		{"bad2", bad2.ID, 1},
	} {
		art, _ := q.GetArticle(ctx, dbgen.GetArticleParams{ID: tc.id, UserID: &user.ID})
		if isRead(art.IsRead) != tc.want {
			t.Errorf("%s: is_read = %d, want %d", tc.name, isRead(art.IsRead), tc.want)
		}
	}
}

func TestMarkExcludedArticlesReadForCategory_NoExclusions(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, user := testUser(t, s)
	q := dbgen.New(s.DB)

	cat, _ := q.CreateCategory(ctx, dbgen.CreateCategoryParams{Name: "News", UserID: &user.ID})
	feed := createFeed(t, s, user.ID, "Feed", "https://example.com/feed")
	q.AddFeedToCategory(ctx, dbgen.AddFeedToCategoryParams{FeedID: feed.ID, CategoryID: cat.ID})
	a := createArticle(t, s, feed.ID, "Hello", "g1")

	s.MarkExcludedArticlesReadForCategory(ctx, cat.ID, user.ID)

	art, _ := q.GetArticle(ctx, dbgen.GetArticleParams{ID: a.ID, UserID: &user.ID})
	if isRead(art.IsRead) != 0 {
		t.Error("article should still be unread with no exclusions")
	}
}

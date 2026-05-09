package srv

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/newscientist101/feedreader/db/dbgen"
)

func TestNewRetentionManager(t *testing.T) {
	s := testServer(t)
	rm := NewRetentionManager(s)
	if rm.server != s {
		t.Error("server not set")
	}
}

func TestRetentionManager_StartStop(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	rm := NewRetentionManager(s)
	rm.checkInterval = 10 * time.Second // short interval to avoid blocking
	rm.Start()
	time.Sleep(50 * time.Millisecond)
	rm.Stop()
	// Should not hang or panic
}

func TestRetentionManager_RunCleanupNow(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	_, user := testUser(t, s)
	rm := NewRetentionManager(s)

	deleted, err := rm.RunCleanupNow(user.ID)
	if err != nil {
		t.Fatalf("RunCleanupNow: %v", err)
	}
	if deleted != 0 {
		t.Errorf("expected 0 deleted (empty DB), got %d", deleted)
	}
}

func TestRetentionManager_GetStats_Default(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	_, user := testUser(t, s)
	rm := NewRetentionManager(s)

	stats, err := rm.GetStats(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if stats.RetentionDays != defaultRetentionDays {
		t.Errorf("RetentionDays = %d, want %d", stats.RetentionDays, defaultRetentionDays)
	}
	if stats.ArticlesToDelete != 0 {
		t.Errorf("ArticlesToDelete = %d, want 0", stats.ArticlesToDelete)
	}
}

func TestRetentionManager_GetStats_CustomRetention(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	_, user := testUser(t, s)
	rm := NewRetentionManager(s)

	// Set custom retention
	q := dbgen.New(s.DB)
	_ = q.SetUserSetting(context.Background(), dbgen.SetUserSettingParams{
		UserID: user.ID,
		Key:    "retentionDays",
		Value:  "14",
	})

	stats, err := rm.GetStats(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if stats.RetentionDays != 14 {
		t.Errorf("RetentionDays = %d, want 14", stats.RetentionDays)
	}
}

func TestRetentionManager_CleanupWithData(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	ctx, user := testUser(t, s)
	feed := createFeed(t, s, user.ID, "f", "http://f")

	// Insert an article with an old fetched_at (retention checks fetched_at, not published_at)
	oldDate := time.Now().AddDate(0, 0, -60) // 60 days ago
	_, err := s.DB.Exec(`INSERT INTO articles (feed_id, guid, title, fetched_at, is_read, is_starred) VALUES (?, ?, ?, ?, 0, 0)`,
		feed.ID, "old-guid", "Old Article", oldDate)
	if err != nil {
		t.Fatal(err)
	}

	rm := NewRetentionManager(s)

	// Stats should show 1 article to delete (default 30 days, article is 60 days old)
	stats, _ := rm.GetStats(ctx, user.ID)
	if stats.ArticlesToDelete != 1 {
		t.Errorf("ArticlesToDelete = %d, want 1", stats.ArticlesToDelete)
	}

	// Cleanup
	deleted, err := rm.RunCleanupNow(user.ID)
	if err != nil {
		t.Fatalf("RunCleanupNow: %v", err)
	}
	if deleted != 1 {
		t.Errorf("deleted = %d, want 1", deleted)
	}
}

func TestRetentionManager_CleanupRespectsUserSetting(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	_, user := testUser(t, s)
	feed := createFeed(t, s, user.ID, "f", "http://f")

	// Insert an article 20 days old
	oldDate := time.Now().AddDate(0, 0, -20)
	_, err := s.DB.Exec(`INSERT INTO articles (feed_id, guid, title, fetched_at, is_read, is_starred) VALUES (?, ?, ?, ?, 0, 0)`,
		feed.ID, "twenty-days", "Twenty Day Article", oldDate)
	if err != nil {
		t.Fatal(err)
	}

	rm := NewRetentionManager(s)

	// With default 30-day retention, article should NOT be deleted
	deleted, err := rm.RunCleanupNow(user.ID)
	if err != nil {
		t.Fatalf("RunCleanupNow: %v", err)
	}
	if deleted != 0 {
		t.Errorf("deleted = %d with 30-day retention, want 0", deleted)
	}

	// Set retention to 14 days — now the article should be deleted
	q := dbgen.New(s.DB)
	_ = q.SetUserSetting(context.Background(), dbgen.SetUserSettingParams{
		UserID: user.ID,
		Key:    "retentionDays",
		Value:  "14",
	})

	deleted, err = rm.RunCleanupNow(user.ID)
	if err != nil {
		t.Fatalf("RunCleanupNow after setting 14 days: %v", err)
	}
	if deleted != 1 {
		t.Errorf("deleted = %d with 14-day retention, want 1", deleted)
	}
}

// --- Usenet retention tests (feedreader-6g2.24) ---

// TestRetentionCleansNNTPArticle verifies that an old unstarred Usenet article is
// deleted by the retention manager, including its usenet_article_meta row via
// ON DELETE CASCADE.
func TestRetentionCleansNNTPArticle(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	_, user := testUser(t, s)

	// Create a minimal NNTP feed (no usenet_feed_state needed for retention tests).
	interval := int64(60)
	url := "nntp://news.eternal-september.org/comp.lang.go"
	q := dbgen.New(s.DB)
	nntpFeed, err := q.CreateFeed(context.Background(), dbgen.CreateFeedParams{
		Name: "comp.lang.go", Url: url, FeedType: "nntp",
		FetchIntervalMinutes: &interval, UserID: &user.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Insert an old article (60 days ago) with a usenet_article_meta companion row.
	oldDate := time.Now().AddDate(0, 0, -60)
	var articleID int64
	err = s.DB.QueryRowContext(context.Background(),
		`INSERT INTO articles (feed_id, guid, title, fetched_at, is_read, is_starred)
		 VALUES (?, ?, ?, ?, 0, 0) RETURNING id`,
		nntpFeed.ID, "<old@nntp>", "Old Usenet Post", oldDate).Scan(&articleID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.DB.ExecContext(context.Background(),
		`INSERT INTO usenet_article_meta
			(article_id, feed_id, message_id, root_message_id, group_name, article_number,
			 created_at, updated_at)
		 VALUES (?, ?, '<old@nntp>', '<old@nntp>', 'comp.lang.go', 42,
		         CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		articleID, nntpFeed.ID)
	if err != nil {
		t.Fatal(err)
	}

	rm := NewRetentionManager(s)

	// Stats: 1 article should be eligible.
	stats, err := rm.GetStats(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if stats.ArticlesToDelete != 1 {
		t.Fatalf("ArticlesToDelete = %d, want 1", stats.ArticlesToDelete)
	}

	// Run cleanup.
	deleted, err := rm.RunCleanupNow(user.ID)
	if err != nil {
		t.Fatalf("RunCleanupNow: %v", err)
	}
	if deleted != 1 {
		t.Errorf("deleted = %d, want 1", deleted)
	}

	// Article row should be gone.
	var n int
	err = s.DB.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM articles WHERE id = ?`, articleID).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("articles row still exists after deletion")
	}

	// usenet_article_meta should also be gone via CASCADE.
	err = s.DB.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM usenet_article_meta WHERE article_id = ?`, articleID).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("usenet_article_meta row still exists after CASCADE delete")
	}
}

// TestRetentionSparsNNTPArticleStarred verifies that a starred Usenet article is
// NOT deleted by the retention manager.
func TestRetentionSparsNNTPArticleStarred(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	_, user := testUser(t, s)

	interval := int64(60)
	url := "nntp://news.eternal-september.org/alt.test"
	q := dbgen.New(s.DB)
	nntpFeed, err := q.CreateFeed(context.Background(), dbgen.CreateFeedParams{
		Name: "alt.test", Url: url, FeedType: "nntp",
		FetchIntervalMinutes: &interval, UserID: &user.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Insert a starred article 60 days old.
	oldDate := time.Now().AddDate(0, 0, -60)
	_, err = s.DB.ExecContext(context.Background(),
		`INSERT INTO articles (feed_id, guid, title, fetched_at, is_read, is_starred)
		 VALUES (?, ?, ?, ?, 0, 1)`,
		nntpFeed.ID, "<starred@nntp>", "Starred Usenet Post", oldDate)
	if err != nil {
		t.Fatal(err)
	}

	rm := NewRetentionManager(s)
	deleted, err := rm.RunCleanupNow(user.ID)
	if err != nil {
		t.Fatalf("RunCleanupNow: %v", err)
	}
	if deleted != 0 {
		t.Errorf("starred article should not be deleted; deleted=%d", deleted)
	}
}

// TestRetentionSparsNNTPArticleQueued verifies that a queued Usenet article is
// NOT deleted by the retention manager.
func TestRetentionSparsNNTPArticleQueued(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	_, user := testUser(t, s)

	interval := int64(60)
	url := "nntp://news.eternal-september.org/alt.test2"
	q := dbgen.New(s.DB)
	nntpFeed, err := q.CreateFeed(context.Background(), dbgen.CreateFeedParams{
		Name: "alt.test2", Url: url, FeedType: "nntp",
		FetchIntervalMinutes: &interval, UserID: &user.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Insert an unstarred article 60 days old.
	oldDate := time.Now().AddDate(0, 0, -60)
	var articleID int64
	err = s.DB.QueryRowContext(context.Background(),
		`INSERT INTO articles (feed_id, guid, title, fetched_at, is_read, is_starred)
		 VALUES (?, ?, ?, ?, 0, 0) RETURNING id`,
		nntpFeed.ID, "<queued@nntp>", "Queued Usenet Post", oldDate).Scan(&articleID)
	if err != nil {
		t.Fatal(err)
	}

	// Add article to user's read queue.
	if err := q.AddToQueue(context.Background(), dbgen.AddToQueueParams{
		UserID: user.ID, ArticleID: articleID,
	}); err != nil {
		t.Fatal(err)
	}

	rm := NewRetentionManager(s)
	deleted, err := rm.RunCleanupNow(user.ID)
	if err != nil {
		t.Fatalf("RunCleanupNow: %v", err)
	}
	if deleted != 0 {
		t.Errorf("queued article should not be deleted; deleted=%d", deleted)
	}
}

// TestRetentionNNTPMetaCascadeWithMultipleArticles verifies that bulk retention
// correctly cascades and removes all eligible usenet_article_meta rows.
func TestRetentionNNTPMetaCascadeWithMultipleArticles(t *testing.T) {
	t.Parallel()
	s := testServer(t)
	_, user := testUser(t, s)

	interval := int64(60)
	url := "nntp://news.eternal-september.org/comp.test"
	q := dbgen.New(s.DB)
	nntpFeed, err := q.CreateFeed(context.Background(), dbgen.CreateFeedParams{
		Name: "comp.test", Url: url, FeedType: "nntp",
		FetchIntervalMinutes: &interval, UserID: &user.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	oldDate := time.Now().AddDate(0, 0, -60)

	// Insert 3 old articles with meta rows.
	for i := range 3 {
		var artID int64
		msgID := fmt.Sprintf("<bulk%d@nntp>", i)
		err = s.DB.QueryRowContext(context.Background(),
			`INSERT INTO articles (feed_id, guid, title, fetched_at, is_read, is_starred)
			 VALUES (?, ?, ?, ?, 0, 0) RETURNING id`,
			nntpFeed.ID, msgID, "Bulk Post "+strconv.Itoa(i), oldDate).Scan(&artID)
		if err != nil {
			t.Fatal(err)
		}
		_, err = s.DB.ExecContext(context.Background(),
			`INSERT INTO usenet_article_meta
				(article_id, feed_id, message_id, root_message_id, group_name, article_number,
				 created_at, updated_at)
			 VALUES (?, ?, ?, ?, 'comp.test', ?,
			         CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
			artID, nntpFeed.ID, msgID, msgID, int64(i+1))
		if err != nil {
			t.Fatal(err)
		}
	}

	rm := NewRetentionManager(s)
	deleted, err := rm.RunCleanupNow(user.ID)
	if err != nil {
		t.Fatalf("RunCleanupNow: %v", err)
	}
	if deleted != 3 {
		t.Errorf("deleted = %d, want 3", deleted)
	}

	// All usenet_article_meta rows should be gone.
	var n int
	err = s.DB.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM usenet_article_meta WHERE feed_id = ?`, nntpFeed.ID).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("expected 0 usenet_article_meta rows after cleanup, got %d", n)
	}
}

package srv

import (
	"context"
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

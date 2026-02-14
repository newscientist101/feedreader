package srv

import (
	"context"
	"testing"
	"time"
)

func TestNewRetentionManager(t *testing.T) {
	s := testServer(t)
	rm := NewRetentionManager(s, 30)
	if rm.retentionDays != 30 {
		t.Errorf("retentionDays = %d", rm.retentionDays)
	}
	if rm.server != s {
		t.Error("server not set")
	}
}

func TestRetentionManager_StartStop(t *testing.T) {
	s := testServer(t)
	rm := NewRetentionManager(s, 30)
	rm.checkInterval = 10 * time.Second // short interval to avoid blocking
	rm.Start()
	time.Sleep(50 * time.Millisecond)
	rm.Stop()
	// Should not hang or panic
}

func TestRetentionManager_RunCleanupNow(t *testing.T) {
	s := testServer(t)
	rm := NewRetentionManager(s, 30)

	deleted, err := rm.RunCleanupNow()
	if err != nil {
		t.Fatalf("RunCleanupNow: %v", err)
	}
	if deleted != 0 {
		t.Errorf("expected 0 deleted (empty DB), got %d", deleted)
	}
}

func TestRetentionManager_GetStats(t *testing.T) {
	s := testServer(t)
	rm := NewRetentionManager(s, 30)

	stats, err := rm.GetStats(context.Background())
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if stats.RetentionDays != 30 {
		t.Errorf("RetentionDays = %d", stats.RetentionDays)
	}
	if stats.ArticlesToDelete != 0 {
		t.Errorf("ArticlesToDelete = %d, want 0", stats.ArticlesToDelete)
	}
}

func TestRetentionManager_CleanupWithData(t *testing.T) {
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

	rm := NewRetentionManager(s, 30)

	// Stats should show 1 article to delete
	stats, _ := rm.GetStats(ctx)
	if stats.ArticlesToDelete != 1 {
		t.Errorf("ArticlesToDelete = %d, want 1", stats.ArticlesToDelete)
	}

	// Cleanup
	deleted, err := rm.RunCleanupNow()
	if err != nil {
		t.Fatalf("RunCleanupNow: %v", err)
	}
	if deleted != 1 {
		t.Errorf("deleted = %d, want 1", deleted)
	}
}

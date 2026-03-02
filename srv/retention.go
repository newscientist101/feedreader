package srv

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/newscientist101/feedreader/db/dbgen"
)

// RetentionManager handles automatic cleanup of old articles
type RetentionManager struct {
	server        *Server
	retentionDays int
	checkInterval time.Duration
	stopChan      chan struct{}
	wg            sync.WaitGroup
}

// NewRetentionManager creates a new retention manager
func NewRetentionManager(s *Server, retentionDays int) *RetentionManager {
	return &RetentionManager{
		server:        s,
		retentionDays: retentionDays,
		checkInterval: 6 * time.Hour, // Check every 6 hours
		stopChan:      make(chan struct{}),
	}
}

// Start begins the retention cleanup background task
func (rm *RetentionManager) Start() {
	rm.wg.Add(1)
	go rm.run()
	slog.Info("retention manager started", "retention_days", rm.retentionDays)
}

// Stop gracefully stops the retention manager
func (rm *RetentionManager) Stop() {
	close(rm.stopChan)
	rm.wg.Wait()
}

func (rm *RetentionManager) run() {
	defer rm.wg.Done()

	// Run cleanup immediately on start
	rm.cleanup()

	ticker := time.NewTicker(rm.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rm.cleanup()
		case <-rm.stopChan:
			return
		}
	}
}

func (rm *RetentionManager) cleanup() {
	ctx := context.Background()
	q := dbgen.New(rm.server.DB)

	// First count how many will be deleted
	daysStr := fmt.Sprintf("%d", rm.retentionDays)
	count, err := q.CountOldUnstarredArticlesGlobal(ctx, &daysStr)
	if err != nil {
		slog.Error("retention: count old articles", "error", err)
		return
	}

	if count == 0 {
		slog.Debug("retention: no old articles to clean up")
		return
	}

	// Delete old unstarred articles
	result, err := q.DeleteOldUnstarredArticlesGlobal(ctx, &daysStr)
	if err != nil {
		slog.Error("retention: delete old articles", "error", err)
		return
	}

	deleted, _ := result.RowsAffected()
	if deleted > 0 {
		rm.server.CountsCache.InvalidateAll()
	}
	slog.Info("retention: cleaned up old articles", "deleted", deleted, "retention_days", rm.retentionDays)
}

// RunCleanupNow triggers an immediate cleanup (for manual/API use)
func (rm *RetentionManager) RunCleanupNow() (int64, error) {
	ctx := context.Background()
	q := dbgen.New(rm.server.DB)

	daysStr := fmt.Sprintf("%d", rm.retentionDays)
	result, err := q.DeleteOldUnstarredArticlesGlobal(ctx, &daysStr)
	if err != nil {
		return 0, err
	}

	deleted, err := result.RowsAffected()
	if deleted > 0 {
		rm.server.CountsCache.InvalidateAll()
	}
	return deleted, err
}

// GetStats returns retention statistics
func (rm *RetentionManager) GetStats(ctx context.Context) (RetentionStats, error) {
	q := dbgen.New(rm.server.DB)

	daysStr := fmt.Sprintf("%d", rm.retentionDays)
	oldCount, err := q.CountOldUnstarredArticlesGlobal(ctx, &daysStr)
	if err != nil {
		return RetentionStats{}, err
	}

	oldestDate, _ := q.GetOldestArticleDateGlobal(ctx)
	var oldest *time.Time
	if oldestDate != nil {
		if t, ok := oldestDate.(time.Time); ok {
			oldest = &t
		}
	}

	return RetentionStats{
		RetentionDays:    rm.retentionDays,
		ArticlesToDelete: oldCount,
		OldestArticle:    oldest,
	}, nil
}

type RetentionStats struct {
	RetentionDays    int        `json:"retention_days"`
	ArticlesToDelete int64      `json:"articles_to_delete"`
	OldestArticle    *time.Time `json:"oldest_article"`
}

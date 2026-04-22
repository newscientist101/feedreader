package srv

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/newscientist101/feedreader/db/dbgen"
)

const defaultRetentionDays = 30

// RetentionManager handles automatic cleanup of old articles
type RetentionManager struct {
	server        *Server
	checkInterval time.Duration
	stopChan      chan struct{}
	wg            sync.WaitGroup
}

// NewRetentionManager creates a new retention manager
func NewRetentionManager(s *Server) *RetentionManager {
	return &RetentionManager{
		server:        s,
		checkInterval: 6 * time.Hour, // Check every 6 hours
		stopChan:      make(chan struct{}),
	}
}

// Start begins the retention cleanup background task
func (rm *RetentionManager) Start() {
	rm.wg.Add(1)
	go rm.run()
	slog.Info("retention manager started", "default_retention_days", defaultRetentionDays)
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

// getUserRetentionDays returns the configured retention days for a user,
// falling back to the default if not set.
func (rm *RetentionManager) getUserRetentionDays(ctx context.Context, q *dbgen.Queries, userID int64) int {
	val, err := q.GetUserSetting(ctx, dbgen.GetUserSettingParams{
		UserID: userID,
		Key:    "retentionDays",
	})
	if err != nil {
		return defaultRetentionDays
	}
	days, err := strconv.Atoi(val)
	if err != nil || days < 1 {
		return defaultRetentionDays
	}
	return days
}

func (rm *RetentionManager) cleanup() {
	ctx := context.Background()
	q := dbgen.New(rm.server.DB)

	userIDs, err := q.ListAllUserIDs(ctx)
	if err != nil {
		slog.Error("retention: list users", "error", err)
		return
	}

	var totalDeleted int64
	for _, userID := range userIDs {
		uid := userID
		days := rm.getUserRetentionDays(ctx, q, uid)
		daysStr := fmt.Sprintf("%d", days)

		// Record GUIDs before deletion so the fetcher can skip re-insertion.
		if err := q.InsertSeenGuids(ctx, dbgen.InsertSeenGuidsParams{
			Column1: &daysStr,
			UserID:  &uid,
		}); err != nil {
			slog.Warn("retention: record seen guids", "user_id", uid, "error", err)
		}

		result, err := q.DeleteOldUnstarredArticles(ctx, dbgen.DeleteOldUnstarredArticlesParams{
			Column1: &daysStr,
			UserID:  &uid,
		})
		if err != nil {
			slog.Error("retention: delete old articles", "user_id", uid, "error", err)
			continue
		}

		deleted, _ := result.RowsAffected()
		if deleted > 0 {
			slog.Info("retention: cleaned up articles", "user_id", uid, "deleted", deleted, "retention_days", days)
			totalDeleted += deleted
		}

		// Prune seen_guids older than 2x the retention window to bound table growth.
		pruneDays := fmt.Sprintf("%d", days*2)
		if err := q.PruneSeenGuids(ctx, &pruneDays); err != nil {
			slog.Warn("retention: prune seen guids", "error", err)
		}
	}

	if totalDeleted > 0 {
		rm.server.CountsCache.InvalidateAll()
	}
	if totalDeleted == 0 {
		slog.Debug("retention: no old articles to clean up")
	}
}

// RunCleanupNow triggers an immediate cleanup for a specific user (for manual/API use)
func (rm *RetentionManager) RunCleanupNow(userID int64) (int64, error) {
	ctx := context.Background()
	q := dbgen.New(rm.server.DB)

	days := rm.getUserRetentionDays(ctx, q, userID)
	daysStr := fmt.Sprintf("%d", days)

	// Record GUIDs before deletion so the fetcher can skip re-insertion.
	if err := q.InsertSeenGuids(ctx, dbgen.InsertSeenGuidsParams{
		Column1: &daysStr,
		UserID:  &userID,
	}); err != nil {
		slog.Warn("retention: record seen guids", "user_id", userID, "error", err)
	}

	result, err := q.DeleteOldUnstarredArticles(ctx, dbgen.DeleteOldUnstarredArticlesParams{
		Column1: &daysStr,
		UserID:  &userID,
	})
	if err != nil {
		return 0, err
	}

	deleted, err := result.RowsAffected()
	if deleted > 0 {
		rm.server.CountsCache.InvalidateAll()
	}
	return deleted, err
}

// GetStats returns retention statistics for a specific user
func (rm *RetentionManager) GetStats(ctx context.Context, userID int64) (RetentionStats, error) {
	q := dbgen.New(rm.server.DB)

	days := rm.getUserRetentionDays(ctx, q, userID)
	daysStr := fmt.Sprintf("%d", days)

	oldCount, err := q.CountOldUnstarredArticles(ctx, dbgen.CountOldUnstarredArticlesParams{
		Column1: &daysStr,
		UserID:  &userID,
	})
	if err != nil {
		return RetentionStats{}, err
	}

	oldestDate, _ := q.GetOldestArticleDate(ctx, &userID)
	var oldest *time.Time
	if oldestDate != nil {
		if t, ok := oldestDate.(time.Time); ok {
			oldest = &t
		}
	}

	return RetentionStats{
		RetentionDays:    days,
		ArticlesToDelete: oldCount,
		OldestArticle:    oldest,
	}, nil
}

type RetentionStats struct {
	RetentionDays    int        `json:"retention_days"`
	ArticlesToDelete int64      `json:"articles_to_delete"`
	OldestArticle    *time.Time `json:"oldest_article"`
}

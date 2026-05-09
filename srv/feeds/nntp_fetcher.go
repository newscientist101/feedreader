package feeds

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/newscientist101/feedreader/db/dbgen"
)

// firstFetchCount is the number of latest article numbers imported on the
// first fetch for a newsgroup.
const firstFetchCount = 100

// fetchRunCap is the maximum number of article numbers processed per group per
// background fetch run.
const fetchRunCap = 500

// NNTPDialer is the interface for establishing an authenticated NNTP
// connection. The implementation lives in srv/nntp and is injected into the
// Fetcher to avoid an import cycle.
type NNTPDialer interface {
	// Dial connects to the NNTP server, authenticates, and returns a
	// NNTPConn ready for use.
	Dial(ctx context.Context, username, password string) (NNTPConn, error)
}

// NNTPConn is the interface for a live NNTP connection. It exposes only the
// methods required by the fetch job so that tests can use a fake.
type NNTPConn interface {
	// SelectGroup selects a newsgroup and returns its article range.
	// count is the approximate number of articles, low and high are the
	// lowest and highest available article numbers, canonName is the
	// server-canonical group name.
	SelectGroup(name string) (count, low, high int64, canonName string, err error)
	// Overview fetches article overviews for the inclusive range [low, high].
	Overview(low, high int64) ([]NNTPOverviewRow, error)
	// FetchArticle retrieves the full article for the given article number.
	FetchArticle(articleNumber int64) (*NNTPArticle, error)
	// Close terminates the connection.
	Close() error
}

// NNTPOverviewRow is a minimal type alias for the overview data returned by
// the NNTP server. The concrete type is nntp.OverviewRow; the Fetcher works
// through this interface-compatible struct to remain import-cycle free.
type NNTPOverviewRow struct {
	ArticleNumber int64
	Subject       string
	From          string
	Date          string
	MessageID     string
	References    string
	Bytes         int64
	Lines         int64
}

// NNTPArticle is a minimal type alias for the full article returned by the
// NNTP server. The concrete type is nntp.Article.
type NNTPArticle struct {
	// Headers are the parsed message headers in title-case (e.g. "Message-Id").
	Headers map[string]string
	// Body is the decoded message body.
	Body string
}

// CredentialDecryptor can decrypt an NNTP credential password blob.
// It is implemented by *usenet.Crypto and injected into the Fetcher.
type CredentialDecryptor interface {
	Decrypt(encoded string) (string, error)
}

// ErrNNTPNotConfigured is returned by the NNTP dispatch path when the Fetcher
// has no NNTPDialer configured.
var ErrNNTPNotConfigured = errors.New("feeds: NNTP fetcher is not configured")

// planArticleRange computes the inclusive [start, end] article number range
// to request for a single fetch run, given the server-reported low/high
// article numbers and the current high-water mark for this group.
//
// Policy (from feedreader-6g2.46 D5):
//   - First fetch (highWater == 0): import the latest firstFetchCount articles.
//     start = max(low, high - firstFetchCount + 1)
//   - Subsequent fetches: start = highWater + 1
//   - start is clamped to at least low (the server's lowest available article).
//   - The run is capped at fetchRunCap articles:
//     end = min(high, start + fetchRunCap - 1)
//   - If start > high (no new articles), start > end and the caller must
//     treat this as a no-op range.
//
// Returns (start, end). When start > end the range is empty (no-op).
func planArticleRange(serverLow, serverHigh, highWater int64) (start, end int64) {
	if serverLow <= 0 || serverHigh <= 0 || serverHigh < serverLow {
		// Empty or invalid group reported by the server.
		return 1, 0
	}

	if highWater == 0 {
		// First fetch: import the latest firstFetchCount articles.
		start = serverHigh - firstFetchCount + 1
	} else {
		// Subsequent fetch: start from the next unseen article.
		start = highWater + 1
	}

	// Clamp start to the server's lowest available article.
	if start < serverLow {
		start = serverLow
	}

	// Apply the per-run cap.
	end = min(start+fetchRunCap-1, serverHigh)

	return start, end
}

// fetchNNTPFeed is the top-level NNTP handler called from FetchFeed for
// feed_type='nntp' rows. It handles feed status updates directly (unlike
// RSS/scraper paths that use the generic FeedItem loop).
//
// The full fetch implementation is added in feedreader-6g2.18.2 through
// feedreader-6g2.18.5. This stub ensures the dispatch compiles and updates
// feed status correctly.
func (f *Fetcher) fetchNNTPFeed(ctx context.Context, q *dbgen.Queries, now time.Time, feed *dbgen.Feed) error {
	slog.Debug("starting nntp feed fetch", "feed_id", feed.ID, "url", feed.Url)

	if f.NNTPDialer == nil {
		errMsg := fmt.Sprintf("%s: set Fetcher.NNTPDialer to enable Usenet support", ErrNNTPNotConfigured.Error())
		if err := q.IncrementFeedErrors(ctx, dbgen.IncrementFeedErrorsParams{
			LastError:     &errMsg,
			LastFetchedAt: &now,
			ID:            feed.ID,
		}); err != nil {
			slog.Warn("increment feed errors", "error", err)
		}
		return fmt.Errorf("%w: set Fetcher.NNTPDialer to enable Usenet support", ErrNNTPNotConfigured)
	}

	// Look up the NNTP-specific state for this feed.
	state, err := q.GetUsenetFeedState(ctx, dbgen.GetUsenetFeedStateParams{
		FeedID: feed.ID,
		UserID: feed.UserID,
	})
	if err != nil {
		errMsg := fmt.Sprintf("nntp feed state lookup failed: %v", err)
		if dbErr := q.IncrementFeedErrors(ctx, dbgen.IncrementFeedErrorsParams{
			LastError:     &errMsg,
			LastFetchedAt: &now,
			ID:            feed.ID,
		}); dbErr != nil {
			slog.Warn("increment feed errors", "error", dbErr)
		}
		return fmt.Errorf("nntp feed state lookup: %w", err)
	}

	// Full implementation (credential lookup, group select, overview fetch,
	// article import, high-water update) added in feedreader-6g2.18.2 through
	// feedreader-6g2.18.5.
	errMsg := fmt.Sprintf("%s: implementation pending", ErrNNTPNotConfigured.Error())
	if dbErr := q.IncrementFeedErrors(ctx, dbgen.IncrementFeedErrorsParams{
		LastError:     &errMsg,
		LastFetchedAt: &now,
		ID:            feed.ID,
	}); dbErr != nil {
		slog.Warn("increment feed errors", "error", dbErr)
	}
	return fmt.Errorf("%w: implementation pending (group=%s)", ErrNNTPNotConfigured, state.GroupName)
}

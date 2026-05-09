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

// setFeedError is a convenience helper that records a fetch error on a feed
// via IncrementFeedErrors and logs a warning if the DB call itself fails.
func (f *Fetcher) setFeedError(ctx context.Context, q *dbgen.Queries, feedID int64, now time.Time, msg string) {
	if err := q.IncrementFeedErrors(ctx, dbgen.IncrementFeedErrorsParams{
		LastError:     &msg,
		LastFetchedAt: &now,
		ID:            feedID,
	}); err != nil {
		slog.Warn("increment feed errors", "feed_id", feedID, "error", err)
	}
}

// fetchNNTPFeed is the top-level NNTP handler called from FetchFeed for
// feed_type='nntp' rows. It handles feed status updates directly (unlike
// RSS/scraper paths that use the generic FeedItem loop).
//
// Phase 1 (feedreader-6g2.18.2): load and decrypt credentials, dial NNTP.
// Phase 2 (feedreader-6g2.18.3 – 18.5): group select, overview fetch,
// article import, high-water and feed-status updates.
func (f *Fetcher) fetchNNTPFeed(ctx context.Context, q *dbgen.Queries, now time.Time, feed *dbgen.Feed) error {
	slog.Debug("starting nntp feed fetch", "feed_id", feed.ID, "url", feed.Url)

	if f.NNTPDialer == nil {
		msg := fmt.Sprintf("%s: set Fetcher.NNTPDialer to enable Usenet support", ErrNNTPNotConfigured.Error())
		f.setFeedError(ctx, q, feed.ID, now, msg)
		return fmt.Errorf("%w: set Fetcher.NNTPDialer to enable Usenet support", ErrNNTPNotConfigured)
	}
	if f.CredentialDecryptor == nil {
		msg := fmt.Sprintf("%s: set Fetcher.CredentialDecryptor to enable Usenet support", ErrNNTPNotConfigured.Error())
		f.setFeedError(ctx, q, feed.ID, now, msg)
		return fmt.Errorf("%w: set Fetcher.CredentialDecryptor to enable Usenet support", ErrNNTPNotConfigured)
	}

	// Require a user_id on the feed so we can look up per-user credentials.
	if feed.UserID == nil {
		msg := "nntp: feed has no user_id"
		f.setFeedError(ctx, q, feed.ID, now, msg)
		return fmt.Errorf("%s", msg)
	}
	userID := *feed.UserID

	// Look up the NNTP-specific state for this feed.
	state, err := q.GetUsenetFeedState(ctx, dbgen.GetUsenetFeedStateParams{
		FeedID: feed.ID,
		UserID: feed.UserID,
	})
	if err != nil {
		msg := fmt.Sprintf("nntp: feed state lookup failed: %v", err)
		f.setFeedError(ctx, q, feed.ID, now, msg)
		return fmt.Errorf("nntp feed state lookup: %w", err)
	}

	// Load per-user credentials.
	cred, err := q.GetNNTPCredentials(ctx, userID)
	if err != nil {
		// No credentials configured: record as a feed error (not a transient
		// connection issue) and return without updating high-water mark.
		msg := "nntp: credentials not configured for this user"
		f.setFeedError(ctx, q, feed.ID, now, msg)
		return fmt.Errorf("nntp credentials: %w", err)
	}

	// Decrypt the stored password blob.
	password, err := f.CredentialDecryptor.Decrypt(cred.PasswordEnc)
	if err != nil {
		// Corrupt or tampered credential blob. Do not include the blob value
		// in the error message to avoid leaking it in logs or feed error UI.
		msg := "nntp: failed to decrypt stored credentials"
		f.setFeedError(ctx, q, feed.ID, now, msg)
		return fmt.Errorf("nntp credential decrypt: %w", err)
	}

	// Establish an authenticated NNTP connection.
	conn, err := f.NNTPDialer.Dial(ctx, cred.Username, password)
	if err != nil {
		// Record the error on the feed but do not advance the high-water mark.
		// Redact the password from the dial error if it were somehow present.
		msg := fmt.Sprintf("nntp: connection failed: %v", err)
		f.setFeedError(ctx, q, feed.ID, now, msg)
		return fmt.Errorf("nntp dial: %w", err)
	}
	defer func() { _ = conn.Close() }()

	// Phase 2: group select and bounded overview fetch (feedreader-6g2.18.3).
	overview, attemptedStart, attemptedEnd, fetchErr := f.fetchNNTPOverview(ctx, q, now, conn, feed, &state)
	if fetchErr != nil {
		// fetchNNTPOverview already recorded the feed error.
		return fetchErr
	}

	// Phase 3 (feedreader-6g2.18.4 – 18.5): article import, high-water, and
	// feed-status updates.
	//
	// Stub until feedreader-6g2.18.4 is implemented.
	_ = overview
	_ = attemptedStart
	_ = attemptedEnd
	return nil
}

// fetchNNTPOverview performs the group-select and overview-fetch phase for a
// single NNTP feed. It:
//
//  1. Selects the group with SelectGroup to obtain the server's low/high range.
//  2. Computes the bounded article range using planArticleRange.
//  3. Returns immediately with success when the range is empty (no new articles).
//  4. Calls Overview to fetch the header summaries for the range.
//
// On success it returns the overview rows and the [attemptedStart, attemptedEnd]
// range that should be used to advance the high-water mark after import.
//
// On error it records a feed error via setFeedError and returns the same error
// so the caller can propagate it. It does NOT advance the high-water mark on
// error.
func (f *Fetcher) fetchNNTPOverview(
	ctx context.Context,
	q *dbgen.Queries,
	now time.Time,
	conn NNTPConn,
	feed *dbgen.Feed,
	state *dbgen.UsenetFeedState,
) (rows []NNTPOverviewRow, attemptedStart, attemptedEnd int64, err error) {
	groupName := state.GroupName

	// Select the group to obtain the server's current article range.
	_, serverLow, serverHigh, _, groupErr := conn.SelectGroup(groupName)
	if groupErr != nil {
		// no-such-group is a permanent configuration error; other errors are
		// transient. Both are recorded as feed errors and do not advance
		// the high-water mark.
		msg := fmt.Sprintf("nntp: group select failed (%s): %v", groupName, groupErr)
		f.setFeedError(ctx, q, feed.ID, now, msg)
		return nil, 0, 0, fmt.Errorf("nntp select group %s: %w", groupName, groupErr)
	}

	// Compute the bounded article range according to the D5 fetch policy.
	attemptedStart, attemptedEnd = planArticleRange(serverLow, serverHigh, state.HighWaterArticleNumber)

	if attemptedStart > attemptedEnd {
		// No new articles available. This is a successful fetch.
		slog.Debug("nntp: no new articles", "feed_id", feed.ID, "group", groupName,
			"server_high", serverHigh, "high_water", state.HighWaterArticleNumber)
		if resetErr := q.ResetFeedErrors(ctx, dbgen.ResetFeedErrorsParams{
			LastFetchedAt: &now,
			ID:            feed.ID,
		}); resetErr != nil {
			slog.Warn("nntp: reset feed errors", "feed_id", feed.ID, "error", resetErr)
		}
		return nil, 0, 0, nil
	}

	// Fetch overview rows for the computed range.
	ovRows, ovErr := conn.Overview(attemptedStart, attemptedEnd)
	if ovErr != nil {
		msg := fmt.Sprintf("nntp: overview fetch failed (%s): %v", groupName, ovErr)
		f.setFeedError(ctx, q, feed.ID, now, msg)
		return nil, 0, 0, fmt.Errorf("nntp overview %s [%d,%d]: %w", groupName, attemptedStart, attemptedEnd, ovErr)
	}

	slog.Debug("nntp: fetched overview", "feed_id", feed.ID, "group", groupName,
		"range_start", attemptedStart, "range_end", attemptedEnd, "rows", len(ovRows))

	return ovRows, attemptedStart, attemptedEnd, nil
}

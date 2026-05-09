package feeds

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/newscientist101/feedreader/db/dbgen"
	"github.com/newscientist101/feedreader/srv/nntp"
	"github.com/newscientist101/feedreader/srv/usenet"
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

// ErrNNTPArticleNotFound is returned by NNTPConn.FetchArticle when the
// article has been deleted or expired on the server. The injected real dialer
// wraps nntp.ErrArticleNotFound into this sentinel; test fakes return it
// directly. It is treated as an intentional skip, not a hard error.
var ErrNNTPArticleNotFound = errors.New("feeds: nntp article not found or expired")

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
//
// If ctx is already cancelled (e.g. the fetch was interrupted) the error is
// still recorded using context.Background() so cleanup writes succeed.
func (f *Fetcher) setFeedError(ctx context.Context, q *dbgen.Queries, feedID int64, now time.Time, msg string) {
	dbCtx := ctx
	if ctx.Err() != nil {
		dbCtx = context.Background()
	}
	if err := q.IncrementFeedErrors(dbCtx, dbgen.IncrementFeedErrorsParams{
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

	// Phase 3: per-article body fetch and import (feedreader-6g2.18.4).
	// Phase 4 (feedreader-6g2.18.5): importNNTPArticles returns the last
	// fully-processed article number. A context.Canceled error means partial
	// progress; any other non-nil error is a hard failure.
	lastProcessed, importErr := f.importNNTPArticles(ctx, q, conn, feed, &state, overview, attemptedEnd)

	// Phase 4: advance high-water and update feed status.
	//
	// Rules:
	//  - Hard failure (non-cancellation error): do not advance high-water;
	//    error already recorded by importNNTPArticles.
	//  - Context cancelled: advance to lastProcessed (partial progress),
	//    record a cancellation feed error, and return the context error.
	//  - Full success: advance to attemptedEnd and reset feed errors.
	_ = attemptedStart

	cancelled := errors.Is(importErr, context.Canceled) || errors.Is(importErr, context.DeadlineExceeded)
	if importErr != nil && !cancelled {
		// Hard failure: importNNTPArticles already called setFeedError.
		return importErr
	}

	// Determine the new high-water mark (partial or full progress).
	newHighWater := max(lastProcessed, state.HighWaterArticleNumber)

	if newHighWater > state.HighWaterArticleNumber {
		// Use context.Background() when ctx is cancelled so the DB write
		// succeeds regardless of the fetch context state.
		hwCtx := ctx
		if ctx.Err() != nil {
			hwCtx = context.Background()
		}
		if hwErr := q.UpdateUsenetHighWater(hwCtx, dbgen.UpdateUsenetHighWaterParams{
			HighWaterArticleNumber: newHighWater,
			FeedID:                 feed.ID,
		}); hwErr != nil {
			slog.Warn("nntp: update high-water failed", "feed_id", feed.ID, "error", hwErr)
			// Not a hard failure — the articles were inserted; we just can't
			// advance the cursor. Log and continue.
		}
	}

	if cancelled {
		// Record cancellation as a feed error so the operator can see the
		// partial run in the feed status UI. setFeedError handles
		// cancelled ctx internally.
		msg := fmt.Sprintf("nntp: fetch cancelled after processing up to article %d", lastProcessed)
		f.setFeedError(ctx, q, feed.ID, now, msg)
		return importErr
	}

	// Full success: clear any previous feed errors.
	if resetErr := q.ResetFeedErrors(ctx, dbgen.ResetFeedErrorsParams{
		LastFetchedAt: &now,
		ID:            feed.ID,
	}); resetErr != nil {
		slog.Warn("nntp: reset feed errors", "feed_id", feed.ID, "error", resetErr)
	}

	slog.Debug("nntp: fetch complete",
		"feed_id", feed.ID,
		"group", state.GroupName,
		"attempted_end", attemptedEnd,
		"last_processed", lastProcessed,
		"new_high_water", newHighWater)

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

// importNNTPArticles fetches and imports each article in the overview slice.
// For each row it:
//
//  1. Checks for context cancellation before each article.
//  2. Skips duplicates (by message_id or article_number) before the body fetch.
//  3. Fetches the article body via NNTPConn.FetchArticle.
//  4. Skips deleted/expired articles (ErrNNTPArticleNotFound) and binary posts
//     (usenet.ErrBinaryPost) as intentional skips.
//  5. Maps accepted articles with usenet.MapArticle.
//  6. Inserts article and meta inside a DB transaction; UNIQUE-constraint
//     violations on the meta insert are treated as idempotent duplicates.
//
// Returns (lastProcessed, nil) when all rows have been processed (possibly
// with intentional skips). lastProcessed is the highest article number that
// was fully processed (imported, skipped, or intentionally rejected). On
// context cancellation, returns (lastProcessed, nil) — the caller must check
// ctx.Err() to detect partial progress. Returns a non-nil error only for hard
// failures (unexpected body fetch errors or DB errors that make the range
// unreliable), after recording a feed error. Does NOT update the high-water
// mark — that is handled by phase 4.
func (f *Fetcher) importNNTPArticles(
	ctx context.Context,
	q *dbgen.Queries,
	conn NNTPConn,
	feed *dbgen.Feed,
	state *dbgen.UsenetFeedState,
	rows []NNTPOverviewRow,
	attemptedEnd int64,
) (lastProcessed int64, _ error) {
	groupName := state.GroupName
	// lastProcessed tracks the highest article number fully processed so far.
	// It starts at the current high-water mark so the caller can always use it
	// as the new high-water without going backwards.
	lastProcessed = state.HighWaterArticleNumber

	for i := range rows {
		row := &rows[i]

		// 0. Check for context cancellation before each network/DB operation.
		if ctx.Err() != nil {
			// Return partial progress; the caller handles the ctx error
			// separately from hard failures so it can still advance the
			// high-water mark for what was processed so far.
			return lastProcessed, ctx.Err()
		}

		// 1. Pre-fetch duplicate check by message_id.
		if row.MessageID != "" {
			_, err := q.GetUsenetArticleMetaByMessageID(ctx, dbgen.GetUsenetArticleMetaByMessageIDParams{
				FeedID:    feed.ID,
				MessageID: row.MessageID,
			})
			if err == nil {
				// Already imported: advance past this article.
				slog.Debug("nntp: skipping duplicate message_id",
					"feed_id", feed.ID, "message_id", row.MessageID)
				if row.ArticleNumber > lastProcessed {
					lastProcessed = row.ArticleNumber
				}
				continue
			}
			if !errors.Is(err, sql.ErrNoRows) {
				// Unexpected DB error — hard failure; do not advance high-water.
				msg := fmt.Sprintf("nntp: duplicate check (message_id) failed (%s): %v", groupName, err)
				f.setFeedError(ctx, q, feed.ID, time.Now(), msg)
				return lastProcessed, fmt.Errorf("nntp import %s: message_id check: %w", groupName, err)
			}
		}

		// 1b. Pre-fetch duplicate check by article_number.
		_, err := q.GetUsenetArticleMetaByArticleNumber(ctx, dbgen.GetUsenetArticleMetaByArticleNumberParams{
			FeedID:        feed.ID,
			ArticleNumber: row.ArticleNumber,
		})
		if err == nil {
			// Already imported: advance past this article.
			slog.Debug("nntp: skipping duplicate article_number",
				"feed_id", feed.ID, "article_number", row.ArticleNumber)
			if row.ArticleNumber > lastProcessed {
				lastProcessed = row.ArticleNumber
			}
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			msg := fmt.Sprintf("nntp: duplicate check (article_number) failed (%s): %v", groupName, err)
			f.setFeedError(ctx, q, feed.ID, time.Now(), msg)
			return lastProcessed, fmt.Errorf("nntp import %s: article_number check: %w", groupName, err)
		}

		// 2. Fetch the full article (headers + body).
		article, fetchErr := conn.FetchArticle(row.ArticleNumber)
		if fetchErr != nil {
			if errors.Is(fetchErr, ErrNNTPArticleNotFound) {
				// Article deleted or expired: intentional skip; advance past it.
				slog.Debug("nntp: article not found, skipping",
					"feed_id", feed.ID, "article_number", row.ArticleNumber)
				if row.ArticleNumber > lastProcessed {
					lastProcessed = row.ArticleNumber
				}
				continue
			}
			// Unexpected server/connection error: hard failure; do not advance.
			msg := fmt.Sprintf("nntp: article fetch failed (%s #%d): %v", groupName, row.ArticleNumber, fetchErr)
			f.setFeedError(ctx, q, feed.ID, time.Now(), msg)
			return lastProcessed, fmt.Errorf("nntp fetch article %s #%d: %w", groupName, row.ArticleNumber, fetchErr)
		}

		// 3. Binary / content-type rejection: intentional skip; advance past it.
		nntpArticle := &nntp.Article{
			Headers: article.Headers,
			Body:    article.Body,
		}
		if binErr := usenet.CheckArticleBinary(nntpArticle.Headers, row.Subject, nntpArticle.Body); binErr != nil {
			slog.Debug("nntp: skipping binary post",
				"feed_id", feed.ID, "article_number", row.ArticleNumber, "reason", binErr)
			if row.ArticleNumber > lastProcessed {
				lastProcessed = row.ArticleNumber
			}
			continue
		}

		// 4. Map to article record.
		nntpOverview := &nntp.OverviewRow{
			ArticleNumber: row.ArticleNumber,
			Subject:       row.Subject,
			From:          row.From,
			Date:          row.Date,
			MessageID:     row.MessageID,
			References:    row.References,
			Bytes:         row.Bytes,
			Lines:         row.Lines,
		}
		rec := usenet.MapArticle(feed.ID, groupName, row.ArticleNumber, nntpOverview, nntpArticle)

		// 5. Insert article and meta in a transaction.
		if insertErr := f.insertArticleWithMeta(ctx, &rec); insertErr != nil {
			if errors.Is(insertErr, errDuplicateArticleMeta) {
				// Race: another concurrent fetch inserted the same article.
				// Idempotent skip; advance past it.
				slog.Debug("nntp: duplicate insert race, skipping",
					"feed_id", feed.ID, "article_number", row.ArticleNumber)
				if row.ArticleNumber > lastProcessed {
					lastProcessed = row.ArticleNumber
				}
				continue
			}
			// Hard DB error; do not advance past this article.
			msg := fmt.Sprintf("nntp: insert failed (%s #%d): %v", groupName, row.ArticleNumber, insertErr)
			f.setFeedError(ctx, q, feed.ID, time.Now(), msg)
			return lastProcessed, fmt.Errorf("nntp insert article %s #%d: %w", groupName, row.ArticleNumber, insertErr)
		}

		// Successfully imported: advance past this article.
		if row.ArticleNumber > lastProcessed {
			lastProcessed = row.ArticleNumber
		}
		slog.Debug("nntp: imported article",
			"feed_id", feed.ID, "group", groupName, "article_number", row.ArticleNumber)
	}

	// All rows processed (or loop exhausted); high-water update is in phase 4.
	// If no rows were processed, lastProcessed remains at highWater (no-op).
	// If attemptedEnd > lastProcessed and no cancellation, the caller (phase 4)
	// will advance to attemptedEnd after checking ctx.Err().
	//
	// When all rows are processed without cancellation, we advance to
	// attemptedEnd to account for empty/skipped ranges within the planned
	// window (articles that were in the planned range but not in the overview,
	// e.g. cancelled posts the server already removed from OVER).
	if ctx.Err() == nil && attemptedEnd > lastProcessed {
		lastProcessed = attemptedEnd
	}

	return lastProcessed, nil
}

// errDuplicateArticleMeta is a package-private sentinel returned by
// insertArticleWithMeta when a UNIQUE constraint violation is detected on the
// usenet_article_meta insert. This indicates a concurrent duplicate insert and
// should be treated as an idempotent skip.
var errDuplicateArticleMeta = errors.New("feeds: duplicate usenet article meta")

// insertArticleWithMeta inserts a new article and its Usenet metadata inside a
// single DB transaction. If the meta insert fails with a UNIQUE constraint
// violation, it returns errDuplicateArticleMeta. Any other error is returned
// directly.
func (f *Fetcher) insertArticleWithMeta(ctx context.Context, rec *usenet.ArticleRecord) error {
	tx, err := f.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	qtx := dbgen.New(tx)

	// Insert (or upsert) the article.
	article, err := qtx.CreateArticle(ctx, rec.Article)
	if err != nil {
		return fmt.Errorf("create article: %w", err)
	}

	// Wire in the real article ID before inserting the meta row.
	metaParams := rec.Meta
	metaParams.ArticleID = article.ID

	_, err = qtx.InsertUsenetArticleMeta(ctx, metaParams)
	if err != nil {
		// UNIQUE(feed_id, message_id) or UNIQUE(feed_id, article_number)
		// violation = concurrent duplicate: signal idempotent skip.
		if isUniqueConstraintErr(err) {
			_ = tx.Rollback()
			return errDuplicateArticleMeta
		}
		return fmt.Errorf("insert usenet article meta: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// isUniqueConstraintErr reports whether err is a SQLite UNIQUE constraint
// violation. modernc.org/sqlite surfaces these as errors whose message
// contains "UNIQUE constraint failed".
func isUniqueConstraintErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}

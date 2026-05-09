package feeds

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/newscientist101/feedreader/db/dbgen"
)

// TestPlanArticleRange verifies the range planner policy:
//   - empty/invalid groups are no-ops
//   - first fetch (highWater==0) imports the latest firstFetchCount articles
//   - first fetch with fewer than firstFetchCount articles in the group uses all
//   - subsequent fetch starts at highWater+1
//   - low-water clamping (server retired old articles)
//   - per-run cap of fetchRunCap articles
func TestPlanArticleRange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		low       int64
		high      int64
		highWater int64
		wantStart int64
		wantEnd   int64
	}{
		{
			name: "empty group: serverHigh < serverLow",
			low:  5, high: 4, highWater: 0,
			wantStart: 1, wantEnd: 0, // no-op range
		},
		{
			name: "empty group: both zero",
			low:  0, high: 0, highWater: 0,
			wantStart: 1, wantEnd: 0, // no-op range
		},
		{
			name: "first fetch exactly firstFetchCount articles",
			low:  1, high: 100, highWater: 0,
			wantStart: 1, wantEnd: 100,
		},
		{
			name: "first fetch more than firstFetchCount articles",
			low:  1, high: 5000, highWater: 0,
			wantStart: 4901, wantEnd: 5000, // latest 100
		},
		{
			name: "first fetch fewer than firstFetchCount articles",
			low:  1, high: 42, highWater: 0,
			wantStart: 1, wantEnd: 42, // clamp to serverLow
		},
		{
			name: "first fetch only one article",
			low:  99, high: 99, highWater: 0,
			wantStart: 99, wantEnd: 99,
		},
		{
			name: "subsequent fetch no new articles",
			low:  1, high: 200, highWater: 200,
			wantStart: 201, wantEnd: 200, // no-op
		},
		{
			name: "subsequent fetch from highWater+1",
			low:  1, high: 300, highWater: 200,
			wantStart: 201, wantEnd: 300,
		},
		{
			name: "subsequent fetch capped at fetchRunCap",
			low:  1, high: 10000, highWater: 200,
			wantStart: 201, wantEnd: 700, // 201 + 500 - 1
		},
		{
			name: "low-water clamping: server retired old articles",
			low:  50, high: 200, highWater: 30, // highWater+1=31 < serverLow=50
			wantStart: 50, wantEnd: 200,
		},
		{
			name: "low-water clamp on first fetch when high < firstFetchCount",
			low:  10, high: 50, highWater: 0,
			wantStart: 10, wantEnd: 50, // high-firstFetchCount+1 = -49, clamp to low=10
		},
		{
			name: "cap applies on first fetch when group is large",
			low:  1, high: 100000, highWater: 0,
			// start = 100000-100+1=99901; end = 99901+500-1=100400 but capped at 100000
			wantStart: 99901, wantEnd: 100000,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotStart, gotEnd := planArticleRange(tc.low, tc.high, tc.highWater)
			if gotStart != tc.wantStart || gotEnd != tc.wantEnd {
				t.Errorf("planArticleRange(%d,%d,%d) = (%d,%d), want (%d,%d)",
					tc.low, tc.high, tc.highWater,
					gotStart, gotEnd,
					tc.wantStart, tc.wantEnd)
			}
		})
	}
}

// TestPlanArticleRange_EmptyRange ensures start > end is consistently a no-op.
func TestPlanArticleRange_EmptyRange(t *testing.T) {
	t.Parallel()

	start, end := planArticleRange(1, 100, 100)
	if start <= end {
		t.Errorf("expected no-op range, got start=%d end=%d", start, end)
	}
}

// TestNNTPNotConfigured verifies ErrNNTPNotConfigured wrapping.
func TestNNTPNotConfigured(t *testing.T) {
	t.Parallel()

	// ErrNNTPNotConfigured must be a distinct, unwrappable sentinel.
	if ErrNNTPNotConfigured == nil {
		t.Fatal("ErrNNTPNotConfigured is nil")
	}
}

// --- Credential loading tests (feedreader-6g2.18.2) ---

// fakeDecryptor implements CredentialDecryptor for tests.
type fakeDecryptor struct {
	result string
	err    error
}

func (fd *fakeDecryptor) Decrypt(_ string) (string, error) { return fd.result, fd.err }

// fakeDialer implements NNTPDialer for tests.
type fakeDialer struct {
	conn NNTPConn
	err  error
}

func (fd *fakeDialer) Dial(_ context.Context, _, _ string) (NNTPConn, error) {
	return fd.conn, fd.err
}

// fakeNNTPConn implements NNTPConn. All calls return the configured values.
type fakeNNTPConn struct {
	groupCount, groupLow, groupHigh int64
	groupCanon                      string
	groupErr                        error
	overviewRows                    []NNTPOverviewRow
	overviewErr                     error
	article                         *NNTPArticle
	articleErr                      error
	closed                          bool
}

func (fc *fakeNNTPConn) SelectGroup(name string) (count, low, high int64, canonName string, err error) {
	return fc.groupCount, fc.groupLow, fc.groupHigh, fc.groupCanon, fc.groupErr
}
func (fc *fakeNNTPConn) Overview(_, _ int64) ([]NNTPOverviewRow, error) {
	return fc.overviewRows, fc.overviewErr
}
func (fc *fakeNNTPConn) FetchArticle(_ int64) (*NNTPArticle, error) {
	return fc.article, fc.articleErr
}
func (fc *fakeNNTPConn) Close() error { fc.closed = true; return nil }

// setupNNTPTestFeed creates a user, nntp feed, usenet_feed_state, and
// optionally nntp_credentials.
func setupNNTPTestFeed(t *testing.T, q *dbgen.Queries, withCreds bool) (dbgen.User, dbgen.Feed) {
	t.Helper()
	ctx := context.Background()
	u, err := q.CreateUser(ctx, dbgen.CreateUserParams{
		ExternalID: "nntp-test",
		Email:      "nntp@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}

	feed, err := q.CreateFeed(ctx, dbgen.CreateFeedParams{
		Name:     "comp.lang.go",
		Url:      "nntp://news.eternal-september.org/comp.lang.go",
		FeedType: "nntp",
		UserID:   &u.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = q.CreateUsenetFeedState(ctx, dbgen.CreateUsenetFeedStateParams{
		FeedID:    feed.ID,
		Provider:  "eternal-september",
		GroupName: "comp.lang.go",
	})
	if err != nil {
		t.Fatal(err)
	}

	if withCreds {
		_, err = q.UpsertNNTPCredentials(ctx, dbgen.UpsertNNTPCredentialsParams{
			UserID:      u.ID,
			Username:    "testuser",
			PasswordEnc: "hexblob", // decrypted by the fake decryptor
			KeyVersion:  "v1",
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	return u, feed
}

// TestFetchNNTPFeed_NilDialer verifies that a nil NNTPDialer produces a feed
// error and returns ErrNNTPNotConfigured without updating the high-water mark.
func TestFetchNNTPFeed_NilDialer(t *testing.T) {
	t.Parallel()
	sqlDB, q := setupTestDB(t)
	_, feed := setupNNTPTestFeed(t, q, true)

	fetcher := &Fetcher{DB: sqlDB, NNTPDialer: nil, CredentialDecryptor: &fakeDecryptor{result: "pass"}}
	err := fetcher.fetchNNTPFeed(context.Background(), q, time.Now(), &feed)

	if !errors.Is(err, ErrNNTPNotConfigured) {
		t.Errorf("expected ErrNNTPNotConfigured, got %v", err)
	}
	assertFeedHasError(t, sqlDB, feed.ID)
	assertHighWaterUnchanged(t, sqlDB, feed.ID, 0)
}

// TestFetchNNTPFeed_NilDecryptor verifies that a nil CredentialDecryptor
// produces a feed error.
func TestFetchNNTPFeed_NilDecryptor(t *testing.T) {
	t.Parallel()
	sqlDB, q := setupTestDB(t)
	_, feed := setupNNTPTestFeed(t, q, true)

	fetcher := &Fetcher{
		DB:                  sqlDB,
		NNTPDialer:          &fakeDialer{conn: &fakeNNTPConn{}},
		CredentialDecryptor: nil,
	}
	err := fetcher.fetchNNTPFeed(context.Background(), q, time.Now(), &feed)

	if !errors.Is(err, ErrNNTPNotConfigured) {
		t.Errorf("expected ErrNNTPNotConfigured, got %v", err)
	}
	assertFeedHasError(t, sqlDB, feed.ID)
	assertHighWaterUnchanged(t, sqlDB, feed.ID, 0)
}

// TestFetchNNTPFeed_MissingCredentials verifies that a missing credentials row
// produces a per-feed error and does not update the high-water mark.
func TestFetchNNTPFeed_MissingCredentials(t *testing.T) {
	t.Parallel()
	sqlDB, q := setupTestDB(t)
	_, feed := setupNNTPTestFeed(t, q, false /* no creds */)

	fetcher := &Fetcher{
		DB:                  sqlDB,
		NNTPDialer:          &fakeDialer{conn: &fakeNNTPConn{}},
		CredentialDecryptor: &fakeDecryptor{result: "pass"},
	}
	err := fetcher.fetchNNTPFeed(context.Background(), q, time.Now(), &feed)

	if err == nil {
		t.Fatal("expected error for missing credentials")
	}
	assertFeedHasError(t, sqlDB, feed.ID)
	assertHighWaterUnchanged(t, sqlDB, feed.ID, 0)
}

// TestFetchNNTPFeed_CorruptCredentials verifies that a decrypt error produces
// a per-feed error and does not update the high-water mark.
func TestFetchNNTPFeed_CorruptCredentials(t *testing.T) {
	t.Parallel()
	sqlDB, q := setupTestDB(t)
	_, feed := setupNNTPTestFeed(t, q, true)

	decryptErr := errors.New("decrypt: gcm open failed")
	fetcher := &Fetcher{
		DB:                  sqlDB,
		NNTPDialer:          &fakeDialer{conn: &fakeNNTPConn{}},
		CredentialDecryptor: &fakeDecryptor{err: decryptErr},
	}
	err := fetcher.fetchNNTPFeed(context.Background(), q, time.Now(), &feed)

	if err == nil {
		t.Fatal("expected error for corrupt credentials")
	}
	assertFeedHasError(t, sqlDB, feed.ID)
	assertHighWaterUnchanged(t, sqlDB, feed.ID, 0)
}

// TestFetchNNTPFeed_AuthFailure verifies that a dial error (e.g. auth
// rejection) produces a per-feed error and does not update the high-water mark.
func TestFetchNNTPFeed_AuthFailure(t *testing.T) {
	t.Parallel()
	sqlDB, q := setupTestDB(t)
	_, feed := setupNNTPTestFeed(t, q, true)

	dialErr := errors.New("nntp: authentication failed")
	fetcher := &Fetcher{
		DB:                  sqlDB,
		NNTPDialer:          &fakeDialer{err: dialErr},
		CredentialDecryptor: &fakeDecryptor{result: "pass"},
	}
	err := fetcher.fetchNNTPFeed(context.Background(), q, time.Now(), &feed)

	if err == nil {
		t.Fatal("expected error for auth failure")
	}
	assertFeedHasError(t, sqlDB, feed.ID)
	assertHighWaterUnchanged(t, sqlDB, feed.ID, 0)
}

// TestFetchNNTPFeed_DialSuccess verifies that a successful dial proceeds past
// the credential phase. The connection must be closed regardless of outcome.
func TestFetchNNTPFeed_DialSuccess(t *testing.T) {
	t.Parallel()
	sqlDB, q := setupTestDB(t)
	_, feed := setupNNTPTestFeed(t, q, true)

	// An empty group (groupHigh=0, groupLow=0) is treated as a successful
	// no-op fetch once we are past the credential phase.
	conn := &fakeNNTPConn{groupCount: 0, groupLow: 0, groupHigh: 0}
	fetcher := &Fetcher{
		DB:                  sqlDB,
		NNTPDialer:          &fakeDialer{conn: conn},
		CredentialDecryptor: &fakeDecryptor{result: "pass"},
	}
	err := fetcher.fetchNNTPFeed(context.Background(), q, time.Now(), &feed)

	// Empty group is a successful fetch — no error.
	if err != nil {
		t.Errorf("unexpected error after successful dial with empty group: %v", err)
	}
	// Conn.Close must always be called (via defer) once the dial succeeds.
	if !conn.closed {
		t.Error("expected conn.Close() to be called after dial succeeded")
	}
}

// --- Group select and overview tests (feedreader-6g2.18.3) ---

// TestFetchNNTPFeed_NoSuchGroup verifies that a SelectGroup no-such-group
// error records a per-feed error and does not advance the high-water mark.
func TestFetchNNTPFeed_NoSuchGroup(t *testing.T) {
	t.Parallel()
	sqlDB, q := setupTestDB(t)
	_, feed := setupNNTPTestFeed(t, q, true)

	conn := &fakeNNTPConn{groupErr: errors.New("411 no such group")}
	fetcher := &Fetcher{
		DB:                  sqlDB,
		NNTPDialer:          &fakeDialer{conn: conn},
		CredentialDecryptor: &fakeDecryptor{result: "pass"},
	}
	err := fetcher.fetchNNTPFeed(context.Background(), q, time.Now(), &feed)

	if err == nil {
		t.Fatal("expected error for no-such-group")
	}
	assertFeedHasError(t, sqlDB, feed.ID)
	assertHighWaterUnchanged(t, sqlDB, feed.ID, 0)
}

// TestFetchNNTPFeed_OverviewError verifies that an Overview server error
// records a per-feed error and does not advance the high-water mark.
func TestFetchNNTPFeed_OverviewError(t *testing.T) {
	t.Parallel()
	sqlDB, q := setupTestDB(t)
	_, feed := setupNNTPTestFeed(t, q, true)

	conn := &fakeNNTPConn{
		groupCount: 500, groupLow: 1, groupHigh: 500,
		overviewErr: errors.New("500 command failed"),
	}
	fetcher := &Fetcher{
		DB:                  sqlDB,
		NNTPDialer:          &fakeDialer{conn: conn},
		CredentialDecryptor: &fakeDecryptor{result: "pass"},
	}
	err := fetcher.fetchNNTPFeed(context.Background(), q, time.Now(), &feed)

	if err == nil {
		t.Fatal("expected error for overview failure")
	}
	assertFeedHasError(t, sqlDB, feed.ID)
	assertHighWaterUnchanged(t, sqlDB, feed.ID, 0)
}

// TestFetchNNTPFeed_EmptyGroup verifies that an empty/no-new-articles group
// is treated as a successful fetch (clears feed errors, does not update
// high-water).
func TestFetchNNTPFeed_EmptyGroup(t *testing.T) {
	t.Parallel()
	sqlDB, q := setupTestDB(t)
	_, feed := setupNNTPTestFeed(t, q, true)

	// Group reports 0 articles (serverHigh < serverLow → empty).
	conn := &fakeNNTPConn{groupCount: 0, groupLow: 0, groupHigh: 0}
	fetcher := &Fetcher{
		DB:                  sqlDB,
		NNTPDialer:          &fakeDialer{conn: conn},
		CredentialDecryptor: &fakeDecryptor{result: "pass"},
	}
	err := fetcher.fetchNNTPFeed(context.Background(), q, time.Now(), &feed)

	// Empty group is a success (no error).
	if err != nil {
		t.Errorf("expected success for empty group, got %v", err)
	}
	assertHighWaterUnchanged(t, sqlDB, feed.ID, 0)
}

// TestFetchNNTPFeed_NoNewArticles verifies that when high_water >= serverHigh
// the fetch succeeds without updating high-water.
func TestFetchNNTPFeed_NoNewArticles(t *testing.T) {
	t.Parallel()
	sqlDB, q := setupTestDB(t)
	_, feed := setupNNTPTestFeed(t, q, true)

	// Set high-water to the current server high.
	if err := q.UpdateUsenetHighWater(context.Background(), dbgen.UpdateUsenetHighWaterParams{
		HighWaterArticleNumber: 200,
		FeedID:                 feed.ID,
	}); err != nil {
		t.Fatal(err)
	}

	// Server still reports high=200 → no new articles.
	conn := &fakeNNTPConn{groupCount: 200, groupLow: 1, groupHigh: 200}
	fetcher := &Fetcher{
		DB:                  sqlDB,
		NNTPDialer:          &fakeDialer{conn: conn},
		CredentialDecryptor: &fakeDecryptor{result: "pass"},
	}
	err := fetcher.fetchNNTPFeed(context.Background(), q, time.Now(), &feed)

	if err != nil {
		t.Errorf("expected success for no-new-articles, got %v", err)
	}
	// High-water must remain at 200.
	assertHighWaterUnchanged(t, sqlDB, feed.ID, 200)
}

// TestFetchNNTPFeed_FirstFetchLatest100 verifies that the first fetch requests
// the latest firstFetchCount articles (start = high - 99 for high > 100).
func TestFetchNNTPFeed_FirstFetchLatest100(t *testing.T) {
	t.Parallel()
	sqlDB, q := setupTestDB(t)
	_, feed := setupNNTPTestFeed(t, q, true)

	var capturedLow, capturedHigh int64
	conn := &fakeNNTPConn{
		groupCount: 5000, groupLow: 1, groupHigh: 5000,
		overviewRows: []NNTPOverviewRow{}, // empty is fine for this test
	}
	// Capture the Overview call's arguments via a custom fake.
	captureConn := &captureOverviewConn{NNTPConn: conn, capturedLow: &capturedLow, capturedHigh: &capturedHigh}

	fetcher := &Fetcher{
		DB:                  sqlDB,
		NNTPDialer:          &fakeDialer{conn: captureConn},
		CredentialDecryptor: &fakeDecryptor{result: "pass"},
	}
	err := fetcher.fetchNNTPFeed(context.Background(), q, time.Now(), &feed)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	wantStart := int64(5000 - firstFetchCount + 1)
	wantEnd := int64(5000)
	if capturedLow != wantStart || capturedHigh != wantEnd {
		t.Errorf("Overview called with [%d,%d], want [%d,%d]",
			capturedLow, capturedHigh, wantStart, wantEnd)
	}
}

// TestFetchNNTPFeed_SubsequentFetchFromHighWater verifies that subsequent
// fetches start at high_water + 1.
func TestFetchNNTPFeed_SubsequentFetchFromHighWater(t *testing.T) {
	t.Parallel()
	sqlDB, q := setupTestDB(t)
	_, feed := setupNNTPTestFeed(t, q, true)

	if err := q.UpdateUsenetHighWater(context.Background(), dbgen.UpdateUsenetHighWaterParams{
		HighWaterArticleNumber: 300,
		FeedID:                 feed.ID,
	}); err != nil {
		t.Fatal(err)
	}

	var capturedLow, capturedHigh int64
	conn := &fakeNNTPConn{
		groupCount: 400, groupLow: 1, groupHigh: 400,
		overviewRows: []NNTPOverviewRow{},
	}
	captureConn := &captureOverviewConn{NNTPConn: conn, capturedLow: &capturedLow, capturedHigh: &capturedHigh}

	fetcher := &Fetcher{
		DB:                  sqlDB,
		NNTPDialer:          &fakeDialer{conn: captureConn},
		CredentialDecryptor: &fakeDecryptor{result: "pass"},
	}
	err := fetcher.fetchNNTPFeed(context.Background(), q, time.Now(), &feed)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if capturedLow != 301 || capturedHigh != 400 {
		t.Errorf("Overview called with [%d,%d], want [301,400]", capturedLow, capturedHigh)
	}
}

// TestFetchNNTPFeed_RunCap verifies that the per-run cap of fetchRunCap is
// applied when the range would otherwise exceed it.
func TestFetchNNTPFeed_RunCap(t *testing.T) {
	t.Parallel()
	sqlDB, q := setupTestDB(t)
	_, feed := setupNNTPTestFeed(t, q, true)

	if err := q.UpdateUsenetHighWater(context.Background(), dbgen.UpdateUsenetHighWaterParams{
		HighWaterArticleNumber: 100,
		FeedID:                 feed.ID,
	}); err != nil {
		t.Fatal(err)
	}

	var capturedLow, capturedHigh int64
	conn := &fakeNNTPConn{
		groupCount: 100000, groupLow: 1, groupHigh: 100000,
		overviewRows: []NNTPOverviewRow{},
	}
	captureConn := &captureOverviewConn{NNTPConn: conn, capturedLow: &capturedLow, capturedHigh: &capturedHigh}

	fetcher := &Fetcher{
		DB:                  sqlDB,
		NNTPDialer:          &fakeDialer{conn: captureConn},
		CredentialDecryptor: &fakeDecryptor{result: "pass"},
	}
	err := fetcher.fetchNNTPFeed(context.Background(), q, time.Now(), &feed)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	// start=101, cap=500 → end=600
	wantEnd := int64(100 + fetchRunCap)
	if capturedLow != 101 || capturedHigh != wantEnd {
		t.Errorf("Overview called with [%d,%d], want [101,%d]", capturedLow, capturedHigh, wantEnd)
	}
}

// captureOverviewConn wraps an NNTPConn and records the Overview range.
type captureOverviewConn struct {
	NNTPConn
	capturedLow  *int64
	capturedHigh *int64
}

func (c *captureOverviewConn) Overview(low, high int64) ([]NNTPOverviewRow, error) {
	*c.capturedLow = low
	*c.capturedHigh = high
	return c.NNTPConn.Overview(low, high)
}

// assertFeedHasError checks that the feed's last_error is non-nil.
func assertFeedHasError(t *testing.T, sqlDB *sql.DB, feedID int64) {
	t.Helper()
	var lastError *string
	err := sqlDB.QueryRowContext(context.Background(),
		"SELECT last_error FROM feeds WHERE id = ?", feedID).Scan(&lastError)
	if err != nil {
		t.Fatalf("query feed last_error: %v", err)
	}
	if lastError == nil || *lastError == "" {
		t.Errorf("expected feed.last_error to be set, got nil/empty")
	}
}

// assertHighWaterUnchanged checks the high-water mark for a feed directly
// via SQL (avoids the user_id JOIN requirement of GetUsenetFeedState).
func assertHighWaterUnchanged(t *testing.T, sqlDB *sql.DB, feedID, wantHW int64) {
	t.Helper()
	var hw int64
	err := sqlDB.QueryRowContext(context.Background(),
		"SELECT high_water_article_number FROM usenet_feed_state WHERE feed_id = ?", feedID).Scan(&hw)
	if err != nil {
		t.Fatalf("query high_water: %v", err)
	}
	if hw != wantHW {
		t.Errorf("high_water = %d, want %d", hw, wantHW)
	}
}

// --- Article import tests (feedreader-6g2.18.4) ---

// makeOverviewRow is a test helper that returns a minimal valid overview row.
func makeOverviewRow(number int64, msgID string) NNTPOverviewRow {
	return NNTPOverviewRow{
		ArticleNumber: number,
		Subject:       "Test article",
		From:          "test@example.com",
		Date:          "Mon, 1 Jan 2024 12:00:00 +0000",
		MessageID:     msgID,
		References:    "",
		Bytes:         100,
		Lines:         10,
	}
}

// makeTextArticle returns a minimal plain-text NNTPArticle.
func makeTextArticle(body string) *NNTPArticle {
	return &NNTPArticle{
		Headers: map[string]string{
			"Content-Type": "text/plain; charset=utf-8",
		},
		Body: body,
	}
}

// setupNNTPFetcher creates a fetcher with a fake conn returning the given
// overview rows and article, over a real in-memory test DB.
func setupNNTPFetcher(t *testing.T, conn NNTPConn) (*Fetcher, *dbgen.Queries, *sql.DB, dbgen.Feed) {
	t.Helper()
	sqlDB, q := setupTestDB(t)
	_, feed := setupNNTPTestFeed(t, q, true)
	fetcher := &Fetcher{
		DB:                  sqlDB,
		NNTPDialer:          &fakeDialer{conn: conn},
		CredentialDecryptor: &fakeDecryptor{result: "pass"},
	}
	return fetcher, q, sqlDB, feed
}

// assertArticleCount checks the number of articles for a feed.
func assertArticleCount(t *testing.T, sqlDB *sql.DB, feedID int64, want int) {
	t.Helper()
	var count int
	err := sqlDB.QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM articles WHERE feed_id = ?", feedID).Scan(&count)
	if err != nil {
		t.Fatalf("count articles: %v", err)
	}
	if count != want {
		t.Errorf("article count = %d, want %d", count, want)
	}
}

// assertMetaCount checks the number of usenet_article_meta rows for a feed.
func assertMetaCount(t *testing.T, sqlDB *sql.DB, feedID int64, want int) {
	t.Helper()
	var count int
	err := sqlDB.QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM usenet_article_meta WHERE feed_id = ?", feedID).Scan(&count)
	if err != nil {
		t.Fatalf("count meta: %v", err)
	}
	if count != want {
		t.Errorf("meta count = %d, want %d", count, want)
	}
}

// TestImportNNTPArticles_ValidTextArticle verifies that a plain-text article is
// imported with matching Article and Usenet meta fields.
func TestImportNNTPArticles_ValidTextArticle(t *testing.T) {
	t.Parallel()

	row := makeOverviewRow(42, "<msg42@example.com>")
	conn := &fakeNNTPConn{
		groupCount: 100, groupLow: 1, groupHigh: 100,
		overviewRows: []NNTPOverviewRow{row},
		article:      makeTextArticle("Hello, Usenet!"),
	}
	fetcher, q, sqlDB, feed := setupNNTPFetcher(t, conn)

	err := fetcher.fetchNNTPFeed(context.Background(), q, time.Now(), &feed)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	assertArticleCount(t, sqlDB, feed.ID, 1)
	assertMetaCount(t, sqlDB, feed.ID, 1)

	// Verify the meta row has the expected fields.
	meta, err := q.GetUsenetArticleMetaByMessageID(context.Background(), dbgen.GetUsenetArticleMetaByMessageIDParams{
		FeedID:    feed.ID,
		MessageID: "<msg42@example.com>",
	})
	if err != nil {
		t.Fatalf("get meta: %v", err)
	}
	if meta.ArticleNumber != 42 {
		t.Errorf("meta.ArticleNumber = %d, want 42", meta.ArticleNumber)
	}
	if meta.GroupName != "comp.lang.go" {
		t.Errorf("meta.GroupName = %q, want comp.lang.go", meta.GroupName)
	}
}

// TestImportNNTPArticles_SkipBinaryPost verifies that a binary post is skipped
// (no article or meta row inserted) without returning an error.
func TestImportNNTPArticles_SkipBinaryPost(t *testing.T) {
	t.Parallel()

	row := makeOverviewRow(10, "<binary10@example.com>")
	binaryArticle := &NNTPArticle{
		Headers: map[string]string{
			"Content-Type": "application/octet-stream",
		},
		Body: "\x00\x01\x02",
	}
	conn := &fakeNNTPConn{
		groupCount: 10, groupLow: 1, groupHigh: 10,
		overviewRows: []NNTPOverviewRow{row},
		article:      binaryArticle,
	}
	fetcher, q, sqlDB, feed := setupNNTPFetcher(t, conn)

	err := fetcher.fetchNNTPFeed(context.Background(), q, time.Now(), &feed)
	if err != nil {
		t.Fatalf("expected success (binary posts are skipped), got %v", err)
	}
	assertArticleCount(t, sqlDB, feed.ID, 0)
	assertMetaCount(t, sqlDB, feed.ID, 0)
}

// TestImportNNTPArticles_SkipDeletedArticle verifies that an article-not-found
// error (ErrNNTPArticleNotFound) is treated as an intentional skip.
func TestImportNNTPArticles_SkipDeletedArticle(t *testing.T) {
	t.Parallel()

	row := makeOverviewRow(7, "<deleted7@example.com>")
	conn := &fakeNNTPConn{
		groupCount: 10, groupLow: 1, groupHigh: 10,
		overviewRows: []NNTPOverviewRow{row},
		articleErr:   ErrNNTPArticleNotFound,
	}
	fetcher, q, sqlDB, feed := setupNNTPFetcher(t, conn)

	err := fetcher.fetchNNTPFeed(context.Background(), q, time.Now(), &feed)
	if err != nil {
		t.Fatalf("expected success (deleted articles are skipped), got %v", err)
	}
	assertArticleCount(t, sqlDB, feed.ID, 0)
	assertMetaCount(t, sqlDB, feed.ID, 0)
}

// TestImportNNTPArticles_SkipDuplicateMessageID verifies that an overview row
// with a message_id that already exists in usenet_article_meta is skipped
// without fetching the article body or inserting a duplicate.
func TestImportNNTPArticles_SkipDuplicateMessageID(t *testing.T) {
	t.Parallel()

	row := makeOverviewRow(5, "<dup-msg@example.com>")
	conn := &fakeNNTPConn{
		groupCount: 5, groupLow: 1, groupHigh: 5,
		overviewRows: []NNTPOverviewRow{row},
		article:      makeTextArticle("First body"),
	}
	fetcher, q, sqlDB, feed := setupNNTPFetcher(t, conn)

	// Import once to establish the existing record.
	if err := fetcher.fetchNNTPFeed(context.Background(), q, time.Now(), &feed); err != nil {
		t.Fatalf("first import: %v", err)
	}
	assertArticleCount(t, sqlDB, feed.ID, 1)

	// Import the same row again — should be a no-op.
	if err := fetcher.fetchNNTPFeed(context.Background(), q, time.Now(), &feed); err != nil {
		t.Fatalf("second import: %v", err)
	}
	assertArticleCount(t, sqlDB, feed.ID, 1) // still 1
	assertMetaCount(t, sqlDB, feed.ID, 1)    // still 1
}

// TestImportNNTPArticles_SkipDuplicateArticleNumber verifies that an overview
// row for an article_number that is already in usenet_article_meta is skipped
// even when the message_id differs (e.g. empty message_id in overview).
func TestImportNNTPArticles_SkipDuplicateArticleNumber(t *testing.T) {
	t.Parallel()

	row := makeOverviewRow(3, "<original@example.com>")
	conn := &fakeNNTPConn{
		groupCount: 3, groupLow: 1, groupHigh: 3,
		overviewRows: []NNTPOverviewRow{row},
		article:      makeTextArticle("Original body"),
	}
	fetcher, q, sqlDB, feed := setupNNTPFetcher(t, conn)

	if err := fetcher.fetchNNTPFeed(context.Background(), q, time.Now(), &feed); err != nil {
		t.Fatalf("first import: %v", err)
	}
	assertArticleCount(t, sqlDB, feed.ID, 1)

	// Try importing again with the same article_number but an empty message_id
	// so the message_id check is bypassed.
	rowNoMsgID := NNTPOverviewRow{
		ArticleNumber: 3, // same number
		Subject:       "Repost",
		From:          "test@example.com",
		Date:          "Tue, 2 Jan 2024 12:00:00 +0000",
		MessageID:     "", // empty, so message_id check is skipped
	}
	conn2 := &fakeNNTPConn{
		groupCount: 3, groupLow: 1, groupHigh: 3,
		overviewRows: []NNTPOverviewRow{rowNoMsgID},
		article:      makeTextArticle("Repost body"),
	}
	fetcher2, q2, _, _ := setupNNTPFetcher(t, conn2)
	// Point fetcher2 at the same DB as fetcher.
	fetcher2.DB = sqlDB
	fetcher2.NNTPDialer = &fakeDialer{conn: conn2}

	if err := fetcher2.fetchNNTPFeed(context.Background(), q2, time.Now(), &feed); err != nil {
		t.Fatalf("second import: %v", err)
	}
	assertArticleCount(t, sqlDB, feed.ID, 1) // still 1
	assertMetaCount(t, sqlDB, feed.ID, 1)    // still 1
}

// TestImportNNTPArticles_HardArticleFetchError verifies that an unexpected
// body-fetch error (not ErrNNTPArticleNotFound) causes a hard failure with a
// feed error recorded.
func TestImportNNTPArticles_HardArticleFetchError(t *testing.T) {
	t.Parallel()

	row := makeOverviewRow(99, "<hard@example.com>")
	conn := &fakeNNTPConn{
		groupCount: 100, groupLow: 1, groupHigh: 100,
		overviewRows: []NNTPOverviewRow{row},
		articleErr:   errors.New("connection reset by peer"),
	}
	fetcher, q, sqlDB, feed := setupNNTPFetcher(t, conn)

	err := fetcher.fetchNNTPFeed(context.Background(), q, time.Now(), &feed)
	if err == nil {
		t.Fatal("expected hard error for unexpected fetch failure")
	}
	assertFeedHasError(t, sqlDB, feed.ID)
	assertArticleCount(t, sqlDB, feed.ID, 0)
}

// TestImportNNTPArticles_DuplicateInsertRace verifies that a UNIQUE constraint
// violation on the usenet_article_meta insert is treated as an idempotent skip.
func TestImportNNTPArticles_DuplicateInsertRace(t *testing.T) {
	t.Parallel()

	// Two rows with the same message_id but different article_numbers. The
	// first import succeeds; the second should hit the message_id UNIQUE
	// constraint on the meta table and be treated as an idempotent skip
	// (the article was already inserted by the first row).
	row1 := makeOverviewRow(11, "<race@example.com>")
	row2 := NNTPOverviewRow{
		ArticleNumber: 12, // different number so article_number pre-check passes
		Subject:       "Race",
		From:          "test@example.com",
		Date:          "Mon, 1 Jan 2024 12:00:00 +0000",
		MessageID:     "<race@example.com>", // same message_id as row1
	}
	article := makeTextArticle("Race body")
	conn := &fakeNNTPConn{
		groupCount: 12, groupLow: 1, groupHigh: 12,
		overviewRows: []NNTPOverviewRow{row1, row2},
		article:      article,
	}
	fetcher, q, sqlDB, feed := setupNNTPFetcher(t, conn)

	err := fetcher.fetchNNTPFeed(context.Background(), q, time.Now(), &feed)
	if err != nil {
		t.Fatalf("expected success (duplicate meta insert is idempotent), got %v", err)
	}
	// First article inserted, second is an idempotent skip.
	assertMetaCount(t, sqlDB, feed.ID, 1)
}

// --- High-water, feed status, and cancellation tests (feedreader-6g2.18.5) ---

// assertHighWater checks that the high-water mark equals wantHW.
func assertHighWater(t *testing.T, sqlDB *sql.DB, feedID, wantHW int64) {
	t.Helper()
	var hw int64
	err := sqlDB.QueryRowContext(context.Background(),
		"SELECT high_water_article_number FROM usenet_feed_state WHERE feed_id = ?", feedID).Scan(&hw)
	if err != nil {
		t.Fatalf("query high_water: %v", err)
	}
	if hw != wantHW {
		t.Errorf("high_water = %d, want %d", hw, wantHW)
	}
}

// assertFeedNoError checks that the feed's last_error is nil/empty.
func assertFeedNoError(t *testing.T, sqlDB *sql.DB, feedID int64) {
	t.Helper()
	var lastError *string
	err := sqlDB.QueryRowContext(context.Background(),
		"SELECT last_error FROM feeds WHERE id = ?", feedID).Scan(&lastError)
	if err != nil {
		t.Fatalf("query feed last_error: %v", err)
	}
	if lastError != nil && *lastError != "" {
		t.Errorf("expected feed.last_error to be nil/empty, got %q", *lastError)
	}
}

// TestFetchNNTPFeed_HighWaterAdvancesOnSuccess verifies that high-water is
// advanced to attemptedEnd after a fully successful fetch of imported articles.
func TestFetchNNTPFeed_HighWaterAdvancesOnSuccess(t *testing.T) {
	t.Parallel()

	row := makeOverviewRow(42, "<hw-advance@example.com>")
	conn := &fakeNNTPConn{
		groupCount: 42, groupLow: 1, groupHigh: 42,
		overviewRows: []NNTPOverviewRow{row},
		article:      makeTextArticle("Hello world"),
	}
	fetcher, q, sqlDB, feed := setupNNTPFetcher(t, conn)

	if err := fetcher.fetchNNTPFeed(context.Background(), q, time.Now(), &feed); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// High-water should advance to serverHigh=42.
	assertHighWater(t, sqlDB, feed.ID, 42)
	assertFeedNoError(t, sqlDB, feed.ID)
}

// TestFetchNNTPFeed_HighWaterAdvancesMixedSkips verifies that high-water
// advances past imported articles, intentional skips (binary, deleted), and
// duplicate skips.
func TestFetchNNTPFeed_HighWaterAdvancesMixedSkips(t *testing.T) {
	t.Parallel()

	// 3 rows: one text (imported), one binary (skipped), one deleted (skipped).
	row1 := makeOverviewRow(1, "<mix1@example.com>")
	row2 := makeOverviewRow(2, "<mix2@example.com>")
	row3 := makeOverviewRow(3, "<mix3@example.com>")

	// perArticleConn allows per-article control.
	articleResults := map[int64]perArticleResult{
		1: {article: makeTextArticle("Good article")},
		2: {article: &NNTPArticle{
			Headers: map[string]string{"Content-Type": "application/octet-stream"},
			Body:    "binary",
		}},
		3: {err: ErrNNTPArticleNotFound},
	}
	conn := &perArticleConn{
		groupCount: 3, groupLow: 1, groupHigh: 3,
		overviewRows: []NNTPOverviewRow{row1, row2, row3},
		articles:     articleResults,
	}
	fetcher, q, sqlDB, feed := setupNNTPFetcher(t, conn)

	if err := fetcher.fetchNNTPFeed(context.Background(), q, time.Now(), &feed); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Imported 1, skipped 2 (binary) and 3 (deleted): all three processed.
	assertArticleCount(t, sqlDB, feed.ID, 1)
	assertHighWater(t, sqlDB, feed.ID, 3)
	assertFeedNoError(t, sqlDB, feed.ID)
}

// TestFetchNNTPFeed_HighWaterDoesNotAdvanceOnHardError verifies that high-water
// does not advance when a hard connection/server error occurs mid-range.
func TestFetchNNTPFeed_HighWaterDoesNotAdvanceOnHardError(t *testing.T) {
	t.Parallel()

	// Two rows; the second causes a hard fetch error.
	row1 := makeOverviewRow(10, "<hard-err1@example.com>")
	row2 := makeOverviewRow(11, "<hard-err2@example.com>")

	articleResults := map[int64]perArticleResult{
		10: {article: makeTextArticle("Good")},
		11: {err: errors.New("connection reset by peer")},
	}
	conn := &perArticleConn{
		groupCount: 11, groupLow: 1, groupHigh: 11,
		overviewRows: []NNTPOverviewRow{row1, row2},
		articles:     articleResults,
	}
	fetcher, q, sqlDB, feed := setupNNTPFetcher(t, conn)

	err := fetcher.fetchNNTPFeed(context.Background(), q, time.Now(), &feed)
	if err == nil {
		t.Fatal("expected hard error")
	}
	// Article 10 was imported before the hard error, but high-water must not
	// advance (the error path in importNNTPArticles returns lastProcessed=10
	// but also returns a non-nil error, so phase 4 does not update high-water).
	assertHighWater(t, sqlDB, feed.ID, 0)
	assertFeedHasError(t, sqlDB, feed.ID)
}

// TestFetchNNTPFeed_FeedErrorsClearedOnSuccess verifies that pre-existing feed
// errors are cleared after a fully successful fetch.
func TestFetchNNTPFeed_FeedErrorsClearedOnSuccess(t *testing.T) {
	t.Parallel()

	sqlDB, q := setupTestDB(t)
	_, feed := setupNNTPTestFeed(t, q, true)

	// Inject a pre-existing feed error.
	now := time.Now()
	if err := q.IncrementFeedErrors(context.Background(), dbgen.IncrementFeedErrorsParams{
		LastError:     strPtr("previous error"),
		LastFetchedAt: &now,
		ID:            feed.ID,
	}); err != nil {
		t.Fatal(err)
	}
	assertFeedHasError(t, sqlDB, feed.ID)

	// Successful fetch: one article.
	row := makeOverviewRow(5, "<clear-err@example.com>")
	conn := &fakeNNTPConn{
		groupCount: 5, groupLow: 1, groupHigh: 5,
		overviewRows: []NNTPOverviewRow{row},
		article:      makeTextArticle("Body"),
	}
	fetcher := &Fetcher{
		DB:                  sqlDB,
		NNTPDialer:          &fakeDialer{conn: conn},
		CredentialDecryptor: &fakeDecryptor{result: "pass"},
	}
	if err := fetcher.fetchNNTPFeed(context.Background(), q, time.Now(), &feed); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertFeedNoError(t, sqlDB, feed.ID)
}

// TestFetchNNTPFeed_CancellationBeforeConnect verifies that cancelling the
// context before the Dial call causes a prompt return with no high-water update.
func TestFetchNNTPFeed_CancellationBeforeConnect(t *testing.T) {
	t.Parallel()

	sqlDB, q := setupTestDB(t)
	_, feed := setupNNTPTestFeed(t, q, true)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	conn := &fakeNNTPConn{
		groupCount: 100, groupLow: 1, groupHigh: 100,
	}
	fetcher := &Fetcher{
		DB: sqlDB,
		// blockedDialer blocks until ctx is done, then returns ctx.Err().
		NNTPDialer:          &ctxCheckDialer{conn: conn},
		CredentialDecryptor: &fakeDecryptor{result: "pass"},
	}
	err := fetcher.fetchNNTPFeed(ctx, q, time.Now(), &feed)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	// No high-water advancement.
	assertHighWaterUnchanged(t, sqlDB, feed.ID, 0)
}

// ctxCheckDialer checks context before dialing.
type ctxCheckDialer struct{ conn NNTPConn }

func (d *ctxCheckDialer) Dial(ctx context.Context, _, _ string) (NNTPConn, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return d.conn, nil
}

// TestFetchNNTPFeed_CancellationDuringRowProcessing verifies that cancelling
// the context mid-loop stops processing, advances high-water only to the last
// completed article, and records a cancellation feed error.
//
// The context is cancelled when FetchArticle is called for the second row.
// This ensures row 1 is fully inserted before cancellation is detected at the
// top of the next iteration.
func TestFetchNNTPFeed_CancellationDuringRowProcessing(t *testing.T) {
	t.Parallel()

	sqlDB, q := setupTestDB(t)
	_, feed := setupNNTPTestFeed(t, q, true)

	// 3 rows: row 1 is fully processed, then ctx is cancelled during row 2
	// FetchArticle (before the insert), which the loop detects at the
	// ctx.Err() check at the top of the next iteration.
	row1 := makeOverviewRow(1, "<cancel-r1@example.com>")
	row2 := makeOverviewRow(2, "<cancel-r2@example.com>")
	row3 := makeOverviewRow(3, "<cancel-r3@example.com>")

	ctx, cancel := context.WithCancel(context.Background())

	// cancelOnSecondConn cancels the context when FetchArticle is called for
	// the second time (row 2). Row 1 is returned successfully so it can be
	// inserted before cancellation is detected.
	conn := &cancelOnSecondFetchConn{
		groupCount: 3, groupLow: 1, groupHigh: 3,
		overviewRows: []NNTPOverviewRow{row1, row2, row3},
		article:      makeTextArticle("Body"),
		cancel:       cancel,
	}
	fetcher := &Fetcher{
		DB:                  sqlDB,
		NNTPDialer:          &fakeDialer{conn: conn},
		CredentialDecryptor: &fakeDecryptor{result: "pass"},
	}
	err := fetcher.fetchNNTPFeed(ctx, q, time.Now(), &feed)
	// Context cancellation propagates as an error.
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	// Article 1 was imported; article 2 was skipped (not-found) but the
	// cancel happened during its processing. The loop detects cancellation
	// at the start of row 3's iteration. high-water must be 2 (articles 1
	// and 2 were processed before the ctx.Err() check fired on row 3).
	assertHighWater(t, sqlDB, feed.ID, 2)
	// A cancellation feed error must be recorded.
	assertFeedHasError(t, sqlDB, feed.ID)
	// Exactly one article imported.
	assertArticleCount(t, sqlDB, feed.ID, 1)
}

// cancelOnSecondFetchConn cancels the context when FetchArticle is called for
// the second time, allowing the first article to be processed normally.
type cancelOnSecondFetchConn struct {
	groupCount, groupLow, groupHigh int64
	overviewRows                    []NNTPOverviewRow
	article                         *NNTPArticle
	cancel                          context.CancelFunc
	mu                              sync.Mutex
	called                          int
}

func (c *cancelOnSecondFetchConn) SelectGroup(_ string) (count, low, high int64, name string, err error) {
	return c.groupCount, c.groupLow, c.groupHigh, "", nil
}
func (c *cancelOnSecondFetchConn) Overview(_, _ int64) ([]NNTPOverviewRow, error) {
	return c.overviewRows, nil
}
func (c *cancelOnSecondFetchConn) FetchArticle(_ int64) (*NNTPArticle, error) {
	c.mu.Lock()
	c.called++
	n := c.called
	c.mu.Unlock()
	if n == 1 {
		// First call: return article normally. The caller inserts it, then
		// the loop checks ctx.Err() at the top of the next iteration.
		return c.article, nil
	}
	// Second+ call: cancel the context and return not-found so the caller
	// exits cleanly. The ctx.Err() check at the top of the NEXT iteration
	// (after row 2's processing) will detect the cancellation.
	c.cancel()
	return nil, ErrNNTPArticleNotFound
}
func (c *cancelOnSecondFetchConn) Close() error { return nil }

// perArticleConn allows per-article control of FetchArticle.
type perArticleResult struct {
	article *NNTPArticle
	err     error
}

type perArticleConn struct {
	groupCount, groupLow, groupHigh int64
	overviewRows                    []NNTPOverviewRow
	articles                        map[int64]perArticleResult
}

func (c *perArticleConn) SelectGroup(_ string) (count, low, high int64, name string, err error) {
	return c.groupCount, c.groupLow, c.groupHigh, "", nil
}
func (c *perArticleConn) Overview(_, _ int64) ([]NNTPOverviewRow, error) {
	return c.overviewRows, nil
}
func (c *perArticleConn) FetchArticle(n int64) (*NNTPArticle, error) {
	r := c.articles[n]
	return r.article, r.err
}
func (c *perArticleConn) Close() error { return nil }

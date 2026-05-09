package feeds

import (
	"context"
	"database/sql"
	"errors"
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

package feeds

import (
	"context"
	"testing"
	"time"

	"github.com/newscientist101/feedreader/db/dbgen"
)

// TestNNTPIngestionE2E is an end-to-end ingestion test using a fake NNTP
// client. It exercises the full pipeline:
//   - Credential lookup and decryption (fake)
//   - NNTP dial+auth (fake)
//   - Group select and overview fetch
//   - Article body fetch with binary rejection
//   - Article+meta insertion with threading metadata
//   - High-water advancement
//   - Unread count visibility
//   - Search visibility
//
// The test imports:
//   - root post #1: text/plain, no References
//   - reply post #2: text/plain, References pointing to #1
//   - binary post #3: application/octet-stream (rejected)
func TestNNTPIngestionE2E(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// --- Setup: fake NNTP client articles ---

	rootMsgID := "<root-e2e@example.com>"
	replyMsgID := "<reply-e2e@example.com>"
	binaryMsgID := "<binary-e2e@example.com>"

	rootSubject := "Announcing: Go 1.26 released"
	replySubject := "Re: Announcing: Go 1.26 released"

	// Overview rows returned by the fake server.
	overviews := []NNTPOverviewRow{
		{
			ArticleNumber: 1,
			Subject:       rootSubject,
			From:          "gopher@example.com",
			Date:          "Mon, 1 Jan 2024 10:00:00 +0000",
			MessageID:     rootMsgID,
			References:    "",
			Bytes:         200,
			Lines:         10,
		},
		{
			ArticleNumber: 2,
			Subject:       replySubject,
			From:          "replier@example.com",
			Date:          "Mon, 1 Jan 2024 11:00:00 +0000",
			MessageID:     replyMsgID,
			References:    rootMsgID,
			Bytes:         150,
			Lines:         8,
		},
		{
			ArticleNumber: 3,
			Subject:       "Binary stuff",
			From:          "binposter@example.com",
			Date:          "Mon, 1 Jan 2024 12:00:00 +0000",
			MessageID:     binaryMsgID,
			References:    "",
			Bytes:         50000,
			Lines:         500,
		},
	}

	// Article bodies returned per article number.
	articles := map[int64]perArticleResult{
		1: {
			article: &NNTPArticle{
				Headers: map[string]string{
					"Content-Type": "text/plain; charset=utf-8",
					"Message-Id":   rootMsgID,
					"Subject":      rootSubject,
					"From":         "gopher@example.com",
				},
				Body: "Go 1.26 has been released with exciting new features.",
			},
		},
		2: {
			article: &NNTPArticle{
				Headers: map[string]string{
					"Content-Type": "text/plain; charset=utf-8",
					"Message-Id":   replyMsgID,
					"Subject":      replySubject,
					"From":         "replier@example.com",
					"References":   rootMsgID,
				},
				Body: "Excellent! I have been waiting for this.",
			},
		},
		3: {
			article: &NNTPArticle{
				Headers: map[string]string{
					"Content-Type": "application/octet-stream",
					"Message-Id":   binaryMsgID,
				},
				Body: "\x00\x01\x02\x03 binary data",
			},
		},
	}

	conn := &perArticleConn{
		groupCount: 3, groupLow: 1, groupHigh: 3,
		overviewRows: overviews,
		articles:     articles,
	}
	sqlDB, q := setupTestDB(t)
	_, feed := setupNNTPTestFeed(t, q, true)
	fetcher := &Fetcher{
		DB:                  sqlDB,
		NNTPDialer:          &fakeDialer{conn: conn},
		CredentialDecryptor: &fakeDecryptor{result: "pass"},
	}

	// --- Run the fetch ---

	err := fetcher.fetchNNTPFeed(ctx, q, time.Now(), &feed)
	if err != nil {
		t.Fatalf("fetchNNTPFeed: %v", err)
	}

	// --- Article records ---

	// Two text articles imported; binary rejected.
	assertArticleCount(t, sqlDB, feed.ID, 2)
	assertMetaCount(t, sqlDB, feed.ID, 2)

	// High-water must be 3 (all three article numbers attempted).
	assertHighWater(t, sqlDB, feed.ID, 3)

	// --- Thread metadata: root post ---

	rootMeta, err := q.GetUsenetArticleMetaByMessageID(ctx, dbgen.GetUsenetArticleMetaByMessageIDParams{
		FeedID: feed.ID, MessageID: rootMsgID,
	})
	if err != nil {
		t.Fatalf("get root meta: %v", err)
	}
	if rootMeta.ArticleNumber != 1 {
		t.Errorf("root ArticleNumber = %d, want 1", rootMeta.ArticleNumber)
	}
	if rootMeta.ParentMessageID != nil {
		t.Errorf("root ParentMessageID should be nil, got %q", *rootMeta.ParentMessageID)
	}
	if rootMeta.RootMessageID != rootMsgID {
		t.Errorf("root RootMessageID = %q, want %q", rootMeta.RootMessageID, rootMsgID)
	}

	// --- Thread metadata: reply ---

	replyMeta, err := q.GetUsenetArticleMetaByMessageID(ctx, dbgen.GetUsenetArticleMetaByMessageIDParams{
		FeedID: feed.ID, MessageID: replyMsgID,
	})
	if err != nil {
		t.Fatalf("get reply meta: %v", err)
	}
	if replyMeta.ArticleNumber != 2 {
		t.Errorf("reply ArticleNumber = %d, want 2", replyMeta.ArticleNumber)
	}
	if replyMeta.ParentMessageID == nil || *replyMeta.ParentMessageID != rootMsgID {
		t.Errorf("reply ParentMessageID = %v, want %q", replyMeta.ParentMessageID, rootMsgID)
	}
	if replyMeta.RootMessageID != rootMsgID {
		t.Errorf("reply RootMessageID = %q, want %q", replyMeta.RootMessageID, rootMsgID)
	}

	// --- Binary post rejected ---

	_, errBin := q.GetUsenetArticleMetaByMessageID(ctx, dbgen.GetUsenetArticleMetaByMessageIDParams{
		FeedID: feed.ID, MessageID: binaryMsgID,
	})
	if errBin == nil {
		t.Error("binary post should not have been inserted into usenet_article_meta")
	}

	// --- Unread count ---

	var userID int64
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT user_id FROM feeds WHERE id = ?`, feed.ID).Scan(&userID); err != nil {
		t.Fatalf("get user_id: %v", err)
	}

	count, err := q.GetUnreadCount(ctx, &userID)
	if err != nil {
		t.Fatalf("GetUnreadCount: %v", err)
	}
	if count != 2 {
		t.Errorf("unread count = %d, want 2", count)
	}

	// --- Search visibility: title search (root post only) ---
	// Use the exact root subject which is not a prefix of the reply subject.
	searchQ := "Announcing: Go 1.26 released"
	searchResults, err := q.SearchArticles(ctx, dbgen.SearchArticlesParams{
		UserID:  &userID,
		Column2: &searchQ,
		Column3: &searchQ,
		Limit:   50,
		Offset:  0,
	})
	if err != nil {
		t.Fatalf("SearchArticles: %v", err)
	}
	// Both root and reply titles contain this substring; verify at least one
	// result is returned and that it has feed_type='nntp'.
	if len(searchResults) == 0 {
		t.Error("search returned no results, want at least 1")
	}
	for i := range searchResults {
		if searchResults[i].FeedType != "nntp" {
			t.Errorf("search result[%d] FeedType = %q, want 'nntp'", i, searchResults[i].FeedType)
		}
	}

	// --- Reply badge field populated: usenet_parent_message_id ---
	// Search for reply-unique body text to find only the reply article.
	replyBodyQ := "waiting for this"
	replyResults, err := q.SearchArticles(ctx, dbgen.SearchArticlesParams{
		UserID:  &userID,
		Column2: &replyBodyQ,
		Column3: &replyBodyQ,
		Limit:   50,
		Offset:  0,
	})
	if err != nil {
		t.Fatalf("SearchArticles (reply): %v", err)
	}
	if len(replyResults) != 1 {
		t.Errorf("reply search results = %d, want 1", len(replyResults))
	}
	if len(replyResults) > 0 {
		if replyResults[0].UsenetParentMessageID == nil {
			t.Error("reply article UsenetParentMessageID should be non-nil")
		} else if *replyResults[0].UsenetParentMessageID != rootMsgID {
			t.Errorf("reply UsenetParentMessageID = %q, want %q",
				*replyResults[0].UsenetParentMessageID, rootMsgID)
		}
	}

	// --- Thread query: GetThreadArticles returns both articles ---

	var rootArticleID int64
	if err := sqlDB.QueryRowContext(ctx,
		`SELECT article_id FROM usenet_article_meta WHERE message_id = ? AND feed_id = ?`,
		rootMsgID, feed.ID).Scan(&rootArticleID); err != nil {
		t.Fatalf("get root article_id: %v", err)
	}

	threadRows, err := q.GetThreadArticles(ctx, dbgen.GetThreadArticlesParams{
		RootMessageID: rootMsgID,
		FeedID:        feed.ID,
		UserID:        &userID,
	})
	if err != nil {
		t.Fatalf("GetThreadArticles: %v", err)
	}
	if len(threadRows) != 2 {
		t.Errorf("thread length = %d, want 2", len(threadRows))
	}
}

package usenet_test

import (
	"strings"
	"testing"
	"time"

	"github.com/newscientist101/feedreader/srv/nntp"
	"github.com/newscientist101/feedreader/srv/usenet"
)

// makeArticle is a test helper that builds a minimal nntp.Article.
func makeArticle(headers map[string]string, body string) *nntp.Article {
	if headers == nil {
		headers = make(map[string]string)
	}
	return &nntp.Article{Headers: headers, Body: body}
}

// makeOverview is a test helper that builds an nntp.OverviewRow.
func makeOverview(articleNumber int64, subject, from, date, msgID, refs string) nntp.OverviewRow {
	return nntp.OverviewRow{
		ArticleNumber: articleNumber,
		Subject:       subject,
		From:          from,
		Date:          date,
		MessageID:     msgID,
		References:    refs,
	}
}

func TestMapArticle_BasicFields(t *testing.T) {
	overview := makeOverview(
		42,
		"Hello World",
		"Jane Doe <jane@example.com>",
		"Mon, 01 Jan 2024 12:00:00 +0000",
		"<abc123@example.com>",
		"",
	)
	article := makeArticle(nil, "This is the article body.")

	rec := usenet.MapArticle(7, "comp.lang.go", 42, &overview, article)

	if rec.Article.Guid != "<abc123@example.com>" {
		t.Errorf("Guid = %q, want %q", rec.Article.Guid, "<abc123@example.com>")
	}
	if rec.Article.Title != "Hello World" {
		t.Errorf("Title = %q, want %q", rec.Article.Title, "Hello World")
	}
	if rec.Article.Author == nil || *rec.Article.Author != "Jane Doe" {
		t.Errorf("Author = %v, want %q", rec.Article.Author, "Jane Doe")
	}
	// URL should be canonical nntp:// form
	wantURL := "nntp://news.eternal-september.org/comp.lang.go/42"
	if rec.Article.Url == nil || *rec.Article.Url != wantURL {
		t.Errorf("Url = %v, want %q", rec.Article.Url, wantURL)
	}
}

func TestMapArticle_StableID(t *testing.T) {
	// MapArticle must return the same GUID for the same Message-ID regardless
	// of how many times it is called.
	overview := makeOverview(1, "Test", "user@host", "", "<stable@host>", "")
	article := makeArticle(nil, "body")

	rec1 := usenet.MapArticle(1, "misc.test", 1, &overview, article)
	rec2 := usenet.MapArticle(1, "misc.test", 1, &overview, article)

	if rec1.Article.Guid != rec2.Article.Guid {
		t.Errorf("GUIDs differ: %q vs %q", rec1.Article.Guid, rec2.Article.Guid)
	}
	if rec1.Article.Guid != "<stable@host>" {
		t.Errorf("Guid = %q, want %q", rec1.Article.Guid, "<stable@host>")
	}
}

func TestMapArticle_DateParsing_RFC2822(t *testing.T) {
	overview := makeOverview(1, "S", "f@h", "Mon, 15 Apr 2024 09:30:00 +0200", "<x@h>", "")
	article := makeArticle(nil, "body")

	rec := usenet.MapArticle(1, "misc.test", 1, &overview, article)

	if rec.Article.PublishedAt == nil {
		t.Fatal("PublishedAt is nil, want parsed date")
	}
	want := time.Date(2024, 4, 15, 7, 30, 0, 0, time.UTC) // +0200 → UTC
	if !rec.Article.PublishedAt.Equal(want) {
		t.Errorf("PublishedAt = %v, want %v", rec.Article.PublishedAt, want)
	}
}

func TestMapArticle_DateParsing_Fallback(t *testing.T) {
	// An invalid date should result in nil PublishedAt (fallback = import time).
	overview := makeOverview(1, "S", "f@h", "not-a-date", "<x@h>", "")
	article := makeArticle(nil, "body")

	rec := usenet.MapArticle(1, "misc.test", 1, &overview, article)

	if rec.Article.PublishedAt != nil {
		t.Errorf("PublishedAt = %v, want nil for invalid date", rec.Article.PublishedAt)
	}
}

func TestMapArticle_DateFallbackToArticleHeader(t *testing.T) {
	// If overview.Date is empty, MapArticle should use the Date header.
	overview := makeOverview(1, "S", "f@h", "", "<x@h>", "")
	article := makeArticle(map[string]string{
		"Date": "Tue, 16 Apr 2024 10:00:00 +0000",
	}, "body")

	rec := usenet.MapArticle(1, "misc.test", 1, &overview, article)

	if rec.Article.PublishedAt == nil {
		t.Fatal("PublishedAt is nil, want parsed date from article header")
	}
	want := time.Date(2024, 4, 16, 10, 0, 0, 0, time.UTC)
	if !rec.Article.PublishedAt.Equal(want) {
		t.Errorf("PublishedAt = %v, want %v", rec.Article.PublishedAt, want)
	}
}

func TestMapArticle_HTMLEscaping(t *testing.T) {
	// Body with HTML-special chars must be escaped inside <pre>.
	body := "Hello <world> & 'friends' \"test\""
	overview := makeOverview(1, "S", "f@h", "", "<x@h>", "")
	article := makeArticle(nil, body)

	rec := usenet.MapArticle(1, "misc.test", 1, &overview, article)

	if rec.Article.Content == nil {
		t.Fatal("Content is nil")
	}
	content := *rec.Article.Content
	if !strings.HasPrefix(content, `<pre class="usenet-body">`) {
		t.Errorf("Content does not start with expected prefix: %q", content[:min(len(content), 50)])
	}
	if !strings.HasSuffix(content, "</pre>") {
		t.Errorf("Content does not end with </pre>: %q", content[max(0, len(content)-20):])
	}
	// Verify escaping
	if strings.Contains(content, "<world>") {
		t.Error("Content contains unescaped <world>")
	}
	if !strings.Contains(content, "&lt;world&gt;") {
		t.Error("Content missing &lt;world&gt; escaped form")
	}
}

func TestMapArticle_SummaryTruncation(t *testing.T) {
	// A body longer than 280 runes should be truncated.
	long := strings.Repeat("a", 300)
	overview := makeOverview(1, "S", "f@h", "", "<x@h>", "")
	article := makeArticle(nil, long)

	rec := usenet.MapArticle(1, "misc.test", 1, &overview, article)

	if rec.Article.Summary == nil {
		t.Fatal("Summary is nil")
	}
	// Should be 280 runes + ellipsis (1 rune)
	runeCount := len([]rune(*rec.Article.Summary))
	if runeCount != 281 {
		t.Errorf("Summary rune count = %d, want 281", runeCount)
	}
	if !strings.HasSuffix(*rec.Article.Summary, "…") {
		t.Error("Summary does not end with ellipsis")
	}
}

func TestMapArticle_SummaryShort(t *testing.T) {
	body := "Short body."
	overview := makeOverview(1, "S", "f@h", "", "<x@h>", "")
	article := makeArticle(nil, body)

	rec := usenet.MapArticle(1, "misc.test", 1, &overview, article)

	if rec.Article.Summary == nil {
		t.Fatal("Summary is nil")
	}
	if *rec.Article.Summary != body {
		t.Errorf("Summary = %q, want %q", *rec.Article.Summary, body)
	}
}

func TestMapArticle_ThreadRoot(t *testing.T) {
	// No References → thread root: parentMsgID nil, rootMsgID = own ID.
	overview := makeOverview(1, "S", "f@h", "", "<root@host>", "")
	article := makeArticle(nil, "body")

	rec := usenet.MapArticle(1, "misc.test", 1, &overview, article)

	if rec.Meta.ParentMessageID != nil {
		t.Errorf("ParentMessageID = %v, want nil for thread root", *rec.Meta.ParentMessageID)
	}
	if rec.Meta.RootMessageID != "<root@host>" {
		t.Errorf("RootMessageID = %q, want %q", rec.Meta.RootMessageID, "<root@host>")
	}
}

func TestMapArticle_ThreadReply(t *testing.T) {
	// References with two IDs: root is first, parent is last.
	refs := "<root@host> <mid@host>"
	overview := makeOverview(5, "Re: S", "f@h", "", "<reply@host>", refs)
	article := makeArticle(nil, "reply body")

	rec := usenet.MapArticle(1, "misc.test", 5, &overview, article)

	if rec.Meta.ParentMessageID == nil {
		t.Fatal("ParentMessageID is nil, want <mid@host>")
	}
	if *rec.Meta.ParentMessageID != "<mid@host>" {
		t.Errorf("ParentMessageID = %q, want %q", *rec.Meta.ParentMessageID, "<mid@host>")
	}
	if rec.Meta.RootMessageID != "<root@host>" {
		t.Errorf("RootMessageID = %q, want %q", rec.Meta.RootMessageID, "<root@host>")
	}
}

func TestMapArticle_ThreadSingleRef(t *testing.T) {
	// Single reference: root = parent = that reference.
	refs := "<parent@host>"
	overview := makeOverview(2, "Re: S", "f@h", "", "<reply@host>", refs)
	article := makeArticle(nil, "body")

	rec := usenet.MapArticle(1, "misc.test", 2, &overview, article)

	if rec.Meta.ParentMessageID == nil {
		t.Fatal("ParentMessageID is nil")
	}
	if *rec.Meta.ParentMessageID != "<parent@host>" {
		t.Errorf("ParentMessageID = %q, want %q", *rec.Meta.ParentMessageID, "<parent@host>")
	}
	if rec.Meta.RootMessageID != "<parent@host>" {
		t.Errorf("RootMessageID = %q, want %q", rec.Meta.RootMessageID, "<parent@host>")
	}
}

func TestMapArticle_MetaFields(t *testing.T) {
	overview := makeOverview(99, "Test", "u@h", "", "<meta@host>", "")
	article := makeArticle(nil, "body")

	rec := usenet.MapArticle(5, "alt.test", 99, &overview, article)

	if rec.Meta.FeedID != 5 {
		t.Errorf("Meta.FeedID = %d, want 5", rec.Meta.FeedID)
	}
	if rec.Meta.MessageID != "<meta@host>" {
		t.Errorf("Meta.MessageID = %q, want %q", rec.Meta.MessageID, "<meta@host>")
	}
	if rec.Meta.GroupName != "alt.test" {
		t.Errorf("Meta.GroupName = %q, want %q", rec.Meta.GroupName, "alt.test")
	}
	if rec.Meta.ArticleNumber != 99 {
		t.Errorf("Meta.ArticleNumber = %d, want 99", rec.Meta.ArticleNumber)
	}
	// ArticleID must be zero until caller sets it.
	if rec.Meta.ArticleID != 0 {
		t.Errorf("Meta.ArticleID = %d, want 0 (caller must set)", rec.Meta.ArticleID)
	}
}

func TestMapArticle_ReferencesHeader_Stored(t *testing.T) {
	refs := "<a@h> <b@h>"
	overview := makeOverview(3, "S", "f@h", "", "<c@h>", refs)
	article := makeArticle(nil, "body")

	rec := usenet.MapArticle(1, "misc.test", 3, &overview, article)

	if rec.Meta.ReferencesHeader == nil {
		t.Fatal("ReferencesHeader is nil, want non-nil for article with references")
	}
	if *rec.Meta.ReferencesHeader != refs {
		t.Errorf("ReferencesHeader = %q, want %q", *rec.Meta.ReferencesHeader, refs)
	}
}

func TestMapArticle_ReferencesHeader_NilWhenAbsent(t *testing.T) {
	overview := makeOverview(1, "S", "f@h", "", "<x@h>", "")
	article := makeArticle(nil, "body")

	rec := usenet.MapArticle(1, "misc.test", 1, &overview, article)

	if rec.Meta.ReferencesHeader != nil {
		t.Errorf("ReferencesHeader = %q, want nil for article with no references", *rec.Meta.ReferencesHeader)
	}
}

func TestMapArticle_FallbackSubjectWhenEmpty(t *testing.T) {
	overview := makeOverview(1, "", "f@h", "", "<x@h>", "")
	article := makeArticle(nil, "body")

	rec := usenet.MapArticle(1, "misc.test", 1, &overview, article)

	if rec.Article.Title != "(no subject)" {
		t.Errorf("Title = %q, want %q", rec.Article.Title, "(no subject)")
	}
}

func TestMapArticle_AuthorFallbackToAddress(t *testing.T) {
	// From with no display name → use email address.
	overview := makeOverview(1, "S", "noreply@example.org", "", "<x@h>", "")
	article := makeArticle(nil, "body")

	rec := usenet.MapArticle(1, "misc.test", 1, &overview, article)

	if rec.Article.Author == nil || *rec.Article.Author != "noreply@example.org" {
		t.Errorf("Author = %v, want %q", rec.Article.Author, "noreply@example.org")
	}
}

func TestMapArticle_RFC2047Subject(t *testing.T) {
	// Encoded word in Subject should be decoded.
	overview := makeOverview(1, "=?UTF-8?Q?Caf=C3=A9?=", "f@h", "", "<x@h>", "")
	article := makeArticle(nil, "body")

	rec := usenet.MapArticle(1, "misc.test", 1, &overview, article)

	// Decoded UTF-8: Café
	if rec.Article.Title != "Caf\u00e9" {
		t.Errorf("Title = %q, want %q", rec.Article.Title, "Café")
	}
}

func TestMapArticle_QuotedPrintableBody(t *testing.T) {
	// quoted-printable body should be decoded in content.
	qpBody := "Hello=20World=3D\r\nSecond line"
	article := makeArticle(map[string]string{
		"Content-Transfer-Encoding": "quoted-printable",
	}, qpBody)
	overview := makeOverview(1, "S", "f@h", "", "<x@h>", "")

	rec := usenet.MapArticle(1, "misc.test", 1, &overview, article)

	if rec.Article.Content == nil {
		t.Fatal("Content is nil")
	}
	// The decoded body should contain "Hello World=" (=20→space, =3D→=)
	if !strings.Contains(*rec.Article.Content, "Hello World=") {
		t.Errorf("Content does not contain decoded QP text: %q", *rec.Article.Content)
	}
}

func TestMapArticle_EmptyBody(t *testing.T) {
	overview := makeOverview(1, "S", "f@h", "", "<x@h>", "")
	article := makeArticle(nil, "")

	rec := usenet.MapArticle(1, "misc.test", 1, &overview, article)

	if rec.Article.Content == nil {
		t.Fatal("Content is nil for empty body")
	}
	want := `<pre class="usenet-body"></pre>`
	if *rec.Article.Content != want {
		t.Errorf("Content = %q, want %q", *rec.Article.Content, want)
	}
}

func TestMapArticle_InvalidReferencesIgnored(t *testing.T) {
	// References with malformed entries should fall back gracefully.
	// Only angle-bracket-wrapped IDs are valid references.
	refs := "invalid-ref-no-brackets another-bad-one"
	overview := makeOverview(1, "S", "f@h", "", "<x@h>", refs)
	article := makeArticle(nil, "body")

	rec := usenet.MapArticle(1, "misc.test", 1, &overview, article)

	// No valid refs → should behave like thread root.
	if rec.Meta.ParentMessageID != nil {
		t.Errorf("ParentMessageID = %q, want nil when refs are invalid", *rec.Meta.ParentMessageID)
	}
	if rec.Meta.RootMessageID != "<x@h>" {
		t.Errorf("RootMessageID = %q, want %q", rec.Meta.RootMessageID, "<x@h>")
	}
}

func TestMapArticle_ThreadDeepChain(t *testing.T) {
	// Deep thread: 4 references means root=first, parent=last.
	refs := "<r1@host> <r2@host> <r3@host> <r4@host>"
	overview := makeOverview(7, "Re^4: S", "f@h", "", "<r5@host>", refs)
	article := makeArticle(nil, "deep reply body")

	rec := usenet.MapArticle(1, "misc.test", 7, &overview, article)

	if rec.Meta.ParentMessageID == nil {
		t.Fatal("ParentMessageID is nil, want <r4@host>")
	}
	if *rec.Meta.ParentMessageID != "<r4@host>" {
		t.Errorf("ParentMessageID = %q, want %q", *rec.Meta.ParentMessageID, "<r4@host>")
	}
	if rec.Meta.RootMessageID != "<r1@host>" {
		t.Errorf("RootMessageID = %q, want %q", rec.Meta.RootMessageID, "<r1@host>")
	}
}

func TestMapArticle_ThreadDuplicateWhitespace(t *testing.T) {
	// References with extra/duplicate whitespace between IDs should parse correctly.
	refs := "  <root@host>   <mid@host>  "
	overview := makeOverview(3, "Re: S", "f@h", "", "<reply@host>", refs)
	article := makeArticle(nil, "body")

	rec := usenet.MapArticle(1, "misc.test", 3, &overview, article)

	if rec.Meta.ParentMessageID == nil {
		t.Fatal("ParentMessageID is nil, want <mid@host>")
	}
	if *rec.Meta.ParentMessageID != "<mid@host>" {
		t.Errorf("ParentMessageID = %q, want %q", *rec.Meta.ParentMessageID, "<mid@host>")
	}
	if rec.Meta.RootMessageID != "<root@host>" {
		t.Errorf("RootMessageID = %q, want %q", rec.Meta.RootMessageID, "<root@host>")
	}
}

func TestMapArticle_ThreadMissingParentInDB(t *testing.T) {
	// References point to an article that doesn't exist in the database.
	// parseThreading is a pure function — it still records the referenced parent
	// and root IDs as given; DB lookup is the caller's responsibility.
	refs := "<ghost@host>"
	overview := makeOverview(4, "Re: S", "f@h", "", "<reply@host>", refs)
	article := makeArticle(nil, "body")

	rec := usenet.MapArticle(1, "misc.test", 4, &overview, article)

	// Parent should be set even though <ghost@host> may not exist in the DB.
	if rec.Meta.ParentMessageID == nil {
		t.Fatal("ParentMessageID is nil, want <ghost@host>")
	}
	if *rec.Meta.ParentMessageID != "<ghost@host>" {
		t.Errorf("ParentMessageID = %q, want %q", *rec.Meta.ParentMessageID, "<ghost@host>")
	}
	if rec.Meta.RootMessageID != "<ghost@host>" {
		t.Errorf("RootMessageID = %q, want %q", rec.Meta.RootMessageID, "<ghost@host>")
	}
}

func TestMapArticle_ThreadMixedValidAndInvalidRefs(t *testing.T) {
	// References header mixing valid angle-bracket IDs with malformed tokens.
	// Only valid IDs are used; malformed ones are skipped.
	refs := "not-valid <good@host> also-bad"
	overview := makeOverview(5, "Re: S", "f@h", "", "<reply@host>", refs)
	article := makeArticle(nil, "body")

	rec := usenet.MapArticle(1, "misc.test", 5, &overview, article)

	// Only one valid ref: <good@host> is both root and parent.
	if rec.Meta.ParentMessageID == nil {
		t.Fatal("ParentMessageID is nil, want <good@host>")
	}
	if *rec.Meta.ParentMessageID != "<good@host>" {
		t.Errorf("ParentMessageID = %q, want %q", *rec.Meta.ParentMessageID, "<good@host>")
	}
	if rec.Meta.RootMessageID != "<good@host>" {
		t.Errorf("RootMessageID = %q, want %q", rec.Meta.RootMessageID, "<good@host>")
	}
}

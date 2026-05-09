// Package usenet provides shared types, validation, and mapping helpers for
// the Usenet newsgroup reader feature.
package usenet

import (
	"fmt"
	"html"
	"mime"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/newscientist101/feedreader/db/dbgen"
	"github.com/newscientist101/feedreader/srv/nntp"
)

// maxSummaryRunes is the maximum number of UTF-8 runes used for the article
// summary (plain-text preview).
const maxSummaryRunes = 280

// ArticleRecord is the result of mapping an accepted NNTP article into the
// application's article model. It is ready to be stored with CreateArticle
// followed by InsertUsenetArticleMeta.
type ArticleRecord struct {
	// Article holds the parameters for dbgen.Queries.CreateArticle.
	Article dbgen.CreateArticleParams
	// Meta holds the parameters for dbgen.Queries.InsertUsenetArticleMeta.
	// ArticleID must be filled in by the caller after the article is inserted.
	Meta dbgen.InsertUsenetArticleMetaParams
}

// MapArticle converts an accepted NNTP article into an ArticleRecord.
//
// feedID is the feeds.id of the subscribed newsgroup.
// groupName is the canonical (lowercase) newsgroup name, e.g. "comp.lang.go".
// articleNumber is the numeric article number within the group.
// overview is the overview row for the article (provides Subject, From, Date,
// MessageID, and References without requiring a full body fetch).
// article is the full fetched article (headers + body). It must already have
// passed CheckArticleBinary without error.
//
// MapArticle does not contact the network or database; it performs pure
// in-memory mapping. The caller is responsible for:
//  1. Calling CheckArticleBinary before MapArticle.
//  2. Inserting result.Article via CreateArticle.
//  3. Setting result.Meta.ArticleID to the returned article ID.
//  4. Inserting result.Meta via InsertUsenetArticleMeta.
func MapArticle(
	feedID int64,
	groupName string,
	articleNumber int64,
	overview *nntp.OverviewRow,
	article *nntp.Article,
) ArticleRecord {
	// -- Field: guid = Message-ID (canonical, from overview or header) --
	msgID := overview.MessageID
	if msgID == "" {
		msgID = article.GetHeader("Message-Id")
	}

	// -- Field: url = nntp://host/group/articleNumber --
	articleURL := fmt.Sprintf("nntp://%s/%s/%d",
		nntp.EternalSeptemberHost, groupName, articleNumber)

	// -- Field: title = Subject --
	subject := overview.Subject
	if subject == "" {
		subject = article.GetHeader("Subject")
	}
	subject = decodeRFC2047(subject)
	if subject == "" {
		subject = "(no subject)"
	}

	// -- Field: author = From (display name preferred) --
	author := overview.From
	if author == "" {
		author = article.GetHeader("From")
	}
	author = parseDisplayName(author)

	// -- Field: published_at = Date (parsed; fallback to nil = import time) --
	var publishedAt *time.Time
	dateStr := overview.Date
	if dateStr == "" {
		dateStr = article.GetHeader("Date")
	}
	if dateStr != "" {
		if t, err := parseNNTPDate(dateStr); err == nil {
			utc := t.UTC()
			publishedAt = &utc
		}
	}

	// -- Field: content = escaped plain text in <pre class="usenet-body"> --
	// Decode quoted-printable before escaping; other encodings pass through.
	body := decodeBodyTransferEncoding(article)
	content := buildContent(body)

	// -- Field: summary = plain-text preview (first maxSummaryRunes runes) --
	summary := buildSummary(body)

	// -- Thread metadata --
	refsHeader := article.GetHeader("References")
	if refsHeader == "" {
		refsHeader = overview.References
	}
	parentMsgID, rootMsgID := parseThreading(msgID, refsHeader)

	// Normalise refsHeader for storage (nil when absent).
	var refsPtr *string
	if refsHeader != "" {
		refsPtr = &refsHeader
	}

	return ArticleRecord{
		Article: dbgen.CreateArticleParams{
			FeedID:      feedID,
			Guid:        msgID,
			Title:       subject,
			Url:         strPtr(articleURL),
			Author:      strPtr(author),
			Content:     strPtr(content),
			Summary:     strPtr(summary),
			ImageUrl:    nil,
			PublishedAt: publishedAt,
		},
		Meta: dbgen.InsertUsenetArticleMetaParams{
			// ArticleID must be set by the caller after CreateArticle returns.
			FeedID:           feedID,
			MessageID:        msgID,
			ReferencesHeader: refsPtr,
			ParentMessageID:  parentMsgID,
			RootMessageID:    rootMsgID,
			GroupName:        groupName,
			ArticleNumber:    articleNumber,
		},
	}
}

// buildContent wraps plain-text body in a <pre> element with HTML escaping.
func buildContent(body string) string {
	if body == "" {
		return `<pre class="usenet-body"></pre>`
	}
	return `<pre class="usenet-body">` + html.EscapeString(body) + `</pre>`
}

// buildSummary returns a plain-text preview of the body, truncated to
// maxSummaryRunes runes. Trailing whitespace is stripped.
func buildSummary(body string) string {
	body = strings.TrimSpace(body)
	if utf8.RuneCountInString(body) <= maxSummaryRunes {
		return body
	}
	// Truncate at a rune boundary.
	runes := []rune(body)
	return string(runes[:maxSummaryRunes]) + "…"
}

// parseThreading extracts parent and root message IDs from a References header.
//
// If refsHeader is empty, the article is a thread root: parentMsgID is nil
// and rootMsgID equals ownMsgID.
//
// If refsHeader contains valid message IDs, parentMsgID is the last reference
// and rootMsgID is the first reference.
func parseThreading(ownMsgID, refsHeader string) (parentMsgID *string, rootMsgID string) {
	if refsHeader == "" {
		return nil, ownMsgID
	}

	// Parse individual message IDs from the References field.
	// Each ID is delimited by whitespace and optionally wrapped in < >.
	refs := extractMessageIDs(refsHeader)
	if len(refs) == 0 {
		return nil, ownMsgID
	}

	root := refs[0]
	parent := refs[len(refs)-1]
	return &parent, root
}

// extractMessageIDs splits a References header into individual message IDs.
// Each entry is trimmed of surrounding whitespace; angle brackets are preserved
// for consistent storage (the NNTP spec uses <ID@host> form).
func extractMessageIDs(refs string) []string {
	var ids []string
	for part := range strings.FieldsSeq(refs) {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		// Validate: must look like <something@something>
		if strings.HasPrefix(part, "<") && strings.HasSuffix(part, ">") && len(part) > 2 {
			ids = append(ids, part)
		}
	}
	return ids
}

// parseNNTPDate attempts to parse a Usenet Date header using the standard
// email/NNTP date formats. Returns an error if none match.
func parseNNTPDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("usenet: empty date string")
	}
	// net/mail.ParseDate handles RFC 5322 / RFC 2822 / RFC 822 dates,
	// which covers all standard Usenet Date header formats.
	if t, err := mail.ParseDate(s); err == nil {
		return t, nil
	}
	// Fallback formats that appear in practice on Usenet.
	extraFormats := []string{
		time.RFC1123Z,
		time.RFC1123,
		"2 Jan 2006 15:04:05 MST",
		"2 Jan 2006 15:04:05 -0700",
		"02 Jan 2006 15:04:05 MST",
		"02 Jan 2006 15:04:05 -0700",
		"Mon, 2 Jan 2006 15:04:05 MST",
		"Mon, 2 Jan 2006 15:04:05 -0700",
	}
	for _, format := range extraFormats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("usenet: cannot parse date %q", s)
}

// parseDisplayName extracts the display name from a From header value.
// If no display name is present, returns the full raw string.
func parseDisplayName(from string) string {
	if from == "" {
		return ""
	}
	addr, err := mail.ParseAddress(decodeRFC2047(from))
	if err != nil {
		// Return cleaned-up raw value on parse failure.
		return strings.TrimSpace(from)
	}
	if addr.Name != "" {
		return addr.Name
	}
	return addr.Address
}

// decodeRFC2047 decodes RFC 2047 encoded words (e.g. =?UTF-8?Q?...?=) in
// header values. Falls back to the raw string on decode error.
func decodeRFC2047(s string) string {
	dec := new(mime.WordDecoder)
	decoded, err := dec.DecodeHeader(s)
	if err != nil {
		return s
	}
	return decoded
}

// decodeBodyTransferEncoding handles quoted-printable transfer encoding.
// For all other encodings (7bit, 8bit, binary, identity) it returns the
// body string unchanged.
func decodeBodyTransferEncoding(article *nntp.Article) string {
	cte := strings.ToLower(strings.TrimSpace(article.GetHeader("Content-Transfer-Encoding")))
	if cte != "quoted-printable" {
		return article.Body
	}
	reader := quotedprintable.NewReader(strings.NewReader(article.Body))
	var sb strings.Builder
	var buf [4096]byte
	for {
		n, err := reader.Read(buf[:])
		if n > 0 {
			sb.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}
	return sb.String()
}

// strPtr returns a pointer to s, or nil if s is empty.
func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

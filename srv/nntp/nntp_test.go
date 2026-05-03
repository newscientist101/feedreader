package nntp_test

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/newscientist101/feedreader/srv/nntp"
)

// fakeRWC is a simple ReadWriteCloser backed by a read buffer and capturing
// writes. It is used to simulate the network layer in tests.
type fakeRWC struct {
	r io.Reader
	w bytes.Buffer
}

func (f *fakeRWC) Read(p []byte) (int, error)  { return f.r.Read(p) }
func (f *fakeRWC) Write(p []byte) (int, error) { return f.w.Write(p) }
func (f *fakeRWC) Close() error                { return nil }

// newFakeConn constructs an nntp.Conn whose reads come from serverData and
// whose writes are captured for later inspection.
func newFakeConn(serverData string) (*nntp.Conn, *fakeRWC) {
	rwc := &fakeRWC{r: strings.NewReader(serverData)}
	dead := &nntp.FakeDeadlineConn{ReadWriteCloser: rwc}
	conn := nntp.NewConn(dead)
	return conn, rwc
}

// --- ReadResponse ---

func TestReadResponse_Simple(t *testing.T) {
	conn, _ := newFakeConn("200 ready\r\n")
	resp, err := conn.ReadResponse()
	require.NoError(t, err)
	assert.Equal(t, 200, resp.Code)
	assert.Equal(t, "ready", resp.Text)
}

func TestReadResponse_ShortLine(t *testing.T) {
	conn, _ := newFakeConn("20\r\n")
	_, err := conn.ReadResponse()
	assert.Error(t, err)
}

func TestReadResponse_NonNumericCode(t *testing.T) {
	conn, _ := newFakeConn("abc hello\r\n")
	_, err := conn.ReadResponse()
	assert.Error(t, err)
}

func TestReadResponse_NoText(t *testing.T) {
	conn, _ := newFakeConn("200\r\n")
	resp, err := conn.ReadResponse()
	require.NoError(t, err)
	assert.Equal(t, 200, resp.Code)
	assert.Equal(t, "", resp.Text)
}

// --- ReadMultiLine ---

func TestReadMultiLine_Basic(t *testing.T) {
	conn, _ := newFakeConn("line1\r\nline2\r\n.\r\n")
	lines, err := conn.ReadMultiLine()
	require.NoError(t, err)
	assert.Equal(t, []string{"line1", "line2"}, lines)
}

func TestReadMultiLine_DotUnstuffing(t *testing.T) {
	conn, _ := newFakeConn("..dotline\r\nnormal\r\n.\r\n")
	lines, err := conn.ReadMultiLine()
	require.NoError(t, err)
	assert.Equal(t, []string{".dotline", "normal"}, lines)
}

func TestReadMultiLine_Empty(t *testing.T) {
	conn, _ := newFakeConn(".\r\n")
	lines, err := conn.ReadMultiLine()
	require.NoError(t, err)
	assert.Empty(t, lines)
}

// --- Authenticate ---

func TestAuthenticate_Success(t *testing.T) {
	// Server: 381 (want password) then 281 (authenticated)
	server := "381 Password required\r\n281 Authentication accepted\r\n"
	conn, rwc := newFakeConn(server)
	err := conn.Authenticate("user", "pass")
	require.NoError(t, err)

	// Check commands sent to server
	sent := rwc.w.String()
	assert.Contains(t, sent, "AUTHINFO USER user\r\n")
	assert.Contains(t, sent, "AUTHINFO PASS pass\r\n")
}

func TestAuthenticate_SuccessUsernameOnly(t *testing.T) {
	// Some servers accept at USER stage (281 immediately)
	server := "281 Authentication accepted\r\n"
	conn, _ := newFakeConn(server)
	err := conn.Authenticate("user", "pass")
	require.NoError(t, err)
}

func TestAuthenticate_BadCredentials_481(t *testing.T) {
	server := "381 Password required\r\n481 Authentication failed\r\n"
	conn, _ := newFakeConn(server)
	err := conn.Authenticate("user", "wrongpass")
	require.ErrorIs(t, err, nntp.ErrAuthFailed)
}

func TestAuthenticate_BadCredentials_482(t *testing.T) {
	server := "381 Password required\r\n482 Authentication commands issued out of sequence\r\n"
	conn, _ := newFakeConn(server)
	err := conn.Authenticate("user", "pass")
	require.ErrorIs(t, err, nntp.ErrAuthFailed)
}

func TestAuthenticate_RejectedAtUserStep(t *testing.T) {
	// 502 at USER step (server requires TLS or disallows login)
	server := "502 Command unavailable\r\n"
	conn, _ := newFakeConn(server)
	err := conn.Authenticate("user", "pass")
	require.ErrorIs(t, err, nntp.ErrAuthFailed)
}

func TestAuthenticate_UnexpectedCode(t *testing.T) {
	server := "500 Unknown command\r\n"
	conn, _ := newFakeConn(server)
	err := conn.Authenticate("user", "pass")
	require.Error(t, err)
	assert.NotErrorIs(t, err, nntp.ErrAuthFailed)
}

// --- SelectGroup ---

func TestSelectGroup_Success(t *testing.T) {
	server := "211 1234 100 1333 comp.lang.go\r\n"
	conn, _ := newFakeConn(server)
	count, low, high, name, err := conn.SelectGroup("comp.lang.go")
	require.NoError(t, err)
	assert.Equal(t, int64(1234), count)
	assert.Equal(t, int64(100), low)
	assert.Equal(t, int64(1333), high)
	assert.Equal(t, "comp.lang.go", name)
}

func TestSelectGroup_NoSuchGroup(t *testing.T) {
	server := "411 No such newsgroup\r\n"
	conn, _ := newFakeConn(server)
	_, _, _, _, err := conn.SelectGroup("no.such.group")
	require.ErrorIs(t, err, nntp.ErrNoSuchGroup)
}

func TestSelectGroup_MalformedResponse(t *testing.T) {
	// 211 with too few fields
	server := "211 1234 100\r\n"
	conn, _ := newFakeConn(server)
	_, _, _, _, err := conn.SelectGroup("comp.lang.go")
	require.Error(t, err)
}

// --- Close / Quit ---

func TestClose(t *testing.T) {
	conn, _ := newFakeConn("")
	err := conn.Close()
	require.NoError(t, err)
}

func TestQuit_SendsQuit(t *testing.T) {
	// Server responds with 205
	conn, rwc := newFakeConn("205 Goodbye\r\n")
	_ = conn.Quit()
	assert.Contains(t, rwc.w.String(), "QUIT\r\n")
}

// --- IsPositive ---

func TestResponseIsPositive(t *testing.T) {
	for _, tc := range []struct {
		code int
		want bool
	}{
		{100, true},
		{200, true},
		{300, true},
		{399, true},
		{400, false},
		{500, false},
		{99, false},
	} {
		r := &nntp.Response{Code: tc.code}
		assert.Equal(t, tc.want, r.IsPositive(), "code %d", tc.code)
	}
}

// --- Overview (OVER/XOVER) ---

func TestOverview_Normal(t *testing.T) {
	// 224 response followed by two overview rows and the terminating dot.
	overviewData := "224 Overview information follows\r\n" +
		"100\tRe: Hello\tAlice <a@example.com>\tMon, 1 Jan 2024 00:00:00 +0000\t<msg100@host>\t<msg1@host>\t1234\t20\r\n" +
		"101\tAnother topic\tBob <b@example.com>\tTue, 2 Jan 2024 00:00:00 +0000\t<msg101@host>\t\t512\t8\r\n" +
		".\r\n"
	conn, _ := newFakeConn(overviewData)
	rows, err := conn.Overview(100, 101)
	require.NoError(t, err)
	require.Len(t, rows, 2)

	assert.Equal(t, int64(100), rows[0].ArticleNumber)
	assert.Equal(t, "Re: Hello", rows[0].Subject)
	assert.Equal(t, "Alice <a@example.com>", rows[0].From)
	assert.Equal(t, "Mon, 1 Jan 2024 00:00:00 +0000", rows[0].Date)
	assert.Equal(t, "<msg100@host>", rows[0].MessageID)
	assert.Equal(t, "<msg1@host>", rows[0].References)
	assert.Equal(t, int64(1234), rows[0].Bytes)
	assert.Equal(t, int64(20), rows[0].Lines)

	assert.Equal(t, int64(101), rows[1].ArticleNumber)
	assert.Equal(t, "Another topic", rows[1].Subject)
	assert.Equal(t, "", rows[1].References)
}

func TestOverview_EmptyRange(t *testing.T) {
	// 423 means no articles in the requested range.
	conn, _ := newFakeConn("423 No articles in range\r\n")
	rows, err := conn.Overview(200, 300)
	require.NoError(t, err)
	assert.Nil(t, rows)
}

func TestOverview_FallbackToXOVER(t *testing.T) {
	// First response: 500 (OVER not supported). Second: XOVER 224 + data.
	overviewData := "500 Unknown command\r\n" +
		"224 Overview information follows\r\n" +
		"50\tTest subject\tEve <e@host>\tWed, 3 Jan 2024 00:00:00 +0000\t<msg50@host>\t\t100\t5\r\n" +
		".\r\n"
	conn, _ := newFakeConn(overviewData)
	rows, err := conn.Overview(50, 50)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, int64(50), rows[0].ArticleNumber)
	assert.Equal(t, "Test subject", rows[0].Subject)
}

func TestOverview_MissingOptionalFields(t *testing.T) {
	// Only five fields: number, subject, from, date, message-id (no refs/bytes/lines).
	overviewData := "224 Overview information follows\r\n" +
		"77\tSubj\tAuthor\tSome Date\t<77@h>\r\n" +
		".\r\n"
	conn, _ := newFakeConn(overviewData)
	rows, err := conn.Overview(77, 77)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, int64(77), rows[0].ArticleNumber)
	assert.Equal(t, "", rows[0].References)
	assert.Equal(t, int64(0), rows[0].Bytes)
	assert.Equal(t, int64(0), rows[0].Lines)
}

func TestOverview_MalformedRowsSkipped(t *testing.T) {
	// A row where the article number is not numeric is skipped; the good row
	// is still returned.
	overviewData := "224 Overview information follows\r\n" +
		"bad\tSubj\tAuthor\tDate\t<m@h>\t\t0\t0\r\n" +
		"42\tGood subject\tAuthor\tDate\t<42@h>\t\t10\t1\r\n" +
		".\r\n"
	conn, _ := newFakeConn(overviewData)
	rows, err := conn.Overview(42, 42)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, int64(42), rows[0].ArticleNumber)
}

func TestOverview_BytesLinesWithColonPrefix(t *testing.T) {
	// Some servers prefix byte/line counts with ":" per RFC 2980.
	overviewData := "224 Overview information follows\r\n" +
		"10\tSubj\tFrom\tDate\t<10@h>\t\t:999\t:33\r\n" +
		".\r\n"
	conn, _ := newFakeConn(overviewData)
	rows, err := conn.Overview(10, 10)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, int64(999), rows[0].Bytes)
	assert.Equal(t, int64(33), rows[0].Lines)
}

func TestOverview_NonNumericBytesAndLines(t *testing.T) {
	// Non-numeric byte/line values are silently zeroed.
	overviewData := "224 Overview information follows\r\n" +
		"10\tSubj\tFrom\tDate\t<10@h>\t\tn/a\tn/a\r\n" +
		".\r\n"
	conn, _ := newFakeConn(overviewData)
	rows, err := conn.Overview(10, 10)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, int64(0), rows[0].Bytes)
	assert.Equal(t, int64(0), rows[0].Lines)
}

// --- Constants ---

func TestProviderConstants(t *testing.T) {
	assert.Equal(t, "news.eternal-september.org", nntp.EternalSeptemberHost)
	assert.Equal(t, "563", nntp.EternalSeptemberPort)
	assert.Equal(t, "news.eternal-september.org:563", nntp.EternalSeptemberAddr)
	assert.Equal(t, "eternal-september", nntp.ProviderName)
}

package email

import (
	"net/mail"
	"strings"
	"testing"
)

func TestParseSender(t *testing.T) {
	tests := []struct {
		from     string
		wantName string
		wantAddr string
	}{
		{`"Newsletter" <news@example.com>`, "Newsletter", "news@example.com"},
		{`news@example.com`, "", "news@example.com"},
		{`<news@example.com>`, "", "news@example.com"},
		{`Bad Header`, "", "Bad Header"},
	}

	for _, tt := range tests {
		name, addr := parseSender(tt.from)
		if name != tt.wantName || addr != tt.wantAddr {
			t.Errorf("parseSender(%q) = (%q, %q), want (%q, %q)",
				tt.from, name, addr, tt.wantName, tt.wantAddr)
		}
	}
}

func TestDecodeHeader(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Simple Subject", "Simple Subject"},
		{"=?UTF-8?B?SGVsbG8gV29ybGQ=?=", "Hello World"},
		{"=?UTF-8?Q?Hello_World?=", "Hello World"},
	}

	for _, tt := range tests {
		got := decodeHeader(tt.input)
		if got != tt.want {
			t.Errorf("decodeHeader(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestExtractBody(t *testing.T) {
	// Simple text/plain email
	raw := "Content-Type: text/plain\r\n\r\nHello World"
	msg, _ := mail.ReadMessage(strings.NewReader(raw))
	html, text := extractBody(msg)
	if html != "" {
		t.Errorf("expected no HTML, got %q", html)
	}
	if text != "Hello World" {
		t.Errorf("expected 'Hello World', got %q", text)
	}

	// Simple text/html email
	raw = "Content-Type: text/html\r\n\r\n<p>Hello</p>"
	msg, _ = mail.ReadMessage(strings.NewReader(raw))
	html, text = extractBody(msg)
	if html != "<p>Hello</p>" {
		t.Errorf("expected '<p>Hello</p>', got %q", html)
	}
	if text != "" {
		t.Errorf("expected no text, got %q", text)
	}

	// Multipart email
	raw = "Content-Type: multipart/alternative; boundary=\"BOUNDARY\"\r\n\r\n" +
		"--BOUNDARY\r\n" +
		"Content-Type: text/plain\r\n\r\n" +
		"Plain text\r\n" +
		"--BOUNDARY\r\n" +
		"Content-Type: text/html\r\n\r\n" +
		"<p>HTML content</p>\r\n" +
		"--BOUNDARY--\r\n"
	msg, _ = mail.ReadMessage(strings.NewReader(raw))
	html, text = extractBody(msg)
	if html != "<p>HTML content</p>" {
		t.Errorf("expected '<p>HTML content</p>', got %q", html)
	}
	if text != "Plain text" {
		t.Errorf("expected 'Plain text', got %q", text)
	}
}

func TestDecodeTransferEncoding(t *testing.T) {
	// Quoted-printable
	qp := []byte("Hello=20World")
	result := decodeTransferEncoding(qp, "quoted-printable")
	if string(result) != "Hello World" {
		t.Errorf("QP decode: got %q, want 'Hello World'", result)
	}

	// Base64
	b64 := []byte("SGVsbG8gV29ybGQ=")
	result = decodeTransferEncoding(b64, "base64")
	if string(result) != "Hello World" {
		t.Errorf("base64 decode: got %q, want 'Hello World'", result)
	}

	// None/identity
	plain := []byte("Hello World")
	result = decodeTransferEncoding(plain, "")
	if string(result) != "Hello World" {
		t.Errorf("plain decode: got %q, want 'Hello World'", result)
	}
}

func TestGenerateToken(t *testing.T) {
	t1, err := GenerateToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(t1) != 24 { // 12 bytes = 24 hex chars
		t.Errorf("token length = %d, want 24", len(t1))
	}

	t2, _ := GenerateToken()
	if t1 == t2 {
		t.Error("tokens should be unique")
	}
}

func TestEmailAddress(t *testing.T) {
	addr := EmailAddress("abc123", "lynx-fairy.exe.xyz")
	if addr != "nl-abc123@lynx-fairy.exe.xyz" {
		t.Errorf("got %q, want 'nl-abc123@lynx-fairy.exe.xyz'", addr)
	}
}

func TestIsForwarded(t *testing.T) {
	tests := []struct {
		subject string
		text    string
		want    bool
	}{
		{"Fwd: Weekly Newsletter", "", true},
		{"Fw: Update", "", true},
		{"fwd: lower case", "", true},
		{"FW: upper case", "", true},
		{"Wg: German forward", "", true},
		{"Regular Subject", "", false},
		{"Regular Subject", "---------- Forwarded message ---------", true},
		{"Regular Subject", "Begin forwarded message:", true},
		{"Regular Subject", "-------- Original Message --------", true},
	}
	for _, tt := range tests {
		got := isForwarded(tt.subject, tt.text, "")
		if got != tt.want {
			t.Errorf("isForwarded(%q, %q) = %v, want %v", tt.subject, tt.text, got, tt.want)
		}
	}
}

func TestStripForwardPrefix(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Fwd: Weekly Newsletter", "Weekly Newsletter"},
		{"Fw: Update", "Update"},
		{"FW: FW: Double forward", "Double forward"},
		{"Fwd: Fwd: Triple", "Triple"},
		{"Regular Subject", "Regular Subject"},
	}
	for _, tt := range tests {
		got := stripForwardPrefix(tt.input)
		if got != tt.want {
			t.Errorf("stripForwardPrefix(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestExtractForwardedSender(t *testing.T) {
	// Gmail format
	gmail := `Some preamble text

---------- Forwarded message ---------
From: Weekly Digest <digest@example.com>
Date: Wed, Feb 18, 2026 at 10:00 AM
Subject: This Week
To: user@gmail.com

Newsletter content here.`

	name, email := extractForwardedSender(gmail, "")
	if name != "Weekly Digest" || email != "digest@example.com" {
		t.Errorf("gmail: got (%q, %q), want ('Weekly Digest', 'digest@example.com')", name, email)
	}

	// Outlook format
	outlook := `
________________________________
From: Tech News <news@techsite.com>
Sent: Wednesday, February 18, 2026 10:00 AM
To: user@outlook.com
Subject: Daily Digest

Content here.`

	name, email = extractForwardedSender(outlook, "")
	if name != "Tech News" || email != "news@techsite.com" {
		t.Errorf("outlook: got (%q, %q), want ('Tech News', 'news@techsite.com')", name, email)
	}

	// Apple Mail format
	apple := `
Begin forwarded message:

From: Apple News <apple@news.com>
Subject: Weekly Update
Date: February 18, 2026

Content.`

	name, email = extractForwardedSender(apple, "")
	if name != "Apple News" || email != "apple@news.com" {
		t.Errorf("apple: got (%q, %q), want ('Apple News', 'apple@news.com')", name, email)
	}

	// Bare email (no name)
	bare := `---------- Forwarded message ---------
From: newsletter@example.com
Subject: Update
`
	name, email = extractForwardedSender(bare, "")
	if email != "newsletter@example.com" {
		t.Errorf("bare: got (%q, %q), want ('', 'newsletter@example.com')", name, email)
	}

	// HTML-only body
	htmlBody := `<div>---------- Forwarded message ---------<br>
<b>From:</b> HTML News &lt;html@news.com&gt;<br>
<b>Subject:</b> Test</div>`

	name, email = extractForwardedSender("", htmlBody)
	if email != "html@news.com" {
		t.Errorf("html: got (%q, %q), want ('HTML News', 'html@news.com')", name, email)
	}

	// No From: line
	noFrom := "---------- Forwarded message ---------\nSubject: Test"
	name, email = extractForwardedSender(noFrom, "")
	if email != "" {
		t.Errorf("noFrom: got (%q, %q), want ('', '')", name, email)
	}
}

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

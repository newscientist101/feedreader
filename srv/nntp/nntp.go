// Package nntp provides a minimal NNTP client for connecting to Eternal
// September (news.eternal-september.org:563) over TLS. It has no database or
// server dependencies; all I/O goes through the Conn interface so tests can
// use fake connections.
package nntp

import (
	"bufio"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// Provider constants for Eternal September.
const (
	EternalSeptemberHost = "news.eternal-september.org"
	EternalSeptemberPort = "563"
	EternalSeptemberAddr = EternalSeptemberHost + ":" + EternalSeptemberPort
	ProviderName         = "eternal-september"
)

// defaultTimeout is the per-operation network timeout.
const defaultTimeout = 30 * time.Second

// Sentinel errors.
var (
	// ErrAuthFailed is returned when the server rejects credentials.
	ErrAuthFailed = errors.New("nntp: authentication failed")
	// ErrNoSuchGroup is returned by SelectGroup when the group does not exist.
	ErrNoSuchGroup = errors.New("nntp: no such group")
	// ErrArticleNotFound is returned when the requested article does not exist.
	ErrArticleNotFound = errors.New("nntp: article not found")
)

// Response holds a parsed NNTP server response.
type Response struct {
	// Code is the three-digit NNTP status code.
	Code int
	// Text is the remainder of the status line after the code.
	Text string
	// Lines contains the body of a multiline response (without the terminating
	// dot line). Nil for single-line responses.
	Lines []string
}

// IsPositive reports whether the response code is 1xx, 2xx, or 3xx.
func (r *Response) IsPositive() bool {
	return r.Code >= 100 && r.Code < 400
}

// rwc is the low-level I/O interface. *net.Conn implements it.
type rwc interface {
	io.ReadWriteCloser
	SetDeadline(time.Time) error
}

// Conn is an authenticated NNTP connection. Create one with Dial or wrap a
// fake io.ReadWriteCloser using NewConn for testing.
type Conn struct {
	conn    rwc
	reader  *bufio.Reader
	timeout time.Duration
}

// NewConn wraps an existing ReadWriteCloser (e.g. a fake net.Conn in tests)
// as an NNTP Conn. The caller must still consume and validate the server
// greeting before calling Authenticate.
func NewConn(c rwc) *Conn {
	return &Conn{
		conn:    c,
		reader:  bufio.NewReader(c),
		timeout: defaultTimeout,
	}
}

// Dial opens a TLS connection to addr, reads the greeting, and returns an
// unauthenticated Conn. addr should be "host:port".
// Call Authenticate next, then use the connection.
func Dial(addr string) (*Conn, error) {
	netConn, err := tls.Dial("tcp", addr, &tls.Config{
		MinVersion: tls.VersionTLS12,
	})
	if err != nil {
		return nil, fmt.Errorf("nntp: dial %s: %w", addr, err)
	}
	c := NewConn(netConn)
	// Read the greeting (200 or 201).
	greeting, err := c.ReadResponse()
	if err != nil {
		_ = netConn.Close()
		return nil, fmt.Errorf("nntp: greeting: %w", err)
	}
	if greeting.Code != 200 && greeting.Code != 201 {
		_ = netConn.Close()
		return nil, fmt.Errorf("nntp: unexpected greeting code %d: %s", greeting.Code, greeting.Text)
	}
	return c, nil
}

// SetTimeout overrides the per-operation deadline. Zero disables deadlines.
func (c *Conn) SetTimeout(d time.Duration) {
	c.timeout = d
}

// setDeadline applies the configured timeout to the underlying connection.
func (c *Conn) setDeadline() error {
	if c.timeout <= 0 {
		return c.conn.SetDeadline(time.Time{})
	}
	return c.conn.SetDeadline(time.Now().Add(c.timeout))
}

// SendLine writes a command line terminated by CRLF.
func (c *Conn) SendLine(line string) error {
	if err := c.setDeadline(); err != nil {
		return fmt.Errorf("nntp: set deadline: %w", err)
	}
	_, err := fmt.Fprintf(c.conn, "%s\r\n", line)
	if err != nil {
		return fmt.Errorf("nntp: write: %w", err)
	}
	return nil
}

// ReadResponse reads one NNTP response line and parses the status code.
func (c *Conn) ReadResponse() (*Response, error) {
	if err := c.setDeadline(); err != nil {
		return nil, fmt.Errorf("nntp: set deadline: %w", err)
	}
	line, err := c.reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("nntp: read response: %w", err)
	}
	line = strings.TrimRight(line, "\r\n")
	if len(line) < 3 {
		return nil, fmt.Errorf("nntp: short response line: %q", line)
	}
	code, err := strconv.Atoi(line[:3])
	if err != nil {
		return nil, fmt.Errorf("nntp: non-numeric response code in %q", line)
	}
	text := ""
	if len(line) > 4 {
		text = line[4:]
	} else if len(line) == 4 {
		// line is "NNN " with trailing space
		text = line[4:]
	}
	return &Response{Code: code, Text: text}, nil
}

// ReadMultiLine reads a multi-line NNTP data block (terminated by a lone
// dot). It returns the lines with CRLF stripped, and dot-unstuffing applied
// (leading double-dot becomes single-dot).
func (c *Conn) ReadMultiLine() ([]string, error) {
	if err := c.setDeadline(); err != nil {
		return nil, fmt.Errorf("nntp: set deadline: %w", err)
	}
	var lines []string
	for {
		line, err := c.reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("nntp: read multiline: %w", err)
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "." {
			break
		}
		// Dot-unstuffing: leading ".." becomes "."
		if strings.HasPrefix(line, "..") {
			line = line[1:]
		}
		lines = append(lines, line)
	}
	return lines, nil
}

// Command sends a command and reads the response in one step.
func (c *Conn) Command(cmd string) (*Response, error) {
	if err := c.SendLine(cmd); err != nil {
		return nil, err
	}
	return c.ReadResponse()
}

// Authenticate sends AUTHINFO USER/PASS and returns ErrAuthFailed on
// credential rejection (481/482), or a wrapped error for unexpected codes.
func (c *Conn) Authenticate(username, password string) error {
	resp, err := c.Command("AUTHINFO USER " + username)
	if err != nil {
		return err
	}
	switch resp.Code {
	case 381:
		// Server wants the password.
	case 281:
		// Authenticated with username alone (unusual but valid).
		return nil
	case 481, 482, 502:
		return ErrAuthFailed
	default:
		return fmt.Errorf("nntp: unexpected USER response %d: %s", resp.Code, resp.Text)
	}

	resp, err = c.Command("AUTHINFO PASS " + password)
	if err != nil {
		return err
	}
	switch resp.Code {
	case 281:
		return nil
	case 481, 482, 502:
		return ErrAuthFailed
	default:
		return fmt.Errorf("nntp: unexpected PASS response %d: %s", resp.Code, resp.Text)
	}
}

// SelectGroup sends a GROUP command and returns the group response fields:
// count, low, high article numbers, and the canonical group name.
func (c *Conn) SelectGroup(name string) (count, low, high int64, canonName string, err error) {
	resp, err := c.Command("GROUP " + name)
	if err != nil {
		return 0, 0, 0, "", err
	}
	switch resp.Code {
	case 211:
		// "211 count low high name"
		parts := strings.Fields(resp.Text)
		if len(parts) < 4 {
			return 0, 0, 0, "", fmt.Errorf("nntp: GROUP response malformed: %q", resp.Text)
		}
		count, err = strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return 0, 0, 0, "", fmt.Errorf("nntp: GROUP parse count: %w", err)
		}
		low, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return 0, 0, 0, "", fmt.Errorf("nntp: GROUP parse low: %w", err)
		}
		high, err = strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			return 0, 0, 0, "", fmt.Errorf("nntp: GROUP parse high: %w", err)
		}
		canonName = parts[3]
		return count, low, high, canonName, nil
	case 411:
		return 0, 0, 0, "", ErrNoSuchGroup
	default:
		return 0, 0, 0, "", fmt.Errorf("nntp: GROUP response %d: %s", resp.Code, resp.Text)
	}
}

// Quit sends the QUIT command and closes the connection.
// Errors from the server response are ignored since we're closing anyway.
func (c *Conn) Quit() error {
	// Best-effort QUIT; ignore send/response errors.
	_ = c.SendLine("QUIT")
	_, _ = c.ReadResponse()
	return c.conn.Close()
}

// Close closes the underlying connection without sending QUIT.
// Prefer Quit() for a clean shutdown.
func (c *Conn) Close() error {
	return c.conn.Close()
}

// DialAndAuth is a convenience wrapper that dials addr, reads the greeting,
// and authenticates in one call. On error the connection is closed.
func DialAndAuth(addr, username, password string) (*Conn, error) {
	c, err := Dial(addr)
	if err != nil {
		return nil, err
	}
	if err := c.Authenticate(username, password); err != nil {
		_ = c.Close()
		return nil, err
	}
	return c, nil
}

// FakeDeadlineConn wraps an io.ReadWriteCloser and satisfies rwc by making
// SetDeadline a no-op. Use this in tests that supply a bytes.Buffer or
// similar as the underlying connection.
type FakeDeadlineConn struct {
	io.ReadWriteCloser
}

// SetDeadline implements rwc by doing nothing (tests don't need real
// network timeouts).
func (f *FakeDeadlineConn) SetDeadline(_ time.Time) error {
	return nil
}

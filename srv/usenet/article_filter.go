package usenet

import (
	"errors"
	"fmt"
	"mime"
	"strings"
)

// ErrBinaryPost is returned by CheckArticleBinary when an article is
// determined to be binary or otherwise non-text content that should not be
// ingested.
var ErrBinaryPost = errors.New("binary post rejected")

// CheckArticleBinary inspects a Usenet article's headers and body (the raw
// article body as a string) and returns ErrBinaryPost when the article must
// not be ingested. A nil error means the article appears to be text-only.
//
// Headers is a map of canonical (header-key-case) header names to their
// first value, e.g. "Content-Type" -> "text/plain; charset=utf-8".
// Subject is the decoded Subject header value.
// Body is the full raw article body text (not yet decoded).
func CheckArticleBinary(headers map[string]string, subject, body string) error {
	// Check Content-Transfer-Encoding first — base64 bodies are rejected
	// regardless of Content-Type.
	if cte := headers["Content-Transfer-Encoding"]; cte != "" {
		norm := strings.ToLower(strings.TrimSpace(cte))
		if norm == "base64" {
			return fmt.Errorf("%w: base64 content-transfer-encoding", ErrBinaryPost)
		}
	}

	// Reject based on Content-Disposition: attachment
	if cd := headers["Content-Disposition"]; cd != "" {
		norm := strings.ToLower(cd)
		if strings.HasPrefix(norm, "attachment") {
			return fmt.Errorf("%w: content-disposition attachment", ErrBinaryPost)
		}
	}

	// Evaluate Content-Type.
	if ct := headers["Content-Type"]; ct != "" {
		mediaType, _, err := mime.ParseMediaType(ct)
		if err == nil {
			mediaType = strings.ToLower(mediaType)
			if err2 := checkMediaType(mediaType); err2 != nil {
				return err2
			}
		}
	}

	// Reject yEnc-encoded content by body markers.
	if isYEnc(body) {
		return fmt.Errorf("%w: yenc-encoded content", ErrBinaryPost)
	}

	// Reject binary-looking subjects (common patterns from binaries groups).
	if isBinarySubject(subject) {
		return fmt.Errorf("%w: binary-looking subject", ErrBinaryPost)
	}

	return nil
}

// checkMediaType inspects a normalized media type string and returns
// ErrBinaryPost if it identifies binary or non-text content.
func checkMediaType(mediaType string) error {
	switch {
	case mediaType == "text/plain":
		return nil
	case strings.HasPrefix(mediaType, "text/"):
		// text/html and other text/* subtypes are rejected.
		return fmt.Errorf("%w: non-plain text content type %q", ErrBinaryPost, mediaType)
	case strings.HasPrefix(mediaType, "multipart/"):
		return fmt.Errorf("%w: multipart content type %q", ErrBinaryPost, mediaType)
	case strings.HasPrefix(mediaType, "application/"):
		return fmt.Errorf("%w: application content type %q", ErrBinaryPost, mediaType)
	case strings.HasPrefix(mediaType, "image/"):
		return fmt.Errorf("%w: image content type %q", ErrBinaryPost, mediaType)
	case strings.HasPrefix(mediaType, "audio/"):
		return fmt.Errorf("%w: audio content type %q", ErrBinaryPost, mediaType)
	case strings.HasPrefix(mediaType, "video/"):
		return fmt.Errorf("%w: video content type %q", ErrBinaryPost, mediaType)
	}
	// Anything else not explicitly allowed is rejected conservatively.
	return fmt.Errorf("%w: unrecognised content type %q", ErrBinaryPost, mediaType)
}

// isYEnc reports whether the body contains yEnc markers.
func isYEnc(body string) bool {
	// yEnc posts begin with a line starting with "=ybegin".
	for line := range strings.SplitSeq(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "=ybegin") {
			return true
		}
		// Only scan the first few lines for performance.
		if trimmed != "" {
			// Break after seeing real content lines (not header blank lines).
			break
		}
	}
	return false
}

// isBinarySubject reports whether the subject looks like a binary post.
// Typical binary subjects contain patterns like [1/23] or (1 of 23) or
// the literal string "yenc" (case-insensitive).
func isBinarySubject(subject string) bool {
	low := strings.ToLower(subject)
	if strings.Contains(low, "yenc") {
		return true
	}
	// [N/M] or [N of M] part indicators.
	if containsPartIndicator(subject) {
		return true
	}
	return false
}

// containsPartIndicator returns true when the subject contains a common
// binary-post part indicator such as [1/23] or (01/23) or (1 of 23).
func containsPartIndicator(subject string) bool {
	// We look for patterns like [digits/digits] or (digits/digits)
	// or "N of M" with surrounding brackets/parens.
	in := []byte(subject)
	for i := range in {
		if in[i] != '[' && in[i] != '(' {
			continue
		}
		closer := byte(')')
		if in[i] == '[' {
			closer = ']'
		}
		j := i + 1
		// skip digits
		for j < len(in) && in[j] >= '0' && in[j] <= '9' {
			j++
		}
		if j == i+1 {
			continue // no digits
		}
		// expect '/' or ' of '
		rest := string(in[j:])
		var afterSep int
		switch {
		case rest != "" && rest[0] == '/':
			afterSep = j + 1
		case strings.HasPrefix(rest, " of "):
			afterSep = j + 4
		default:
			continue
		}
		// skip digits after separator
		k := afterSep
		for k < len(in) && in[k] >= '0' && in[k] <= '9' {
			k++
		}
		if k == afterSep {
			continue // no digits after sep
		}
		if k < len(in) && in[k] == closer {
			return true
		}
	}
	return false
}

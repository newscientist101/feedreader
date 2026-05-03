package usenet

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// ErrInvalidGroupName is returned by ValidateGroupName when the group name
// does not meet syntax or content requirements.
var ErrInvalidGroupName = errors.New("invalid newsgroup name")

// ValidateGroupName validates a newsgroup name entered by a user.
// It normalizes the name (trim whitespace, lowercase) and checks that:
//   - The name is non-empty after normalization.
//   - Each dot-separated segment is non-empty and contains only letters,
//     digits, underscores, plus signs, or hyphens.
//   - The name does not belong to a known binary hierarchy:
//     any segment equal to "binary" or "binaries", or any group that
//     starts with "alt.binaries".
//
// On success it returns the normalized name. On failure it returns an
// ErrInvalidGroupName-wrapping error.
func ValidateGroupName(name string) (string, error) {
	norm := strings.ToLower(strings.TrimSpace(name))
	if norm == "" {
		return "", fmt.Errorf("%w: name is empty", ErrInvalidGroupName)
	}

	segments := strings.SplitSeq(norm, ".")
	for seg := range segments {
		if seg == "" {
			return "", fmt.Errorf("%w: empty segment in %q", ErrInvalidGroupName, norm)
		}
		if err := validateSegment(seg); err != nil {
			return "", fmt.Errorf("%w: %w", ErrInvalidGroupName, err)
		}
		if seg == "binary" || seg == "binaries" {
			return "", fmt.Errorf("%w: binary groups are not allowed", ErrInvalidGroupName)
		}
	}

	// Reject alt.binaries.* wholesale.
	if norm == "alt.binaries" || strings.HasPrefix(norm, "alt.binaries.") {
		return "", fmt.Errorf("%w: binary groups are not allowed", ErrInvalidGroupName)
	}

	return norm, nil
}

// validateSegment returns an error if seg contains any character that is not
// a letter, digit, underscore, plus, or hyphen.
func validateSegment(seg string) error {
	for _, r := range seg {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '+' && r != '-' {
			return fmt.Errorf("invalid character %q in segment %q", r, seg)
		}
	}
	return nil
}

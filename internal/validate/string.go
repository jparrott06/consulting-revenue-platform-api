package validate

import "strings"

// TrimString returns s with leading and trailing ASCII whitespace removed.
func TrimString(s string) string {
	return strings.TrimSpace(s)
}

// NormalizeEmail trims surrounding whitespace and lowercases the address for canonical storage.
func NormalizeEmail(s string) string {
	return strings.TrimSpace(strings.ToLower(s))
}

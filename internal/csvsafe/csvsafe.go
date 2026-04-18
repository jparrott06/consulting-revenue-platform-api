package csvsafe

import "unicode/utf8"

// SafeCell mitigates spreadsheet formula injection by forcing a leading single-quote
// when a field begins with characters that spreadsheet tools may interpret as formulas.
func SafeCell(s string) string {
	if s == "" {
		return s
	}
	if s[0] == '\t' {
		return "'" + s
	}
	r, w := utf8.DecodeRuneInString(s)
	if w == 0 {
		return s
	}
	switch r {
	case '=', '+', '-', '@':
		return "'" + s
	default:
		return s
	}
}

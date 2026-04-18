package logredact

import (
	"net/url"
	"strings"
)

// Sensitive query keys whose values must never appear in access logs.
var sensitiveQueryKeys = map[string]struct{}{
	"password":         {},
	"passwd":           {},
	"pwd":              {},
	"token":            {},
	"access_token":     {},
	"refresh_token":    {},
	"secret":           {},
	"api_key":          {},
	"apikey":           {},
	"authorization":    {},
	"stripe-signature": {},
	"signature":        {},
}

// SanitizeURL returns the request path and a redacted raw query suitable for logs.
func SanitizeURL(u *url.URL) (path string, rawQuery string) {
	if u == nil {
		return "", ""
	}
	path = u.Path
	if u.RawQuery == "" {
		return path, ""
	}
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return path, "[unparsed_query]"
	}
	for k := range q {
		if isSensitiveKey(k) {
			for i := range q[k] {
				q[k][i] = "[redacted]"
			}
		}
	}
	return path, q.Encode()
}

func isSensitiveKey(k string) bool {
	k = strings.ToLower(strings.TrimSpace(k))
	if _, ok := sensitiveQueryKeys[k]; ok {
		return true
	}
	if strings.Contains(k, "password") || strings.Contains(k, "secret") || strings.Contains(k, "token") {
		return true
	}
	return false
}

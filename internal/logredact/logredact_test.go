package logredact

import (
	"net/url"
	"testing"
)

func TestSanitizeURL_RedactsSensitiveQueryKeys(t *testing.T) {
	u, err := url.Parse("https://example.test/v1/foo?token=supersecret&ok=1")
	if err != nil {
		t.Fatal(err)
	}
	path, q := SanitizeURL(u)
	if path != "/v1/foo" {
		t.Fatalf("path: got %q", path)
	}
	parsed, err := url.ParseQuery(q)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Get("token") != "[redacted]" {
		t.Fatalf("token query: got %q", parsed.Get("token"))
	}
	if parsed.Get("ok") != "1" {
		t.Fatalf("ok query: got %q", parsed.Get("ok"))
	}
}

func TestSanitizeURL_EmptyQuery(t *testing.T) {
	u := &url.URL{Path: "/healthz"}
	path, q := SanitizeURL(u)
	if path != "/healthz" || q != "" {
		t.Fatalf("got path=%q query=%q", path, q)
	}
}

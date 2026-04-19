package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMaxBody_ContentLengthExceededReturns413(t *testing.T) {
	cfg := testJWTConfig()
	cfg.HTTPMaxRequestBodyBytes = 2048

	body := strings.Repeat("a", 100)
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(body))
	req.ContentLength = 5000
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	NewHandler(cfg, nil).ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d: %s", rec.Code, rec.Body.String())
	}
	var env map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env["code"] != "payload_too_large" {
		t.Fatalf("expected payload_too_large, got %v", env["code"])
	}
}

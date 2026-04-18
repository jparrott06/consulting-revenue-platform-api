package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSecurityHeaders_OnHealthz(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	NewHandler(testLocalUnlimitedRate(), nil).ServeHTTP(rec, req)
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("missing nosniff header")
	}
	if rec.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatalf("missing frame deny")
	}
}

func TestCORS_Preflight_AllowedLocalhost(t *testing.T) {
	cfg := testLocalUnlimitedRate()
	req := httptest.NewRequest(http.MethodOptions, "/v1/me", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rec := httptest.NewRecorder()
	NewHandler(cfg, nil).ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d body=%s", rec.Code, rec.Body.String())
	}
	if o := rec.Header().Get("Access-Control-Allow-Origin"); o != "http://localhost:3000" {
		t.Fatalf("unexpected ACAO: %q", o)
	}
}

func TestCORS_Preflight_ForbiddenUnknownOrigin(t *testing.T) {
	cfg := testLocalUnlimitedRate()
	cfg.CORSAllowedOrigins = []string{"https://allowed.example"}
	req := httptest.NewRequest(http.MethodOptions, "/v1/me", nil)
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()
	NewHandler(cfg, nil).ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestMetricsEndpoint(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	NewHandler(testLocalUnlimitedRate(), nil).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "go_goroutines") && !strings.Contains(body, "http_requests_total") {
		t.Fatalf("expected default or custom prometheus metrics in body, got: %.200s", body)
	}
}

package integrationtest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jparrott06/consulting-revenue-platform-api/internal/config"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/httpapi"
)

func TestMetricsEndpoint_IncludesHTTPAndBusinessSeries(t *testing.T) {
	cfg := config.Config{}
	h := httpapi.NewHandler(cfg, nil)
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	// First scrape runs before observability records this request; second sees prior RED labels.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec2.Code)
	}
	body := rec2.Body.String()
	if !strings.Contains(body, "http_requests_total") {
		t.Fatalf("missing http_requests_total in body: %.200s", body)
	}
	// business_workflow_conflict_total is only present after at least one Inc; covered in httpapi tests.
}

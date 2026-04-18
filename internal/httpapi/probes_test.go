package httpapi

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/config"
)

func testLocalUnlimitedRate() config.Config {
	c := testJWTConfig()
	c.Environment = "local"
	c.RateLimitAuthPerMinute = 100000
	c.RateLimitDefaultPerMinute = 100000
	c.RateLimitWebhookPerMinute = 100000
	return c
}

func TestLivez(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/livez", nil)
	rec := httptest.NewRecorder()
	NewHandler(testLocalUnlimitedRate(), nil).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestReadyz_NoDatabase(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	NewHandler(testLocalUnlimitedRate(), nil).ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestReadyz_DatabaseOK(t *testing.T) {
	db, mock, err := sqlmock.New(
		sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp),
		sqlmock.MonitorPingsOption(true),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	mock.ExpectPing()
	rows := sqlmock.NewRows([]string{"?column?"})
	mock.ExpectQuery(`SELECT 1 FROM webhook_events LIMIT 1`).WillReturnRows(rows.AddRow(1))

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	NewHandler(testLocalUnlimitedRate(), db).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestReadyz_EmptyWebhookTableOK(t *testing.T) {
	db, mock, err := sqlmock.New(
		sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp),
		sqlmock.MonitorPingsOption(true),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	mock.ExpectPing()
	mock.ExpectQuery(`SELECT 1 FROM webhook_events LIMIT 1`).WillReturnError(sql.ErrNoRows)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	NewHandler(testLocalUnlimitedRate(), db).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

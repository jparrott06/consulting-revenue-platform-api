package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/auth"
)

func TestV1Clients_PostForbiddenForContractor(t *testing.T) {
	db, mock := newSQLMock(t)
	cfg := testJWTConfig()
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	orgID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	sessionID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	mock.ExpectQuery(membershipsRoleQuery).
		WithArgs(userID, orgID).
		WillReturnRows(sqlmock.NewRows([]string{"role"}).AddRow("contractor"))

	token, err := auth.IssueAccessToken([]byte(cfg.JWTSigningKey), userID.String(), sessionID.String(), cfg.JWTAccessTTL)
	if err != nil {
		t.Fatal(err)
	}

	body := `{"name":"Acme","billing_email":"a@b.com"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/clients", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Organization-ID", orgID.String())
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	NewHandler(cfg, db).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestV1Clients_IncludeDeletedForbiddenForContractor(t *testing.T) {
	db, mock := newSQLMock(t)
	cfg := testJWTConfig()
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	orgID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	sessionID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	mock.ExpectQuery(membershipsRoleQuery).
		WithArgs(userID, orgID).
		WillReturnRows(sqlmock.NewRows([]string{"role"}).AddRow("contractor"))

	token, err := auth.IssueAccessToken([]byte(cfg.JWTSigningKey), userID.String(), sessionID.String(), cfg.JWTAccessTTL)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/clients?include_deleted=true", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Organization-ID", orgID.String())
	rec := httptest.NewRecorder()

	NewHandler(cfg, db).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

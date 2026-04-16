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

func TestGenerateInvoice_ContractorForbidden(t *testing.T) {
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

	body := `{"from_date":"2026-01-01","to_date":"2026-01-31","currency":"USD"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/invoices/generate", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Organization-ID", orgID.String())
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	NewHandler(cfg, db).ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

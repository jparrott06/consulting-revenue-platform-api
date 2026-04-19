package httpapi

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/auth"
)

// sqlmock with default query matcher (substring) so multi-line UPDATE statements match ExpectExec prefixes.
func newSQLMockRegexp(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db, mock
}

func TestDeactivateOrganization_ForbiddenForAccountant(t *testing.T) {
	db, mock := newSQLMockRegexp(t)
	cfg := testJWTConfig()
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	orgID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	sessionID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	mock.ExpectQuery(`SELECT role FROM memberships WHERE user_id = \$1 AND organization_id = \$2 AND status = 'active'`).
		WithArgs(userID, orgID).
		WillReturnRows(sqlmock.NewRows([]string{"role"}).AddRow("accountant"))

	token, err := auth.IssueAccessToken([]byte(cfg.JWTSigningKey), userID.String(), sessionID.String(), cfg.JWTAccessTTL)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/organizations/"+orgID.String()+"/deactivate", nil)
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

func TestDeactivateOrganization_OkOwner(t *testing.T) {
	db, mock := newSQLMockRegexp(t)
	cfg := testJWTConfig()
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	orgID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	sessionID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	mock.ExpectQuery(`SELECT role FROM memberships WHERE user_id = \$1 AND organization_id = \$2 AND status = 'active'`).
		WithArgs(userID, orgID).
		WillReturnRows(sqlmock.NewRows([]string{"role"}).AddRow("owner"))

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT deactivated_at FROM organizations WHERE id = \$1`).
		WithArgs(orgID).
		WillReturnRows(sqlmock.NewRows([]string{"deactivated_at"}).AddRow(nil))
	mock.ExpectExec(`UPDATE organizations`).
		WithArgs(orgID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE memberships`).
		WithArgs(orgID).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectCommit()
	mock.ExpectExec(`INSERT INTO audit_logs`).
		WillReturnResult(sqlmock.NewResult(1, 1))

	token, err := auth.IssueAccessToken([]byte(cfg.JWTSigningKey), userID.String(), sessionID.String(), cfg.JWTAccessTTL)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/organizations/"+orgID.String()+"/deactivate", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Organization-ID", orgID.String())
	rec := httptest.NewRecorder()

	NewHandler(cfg, db).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDeactivateOrganization_PathOrgMismatch(t *testing.T) {
	db, mock := newSQLMockRegexp(t)
	cfg := testJWTConfig()
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	orgID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	otherOrg := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	sessionID := uuid.MustParse("44444444-4444-4444-4444-444444444444")

	mock.ExpectQuery(`SELECT role FROM memberships WHERE user_id = \$1 AND organization_id = \$2 AND status = 'active'`).
		WithArgs(userID, orgID).
		WillReturnRows(sqlmock.NewRows([]string{"role"}).AddRow("owner"))

	token, err := auth.IssueAccessToken([]byte(cfg.JWTSigningKey), userID.String(), sessionID.String(), cfg.JWTAccessTTL)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/organizations/"+otherOrg.String()+"/deactivate", nil)
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

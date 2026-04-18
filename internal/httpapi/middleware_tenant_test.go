package httpapi

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/auth"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/config"
)

func testJWTConfig() config.Config {
	return config.Config{
		JWTSigningKey:           "0123456789abcdef0123456789abcdef",
		JWTAccessTTL:            15 * time.Minute,
		HTTPMaxRequestBodyBytes: 4 << 20,
	}
}

const membershipsRoleQuery = `SELECT role FROM memberships WHERE user_id = $1 AND organization_id = $2 AND status = 'active'`

func newSQLMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db, mock
}

func TestV1Me_ServiceUnavailableWithoutDB(t *testing.T) {
	cfg := testJWTConfig()
	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	rec := httptest.NewRecorder()

	NewHandler(cfg, nil).ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestV1Me_UnauthorizedMissingBearer(t *testing.T) {
	db, mock := newSQLMock(t)
	cfg := testJWTConfig()
	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	req.Header.Set("X-Organization-ID", uuid.New().String())
	rec := httptest.NewRecorder()

	NewHandler(cfg, db).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestV1Me_UnauthorizedInvalidBearer(t *testing.T) {
	db, mock := newSQLMock(t)
	cfg := testJWTConfig()
	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	req.Header.Set("Authorization", "Bearer not-a-valid-jwt")
	req.Header.Set("X-Organization-ID", uuid.New().String())
	rec := httptest.NewRecorder()

	NewHandler(cfg, db).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestV1Me_BadRequestMissingOrg(t *testing.T) {
	db, mock := newSQLMock(t)
	cfg := testJWTConfig()
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	sessionID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	token, err := auth.IssueAccessToken([]byte(cfg.JWTSigningKey), userID.String(), sessionID.String(), cfg.JWTAccessTTL)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	NewHandler(cfg, db).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestV1Me_BadRequestInvalidOrgUUID(t *testing.T) {
	db, mock := newSQLMock(t)
	cfg := testJWTConfig()
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	sessionID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	token, err := auth.IssueAccessToken([]byte(cfg.JWTSigningKey), userID.String(), sessionID.String(), cfg.JWTAccessTTL)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Organization-ID", "not-a-uuid")
	rec := httptest.NewRecorder()

	NewHandler(cfg, db).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestV1Me_ForbiddenNoMembership(t *testing.T) {
	db, mock := newSQLMock(t)
	cfg := testJWTConfig()
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	orgID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	sessionID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	mock.ExpectQuery(membershipsRoleQuery).
		WithArgs(userID, orgID).
		WillReturnError(sql.ErrNoRows)

	token, err := auth.IssueAccessToken([]byte(cfg.JWTSigningKey), userID.String(), sessionID.String(), cfg.JWTAccessTTL)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
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

func TestV1Me_OK(t *testing.T) {
	db, mock := newSQLMock(t)
	cfg := testJWTConfig()
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	orgID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	sessionID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	mock.ExpectQuery(membershipsRoleQuery).
		WithArgs(userID, orgID).
		WillReturnRows(sqlmock.NewRows([]string{"role"}).AddRow("owner"))

	token, err := auth.IssueAccessToken([]byte(cfg.JWTSigningKey), userID.String(), sessionID.String(), cfg.JWTAccessTTL)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Organization-ID", orgID.String())
	rec := httptest.NewRecorder()

	NewHandler(cfg, db).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["user_id"] != userID.String() {
		t.Fatalf("user_id: got %q", body["user_id"])
	}
	if body["organization_id"] != orgID.String() {
		t.Fatalf("organization_id: got %q", body["organization_id"])
	}
	if body["role"] != "owner" {
		t.Fatalf("role: got %q", body["role"])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestV1AdminPing_ForbiddenNonOwner(t *testing.T) {
	db, mock := newSQLMock(t)
	cfg := testJWTConfig()
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	orgID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	sessionID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	mock.ExpectQuery(membershipsRoleQuery).
		WithArgs(userID, orgID).
		WillReturnRows(sqlmock.NewRows([]string{"role"}).AddRow("accountant"))

	token, err := auth.IssueAccessToken([]byte(cfg.JWTSigningKey), userID.String(), sessionID.String(), cfg.JWTAccessTTL)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/ping", nil)
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

func TestV1AdminPing_OKOwner(t *testing.T) {
	db, mock := newSQLMock(t)
	cfg := testJWTConfig()
	userID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	orgID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	sessionID := uuid.MustParse("33333333-3333-3333-3333-333333333333")

	mock.ExpectQuery(membershipsRoleQuery).
		WithArgs(userID, orgID).
		WillReturnRows(sqlmock.NewRows([]string{"role"}).AddRow("owner"))

	token, err := auth.IssueAccessToken([]byte(cfg.JWTSigningKey), userID.String(), sessionID.String(), cfg.JWTAccessTTL)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/ping", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Organization-ID", orgID.String())
	rec := httptest.NewRecorder()

	NewHandler(cfg, db).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

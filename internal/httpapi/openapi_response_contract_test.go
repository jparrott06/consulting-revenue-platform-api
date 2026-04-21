package httpapi

import (
	"bytes"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	legacyrouter "github.com/getkin/kin-openapi/routers/legacy"
	"github.com/google/uuid"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/auth"
)

var (
	openAPIRouterOnce sync.Once
	openAPIRouter     routers.Router
	openAPIRouterErr  error
)

func contractRouter(t *testing.T) routers.Router {
	t.Helper()
	openAPIRouterOnce.Do(func() {
		_, file, _, _ := runtime.Caller(0)
		repoRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
		loader := &openapi3.Loader{IsExternalRefsAllowed: false}
		doc, err := loader.LoadFromFile(filepath.Join(repoRoot, "docs", "openapi.yaml"))
		if err != nil {
			openAPIRouterErr = err
			return
		}
		if err := doc.Validate(loader.Context); err != nil {
			openAPIRouterErr = err
			return
		}
		openAPIRouter, openAPIRouterErr = legacyrouter.NewRouter(doc)
	})
	if openAPIRouterErr != nil {
		t.Fatalf("openapi setup: %v", openAPIRouterErr)
	}
	return openAPIRouter
}

func assertResponseMatchesOpenAPI(t *testing.T, req *http.Request, rec *httptest.ResponseRecorder) {
	t.Helper()
	reqForValidation := req.Clone(req.Context())
	reqForValidation.URL.Scheme = "http"
	reqForValidation.URL.Host = "localhost:8080"
	reqForValidation.Host = "localhost:8080"
	route, pathParams, err := contractRouter(t).FindRoute(reqForValidation)
	if err != nil {
		t.Fatalf("find OpenAPI route for %s %s: %v", req.Method, req.URL.Path, err)
	}
	input := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{
			Request:    reqForValidation,
			PathParams: pathParams,
			Route:      route,
		},
		Status: rec.Code,
		Header: rec.Result().Header,
	}
	input.SetBodyBytes(rec.Body.Bytes())
	if err := openapi3filter.ValidateResponse(req.Context(), input); err != nil {
		t.Fatalf("response does not match OpenAPI for %s %s status=%d: %v\nbody=%s", req.Method, req.URL.Path, rec.Code, err, rec.Body.String())
	}
}

func authHeaders(t *testing.T, role string) (cfg map[string]string, userID uuid.UUID, orgID uuid.UUID) {
	t.Helper()
	jwtCfg := testJWTConfig()
	userID = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	orgID = uuid.MustParse("22222222-2222-2222-2222-222222222222")
	sessionID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	token, err := auth.IssueAccessToken([]byte(jwtCfg.JWTSigningKey), userID.String(), sessionID.String(), jwtCfg.JWTAccessTTL)
	if err != nil {
		t.Fatal(err)
	}
	return map[string]string{
		"Authorization":     "Bearer " + token,
		"X-Organization-ID": orgID.String(),
		"_role":             role,
	}, userID, orgID
}

func expectMembershipRole(mock sqlmock.Sqlmock, userID, orgID uuid.UUID, role string) {
	mock.ExpectQuery(`SELECT role FROM memberships WHERE user_id = \$1 AND organization_id = \$2 AND status = 'active'`).
		WithArgs(userID, orgID).
		WillReturnRows(sqlmock.NewRows([]string{"role"}).AddRow(role))
}

func TestOpenAPIContract_LoginResponses(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		db, mock := newSQLMockRegexp(t)
		cfg := testJWTConfig()
		userID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
		sessionID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
		pw, err := auth.HashPassword("DemoPass1!")
		if err != nil {
			t.Fatal(err)
		}
		mock.ExpectQuery(`SELECT id, email, password_hash FROM users WHERE email = \$1`).
			WithArgs("owner@example.com").
			WillReturnRows(sqlmock.NewRows([]string{"id", "email", "password_hash"}).AddRow(userID, "owner@example.com", pw))
		mock.ExpectQuery(`INSERT INTO auth_sessions .* RETURNING id`).
			WithArgs(userID, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(sessionID))
		mock.ExpectExec(`INSERT INTO audit_logs`).WillReturnResult(sqlmock.NewResult(1, 1))

		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(`{"email":"owner@example.com","password":"DemoPass1!"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		NewHandler(cfg, db).ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		assertResponseMatchesOpenAPI(t, req, rec)
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("invalid_credentials", func(t *testing.T) {
		db, mock := newSQLMockRegexp(t)
		cfg := testJWTConfig()
		mock.ExpectQuery(`SELECT id, email, password_hash FROM users WHERE email = \$1`).
			WithArgs("owner@example.com").
			WillReturnRows(sqlmock.NewRows([]string{"id", "email", "password_hash"}))
		mock.ExpectExec(`INSERT INTO audit_logs`).WillReturnResult(sqlmock.NewResult(1, 1))

		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(`{"email":"owner@example.com","password":"bad"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		NewHandler(cfg, db).ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
		}
		assertResponseMatchesOpenAPI(t, req, rec)
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})
}

func TestOpenAPIContract_TimeEntryApproveResponses(t *testing.T) {
	t.Run("success_204", func(t *testing.T) {
		db, mock := newSQLMockRegexp(t)
		cfg := testJWTConfig()
		headers, userID, orgID := authHeaders(t, "owner")
		entryID := uuid.MustParse("44444444-4444-4444-4444-444444444444")
		projectID := uuid.MustParse("55555555-5555-5555-5555-555555555555")
		ownerID := uuid.MustParse("66666666-6666-6666-6666-666666666666")
		now := time.Now().UTC()

		expectMembershipRole(mock, userID, orgID, headers["_role"])
		mock.ExpectQuery(`SELECT id, organization_id, project_id, user_id, work_date, minutes, hourly_rate_minor, status, notes, created_at, updated_at`).
			WithArgs(entryID, orgID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "organization_id", "project_id", "user_id", "work_date", "minutes", "hourly_rate_minor", "status", "notes", "created_at", "updated_at"}).
				AddRow(entryID, orgID, projectID, ownerID, now, 60, int64(10000), "submitted", sql.NullString{}, now, now))
		mock.ExpectBegin()
		mock.ExpectQuery(`SELECT status, user_id FROM time_entries`).
			WithArgs(entryID, orgID).
			WillReturnRows(sqlmock.NewRows([]string{"status", "user_id"}).AddRow("submitted", ownerID))
		mock.ExpectExec(`UPDATE time_entries`).
			WithArgs(userID, entryID, orgID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(`INSERT INTO time_entry_events`).
			WithArgs(orgID, entryID, userID, "submitted", "approved", "approve", "").
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()

		req := httptest.NewRequest(http.MethodPost, "/v1/time-entries/"+entryID.String()+"/approve", nil)
		req.Header.Set("Authorization", headers["Authorization"])
		req.Header.Set("X-Organization-ID", headers["X-Organization-ID"])
		rec := httptest.NewRecorder()
		NewHandler(cfg, db).ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
		}
		assertResponseMatchesOpenAPI(t, req, rec)
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("bad_uuid_400", func(t *testing.T) {
		db, mock := newSQLMockRegexp(t)
		cfg := testJWTConfig()
		headers, userID, orgID := authHeaders(t, "owner")
		expectMembershipRole(mock, userID, orgID, headers["_role"])

		req := httptest.NewRequest(http.MethodPost, "/v1/time-entries/not-a-uuid/approve", nil)
		req.Header.Set("Authorization", headers["Authorization"])
		req.Header.Set("X-Organization-ID", headers["X-Organization-ID"])
		rec := httptest.NewRecorder()
		NewHandler(cfg, db).ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
		}
		assertResponseMatchesOpenAPI(t, req, rec)
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})
}

func TestOpenAPIContract_InvoiceSendResponses(t *testing.T) {
	t.Run("success_200", func(t *testing.T) {
		db, mock := newSQLMockRegexp(t)
		cfg := testJWTConfig()
		headers, userID, orgID := authHeaders(t, "owner")
		invoiceID := uuid.MustParse("77777777-7777-7777-7777-777777777777")
		now := time.Now().UTC()

		expectMembershipRole(mock, userID, orgID, headers["_role"])
		mock.ExpectBegin()
		mock.ExpectQuery(`UPDATE invoices`).
			WithArgs(invoiceID, orgID).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "organization_id", "invoice_number", "status", "currency", "subtotal_minor", "tax_minor", "total_minor", "issued_at", "due_at", "created_at", "updated_at",
			}).AddRow(invoiceID, orgID, int64(101), "issued", "USD", int64(10000), int64(0), int64(10000), now, nil, now, now))
		mock.ExpectExec(`INSERT INTO ledger_entries`).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()
		mock.ExpectExec(`INSERT INTO audit_logs`).WillReturnResult(sqlmock.NewResult(1, 1))

		req := httptest.NewRequest(http.MethodPost, "/v1/invoices/"+invoiceID.String()+"/send", nil)
		req.Header.Set("Authorization", headers["Authorization"])
		req.Header.Set("X-Organization-ID", headers["X-Organization-ID"])
		rec := httptest.NewRecorder()
		NewHandler(cfg, db).ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		assertResponseMatchesOpenAPI(t, req, rec)
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("bad_uuid_400", func(t *testing.T) {
		db, mock := newSQLMockRegexp(t)
		cfg := testJWTConfig()
		headers, userID, orgID := authHeaders(t, "owner")
		expectMembershipRole(mock, userID, orgID, headers["_role"])

		req := httptest.NewRequest(http.MethodPost, "/v1/invoices/not-a-uuid/send", nil)
		req.Header.Set("Authorization", headers["Authorization"])
		req.Header.Set("X-Organization-ID", headers["X-Organization-ID"])
		rec := httptest.NewRecorder()
		NewHandler(cfg, db).ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
		}
		assertResponseMatchesOpenAPI(t, req, rec)
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})
}

func TestOpenAPIContract_OutstandingReportResponses(t *testing.T) {
	t.Run("success_200", func(t *testing.T) {
		db, mock := newSQLMockRegexp(t)
		cfg := testJWTConfig()
		headers, userID, orgID := authHeaders(t, "owner")
		expectMembershipRole(mock, userID, orgID, headers["_role"])
		mock.ExpectQuery(`SELECT currency, COALESCE\(SUM\(total_minor\), 0\)::bigint, COUNT\(\*\)::bigint FROM invoices`).
			WithArgs(orgID).
			WillReturnRows(sqlmock.NewRows([]string{"currency", "sum", "count"}).AddRow("USD", int64(12345), int64(2)))

		req := httptest.NewRequest(http.MethodGet, "/v1/reports/outstanding", nil)
		req.Header.Set("Authorization", headers["Authorization"])
		req.Header.Set("X-Organization-ID", headers["X-Organization-ID"])
		rec := httptest.NewRecorder()
		NewHandler(cfg, db).ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		assertResponseMatchesOpenAPI(t, req, rec)
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("repo_failure_500", func(t *testing.T) {
		db, mock := newSQLMockRegexp(t)
		cfg := testJWTConfig()
		headers, userID, orgID := authHeaders(t, "owner")
		expectMembershipRole(mock, userID, orgID, headers["_role"])
		mock.ExpectQuery(`SELECT currency, COALESCE\(SUM\(total_minor\), 0\)::bigint, COUNT\(\*\)::bigint FROM invoices`).
			WithArgs(orgID).
			WillReturnError(sql.ErrConnDone)

		req := httptest.NewRequest(http.MethodGet, "/v1/reports/outstanding", nil)
		req.Header.Set("Authorization", headers["Authorization"])
		req.Header.Set("X-Organization-ID", headers["X-Organization-ID"])
		rec := httptest.NewRecorder()
		NewHandler(cfg, db).ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
		}
		assertResponseMatchesOpenAPI(t, req, rec)
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})
}

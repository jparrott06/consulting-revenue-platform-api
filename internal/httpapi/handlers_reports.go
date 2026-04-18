package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jparrott06/consulting-revenue-platform-api/internal/authz"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/config"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/repo"
)

func mountReportRoutes(mux *http.ServeMux, cfg config.Config, db *sql.DB) {
	mux.Handle("GET /v1/reports/outstanding", requireTenantAuth(cfg, db, requireRole(authz.ActionReportRead, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleReportsOutstanding(w, r, db)
	}))))
	mux.Handle("GET /v1/reports/paid-this-month", requireTenantAuth(cfg, db, requireRole(authz.ActionReportRead, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleReportsPaidThisMonth(w, r, db)
	}))))
	mux.Handle("GET /v1/reports/aging", requireTenantAuth(cfg, db, requireRole(authz.ActionReportRead, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleReportsAging(w, r, db)
	}))))
}

func handleReportsOutstanding(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	ctx := r.Context()
	if db == nil {
		writeError(ctx, w, http.StatusServiceUnavailable, "service_unavailable", "database is not configured", nil)
		return
	}
	p, ok := PrincipalFromContext(ctx)
	if !ok {
		writeError(ctx, w, http.StatusUnauthorized, "unauthorized", "missing principal", nil)
		return
	}
	rep, err := repo.ReportOutstandingIssued(ctx, db, p.OrganizationID)
	if err != nil {
		writeError(ctx, w, http.StatusInternalServerError, "internal_error", "could not load outstanding report", nil)
		return
	}
	writeJSON(w, http.StatusOK, rep)
}

func handleReportsPaidThisMonth(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	ctx := r.Context()
	if db == nil {
		writeError(ctx, w, http.StatusServiceUnavailable, "service_unavailable", "database is not configured", nil)
		return
	}
	p, ok := PrincipalFromContext(ctx)
	if !ok {
		writeError(ctx, w, http.StatusUnauthorized, "unauthorized", "missing principal", nil)
		return
	}

	now := time.Now().UTC()
	year, month := now.Year(), int(now.Month())
	q := r.URL.Query()
	if ys, ms := strings.TrimSpace(q.Get("year")), strings.TrimSpace(q.Get("month")); ys != "" || ms != "" {
		if ys == "" || ms == "" {
			writeError(ctx, w, http.StatusBadRequest, "validation_error", "both year and month are required when specifying a month range", nil)
			return
		}
		y, err := strconv.Atoi(ys)
		if err != nil || y < 2000 || y > 2100 {
			writeError(ctx, w, http.StatusBadRequest, "validation_error", "year must be an integer between 2000 and 2100", nil)
			return
		}
		m, err := strconv.Atoi(ms)
		if err != nil || m < 1 || m > 12 {
			writeError(ctx, w, http.StatusBadRequest, "validation_error", "month must be an integer 1-12", nil)
			return
		}
		year, month = y, m
	}

	rep, err := repo.ReportPaidInUTCMonth(ctx, db, p.OrganizationID, year, time.Month(month))
	if err != nil {
		if errors.Is(err, repo.ErrReportYearOutOfRange) || errors.Is(err, repo.ErrReportMonthOutOfRange) {
			writeError(ctx, w, http.StatusBadRequest, "validation_error", err.Error(), nil)
			return
		}
		writeError(ctx, w, http.StatusInternalServerError, "internal_error", "could not load paid-this-month report", nil)
		return
	}
	writeJSON(w, http.StatusOK, rep)
}

func handleReportsAging(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	ctx := r.Context()
	if db == nil {
		writeError(ctx, w, http.StatusServiceUnavailable, "service_unavailable", "database is not configured", nil)
		return
	}
	p, ok := PrincipalFromContext(ctx)
	if !ok {
		writeError(ctx, w, http.StatusUnauthorized, "unauthorized", "missing principal", nil)
		return
	}
	rep, err := repo.ReportAgingIssued(ctx, db, p.OrganizationID)
	if err != nil {
		writeError(ctx, w, http.StatusInternalServerError, "internal_error", "could not load aging report", nil)
		return
	}
	writeJSON(w, http.StatusOK, rep)
}

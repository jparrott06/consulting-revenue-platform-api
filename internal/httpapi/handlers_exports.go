package httpapi

import (
	"database/sql"
	"encoding/csv"
	"net/http"

	"github.com/jparrott06/consulting-revenue-platform-api/internal/authz"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/config"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/csvsafe"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/repo"
)

func mountExportRoutes(mux *http.ServeMux, cfg config.Config, db *sql.DB) {
	mux.Handle("GET /v1/exports/invoices.csv", requireTenantAuth(cfg, db, requireRole(authz.ActionReportRead, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleExportInvoicesCSV(w, r, db)
	}))))
	mux.Handle("GET /v1/exports/payments.csv", requireTenantAuth(cfg, db, requireRole(authz.ActionReportRead, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleExportPaymentsCSV(w, r, db)
	}))))
	mux.Handle("GET /v1/exports/time-summary.csv", requireTenantAuth(cfg, db, requireRole(authz.ActionReportRead, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleExportTimeSummaryCSV(w, r, db)
	}))))
}

func handleExportInvoicesCSV(w http.ResponseWriter, r *http.Request, db *sql.DB) {
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

	setCSVHeaders(w, "invoices.csv")
	cw := csv.NewWriter(w)
	if err := cw.Write(safeCSVRecord(repo.InvoiceCSVHeader)); err != nil {
		writeError(ctx, w, http.StatusInternalServerError, "internal_error", "export failed", nil)
		return
	}
	cw.Flush()

	err := repo.StreamInvoicesForCSV(ctx, db, p.OrganizationID, func(row []string) error {
		if err := cw.Write(safeCSVRecord(row)); err != nil {
			return err
		}
		cw.Flush()
		return nil
	})
	if err != nil {
		writeError(ctx, w, http.StatusInternalServerError, "internal_error", "export failed", nil)
		return
	}
	cw.Flush()
}

func handleExportPaymentsCSV(w http.ResponseWriter, r *http.Request, db *sql.DB) {
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

	setCSVHeaders(w, "payments.csv")
	cw := csv.NewWriter(w)
	if err := cw.Write(safeCSVRecord(repo.PaymentCSVHeader)); err != nil {
		writeError(ctx, w, http.StatusInternalServerError, "internal_error", "export failed", nil)
		return
	}
	cw.Flush()

	err := repo.StreamPaymentsForCSV(ctx, db, p.OrganizationID, func(row []string) error {
		if err := cw.Write(safeCSVRecord(row)); err != nil {
			return err
		}
		cw.Flush()
		return nil
	})
	if err != nil {
		writeError(ctx, w, http.StatusInternalServerError, "internal_error", "export failed", nil)
		return
	}
	cw.Flush()
}

func handleExportTimeSummaryCSV(w http.ResponseWriter, r *http.Request, db *sql.DB) {
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

	setCSVHeaders(w, "time-summary.csv")
	cw := csv.NewWriter(w)
	if err := cw.Write(safeCSVRecord(repo.TimeSummaryCSVHeader)); err != nil {
		writeError(ctx, w, http.StatusInternalServerError, "internal_error", "export failed", nil)
		return
	}
	cw.Flush()

	err := repo.StreamTimeSummaryForCSV(ctx, db, p.OrganizationID, func(row []string) error {
		if err := cw.Write(safeCSVRecord(row)); err != nil {
			return err
		}
		cw.Flush()
		return nil
	})
	if err != nil {
		writeError(ctx, w, http.StatusInternalServerError, "internal_error", "export failed", nil)
		return
	}
	cw.Flush()
}

func setCSVHeaders(w http.ResponseWriter, filename string) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
}

func safeCSVRecord(fields []string) []string {
	out := make([]string, len(fields))
	for i, s := range fields {
		out[i] = csvsafe.SafeCell(s)
	}
	return out
}

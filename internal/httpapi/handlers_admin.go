package httpapi

import (
	"database/sql"
	"net/http"

	"github.com/jparrott06/consulting-revenue-platform-api/internal/authz"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/config"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/repo"
)

func mountAdminRoutes(mux *http.ServeMux, cfg config.Config, db *sql.DB) {
	mux.Handle("GET /v1/admin/reconciliation-summary", requireTenantAuth(cfg, db, requireRole(authz.ActionAdminOps, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleReconciliationSummary(w, r, db)
	}))))
}

func handleReconciliationSummary(w http.ResponseWriter, r *http.Request, db *sql.DB) {
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

	summary, err := repo.GetReconciliationSummary(ctx, db, p.OrganizationID)
	if err != nil {
		writeError(ctx, w, http.StatusInternalServerError, "internal_error", "could not load reconciliation summary", nil)
		return
	}

	out := make([]map[string]any, 0, len(summary.Currencies))
	for _, c := range summary.Currencies {
		out = append(out, map[string]any{
			"currency":                      c.Currency,
			"ledger_payment_captured_minor": c.LedgerPaymentCapturedMinor,
			"paid_invoice_payments_minor":   c.PaidInvoicePaymentsMinor,
			"drift_minor":                   c.DriftMinor,
			"aligned":                       c.DriftMinor == 0,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"currencies": out})
}

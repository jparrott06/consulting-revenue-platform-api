package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jparrott06/consulting-revenue-platform-api/internal/authz"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/config"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/repo"
)

func mountInvoiceRoutes(mux *http.ServeMux, cfg config.Config, db *sql.DB) {
	mux.Handle("POST /v1/invoices/generate", requireTenantAuth(cfg, db, requireRole(authz.ActionInvoiceWrite, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleGenerateInvoice(w, r, db)
	}))))
	mountInvoiceLineItemRoutes(mux, cfg, db)
}

type generateInvoiceRequest struct {
	FromDate string  `json:"from_date"`
	ToDate   string  `json:"to_date"`
	Currency string  `json:"currency"`
	DueAt    *string `json:"due_at"`
}

func handleGenerateInvoice(w http.ResponseWriter, r *http.Request, db *sql.DB) {
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

	r.Body = http.MaxBytesReader(w, r.Body, maxTimeEntryBodyBytes)
	var req generateInvoiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(ctx, w, http.StatusBadRequest, "validation_error", "invalid JSON body", nil)
		return
	}

	from, err := time.Parse("2006-01-02", strings.TrimSpace(req.FromDate))
	if err != nil {
		writeError(ctx, w, http.StatusBadRequest, "validation_error", "from_date must be YYYY-MM-DD", nil)
		return
	}
	to, err := time.Parse("2006-01-02", strings.TrimSpace(req.ToDate))
	if err != nil {
		writeError(ctx, w, http.StatusBadRequest, "validation_error", "to_date must be YYYY-MM-DD", nil)
		return
	}
	if to.Before(from) {
		writeError(ctx, w, http.StatusBadRequest, "validation_error", "to_date must be on or after from_date", nil)
		return
	}
	currency := strings.TrimSpace(req.Currency)
	if currency == "" {
		currency = "USD"
	}
	var due *time.Time
	if req.DueAt != nil && strings.TrimSpace(*req.DueAt) != "" {
		dt, err := time.Parse(time.RFC3339, strings.TrimSpace(*req.DueAt))
		if err != nil {
			writeError(ctx, w, http.StatusBadRequest, "validation_error", "due_at must be RFC3339 timestamp", nil)
			return
		}
		due = &dt
	}

	invoice, err := repo.GenerateInvoiceFromApprovedEntries(ctx, db, p.OrganizationID, repo.GenerateInvoiceParams{
		FromDate: from,
		ToDate:   to,
		Currency: currency,
		DueAt:    due,
	})
	if errors.Is(err, repo.ErrNoEligibleTimeEntries) {
		writeError(ctx, w, http.StatusConflict, "conflict", "no eligible approved uninvoiced entries in range", nil)
		return
	}
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "currency") || strings.Contains(msg, "overflow") || errors.Is(err, repo.ErrUnsupportedCurrency) {
			writeError(ctx, w, http.StatusBadRequest, "validation_error", msg, nil)
			return
		}
		writeError(ctx, w, http.StatusInternalServerError, "internal_error", "could not generate invoice", nil)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":             invoice.ID.String(),
		"invoice_number": invoice.InvoiceNumber,
		"subtotal_minor": invoice.SubtotalMinor,
		"total_minor":    invoice.TotalMinor,
		"currency":       invoice.Currency,
	})
}

package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/authz"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/config"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/repo"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/stripepay"
)

func mountInvoicePaymentRoutes(mux *http.ServeMux, cfg config.Config, db *sql.DB) {
	mux.Handle("POST /v1/invoices/{invoiceID}/payment-link", requireTenantAuth(cfg, db, requireRole(authz.ActionInvoiceWrite, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePostInvoicePaymentLink(w, r, cfg, db)
	}))))
}

func handlePostInvoicePaymentLink(w http.ResponseWriter, r *http.Request, cfg config.Config, db *sql.DB) {
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
	if strings.TrimSpace(cfg.StripeSecretKey) == "" {
		writeError(ctx, w, http.StatusServiceUnavailable, "service_unavailable", "Stripe is not configured", nil)
		return
	}

	invoiceID, err := uuid.Parse(strings.TrimSpace(r.PathValue("invoiceID")))
	if err != nil {
		writeError(ctx, w, http.StatusBadRequest, "validation_error", "invoice id must be a UUID", nil)
		return
	}

	rec, err := ensureInvoicePaymentLink(ctx, db, cfg, p.OrganizationID, invoiceID)
	if errors.Is(err, errInvoiceNotPayable) {
		writeError(ctx, w, http.StatusConflict, "conflict", "invoice is not eligible for a payment link", nil)
		return
	}
	if errors.Is(err, repo.ErrInvoiceNotFound) {
		writeError(ctx, w, http.StatusNotFound, "not_found", "invoice not found", nil)
		return
	}
	if err != nil {
		writeError(ctx, w, http.StatusInternalServerError, "internal_error", "could not create payment link", nil)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"invoice_id":             rec.InvoiceID.String(),
		"payment_url":            rec.PaymentURL,
		"stripe_payment_link_id": rec.StripePaymentLinkID,
		"amount_minor":           rec.AmountMinor,
		"currency":               rec.Currency,
	})
}

var errInvoiceNotPayable = errors.New("invoice not payable")

func ensureInvoicePaymentLink(ctx context.Context, db *sql.DB, cfg config.Config, organizationID, invoiceID uuid.UUID) (repo.PaymentRecord, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return repo.PaymentRecord{}, err
	}
	defer func() { _ = tx.Rollback() }()

	inv, err := repo.GetInvoiceForUpdate(ctx, tx, organizationID, invoiceID)
	if err != nil {
		return repo.PaymentRecord{}, err
	}
	if inv.Status != "issued" {
		return repo.PaymentRecord{}, errInvoiceNotPayable
	}
	if inv.TotalMinor <= 0 {
		return repo.PaymentRecord{}, errInvoiceNotPayable
	}

	pay, err := repo.GetPaymentByInvoiceForUpdate(ctx, tx, organizationID, invoiceID)
	if err == nil {
		if err := tx.Commit(); err != nil {
			return repo.PaymentRecord{}, err
		}
		return pay, nil
	}
	if !errors.Is(err, repo.ErrPaymentNotFound) {
		return repo.PaymentRecord{}, err
	}

	idem := fmt.Sprintf("invoice_payment_link:%s", invoiceID)
	url, linkID, err := stripepay.CreateInvoicePaymentLink(cfg.StripeSecretKey, idem, inv.Currency, inv.TotalMinor, inv.InvoiceNumber, invoiceID.String())
	if err != nil {
		return repo.PaymentRecord{}, err
	}

	pay, err = repo.InsertPayment(ctx, tx, organizationID, invoiceID, linkID, url, inv.TotalMinor, inv.Currency, idem)
	if errors.Is(err, repo.ErrPaymentExists) {
		if err := tx.Rollback(); err != nil {
			return repo.PaymentRecord{}, err
		}
		return repo.GetPaymentByInvoice(ctx, db, organizationID, invoiceID)
	}
	if err != nil {
		return repo.PaymentRecord{}, err
	}
	if err := tx.Commit(); err != nil {
		return repo.PaymentRecord{}, err
	}
	return pay, nil
}

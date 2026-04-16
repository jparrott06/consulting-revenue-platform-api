package httpapi

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/authz"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/config"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/invoicepdf"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/repo"
)

func mountInvoicePDFRoutes(mux *http.ServeMux, cfg config.Config, db *sql.DB) {
	mux.Handle("GET /v1/invoices/{invoiceID}/pdf", requireTenantAuth(cfg, db, requireRole(authz.ActionInvoiceWrite, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleGetInvoicePDF(w, r, db)
	}))))
	mux.Handle("POST /v1/invoices/{invoiceID}/pdf-url", requireTenantAuth(cfg, db, requireRole(authz.ActionInvoiceWrite, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePostInvoicePDFURL(w, r, cfg, db)
	}))))
}

func mountPublicDocumentRoutes(mux *http.ServeMux, cfg config.Config, db *sql.DB) {
	mux.HandleFunc("GET /v1/documents/invoice-pdf", handleInvoicePDFByToken(cfg, db))
}

func handleGetInvoicePDF(w http.ResponseWriter, r *http.Request, db *sql.DB) {
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
	invoiceID, err := uuid.Parse(strings.TrimSpace(r.PathValue("invoiceID")))
	if err != nil {
		writeError(ctx, w, http.StatusBadRequest, "validation_error", "invoice id must be a UUID", nil)
		return
	}
	pdfBytes, inv, err := buildInvoicePDF(ctx, db, p.OrganizationID, invoiceID)
	if errors.Is(err, repo.ErrInvoiceNotFound) {
		writeError(ctx, w, http.StatusNotFound, "not_found", "invoice not found", nil)
		return
	}
	if err != nil {
		writeError(ctx, w, http.StatusInternalServerError, "internal_error", "could not build invoice pdf", nil)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "inline; filename="+invoicePDFFilename(inv))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pdfBytes)
}

func handlePostInvoicePDFURL(w http.ResponseWriter, r *http.Request, cfg config.Config, db *sql.DB) {
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
	invoiceID, err := uuid.Parse(strings.TrimSpace(r.PathValue("invoiceID")))
	if err != nil {
		writeError(ctx, w, http.StatusBadRequest, "validation_error", "invoice id must be a UUID", nil)
		return
	}
	if _, err := repo.GetInvoice(ctx, db, p.OrganizationID, invoiceID); err != nil {
		if errors.Is(err, repo.ErrInvoiceNotFound) {
			writeError(ctx, w, http.StatusNotFound, "not_found", "invoice not found", nil)
			return
		}
		writeError(ctx, w, http.StatusInternalServerError, "internal_error", "could not load invoice", nil)
		return
	}
	ttl := cfg.InvoicePDFURLTTL
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	tok, exp, err := mintInvoicePDFToken(cfg, p.OrganizationID, invoiceID, ttl)
	if err != nil {
		writeError(ctx, w, http.StatusInternalServerError, "internal_error", "could not mint document url", nil)
		return
	}
	rel := "/v1/documents/invoice-pdf?token=" + url.QueryEscape(tok)
	outURL := rel
	if cfg.PublicAPIBaseURL != "" {
		outURL = cfg.PublicAPIBaseURL + rel
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"url":        outURL,
		"expires_at": exp.UTC().Format(time.RFC3339Nano),
	})
}

func handleInvoicePDFByToken(cfg config.Config, db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if db == nil {
			writeError(ctx, w, http.StatusServiceUnavailable, "service_unavailable", "database is not configured", nil)
			return
		}
		tok := strings.TrimSpace(r.URL.Query().Get("token"))
		if tok == "" {
			writeError(ctx, w, http.StatusBadRequest, "validation_error", "token is required", nil)
			return
		}
		orgID, invoiceID, err := parseInvoicePDFToken(cfg, tok)
		if err != nil {
			writeError(ctx, w, http.StatusUnauthorized, "unauthorized", "invalid or expired document token", nil)
			return
		}
		pdfBytes, inv, err := buildInvoicePDF(ctx, db, orgID, invoiceID)
		if errors.Is(err, repo.ErrInvoiceNotFound) {
			writeError(ctx, w, http.StatusNotFound, "not_found", "invoice not found", nil)
			return
		}
		if err != nil {
			writeError(ctx, w, http.StatusInternalServerError, "internal_error", "could not build invoice pdf", nil)
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", "inline; filename="+invoicePDFFilename(inv))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(pdfBytes)
	}
}

func buildInvoicePDF(ctx context.Context, db *sql.DB, organizationID, invoiceID uuid.UUID) ([]byte, repo.InvoiceRecord, error) {
	inv, err := repo.GetInvoice(ctx, db, organizationID, invoiceID)
	if err != nil {
		return nil, inv, err
	}
	lines, err := repo.ListInvoiceLineItemsForPDF(ctx, db, organizationID, invoiceID)
	if err != nil {
		return nil, inv, err
	}
	pdfLines := make([]invoicepdf.Line, 0, len(lines))
	for _, row := range lines {
		unitDisp, err := repo.FormatMinorForDisplay(inv.Currency, row.UnitAmountMinor)
		if err != nil {
			return nil, inv, err
		}
		lineDisp, err := repo.FormatMinorForDisplay(inv.Currency, row.LineTotalMinor)
		if err != nil {
			return nil, inv, err
		}
		pdfLines = append(pdfLines, invoicepdf.Line{
			Description: row.Description,
			Quantity:    strings.TrimSpace(row.QuantityText),
			UnitDisplay: unitDisp,
			LineDisplay: lineDisp,
		})
	}
	subDisp, err := repo.FormatMinorForDisplay(inv.Currency, inv.SubtotalMinor)
	if err != nil {
		return nil, inv, err
	}
	taxDisp, err := repo.FormatMinorForDisplay(inv.Currency, inv.TaxMinor)
	if err != nil {
		return nil, inv, err
	}
	totDisp, err := repo.FormatMinorForDisplay(inv.Currency, inv.TotalMinor)
	if err != nil {
		return nil, inv, err
	}
	var issued *time.Time
	if inv.IssuedAt.Valid {
		t := inv.IssuedAt.Time
		issued = &t
	}
	h := invoicepdf.Header{
		InvoiceNumber: inv.InvoiceNumber,
		Currency:      inv.Currency,
		Status:        inv.Status,
		SubtotalDisp:  subDisp,
		TaxDisp:       taxDisp,
		TotalDisp:     totDisp,
		IssuedAt:      issued,
	}
	pdfBytes, err := invoicepdf.Build(h, pdfLines)
	if err != nil {
		return nil, inv, err
	}
	return pdfBytes, inv, nil
}

func invoicePDFFilename(inv repo.InvoiceRecord) string {
	return fmt.Sprintf("invoice-%d.pdf", inv.InvoiceNumber)
}

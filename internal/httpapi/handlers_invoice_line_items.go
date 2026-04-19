package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/authz"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/config"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/repo"
)

func mountInvoiceLineItemRoutes(mux *http.ServeMux, cfg config.Config, db *sql.DB) {
	mux.Handle("PATCH /v1/invoices/{invoiceID}/line-items", requireTenantAuth(cfg, db, requireRole(authz.ActionInvoiceWrite, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePatchInvoiceLineItems(w, r, db)
	}))))
}

type lineItemUpsertRequest struct {
	ID              *string `json:"id"`
	Description     string  `json:"description"`
	Quantity        string  `json:"quantity"`
	UnitAmountMinor int64   `json:"unit_amount_minor"`
}

type patchInvoiceLineItemsRequest struct {
	Upsert    []lineItemUpsertRequest `json:"upsert"`
	RemoveIDs []string                `json:"remove_ids"`
}

func handlePatchInvoiceLineItems(w http.ResponseWriter, r *http.Request, db *sql.DB) {
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

	var req patchInvoiceLineItemsRequest
	if !decodeJSONBody(ctx, w, r, &req) {
		return
	}
	if len(req.Upsert) == 0 && len(req.RemoveIDs) == 0 {
		writeError(ctx, w, http.StatusBadRequest, "validation_error", "upsert or remove_ids is required", nil)
		return
	}

	upserts := make([]repo.LineItemUpsert, 0, len(req.Upsert))
	for _, raw := range req.Upsert {
		var idPtr *uuid.UUID
		if raw.ID != nil && strings.TrimSpace(*raw.ID) != "" {
			id, err := uuid.Parse(strings.TrimSpace(*raw.ID))
			if err != nil {
				writeError(ctx, w, http.StatusBadRequest, "validation_error", "line item id must be a UUID", nil)
				return
			}
			idPtr = &id
		}
		upserts = append(upserts, repo.LineItemUpsert{
			ID:              idPtr,
			Description:     raw.Description,
			Quantity:        raw.Quantity,
			UnitAmountMinor: raw.UnitAmountMinor,
		})
	}

	removeIDs := make([]uuid.UUID, 0, len(req.RemoveIDs))
	for _, raw := range req.RemoveIDs {
		id, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil {
			writeError(ctx, w, http.StatusBadRequest, "validation_error", "remove_ids must contain UUIDs", nil)
			return
		}
		removeIDs = append(removeIDs, id)
	}

	updated, err := repo.PatchInvoiceLineItems(ctx, db, p.OrganizationID, invoiceID, upserts, removeIDs)
	if errors.Is(err, repo.ErrInvoiceNotFound) {
		writeError(ctx, w, http.StatusNotFound, "not_found", "invoice not found", nil)
		return
	}
	if errors.Is(err, repo.ErrInvoiceImmutable) {
		writeError(ctx, w, http.StatusConflict, "conflict", "invoice is not editable in current state", nil)
		return
	}
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "quantity") || strings.Contains(msg, "description") || strings.Contains(msg, "unit_amount_minor") || strings.Contains(msg, "overflow") {
			writeError(ctx, w, http.StatusBadRequest, "validation_error", msg, nil)
			return
		}
		writeError(ctx, w, http.StatusInternalServerError, "internal_error", "could not patch invoice line items", nil)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":             updated.ID.String(),
		"invoice_number": updated.InvoiceNumber,
		"subtotal_minor": updated.SubtotalMinor,
		"tax_minor":      updated.TaxMinor,
		"total_minor":    updated.TotalMinor,
		"currency":       updated.Currency,
	})
}

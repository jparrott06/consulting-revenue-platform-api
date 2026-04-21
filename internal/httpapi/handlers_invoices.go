package httpapi

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/authz"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/config"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/domain"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/repo"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/usecase"
)

func writeInvoiceUsecaseError(ctx context.Context, w http.ResponseWriter, err error, action string) {
	if usecase.Kind(err) == usecase.ErrorKindConflict {
		recordWorkflowConflict("invoice", action)
	}
	writeUsecaseError(ctx, w, err)
}

func mountInvoiceRoutes(mux *http.ServeMux, cfg config.Config, db *sql.DB) {
	mux.Handle("POST /v1/invoices/generate", requireTenantAuth(cfg, db, requireRole(authz.ActionInvoiceWrite, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleGenerateInvoice(w, r, db)
	}))))
	mux.Handle("POST /v1/invoices/{invoiceID}/send", requireTenantAuth(cfg, db, requireRole(authz.ActionInvoiceWrite, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleSendInvoice(w, r, db)
	}))))
	mountInvoiceLineItemRoutes(mux, cfg, db)
	mountInvoicePDFRoutes(mux, cfg, db)
	mountInvoicePaymentRoutes(mux, cfg, db)
}

type generateInvoiceRequest struct {
	FromDate string  `json:"from_date"`
	ToDate   string  `json:"to_date"`
	Currency string  `json:"currency"`
	DueAt    *string `json:"due_at"`
}

type invoiceAuditLogger struct {
	db *sql.DB
}

func (l invoiceAuditLogger) LogInvoiceSent(ctx context.Context, organizationID, actorUserID uuid.UUID, invoice domain.InvoiceSummary) {
	invRef := invoice.ID
	logAudit(ctx, l.db, repo.InsertAuditLogParams{
		OrganizationID: &organizationID,
		ActorUserID:    &actorUserID,
		Action:         "invoice.sent",
		EntityType:     "invoice",
		EntityID:       &invRef,
		Metadata: map[string]any{
			"invoice_number": invoice.InvoiceNumber,
			"from_status":    "draft",
			"to_status":      invoice.Status,
		},
	})
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

	var req generateInvoiceRequest
	if !decodeJSONBody(ctx, w, r, &req) {
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

	svc := usecase.NewInvoiceWorkflowService(
		usecase.RepoInvoiceStore{DB: db},
		invoiceAuditLogger{db: db},
	)
	invoice, err := svc.Generate(ctx, usecase.InvoiceGenerateInput{
		OrganizationID: p.OrganizationID,
		FromDate:       from,
		ToDate:         to,
		Currency:       currency,
		DueAt:          due,
	})
	if err != nil {
		writeInvoiceUsecaseError(ctx, w, err, "generate")
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

func handleSendInvoice(w http.ResponseWriter, r *http.Request, db *sql.DB) {
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

	svc := usecase.NewInvoiceWorkflowService(
		usecase.RepoInvoiceStore{DB: db},
		invoiceAuditLogger{db: db},
	)
	invoice, err := svc.Send(ctx, usecase.InvoiceSendInput{
		OrganizationID: p.OrganizationID,
		InvoiceID:      invoiceID,
		ActorUserID:    p.UserID,
	})
	if err != nil {
		writeInvoiceUsecaseError(ctx, w, err, "send")
		return
	}
	out := map[string]any{
		"id":             invoice.ID.String(),
		"invoice_number": invoice.InvoiceNumber,
		"status":         invoice.Status,
		"subtotal_minor": invoice.SubtotalMinor,
		"tax_minor":      invoice.TaxMinor,
		"total_minor":    invoice.TotalMinor,
		"currency":       invoice.Currency,
	}
	if invoice.IssuedAt != nil {
		out["issued_at"] = invoice.IssuedAt.UTC().Format(time.RFC3339Nano)
	}
	writeJSON(w, http.StatusOK, out)
}

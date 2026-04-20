package usecase

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/domain"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/repo"
)

type InvoiceStore interface {
	GenerateInvoiceFromApprovedEntries(ctx context.Context, organizationID uuid.UUID, params repo.GenerateInvoiceParams) (domain.InvoiceSummary, error)
	SendInvoice(ctx context.Context, organizationID, invoiceID uuid.UUID) (domain.InvoiceSummary, error)
}

type RepoInvoiceStore struct {
	DB *sql.DB
}

func invoiceSummaryFromRecord(rec repo.InvoiceRecord) domain.InvoiceSummary {
	var issuedAt *time.Time
	if rec.IssuedAt.Valid {
		t := rec.IssuedAt.Time
		issuedAt = &t
	}
	return domain.InvoiceSummary{
		ID:            rec.ID,
		InvoiceNumber: rec.InvoiceNumber,
		Status:        rec.Status,
		Currency:      rec.Currency,
		SubtotalMinor: rec.SubtotalMinor,
		TaxMinor:      rec.TaxMinor,
		TotalMinor:    rec.TotalMinor,
		IssuedAt:      issuedAt,
	}
}

func (s RepoInvoiceStore) GenerateInvoiceFromApprovedEntries(ctx context.Context, organizationID uuid.UUID, params repo.GenerateInvoiceParams) (domain.InvoiceSummary, error) {
	rec, err := repo.GenerateInvoiceFromApprovedEntries(ctx, s.DB, organizationID, params)
	if err != nil {
		return domain.InvoiceSummary{}, err
	}
	return invoiceSummaryFromRecord(rec), nil
}

func (s RepoInvoiceStore) SendInvoice(ctx context.Context, organizationID, invoiceID uuid.UUID) (domain.InvoiceSummary, error) {
	rec, err := repo.SendInvoice(ctx, s.DB, organizationID, invoiceID)
	if err != nil {
		return domain.InvoiceSummary{}, err
	}
	return invoiceSummaryFromRecord(rec), nil
}

type InvoiceAuditLogger interface {
	LogInvoiceSent(ctx context.Context, organizationID, actorUserID uuid.UUID, invoice domain.InvoiceSummary)
}

type InvoiceGenerateInput struct {
	OrganizationID uuid.UUID
	FromDate       time.Time
	ToDate         time.Time
	Currency       string
	DueAt          *time.Time
}

type InvoiceSendInput struct {
	OrganizationID uuid.UUID
	InvoiceID      uuid.UUID
	ActorUserID    uuid.UUID
}

type InvoiceWorkflowService struct {
	store InvoiceStore
	audit InvoiceAuditLogger
}

func NewInvoiceWorkflowService(store InvoiceStore, audit InvoiceAuditLogger) InvoiceWorkflowService {
	return InvoiceWorkflowService{store: store, audit: audit}
}

func (s InvoiceWorkflowService) Generate(ctx context.Context, in InvoiceGenerateInput) (domain.InvoiceSummary, error) {
	if in.ToDate.Before(in.FromDate) {
		return domain.InvoiceSummary{}, newError(ErrorKindValidation, "to_date must be on or after from_date", nil)
	}

	rec, err := s.store.GenerateInvoiceFromApprovedEntries(ctx, in.OrganizationID, repo.GenerateInvoiceParams{
		FromDate: in.FromDate,
		ToDate:   in.ToDate,
		Currency: in.Currency,
		DueAt:    in.DueAt,
	})
	if errors.Is(err, repo.ErrNoEligibleTimeEntries) {
		return domain.InvoiceSummary{}, newError(ErrorKindConflict, "no eligible approved uninvoiced entries in range", err)
	}
	if errors.Is(err, repo.ErrUnsupportedCurrency) || errors.Is(err, repo.ErrLineAmountOverflow) || errors.Is(err, repo.ErrInvoiceTotalOverflow) {
		return domain.InvoiceSummary{}, newError(ErrorKindValidation, err.Error(), err)
	}
	if err != nil {
		return domain.InvoiceSummary{}, newError(ErrorKindInternal, "could not generate invoice", err)
	}
	return rec, nil
}

func (s InvoiceWorkflowService) Send(ctx context.Context, in InvoiceSendInput) (domain.InvoiceSummary, error) {
	rec, err := s.store.SendInvoice(ctx, in.OrganizationID, in.InvoiceID)
	if errors.Is(err, repo.ErrInvoiceNotFound) {
		return domain.InvoiceSummary{}, newError(ErrorKindNotFound, "invoice not found", err)
	}
	if errors.Is(err, repo.ErrInvoiceNotSendable) {
		return domain.InvoiceSummary{}, newError(ErrorKindConflict, "invoice cannot be sent in current state", err)
	}
	if err != nil {
		return domain.InvoiceSummary{}, newError(ErrorKindInternal, "could not send invoice", err)
	}

	if s.audit != nil {
		s.audit.LogInvoiceSent(ctx, in.OrganizationID, in.ActorUserID, rec)
	}
	return rec, nil
}

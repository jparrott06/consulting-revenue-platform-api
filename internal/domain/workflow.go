package domain

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

type TimeEntryStatus string

const (
	TimeEntryStatusDraft     TimeEntryStatus = "draft"
	TimeEntryStatusSubmitted TimeEntryStatus = "submitted"
	TimeEntryStatusApproved  TimeEntryStatus = "approved"
	TimeEntryStatusInvoiced  TimeEntryStatus = "invoiced"
)

var (
	ErrTimeEntryInvalidTransition = errors.New("invalid time entry transition")
	ErrTimeEntryForbidden         = errors.New("time entry action forbidden")
	ErrTimeEntryRejectReason      = errors.New("reject reason required")
)

type TimeEntrySnapshot struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	UserID         uuid.UUID
	Status         TimeEntryStatus
}

func ValidateTimeEntrySubmit(current TimeEntryStatus, actorRole string, ownerUserID, actorUserID uuid.UUID) error {
	if actorRole != "contractor" {
		return ErrTimeEntryForbidden
	}
	if ownerUserID != actorUserID {
		return ErrTimeEntryForbidden
	}
	if current != TimeEntryStatusDraft {
		return ErrTimeEntryInvalidTransition
	}
	return nil
}

func ValidateTimeEntryApprove(current TimeEntryStatus, actorRole string) error {
	if actorRole != "owner" && actorRole != "accountant" {
		return ErrTimeEntryForbidden
	}
	if current != TimeEntryStatusSubmitted {
		return ErrTimeEntryInvalidTransition
	}
	return nil
}

func ValidateTimeEntryReject(current TimeEntryStatus, actorRole, reason string) error {
	if actorRole != "owner" && actorRole != "accountant" {
		return ErrTimeEntryForbidden
	}
	if strings.TrimSpace(reason) == "" {
		return ErrTimeEntryRejectReason
	}
	if current != TimeEntryStatusSubmitted {
		return ErrTimeEntryInvalidTransition
	}
	return nil
}

type InvoiceSummary struct {
	ID            uuid.UUID
	InvoiceNumber int64
	Status        string
	Currency      string
	SubtotalMinor int64
	TaxMinor      int64
	TotalMinor    int64
	IssuedAt      *time.Time
}

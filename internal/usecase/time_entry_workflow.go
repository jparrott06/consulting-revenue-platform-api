package usecase

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/domain"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/repo"
)

type TimeEntryWorkflowStore interface {
	GetTimeEntry(ctx context.Context, organizationID, entryID uuid.UUID) (domain.TimeEntrySnapshot, error)
	SubmitTimeEntry(ctx context.Context, organizationID, entryID, actorUserID uuid.UUID) error
	ApproveTimeEntry(ctx context.Context, organizationID, entryID, actorUserID uuid.UUID) error
	RejectTimeEntry(ctx context.Context, organizationID, entryID, actorUserID uuid.UUID, reason string) error
}

type RepoTimeEntryWorkflowStore struct {
	DB *sql.DB
}

func (s RepoTimeEntryWorkflowStore) GetTimeEntry(ctx context.Context, organizationID, entryID uuid.UUID) (domain.TimeEntrySnapshot, error) {
	rec, err := repo.GetTimeEntry(ctx, s.DB, organizationID, entryID)
	if err != nil {
		return domain.TimeEntrySnapshot{}, err
	}
	return domain.TimeEntrySnapshot{
		ID:             rec.ID,
		OrganizationID: rec.OrganizationID,
		UserID:         rec.UserID,
		Status:         domain.TimeEntryStatus(rec.Status),
	}, nil
}

func (s RepoTimeEntryWorkflowStore) SubmitTimeEntry(ctx context.Context, organizationID, entryID, actorUserID uuid.UUID) error {
	return repo.SubmitTimeEntry(ctx, s.DB, organizationID, entryID, actorUserID)
}

func (s RepoTimeEntryWorkflowStore) ApproveTimeEntry(ctx context.Context, organizationID, entryID, actorUserID uuid.UUID) error {
	return repo.ApproveTimeEntry(ctx, s.DB, organizationID, entryID, actorUserID)
}

func (s RepoTimeEntryWorkflowStore) RejectTimeEntry(ctx context.Context, organizationID, entryID, actorUserID uuid.UUID, reason string) error {
	return repo.RejectTimeEntry(ctx, s.DB, organizationID, entryID, actorUserID, reason)
}

type TimeEntryActionInput struct {
	OrganizationID uuid.UUID
	EntryID        uuid.UUID
	ActorUserID    uuid.UUID
	ActorRole      string
}

type TimeEntryRejectInput struct {
	TimeEntryActionInput
	Reason string
}

// TimeEntryWorkflowService orchestrates time entry workflow actions.
// Transaction ownership: each Submit/Approve/Reject call maps to one repo method that runs an atomic transaction (see docs/transaction-matrix.md).
type TimeEntryWorkflowService struct {
	store TimeEntryWorkflowStore
}

func NewTimeEntryWorkflowService(store TimeEntryWorkflowStore) TimeEntryWorkflowService {
	return TimeEntryWorkflowService{store: store}
}

func (s TimeEntryWorkflowService) Submit(ctx context.Context, in TimeEntryActionInput) error {
	rec, err := s.store.GetTimeEntry(ctx, in.OrganizationID, in.EntryID)
	if errors.Is(err, repo.ErrTimeEntryNotFound) {
		return newError(ErrorKindNotFound, "time entry not found", err)
	}
	if err != nil {
		return newError(ErrorKindInternal, "could not load time entry", err)
	}
	if err := domain.ValidateTimeEntrySubmit(rec.Status, in.ActorRole, rec.UserID, in.ActorUserID); err != nil {
		switch {
		case errors.Is(err, domain.ErrTimeEntryInvalidTransition):
			return newError(ErrorKindConflict, "time entry is not in draft state", err)
		case errors.Is(err, domain.ErrTimeEntryForbidden):
			if rec.UserID != in.ActorUserID {
				return newError(ErrorKindForbidden, "contractors may only submit their own entries", err)
			}
			return newError(ErrorKindForbidden, "only contractors can submit time entries", err)
		default:
			return newError(ErrorKindInternal, "could not submit time entry", err)
		}
	}

	err = s.store.SubmitTimeEntry(ctx, in.OrganizationID, in.EntryID, in.ActorUserID)
	if errors.Is(err, repo.ErrInvalidTimeEntryTransition) {
		return newError(ErrorKindConflict, "time entry is not in draft state", err)
	}
	if errors.Is(err, repo.ErrTimeEntryNotFound) {
		return newError(ErrorKindNotFound, "time entry not found", err)
	}
	if err != nil {
		return newError(ErrorKindInternal, "could not submit time entry", err)
	}
	return nil
}

func (s TimeEntryWorkflowService) Approve(ctx context.Context, in TimeEntryActionInput) error {
	rec, err := s.store.GetTimeEntry(ctx, in.OrganizationID, in.EntryID)
	if errors.Is(err, repo.ErrTimeEntryNotFound) {
		return newError(ErrorKindNotFound, "time entry not found", err)
	}
	if err != nil {
		return newError(ErrorKindInternal, "could not load time entry", err)
	}
	if err := domain.ValidateTimeEntryApprove(rec.Status, in.ActorRole); err != nil {
		switch {
		case errors.Is(err, domain.ErrTimeEntryInvalidTransition):
			return newError(ErrorKindConflict, "time entry is not in submitted state", err)
		case errors.Is(err, domain.ErrTimeEntryForbidden):
			return newError(ErrorKindForbidden, "only owner or accountant can approve", err)
		default:
			return newError(ErrorKindInternal, "could not approve time entry", err)
		}
	}

	err = s.store.ApproveTimeEntry(ctx, in.OrganizationID, in.EntryID, in.ActorUserID)
	if errors.Is(err, repo.ErrInvalidTimeEntryTransition) {
		return newError(ErrorKindConflict, "time entry is not in submitted state", err)
	}
	if errors.Is(err, repo.ErrTimeEntryNotFound) {
		return newError(ErrorKindNotFound, "time entry not found", err)
	}
	if err != nil {
		return newError(ErrorKindInternal, "could not approve time entry", err)
	}
	return nil
}

func (s TimeEntryWorkflowService) Reject(ctx context.Context, in TimeEntryRejectInput) error {
	if strings.TrimSpace(in.Reason) == "" {
		return newError(ErrorKindValidation, "reject reason is required", domain.ErrTimeEntryRejectReason)
	}

	rec, err := s.store.GetTimeEntry(ctx, in.OrganizationID, in.EntryID)
	if errors.Is(err, repo.ErrTimeEntryNotFound) {
		return newError(ErrorKindNotFound, "time entry not found", err)
	}
	if err != nil {
		return newError(ErrorKindInternal, "could not load time entry", err)
	}
	if err := domain.ValidateTimeEntryReject(rec.Status, in.ActorRole, in.Reason); err != nil {
		switch {
		case errors.Is(err, domain.ErrTimeEntryRejectReason):
			return newError(ErrorKindValidation, "reject reason is required", err)
		case errors.Is(err, domain.ErrTimeEntryInvalidTransition):
			return newError(ErrorKindConflict, "time entry is not in submitted state", err)
		case errors.Is(err, domain.ErrTimeEntryForbidden):
			return newError(ErrorKindForbidden, "only owner or accountant can reject", err)
		default:
			return newError(ErrorKindInternal, "could not reject time entry", err)
		}
	}

	err = s.store.RejectTimeEntry(ctx, in.OrganizationID, in.EntryID, in.ActorUserID, in.Reason)
	if errors.Is(err, repo.ErrRejectReasonRequired) {
		return newError(ErrorKindValidation, "reject reason is required", err)
	}
	if errors.Is(err, repo.ErrInvalidTimeEntryTransition) {
		return newError(ErrorKindConflict, "time entry is not in submitted state", err)
	}
	if errors.Is(err, repo.ErrTimeEntryNotFound) {
		return newError(ErrorKindNotFound, "time entry not found", err)
	}
	if err != nil {
		return newError(ErrorKindInternal, "could not reject time entry", err)
	}
	return nil
}

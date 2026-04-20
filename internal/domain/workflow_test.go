package domain

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestValidateTimeEntrySubmit(t *testing.T) {
	t.Parallel()

	owner := uuid.New()
	if err := ValidateTimeEntrySubmit(TimeEntryStatusDraft, "contractor", owner, owner); err != nil {
		t.Fatalf("expected valid submit, got %v", err)
	}
	if err := ValidateTimeEntrySubmit(TimeEntryStatusSubmitted, "contractor", owner, owner); !errors.Is(err, ErrTimeEntryInvalidTransition) {
		t.Fatalf("expected invalid transition, got %v", err)
	}
	if err := ValidateTimeEntrySubmit(TimeEntryStatusDraft, "owner", owner, owner); !errors.Is(err, ErrTimeEntryForbidden) {
		t.Fatalf("expected forbidden role, got %v", err)
	}
	if err := ValidateTimeEntrySubmit(TimeEntryStatusDraft, "contractor", owner, uuid.New()); !errors.Is(err, ErrTimeEntryForbidden) {
		t.Fatalf("expected forbidden owner mismatch, got %v", err)
	}
}

func TestValidateTimeEntryApprove(t *testing.T) {
	t.Parallel()

	if err := ValidateTimeEntryApprove(TimeEntryStatusSubmitted, "owner"); err != nil {
		t.Fatalf("expected owner approve to pass: %v", err)
	}
	if err := ValidateTimeEntryApprove(TimeEntryStatusSubmitted, "accountant"); err != nil {
		t.Fatalf("expected accountant approve to pass: %v", err)
	}
	if err := ValidateTimeEntryApprove(TimeEntryStatusSubmitted, "contractor"); !errors.Is(err, ErrTimeEntryForbidden) {
		t.Fatalf("expected forbidden role, got %v", err)
	}
	if err := ValidateTimeEntryApprove(TimeEntryStatusDraft, "owner"); !errors.Is(err, ErrTimeEntryInvalidTransition) {
		t.Fatalf("expected invalid transition, got %v", err)
	}
}

func TestValidateTimeEntryReject(t *testing.T) {
	t.Parallel()

	if err := ValidateTimeEntryReject(TimeEntryStatusSubmitted, "owner", "missing hours details"); err != nil {
		t.Fatalf("expected owner reject to pass: %v", err)
	}
	if err := ValidateTimeEntryReject(TimeEntryStatusSubmitted, "owner", " "); !errors.Is(err, ErrTimeEntryRejectReason) {
		t.Fatalf("expected reject reason error, got %v", err)
	}
	if err := ValidateTimeEntryReject(TimeEntryStatusSubmitted, "contractor", "reason"); !errors.Is(err, ErrTimeEntryForbidden) {
		t.Fatalf("expected forbidden role, got %v", err)
	}
	if err := ValidateTimeEntryReject(TimeEntryStatusDraft, "owner", "reason"); !errors.Is(err, ErrTimeEntryInvalidTransition) {
		t.Fatalf("expected invalid transition, got %v", err)
	}
}

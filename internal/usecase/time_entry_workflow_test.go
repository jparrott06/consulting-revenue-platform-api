package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/domain"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/repo"
)

type timeEntryStoreStub struct {
	getRec domain.TimeEntrySnapshot
	getErr error
}

func (s timeEntryStoreStub) GetTimeEntry(context.Context, uuid.UUID, uuid.UUID) (domain.TimeEntrySnapshot, error) {
	return s.getRec, s.getErr
}
func (timeEntryStoreStub) SubmitTimeEntry(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error {
	return nil
}
func (timeEntryStoreStub) ApproveTimeEntry(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error {
	return nil
}
func (timeEntryStoreStub) RejectTimeEntry(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string) error {
	return nil
}

func TestTimeEntryWorkflowSubmitForbidden(t *testing.T) {
	t.Parallel()

	ownerID := uuid.New()
	svc := NewTimeEntryWorkflowService(timeEntryStoreStub{
		getRec: domain.TimeEntrySnapshot{UserID: ownerID, Status: domain.TimeEntryStatusDraft},
	})
	err := svc.Submit(context.Background(), TimeEntryActionInput{
		OrganizationID: uuid.New(),
		EntryID:        uuid.New(),
		ActorUserID:    ownerID,
		ActorRole:      "owner",
	})
	if Kind(err) != ErrorKindForbidden {
		t.Fatalf("expected forbidden error kind, got %v (%v)", Kind(err), err)
	}
}

func TestTimeEntryWorkflowMapsNotFound(t *testing.T) {
	t.Parallel()

	svc := NewTimeEntryWorkflowService(timeEntryStoreStub{getErr: repo.ErrTimeEntryNotFound})
	err := svc.Approve(context.Background(), TimeEntryActionInput{
		OrganizationID: uuid.New(),
		EntryID:        uuid.New(),
		ActorUserID:    uuid.New(),
		ActorRole:      "owner",
	})
	if Kind(err) != ErrorKindNotFound {
		t.Fatalf("expected not_found kind, got %v (%v)", Kind(err), err)
	}
	if !errors.Is(err, repo.ErrTimeEntryNotFound) {
		t.Fatalf("expected wrapped not found cause")
	}
}

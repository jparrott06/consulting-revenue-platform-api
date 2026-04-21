package httpapi

import (
	"context"
	"database/sql"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/authz"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/config"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/usecase"
)

func mountTimeEntryWorkflowRoutes(mux *http.ServeMux, cfg config.Config, db *sql.DB) {
	mux.Handle("POST /v1/time-entries/{timeEntryID}/submit", requireTenantAuth(cfg, db, requireRole(authz.ActionTimeEntryWrite, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleSubmitTimeEntry(w, r, db)
	}))))

	mux.Handle("POST /v1/time-entries/{timeEntryID}/approve", requireTenantAuth(cfg, db, requireRole(authz.ActionTimeEntryWrite, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleApproveTimeEntry(w, r, db)
	}))))

	mux.Handle("POST /v1/time-entries/{timeEntryID}/reject", requireTenantAuth(cfg, db, requireRole(authz.ActionTimeEntryWrite, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleRejectTimeEntry(w, r, db)
	}))))
}

func parseTimeEntryIDParam(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	ctx := r.Context()
	entryID, err := uuid.Parse(strings.TrimSpace(r.PathValue("timeEntryID")))
	if err != nil {
		writeError(ctx, w, http.StatusBadRequest, "validation_error", "time entry id must be a UUID", nil)
		return uuid.Nil, false
	}
	return entryID, true
}

func handleSubmitTimeEntry(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	ctx := r.Context()
	p, ok := PrincipalFromContext(ctx)
	if !ok {
		writeError(ctx, w, http.StatusUnauthorized, "unauthorized", "missing principal", nil)
		return
	}
	if p.Role != "contractor" {
		writeError(ctx, w, http.StatusForbidden, "forbidden", "only contractors can submit time entries", nil)
		return
	}
	entryID, ok := parseTimeEntryIDParam(w, r)
	if !ok {
		return
	}
	svc := usecase.NewTimeEntryWorkflowService(usecase.RepoTimeEntryWorkflowStore{DB: db})
	err := svc.Submit(ctx, usecase.TimeEntryActionInput{
		OrganizationID: p.OrganizationID,
		EntryID:        entryID,
		ActorUserID:    p.UserID,
		ActorRole:      p.Role,
	})
	if err != nil {
		writeTimeEntryWorkflowUsecaseError(ctx, w, err, "submit")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func handleApproveTimeEntry(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	ctx := r.Context()
	p, ok := PrincipalFromContext(ctx)
	if !ok {
		writeError(ctx, w, http.StatusUnauthorized, "unauthorized", "missing principal", nil)
		return
	}
	if p.Role != "owner" && p.Role != "accountant" {
		writeError(ctx, w, http.StatusForbidden, "forbidden", "only owner or accountant can approve", nil)
		return
	}
	entryID, ok := parseTimeEntryIDParam(w, r)
	if !ok {
		return
	}
	svc := usecase.NewTimeEntryWorkflowService(usecase.RepoTimeEntryWorkflowStore{DB: db})
	err := svc.Approve(ctx, usecase.TimeEntryActionInput{
		OrganizationID: p.OrganizationID,
		EntryID:        entryID,
		ActorUserID:    p.UserID,
		ActorRole:      p.Role,
	})
	if err != nil {
		writeTimeEntryWorkflowUsecaseError(ctx, w, err, "approve")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type rejectTimeEntryRequest struct {
	Reason string `json:"reason"`
}

func handleRejectTimeEntry(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	ctx := r.Context()
	p, ok := PrincipalFromContext(ctx)
	if !ok {
		writeError(ctx, w, http.StatusUnauthorized, "unauthorized", "missing principal", nil)
		return
	}
	if p.Role != "owner" && p.Role != "accountant" {
		writeError(ctx, w, http.StatusForbidden, "forbidden", "only owner or accountant can reject", nil)
		return
	}
	entryID, ok := parseTimeEntryIDParam(w, r)
	if !ok {
		return
	}

	var req rejectTimeEntryRequest
	if !decodeJSONBody(ctx, w, r, &req) {
		return
	}

	svc := usecase.NewTimeEntryWorkflowService(usecase.RepoTimeEntryWorkflowStore{DB: db})
	err := svc.Reject(ctx, usecase.TimeEntryRejectInput{
		TimeEntryActionInput: usecase.TimeEntryActionInput{
			OrganizationID: p.OrganizationID,
			EntryID:        entryID,
			ActorUserID:    p.UserID,
			ActorRole:      p.Role,
		},
		Reason: req.Reason,
	})
	if err != nil {
		writeTimeEntryWorkflowUsecaseError(ctx, w, err, "reject")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeTimeEntryWorkflowUsecaseError(ctx context.Context, w http.ResponseWriter, err error, action string) {
	if usecase.Kind(err) == usecase.ErrorKindConflict {
		recordWorkflowConflict("time_entry", action)
	}
	writeUsecaseError(ctx, w, err)
}

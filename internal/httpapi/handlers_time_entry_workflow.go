package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/authz"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/config"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/repo"
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

	rec, err := repo.GetTimeEntry(ctx, db, p.OrganizationID, entryID)
	if errors.Is(err, repo.ErrTimeEntryNotFound) {
		writeError(ctx, w, http.StatusNotFound, "not_found", "time entry not found", nil)
		return
	}
	if err != nil {
		writeError(ctx, w, http.StatusInternalServerError, "internal_error", "could not load time entry", nil)
		return
	}
	if rec.UserID != p.UserID {
		writeError(ctx, w, http.StatusForbidden, "forbidden", "contractors may only submit their own entries", nil)
		return
	}

	err = repo.SubmitTimeEntry(ctx, db, p.OrganizationID, entryID, p.UserID)
	if errors.Is(err, repo.ErrInvalidTimeEntryTransition) {
		writeError(ctx, w, http.StatusConflict, "conflict", "time entry is not in draft state", nil)
		return
	}
	if errors.Is(err, repo.ErrTimeEntryNotFound) {
		writeError(ctx, w, http.StatusNotFound, "not_found", "time entry not found", nil)
		return
	}
	if err != nil {
		writeError(ctx, w, http.StatusInternalServerError, "internal_error", "could not submit time entry", nil)
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

	err := repo.ApproveTimeEntry(ctx, db, p.OrganizationID, entryID, p.UserID)
	if errors.Is(err, repo.ErrInvalidTimeEntryTransition) {
		writeError(ctx, w, http.StatusConflict, "conflict", "time entry is not in submitted state", nil)
		return
	}
	if errors.Is(err, repo.ErrTimeEntryNotFound) {
		writeError(ctx, w, http.StatusNotFound, "not_found", "time entry not found", nil)
		return
	}
	if err != nil {
		writeError(ctx, w, http.StatusInternalServerError, "internal_error", "could not approve time entry", nil)
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

	r.Body = http.MaxBytesReader(w, r.Body, maxTimeEntryBodyBytes)
	var req rejectTimeEntryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(ctx, w, http.StatusBadRequest, "validation_error", "invalid JSON body", nil)
		return
	}

	err := repo.RejectTimeEntry(ctx, db, p.OrganizationID, entryID, p.UserID, req.Reason)
	if errors.Is(err, repo.ErrRejectReasonRequired) {
		writeError(ctx, w, http.StatusBadRequest, "validation_error", "reject reason is required", nil)
		return
	}
	if errors.Is(err, repo.ErrInvalidTimeEntryTransition) {
		writeError(ctx, w, http.StatusConflict, "conflict", "time entry is not in submitted state", nil)
		return
	}
	if errors.Is(err, repo.ErrTimeEntryNotFound) {
		writeError(ctx, w, http.StatusNotFound, "not_found", "time entry not found", nil)
		return
	}
	if err != nil {
		writeError(ctx, w, http.StatusInternalServerError, "internal_error", "could not reject time entry", nil)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

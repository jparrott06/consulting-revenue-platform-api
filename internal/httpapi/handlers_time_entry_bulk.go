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

const maxBulkTimeEntryIDs = 100

type bulkActionRequest struct {
	TimeEntryIDs []string `json:"time_entry_ids"`
}

type bulkActionItemResult struct {
	ID      string `json:"id"`
	Success bool   `json:"success"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

func mountTimeEntryBulkRoutes(mux *http.ServeMux, cfg config.Config, db *sql.DB) {
	mux.Handle("POST /v1/time-entries/bulk-submit", requireTenantAuth(cfg, db, requireRole(authz.ActionTimeEntryWrite, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleBulkSubmitTimeEntries(w, r, db)
	}))))
	mux.Handle("POST /v1/time-entries/bulk-approve", requireTenantAuth(cfg, db, requireRole(authz.ActionTimeEntryWrite, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleBulkApproveTimeEntries(w, r, db)
	}))))
}

func parseBulkActionRequest(r *http.Request, w http.ResponseWriter) ([]uuid.UUID, bool) {
	ctx := r.Context()
	var req bulkActionRequest
	if !decodeJSONBody(ctx, w, r, &req) {
		return nil, false
	}
	if len(req.TimeEntryIDs) == 0 {
		writeError(ctx, w, http.StatusBadRequest, "validation_error", "time_entry_ids is required", nil)
		return nil, false
	}
	if len(req.TimeEntryIDs) > maxBulkTimeEntryIDs {
		writeError(ctx, w, http.StatusBadRequest, "validation_error", "bulk limit exceeded (max 100)", nil)
		return nil, false
	}
	ids := make([]uuid.UUID, 0, len(req.TimeEntryIDs))
	for _, raw := range req.TimeEntryIDs {
		id, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil {
			writeError(ctx, w, http.StatusBadRequest, "validation_error", "all time_entry_ids must be UUIDs", nil)
			return nil, false
		}
		ids = append(ids, id)
	}
	return ids, true
}

func handleBulkSubmitTimeEntries(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	ctx := r.Context()
	p, ok := PrincipalFromContext(ctx)
	if !ok {
		writeError(ctx, w, http.StatusUnauthorized, "unauthorized", "missing principal", nil)
		return
	}
	if p.Role != "contractor" {
		writeError(ctx, w, http.StatusForbidden, "forbidden", "only contractors can bulk submit", nil)
		return
	}
	ids, ok := parseBulkActionRequest(r, w)
	if !ok {
		return
	}

	results := make([]bulkActionItemResult, 0, len(ids))
	for _, id := range ids {
		res := bulkActionItemResult{ID: id.String()}
		rec, err := repo.GetTimeEntry(ctx, db, p.OrganizationID, id)
		if err != nil {
			res.Code = "not_found"
			res.Message = "time entry not found"
			results = append(results, res)
			continue
		}
		if rec.UserID != p.UserID {
			res.Code = "forbidden"
			res.Message = "contractors may only submit their own entries"
			results = append(results, res)
			continue
		}
		err = repo.SubmitTimeEntry(ctx, db, p.OrganizationID, id, p.UserID)
		if err == nil {
			res.Success = true
			results = append(results, res)
			continue
		}
		if errors.Is(err, repo.ErrInvalidTimeEntryTransition) {
			res.Code = "conflict"
			res.Message = "time entry is not in draft state"
		} else if errors.Is(err, repo.ErrTimeEntryNotFound) {
			res.Code = "not_found"
			res.Message = "time entry not found"
		} else {
			res.Code = "internal_error"
			res.Message = "could not submit time entry"
		}
		results = append(results, res)
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

func handleBulkApproveTimeEntries(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	ctx := r.Context()
	p, ok := PrincipalFromContext(ctx)
	if !ok {
		writeError(ctx, w, http.StatusUnauthorized, "unauthorized", "missing principal", nil)
		return
	}
	if p.Role != "owner" && p.Role != "accountant" {
		writeError(ctx, w, http.StatusForbidden, "forbidden", "only owner or accountant can bulk approve", nil)
		return
	}
	ids, ok := parseBulkActionRequest(r, w)
	if !ok {
		return
	}

	results := make([]bulkActionItemResult, 0, len(ids))
	for _, id := range ids {
		res := bulkActionItemResult{ID: id.String()}
		err := repo.ApproveTimeEntry(ctx, db, p.OrganizationID, id, p.UserID)
		if err == nil {
			res.Success = true
			results = append(results, res)
			continue
		}
		if errors.Is(err, repo.ErrInvalidTimeEntryTransition) {
			res.Code = "conflict"
			res.Message = "time entry is not in submitted state"
		} else if errors.Is(err, repo.ErrTimeEntryNotFound) {
			res.Code = "not_found"
			res.Message = "time entry not found"
		} else {
			res.Code = "internal_error"
			res.Message = "could not approve time entry"
		}
		results = append(results, res)
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

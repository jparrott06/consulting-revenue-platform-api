package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/authz"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/config"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/repo"
)

const maxTimeEntryBodyBytes = 1 << 20

func mountTimeEntryRoutes(mux *http.ServeMux, cfg config.Config, db *sql.DB) {
	mux.Handle("GET /v1/time-entries", requireTenantAuth(cfg, db, requireRole(authz.ActionTimeEntryRead, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleListTimeEntries(w, r, db)
	}))))

	mux.Handle("POST /v1/time-entries", requireTenantAuth(cfg, db, requireRole(authz.ActionTimeEntryWrite, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleCreateTimeEntry(w, r, db)
	}))))

	mux.Handle("PATCH /v1/time-entries/{timeEntryID}", requireTenantAuth(cfg, db, requireRole(authz.ActionTimeEntryWrite, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePatchTimeEntry(w, r, db)
	}))))
}

func handleListTimeEntries(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	ctx := r.Context()
	p, ok := PrincipalFromContext(ctx)
	if !ok {
		writeError(ctx, w, http.StatusUnauthorized, "unauthorized", "missing principal", nil)
		return
	}
	if db == nil {
		writeError(ctx, w, http.StatusServiceUnavailable, "service_unavailable", "database is not configured", nil)
		return
	}

	filters := repo.ListTimeEntryFilters{}
	q := r.URL.Query()
	if s := strings.TrimSpace(q.Get("project_id")); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			writeError(ctx, w, http.StatusBadRequest, "validation_error", "project_id must be a UUID", nil)
			return
		}
		filters.ProjectID = &id
	}
	if s := strings.TrimSpace(q.Get("user_id")); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			writeError(ctx, w, http.StatusBadRequest, "validation_error", "user_id must be a UUID", nil)
			return
		}
		if p.Role == "contractor" && id != p.UserID {
			writeError(ctx, w, http.StatusForbidden, "forbidden", "contractors may only list their own entries", nil)
			return
		}
		filters.UserID = &id
	} else if p.Role == "contractor" {
		filters.UserID = &p.UserID
	}
	if s := strings.TrimSpace(q.Get("status")); s != "" {
		filters.Status = &s
	}
	if s := strings.TrimSpace(q.Get("from")); s != "" {
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			writeError(ctx, w, http.StatusBadRequest, "validation_error", "from must be YYYY-MM-DD", nil)
			return
		}
		filters.From = &t
	}
	if s := strings.TrimSpace(q.Get("to")); s != "" {
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			writeError(ctx, w, http.StatusBadRequest, "validation_error", "to must be YYYY-MM-DD", nil)
			return
		}
		filters.To = &t
	}

	items, err := repo.ListTimeEntries(ctx, db, p.OrganizationID, filters)
	if err != nil {
		writeError(ctx, w, http.StatusInternalServerError, "internal_error", "could not list time entries", nil)
		return
	}

	type row struct {
		ID              string  `json:"id"`
		ProjectID       string  `json:"project_id"`
		UserID          string  `json:"user_id"`
		WorkDate        string  `json:"work_date"`
		Minutes         int     `json:"minutes"`
		HourlyRateMinor int64   `json:"hourly_rate_minor"`
		Status          string  `json:"status"`
		Notes           *string `json:"notes,omitempty"`
		CreatedAt       string  `json:"created_at"`
		UpdatedAt       string  `json:"updated_at"`
	}
	out := make([]row, 0, len(items))
	for _, it := range items {
		rw := row{
			ID:              it.ID.String(),
			ProjectID:       it.ProjectID.String(),
			UserID:          it.UserID.String(),
			WorkDate:        it.WorkDate.Format("2006-01-02"),
			Minutes:         it.Minutes,
			HourlyRateMinor: it.HourlyRateMinor,
			Status:          it.Status,
			CreatedAt:       it.CreatedAt.UTC().Format(time.RFC3339Nano),
			UpdatedAt:       it.UpdatedAt.UTC().Format(time.RFC3339Nano),
		}
		if it.Notes.Valid {
			s := it.Notes.String
			rw.Notes = &s
		}
		out = append(out, rw)
	}
	writeJSON(w, http.StatusOK, out)
}

type createTimeEntryRequest struct {
	ProjectID       string  `json:"project_id"`
	UserID          *string `json:"user_id"`
	WorkDate        string  `json:"work_date"`
	Minutes         int     `json:"minutes"`
	HourlyRateMinor int64   `json:"hourly_rate_minor"`
	Notes           *string `json:"notes"`
}

func handleCreateTimeEntry(w http.ResponseWriter, r *http.Request, db *sql.DB) {
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

	r.Body = http.MaxBytesReader(w, r.Body, maxTimeEntryBodyBytes)
	var req createTimeEntryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(ctx, w, http.StatusBadRequest, "validation_error", "invalid JSON body", nil)
		return
	}

	projectID, err := uuid.Parse(strings.TrimSpace(req.ProjectID))
	if err != nil {
		writeError(ctx, w, http.StatusBadRequest, "validation_error", "project_id must be a UUID", nil)
		return
	}
	wd, err := time.Parse("2006-01-02", strings.TrimSpace(req.WorkDate))
	if err != nil {
		writeError(ctx, w, http.StatusBadRequest, "validation_error", "work_date must be YYYY-MM-DD", nil)
		return
	}

	targetUser := p.UserID
	if req.UserID != nil && strings.TrimSpace(*req.UserID) != "" {
		uid, err := uuid.Parse(strings.TrimSpace(*req.UserID))
		if err != nil {
			writeError(ctx, w, http.StatusBadRequest, "validation_error", "user_id must be a UUID", nil)
			return
		}
		if p.Role == "contractor" && uid != p.UserID {
			writeError(ctx, w, http.StatusForbidden, "forbidden", "contractors may only create entries for themselves", nil)
			return
		}
		targetUser = uid
	}

	id, err := repo.CreateTimeEntry(ctx, db, p.OrganizationID, projectID, targetUser, wd, req.Minutes, req.HourlyRateMinor, req.Notes)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "minutes must") || strings.Contains(msg, "hourly_rate_minor") {
			writeError(ctx, w, http.StatusBadRequest, "validation_error", msg, nil)
			return
		}
		if msg == "project is archived" {
			writeError(ctx, w, http.StatusBadRequest, "validation_error", msg, nil)
			return
		}
		if errors.Is(err, repo.ErrProjectNotFound) {
			writeError(ctx, w, http.StatusBadRequest, "validation_error", "project not found in organization", nil)
			return
		}
		writeError(ctx, w, http.StatusInternalServerError, "internal_error", "could not create time entry", nil)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"id": id.String()})
}

type patchTimeEntryRequest struct {
	ProjectID       *string `json:"project_id"`
	WorkDate        *string `json:"work_date"`
	Minutes         *int    `json:"minutes"`
	HourlyRateMinor *int64  `json:"hourly_rate_minor"`
	Notes           *string `json:"notes"`
}

func handlePatchTimeEntry(w http.ResponseWriter, r *http.Request, db *sql.DB) {
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

	entryID, err := uuid.Parse(strings.TrimSpace(r.PathValue("timeEntryID")))
	if err != nil {
		writeError(ctx, w, http.StatusBadRequest, "validation_error", "time entry id must be a UUID", nil)
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

	if p.Role == "contractor" {
		if rec.UserID != p.UserID {
			writeError(ctx, w, http.StatusForbidden, "forbidden", "contractors may only edit their own entries", nil)
			return
		}
		if rec.Status != "draft" {
			writeError(ctx, w, http.StatusForbidden, "forbidden", "contractors may only edit draft entries", nil)
			return
		}
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxTimeEntryBodyBytes)
	var req patchTimeEntryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(ctx, w, http.StatusBadRequest, "validation_error", "invalid JSON body", nil)
		return
	}
	if req.ProjectID == nil && req.WorkDate == nil && req.Minutes == nil && req.HourlyRateMinor == nil && req.Notes == nil {
		writeError(ctx, w, http.StatusBadRequest, "validation_error", "no fields to update", nil)
		return
	}

	var projectID *uuid.UUID
	if req.ProjectID != nil {
		id, err := uuid.Parse(strings.TrimSpace(*req.ProjectID))
		if err != nil {
			writeError(ctx, w, http.StatusBadRequest, "validation_error", "project_id must be a UUID", nil)
			return
		}
		projectID = &id
	}
	var workDate *time.Time
	if req.WorkDate != nil {
		wd, err := time.Parse("2006-01-02", strings.TrimSpace(*req.WorkDate))
		if err != nil {
			writeError(ctx, w, http.StatusBadRequest, "validation_error", "work_date must be YYYY-MM-DD", nil)
			return
		}
		workDate = &wd
	}

	err = repo.UpdateTimeEntry(ctx, db, p.OrganizationID, entryID, projectID, workDate, req.Minutes, req.HourlyRateMinor, req.Notes)
	if errors.Is(err, repo.ErrTimeEntryLocked) {
		writeError(ctx, w, http.StatusConflict, "conflict", "time entry cannot be edited in its current state", nil)
		return
	}
	if errors.Is(err, repo.ErrTimeEntryNotFound) {
		writeError(ctx, w, http.StatusNotFound, "not_found", "time entry not found", nil)
		return
	}
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "minutes must") || strings.Contains(msg, "hourly_rate_minor") {
			writeError(ctx, w, http.StatusBadRequest, "validation_error", msg, nil)
			return
		}
		if msg == "project is archived" {
			writeError(ctx, w, http.StatusBadRequest, "validation_error", msg, nil)
			return
		}
		if errors.Is(err, repo.ErrProjectNotFound) {
			writeError(ctx, w, http.StatusBadRequest, "validation_error", "project not found in organization", nil)
			return
		}
		writeError(ctx, w, http.StatusInternalServerError, "internal_error", "could not update time entry", nil)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

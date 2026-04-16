package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/authz"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/config"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/repo"
)

const maxProjectBodyBytes = 1 << 20

func mountProjectRoutes(mux *http.ServeMux, cfg config.Config, db *sql.DB) {
	mux.Handle("GET /v1/projects", requireTenantAuth(cfg, db, requireRole(authz.ActionProjectRead, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleListProjects(w, r, db)
	}))))

	mux.Handle("POST /v1/projects", requireTenantAuth(cfg, db, requireRole(authz.ActionProjectWrite, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleCreateProject(w, r, db)
	}))))

	mux.Handle("PATCH /v1/projects/{projectID}", requireTenantAuth(cfg, db, requireRole(authz.ActionProjectWrite, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePatchProject(w, r, db)
	}))))
}

func handleListProjects(w http.ResponseWriter, r *http.Request, db *sql.DB) {
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

	includeArchived, _ := strconv.ParseBool(strings.TrimSpace(r.URL.Query().Get("include_archived")))
	if includeArchived && !authz.RoleAllows(p.Role, authz.ActionProjectWrite) {
		writeError(ctx, w, http.StatusForbidden, "forbidden", "insufficient permissions to list archived projects", nil)
		return
	}

	items, err := repo.ListProjects(ctx, db, p.OrganizationID, includeArchived)
	if err != nil {
		writeError(ctx, w, http.StatusInternalServerError, "internal_error", "could not list projects", nil)
		return
	}

	type row struct {
		ID               string `json:"id"`
		ClientID         string `json:"client_id"`
		Name             string `json:"name"`
		BillingMode      string `json:"billing_mode"`
		DefaultRateMinor int64  `json:"default_rate_minor"`
		Archived         bool   `json:"archived"`
		CreatedAt        string `json:"created_at"`
		UpdatedAt        string `json:"updated_at"`
	}
	out := make([]row, 0, len(items))
	for _, it := range items {
		out = append(out, row{
			ID:               it.ID.String(),
			ClientID:         it.ClientID.String(),
			Name:             it.Name,
			BillingMode:      it.BillingMode,
			DefaultRateMinor: it.DefaultRateMinor,
			Archived:         it.Archived,
			CreatedAt:        it.CreatedAt.UTC().Format(time.RFC3339Nano),
			UpdatedAt:        it.UpdatedAt.UTC().Format(time.RFC3339Nano),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

type createProjectRequest struct {
	ClientID         string `json:"client_id"`
	Name             string `json:"name"`
	BillingMode      string `json:"billing_mode"`
	DefaultRateMinor int64  `json:"default_rate_minor"`
}

func handleCreateProject(w http.ResponseWriter, r *http.Request, db *sql.DB) {
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

	r.Body = http.MaxBytesReader(w, r.Body, maxProjectBodyBytes)
	var req createProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(ctx, w, http.StatusBadRequest, "validation_error", "invalid JSON body", nil)
		return
	}

	clientID, err := uuid.Parse(strings.TrimSpace(req.ClientID))
	if err != nil {
		writeError(ctx, w, http.StatusBadRequest, "validation_error", "client_id must be a UUID", nil)
		return
	}

	id, err := repo.CreateProject(ctx, db, p.OrganizationID, clientID, req.Name, req.BillingMode, req.DefaultRateMinor)
	if errors.Is(err, repo.ErrClientNotFound) {
		writeError(ctx, w, http.StatusBadRequest, "validation_error", "client not found in organization", nil)
		return
	}
	if err != nil {
		msg := err.Error()
		if msg == "name is required" || errors.Is(err, repo.ErrInvalidBillingMode) || msg == "default_rate_minor must be non-negative" {
			writeError(ctx, w, http.StatusBadRequest, "validation_error", msg, nil)
			return
		}
		writeError(ctx, w, http.StatusInternalServerError, "internal_error", "could not create project", nil)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"id": id.String()})
}

type patchProjectRequest struct {
	Name             *string `json:"name"`
	BillingMode      *string `json:"billing_mode"`
	DefaultRateMinor *int64  `json:"default_rate_minor"`
	Archived         *bool   `json:"archived"`
}

func handlePatchProject(w http.ResponseWriter, r *http.Request, db *sql.DB) {
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

	projectID, err := uuid.Parse(strings.TrimSpace(r.PathValue("projectID")))
	if err != nil {
		writeError(ctx, w, http.StatusBadRequest, "validation_error", "project id must be a UUID", nil)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxProjectBodyBytes)
	var req patchProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(ctx, w, http.StatusBadRequest, "validation_error", "invalid JSON body", nil)
		return
	}
	if req.Name == nil && req.BillingMode == nil && req.DefaultRateMinor == nil && req.Archived == nil {
		writeError(ctx, w, http.StatusBadRequest, "validation_error", "no fields to update", nil)
		return
	}

	err = repo.UpdateProject(ctx, db, p.OrganizationID, projectID, req.Name, req.BillingMode, req.DefaultRateMinor, req.Archived)
	if errors.Is(err, repo.ErrProjectNotFound) {
		writeError(ctx, w, http.StatusNotFound, "not_found", "project not found", nil)
		return
	}
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "cannot be empty") || errors.Is(err, repo.ErrInvalidBillingMode) || strings.Contains(msg, "default_rate_minor must be") {
			writeError(ctx, w, http.StatusBadRequest, "validation_error", msg, nil)
			return
		}
		writeError(ctx, w, http.StatusInternalServerError, "internal_error", "could not update project", nil)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

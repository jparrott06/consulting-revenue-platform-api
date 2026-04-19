package httpapi

import (
	"database/sql"
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

func mountClientRoutes(mux *http.ServeMux, cfg config.Config, db *sql.DB) {
	mux.Handle("GET /v1/clients", requireTenantAuth(cfg, db, requireRole(authz.ActionClientRead, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleListClients(w, r, db)
	}))))

	mux.Handle("POST /v1/clients", requireTenantAuth(cfg, db, requireRole(authz.ActionClientWrite, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleCreateClient(w, r, db)
	}))))

	mux.Handle("GET /v1/clients/{clientID}", requireTenantAuth(cfg, db, requireRole(authz.ActionClientRead, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleGetClient(w, r, db)
	}))))

	mux.Handle("PATCH /v1/clients/{clientID}", requireTenantAuth(cfg, db, requireRole(authz.ActionClientWrite, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePatchClient(w, r, db)
	}))))

	mux.Handle("DELETE /v1/clients/{clientID}", requireTenantAuth(cfg, db, requireRole(authz.ActionClientWrite, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleDeleteClient(w, r, db)
	}))))
}

func handleListClients(w http.ResponseWriter, r *http.Request, db *sql.DB) {
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

	includeDeleted, _ := strconv.ParseBool(strings.TrimSpace(r.URL.Query().Get("include_deleted")))
	if includeDeleted && !authz.RoleAllows(p.Role, authz.ActionClientWrite) {
		writeError(ctx, w, http.StatusForbidden, "forbidden", "insufficient permissions to list deleted clients", nil)
		return
	}

	items, err := repo.ListClients(ctx, db, p.OrganizationID, includeDeleted)
	if err != nil {
		writeError(ctx, w, http.StatusInternalServerError, "internal_error", "could not list clients", nil)
		return
	}

	type row struct {
		ID                 string  `json:"id"`
		Name               string  `json:"name"`
		BillingEmail       string  `json:"billing_email"`
		CurrencyPreference string  `json:"currency_preference"`
		DeletedAt          *string `json:"deleted_at,omitempty"`
		CreatedAt          string  `json:"created_at"`
		UpdatedAt          string  `json:"updated_at"`
	}
	out := make([]row, 0, len(items))
	for _, it := range items {
		rw := row{
			ID:                 it.ID.String(),
			Name:               it.Name,
			BillingEmail:       it.BillingEmail,
			CurrencyPreference: it.CurrencyPreference,
			CreatedAt:          it.CreatedAt.UTC().Format(time.RFC3339Nano),
			UpdatedAt:          it.UpdatedAt.UTC().Format(time.RFC3339Nano),
		}
		if it.DeletedAt.Valid {
			s := it.DeletedAt.Time.UTC().Format(time.RFC3339Nano)
			rw.DeletedAt = &s
		}
		out = append(out, rw)
	}
	writeJSON(w, http.StatusOK, out)
}

type createClientRequest struct {
	Name               string `json:"name"`
	BillingEmail       string `json:"billing_email"`
	CurrencyPreference string `json:"currency_preference"`
}

func handleCreateClient(w http.ResponseWriter, r *http.Request, db *sql.DB) {
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

	var req createClientRequest
	if !decodeJSONBody(ctx, w, r, &req) {
		return
	}

	id, err := repo.CreateClient(ctx, db, p.OrganizationID, req.Name, req.BillingEmail, req.CurrencyPreference)
	if err != nil {
		msg := err.Error()
		if msg == "name is required" || msg == "billing_email is required" || strings.Contains(msg, "currency") || errors.Is(err, repo.ErrUnsupportedCurrency) {
			writeError(ctx, w, http.StatusBadRequest, "validation_error", msg, nil)
			return
		}
		writeError(ctx, w, http.StatusInternalServerError, "internal_error", "could not create client", nil)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"id": id.String()})
}

type patchClientRequest struct {
	Name               *string `json:"name"`
	BillingEmail       *string `json:"billing_email"`
	CurrencyPreference *string `json:"currency_preference"`
}

func handleGetClient(w http.ResponseWriter, r *http.Request, db *sql.DB) {
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

	clientID, err := uuid.Parse(strings.TrimSpace(r.PathValue("clientID")))
	if err != nil {
		writeError(ctx, w, http.StatusBadRequest, "validation_error", "client id must be a UUID", nil)
		return
	}

	rec, err := repo.GetClient(ctx, db, p.OrganizationID, clientID)
	if errors.Is(err, repo.ErrClientNotFound) {
		writeError(ctx, w, http.StatusNotFound, "not_found", "client not found", nil)
		return
	}
	if err != nil {
		writeError(ctx, w, http.StatusInternalServerError, "internal_error", "could not load client", nil)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":                  rec.ID.String(),
		"name":                rec.Name,
		"billing_email":       rec.BillingEmail,
		"currency_preference": rec.CurrencyPreference,
		"created_at":          rec.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at":          rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	})
}

func handlePatchClient(w http.ResponseWriter, r *http.Request, db *sql.DB) {
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

	clientID, err := uuid.Parse(strings.TrimSpace(r.PathValue("clientID")))
	if err != nil {
		writeError(ctx, w, http.StatusBadRequest, "validation_error", "client id must be a UUID", nil)
		return
	}

	var req patchClientRequest
	if !decodeJSONBody(ctx, w, r, &req) {
		return
	}
	if req.Name == nil && req.BillingEmail == nil && req.CurrencyPreference == nil {
		writeError(ctx, w, http.StatusBadRequest, "validation_error", "no fields to update", nil)
		return
	}

	err = repo.UpdateClient(ctx, db, p.OrganizationID, clientID, req.Name, req.BillingEmail, req.CurrencyPreference)
	if errors.Is(err, repo.ErrClientNotFound) {
		writeError(ctx, w, http.StatusNotFound, "not_found", "client not found", nil)
		return
	}
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "cannot be empty") || strings.Contains(msg, "currency") || errors.Is(err, repo.ErrUnsupportedCurrency) {
			writeError(ctx, w, http.StatusBadRequest, "validation_error", msg, nil)
			return
		}
		writeError(ctx, w, http.StatusInternalServerError, "internal_error", "could not update client", nil)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func handleDeleteClient(w http.ResponseWriter, r *http.Request, db *sql.DB) {
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

	clientID, err := uuid.Parse(strings.TrimSpace(r.PathValue("clientID")))
	if err != nil {
		writeError(ctx, w, http.StatusBadRequest, "validation_error", "client id must be a UUID", nil)
		return
	}

	err = repo.SoftDeleteClient(ctx, db, p.OrganizationID, clientID)
	if errors.Is(err, repo.ErrClientNotFound) {
		writeError(ctx, w, http.StatusNotFound, "not_found", "client not found", nil)
		return
	}
	if err != nil {
		writeError(ctx, w, http.StatusInternalServerError, "internal_error", "could not delete client", nil)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

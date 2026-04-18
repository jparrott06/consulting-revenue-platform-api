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

const maxMembershipBodyBytes = 1 << 20

func mountMembershipRoutes(mux *http.ServeMux, cfg config.Config, db *sql.DB) {
	mux.Handle("GET /v1/memberships", requireTenantAuth(cfg, db, requireRole(authz.ActionMembershipRead, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleListMemberships(w, r, db)
	}))))

	mux.Handle("POST /v1/memberships", requireTenantAuth(cfg, db, requireRole(authz.ActionMembershipWrite, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleCreateMembership(w, r, db)
	}))))

	mux.Handle("PATCH /v1/memberships/{membershipID}", requireTenantAuth(cfg, db, requireRole(authz.ActionMembershipWrite, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlePatchMembership(w, r, db)
	}))))

	mux.Handle("DELETE /v1/memberships/{membershipID}", requireTenantAuth(cfg, db, requireRole(authz.ActionMembershipWrite, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleDeleteMembership(w, r, db)
	}))))
}

func handleListMemberships(w http.ResponseWriter, r *http.Request, db *sql.DB) {
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

	items, err := repo.ListMembershipsForOrganization(ctx, db, p.OrganizationID)
	if err != nil {
		writeError(ctx, w, http.StatusInternalServerError, "internal_error", "could not list memberships", nil)
		return
	}

	type row struct {
		ID        string `json:"id"`
		UserID    string `json:"user_id"`
		Email     string `json:"email"`
		FullName  string `json:"full_name"`
		Role      string `json:"role"`
		Status    string `json:"status"`
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
	}
	out := make([]row, 0, len(items))
	for _, it := range items {
		out = append(out, row{
			ID:        it.ID.String(),
			UserID:    it.UserID.String(),
			Email:     it.Email,
			FullName:  it.FullName,
			Role:      it.Role,
			Status:    it.Status,
			CreatedAt: it.CreatedAt.UTC().Format(time.RFC3339Nano),
			UpdatedAt: it.UpdatedAt.UTC().Format(time.RFC3339Nano),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

type createMembershipRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

func handleCreateMembership(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	ctx := r.Context()
	if db == nil {
		writeError(ctx, w, http.StatusServiceUnavailable, "service_unavailable", "database is not configured", nil)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxMembershipBodyBytes)
	var req createMembershipRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(ctx, w, http.StatusBadRequest, "validation_error", "invalid JSON body", nil)
		return
	}
	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" || strings.TrimSpace(req.Role) == "" {
		writeError(ctx, w, http.StatusBadRequest, "validation_error", "email and role are required", nil)
		return
	}

	p, ok := PrincipalFromContext(ctx)
	if !ok {
		writeError(ctx, w, http.StatusUnauthorized, "unauthorized", "missing principal", nil)
		return
	}

	targetUser, err := repo.GetUserIDByEmail(ctx, db, req.Email)
	if errors.Is(err, repo.ErrUserNotFound) {
		writeError(ctx, w, http.StatusBadRequest, "validation_error", "user not found for email", nil)
		return
	}
	if err != nil {
		writeError(ctx, w, http.StatusInternalServerError, "internal_error", "could not resolve user", nil)
		return
	}

	if _, err := repo.FindMembershipForUserOrganization(ctx, db, targetUser, p.OrganizationID); err == nil {
		writeError(ctx, w, http.StatusConflict, "conflict", "membership already exists", nil)
		return
	} else if !errors.Is(err, repo.ErrMembershipNotFound) {
		writeError(ctx, w, http.StatusInternalServerError, "internal_error", "could not validate membership", nil)
		return
	}

	id, err := repo.CreateActiveMembership(ctx, db, p.OrganizationID, targetUser, req.Role)
	if errors.Is(err, repo.ErrDuplicateMembership) {
		writeError(ctx, w, http.StatusConflict, "conflict", "membership already exists", nil)
		return
	}
	if errors.Is(err, repo.ErrInvalidMembershipRole) {
		writeError(ctx, w, http.StatusBadRequest, "validation_error", "invalid role", nil)
		return
	}
	if err != nil {
		writeError(ctx, w, http.StatusInternalServerError, "internal_error", "could not create membership", nil)
		return
	}

	mid := id
	logAudit(ctx, db, repo.InsertAuditLogParams{
		OrganizationID: &p.OrganizationID,
		ActorUserID:    &p.UserID,
		Action:         "membership.created",
		EntityType:     "membership",
		EntityID:       &mid,
		Metadata: map[string]any{
			"target_user_id": targetUser.String(),
			"role":           strings.TrimSpace(req.Role),
		},
	})

	writeJSON(w, http.StatusCreated, map[string]string{
		"id": id.String(),
	})
}

type patchMembershipRequest struct {
	Role string `json:"role"`
}

func handlePatchMembership(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	ctx := r.Context()
	if db == nil {
		writeError(ctx, w, http.StatusServiceUnavailable, "service_unavailable", "database is not configured", nil)
		return
	}

	membershipID, err := uuid.Parse(strings.TrimSpace(r.PathValue("membershipID")))
	if err != nil {
		writeError(ctx, w, http.StatusBadRequest, "validation_error", "membership id must be a UUID", nil)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxMembershipBodyBytes)
	var req patchMembershipRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(ctx, w, http.StatusBadRequest, "validation_error", "invalid JSON body", nil)
		return
	}
	if strings.TrimSpace(req.Role) == "" {
		writeError(ctx, w, http.StatusBadRequest, "validation_error", "role is required", nil)
		return
	}

	p, ok := PrincipalFromContext(ctx)
	if !ok {
		writeError(ctx, w, http.StatusUnauthorized, "unauthorized", "missing principal", nil)
		return
	}

	prev, errGet := repo.GetMembershipInOrganization(ctx, db, p.OrganizationID, membershipID)
	if errors.Is(errGet, repo.ErrMembershipNotFound) {
		writeError(ctx, w, http.StatusNotFound, "not_found", "membership not found", nil)
		return
	}
	if errGet != nil {
		writeError(ctx, w, http.StatusInternalServerError, "internal_error", "could not load membership", nil)
		return
	}

	err = repo.UpdateMembershipRole(ctx, db, p.OrganizationID, membershipID, req.Role)
	if errors.Is(err, repo.ErrLastOwnerProtected) {
		writeError(ctx, w, http.StatusConflict, "conflict", "cannot demote the last active owner", nil)
		return
	}
	if errors.Is(err, repo.ErrInvalidMembershipRole) {
		writeError(ctx, w, http.StatusBadRequest, "validation_error", "invalid role", nil)
		return
	}
	if errors.Is(err, repo.ErrMembershipNotFound) {
		writeError(ctx, w, http.StatusNotFound, "not_found", "membership not found", nil)
		return
	}
	if err != nil {
		writeError(ctx, w, http.StatusInternalServerError, "internal_error", "could not update membership", nil)
		return
	}

	mid := membershipID
	fromRole := prev.Role
	toRole := strings.TrimSpace(req.Role)
	logAudit(ctx, db, repo.InsertAuditLogParams{
		OrganizationID: &p.OrganizationID,
		ActorUserID:    &p.UserID,
		Action:         "membership.role_updated",
		EntityType:     "membership",
		EntityID:       &mid,
		Metadata: map[string]any{
			"from_role": fromRole,
			"to_role":   toRole,
		},
	})

	w.WriteHeader(http.StatusNoContent)
}

func handleDeleteMembership(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	ctx := r.Context()
	if db == nil {
		writeError(ctx, w, http.StatusServiceUnavailable, "service_unavailable", "database is not configured", nil)
		return
	}

	membershipID, err := uuid.Parse(strings.TrimSpace(r.PathValue("membershipID")))
	if err != nil {
		writeError(ctx, w, http.StatusBadRequest, "validation_error", "membership id must be a UUID", nil)
		return
	}

	p, ok := PrincipalFromContext(ctx)
	if !ok {
		writeError(ctx, w, http.StatusUnauthorized, "unauthorized", "missing principal", nil)
		return
	}

	prev, errLoad := repo.GetMembershipInOrganization(ctx, db, p.OrganizationID, membershipID)
	if errors.Is(errLoad, repo.ErrMembershipNotFound) {
		writeError(ctx, w, http.StatusNotFound, "not_found", "membership not found", nil)
		return
	}
	if errLoad != nil {
		writeError(ctx, w, http.StatusInternalServerError, "internal_error", "could not load membership", nil)
		return
	}

	err = repo.DeleteMembership(ctx, db, p.OrganizationID, membershipID)
	if errors.Is(err, repo.ErrLastOwnerProtected) {
		writeError(ctx, w, http.StatusConflict, "conflict", "cannot remove the last active owner", nil)
		return
	}
	if errors.Is(err, repo.ErrMembershipNotFound) {
		writeError(ctx, w, http.StatusNotFound, "not_found", "membership not found", nil)
		return
	}
	if err != nil {
		writeError(ctx, w, http.StatusInternalServerError, "internal_error", "could not delete membership", nil)
		return
	}

	mid := membershipID
	logAudit(ctx, db, repo.InsertAuditLogParams{
		OrganizationID: &p.OrganizationID,
		ActorUserID:    &p.UserID,
		Action:         "membership.removed",
		EntityType:     "membership",
		EntityID:       &mid,
		Metadata: map[string]any{
			"removed_user_id": prev.UserID.String(),
		},
	})

	w.WriteHeader(http.StatusNoContent)
}

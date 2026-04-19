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

func mountOrganizationRoutes(mux *http.ServeMux, cfg config.Config, db *sql.DB) {
	mux.Handle("POST /v1/organizations/{organizationID}/deactivate", requireTenantAuth(cfg, db, requireRole(authz.ActionOrganizationDeactivate, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleDeactivateOrganization(w, r, db)
	}))))
}

func handleDeactivateOrganization(w http.ResponseWriter, r *http.Request, db *sql.DB) {
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

	pathOrgID, err := uuid.Parse(strings.TrimSpace(r.PathValue("organizationID")))
	if err != nil {
		writeError(ctx, w, http.StatusBadRequest, "validation_error", "organization id must be a UUID", nil)
		return
	}
	if pathOrgID != p.OrganizationID {
		writeError(ctx, w, http.StatusForbidden, "forbidden", "organization id does not match active tenant", nil)
		return
	}

	err = repo.DeactivateOrganization(ctx, db, pathOrgID)
	if errors.Is(err, repo.ErrOrganizationNotFound) {
		writeError(ctx, w, http.StatusNotFound, "not_found", "organization not found", nil)
		return
	}
	if err != nil {
		writeError(ctx, w, http.StatusInternalServerError, "internal_error", "could not deactivate organization", nil)
		return
	}

	oid := pathOrgID
	uid := p.UserID
	logAudit(ctx, db, repo.InsertAuditLogParams{
		OrganizationID: &oid,
		ActorUserID:    &uid,
		Action:         "organization.deactivated",
		EntityType:     "organization",
		EntityID:       &oid,
	})

	w.WriteHeader(http.StatusNoContent)
}

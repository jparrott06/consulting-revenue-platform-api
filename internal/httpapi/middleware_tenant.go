package httpapi

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/auth"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/authz"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/config"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/repo"
)

func requireTenantAuth(cfg config.Config, db *sql.DB, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if db == nil {
			writeError(ctx, w, http.StatusServiceUnavailable, "service_unavailable", "database is not configured", nil)
			return
		}
		if cfg.JWTSigningKey == "" {
			writeError(ctx, w, http.StatusServiceUnavailable, "service_unavailable", "JWT signing key is not configured", nil)
			return
		}

		raw, ok := bearerToken(r)
		if !ok || raw == "" {
			writeError(ctx, w, http.StatusUnauthorized, "unauthorized", "missing bearer token", nil)
			return
		}

		userIDStr, sessionIDStr, err := auth.ParseAccessToken([]byte(cfg.JWTSigningKey), raw)
		if err != nil {
			writeError(ctx, w, http.StatusUnauthorized, "unauthorized", "invalid access token", nil)
			return
		}

		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			writeError(ctx, w, http.StatusUnauthorized, "unauthorized", "invalid access token", nil)
			return
		}
		sessionID, err := uuid.Parse(sessionIDStr)
		if err != nil {
			writeError(ctx, w, http.StatusUnauthorized, "unauthorized", "invalid access token", nil)
			return
		}

		orgHeader := r.Header.Get("X-Organization-ID")
		if orgHeader == "" {
			writeError(ctx, w, http.StatusBadRequest, "validation_error", "X-Organization-ID header is required", nil)
			return
		}
		orgID, err := uuid.Parse(orgHeader)
		if err != nil {
			writeError(ctx, w, http.StatusBadRequest, "validation_error", "X-Organization-ID must be a UUID", nil)
			return
		}

		role, err := repo.GetActiveMembershipRole(ctx, db, userID, orgID)
		if err != nil {
			if errors.Is(err, repo.ErrMembershipNotFound) {
				writeError(ctx, w, http.StatusForbidden, "forbidden", "no active membership for organization", nil)
				return
			}
			writeError(ctx, w, http.StatusInternalServerError, "internal_error", "could not resolve membership", nil)
			return
		}

		principal := Principal{
			UserID:         userID,
			OrganizationID: orgID,
			Role:           role,
			SessionID:      sessionID,
		}

		next.ServeHTTP(w, r.WithContext(WithPrincipal(ctx, principal)))
	})
}

func requireRole(action string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		p, ok := PrincipalFromContext(ctx)
		if !ok {
			writeError(ctx, w, http.StatusUnauthorized, "unauthorized", "missing authentication context", nil)
			return
		}
		if !authz.RoleAllows(p.Role, action) {
			writeError(ctx, w, http.StatusForbidden, "forbidden", "insufficient permissions", map[string]any{
				"action": action,
				"role":   p.Role,
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

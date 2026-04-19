package httpapi

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/auth"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/repo"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/validate"
)

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	FullName string `json:"full_name"`
}

type registerResponse struct {
	UserID         string `json:"user_id"`
	OrganizationID string `json:"organization_id"`
}

func registerHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if db == nil {
			writeError(ctx, w, http.StatusServiceUnavailable, "service_unavailable", "database is not configured", nil)
			return
		}

		var req registerRequest
		if !decodeJSONBody(ctx, w, r, &req) {
			return
		}
		req.Email = validate.NormalizeEmail(req.Email)
		req.FullName = validate.TrimString(req.FullName)

		if req.Email == "" || req.FullName == "" {
			writeError(ctx, w, http.StatusBadRequest, "validation_error", "email and full_name are required", map[string]any{
				"fields": []string{"email", "full_name"},
			})
			return
		}

		if err := auth.ValidatePassword(req.Password); err != nil {
			writeError(ctx, w, http.StatusBadRequest, "validation_error", err.Error(), map[string]any{"field": "password"})
			return
		}

		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			writeError(ctx, w, http.StatusInternalServerError, "internal_error", "could not process password", nil)
			return
		}

		userID, orgID, err := repo.RegisterUserAndOrganization(ctx, db, req.Email, hash, req.FullName)
		if errors.Is(err, repo.ErrDuplicateEmail) {
			writeError(ctx, w, http.StatusConflict, "registration_failed", "registration could not be completed", nil)
			return
		}
		if err != nil {
			writeError(ctx, w, http.StatusInternalServerError, "internal_error", "registration failed", nil)
			return
		}

		if uid, errUID := uuid.Parse(userID); errUID == nil {
			if oid, errOID := uuid.Parse(orgID); errOID == nil {
				logAudit(ctx, db, repo.InsertAuditLogParams{
					OrganizationID: &oid,
					ActorUserID:    &uid,
					Action:         "auth.register",
					EntityType:     "user",
					EntityID:       &uid,
					Metadata:       map[string]any{"organization_id": oid.String()},
				})
			}
		}

		writeJSON(w, http.StatusCreated, registerResponse{
			UserID:         userID,
			OrganizationID: orgID,
		})
	}
}

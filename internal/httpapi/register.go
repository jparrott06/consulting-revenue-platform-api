package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jparrott06/consulting-revenue-platform-api/internal/auth"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/repo"
)

const maxRegisterBodyBytes = 1 << 20

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

		r.Body = http.MaxBytesReader(w, r.Body, maxRegisterBodyBytes)

		var req registerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(ctx, w, http.StatusBadRequest, "validation_error", "invalid JSON body", nil)
			return
		}

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

		writeJSON(w, http.StatusCreated, registerResponse{
			UserID:         userID,
			OrganizationID: orgID,
		})
	}
}

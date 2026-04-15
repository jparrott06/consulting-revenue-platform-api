package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/auth"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/config"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/repo"
)

const maxAuthBodyBytes = 1 << 20

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func authHandlers(cfg config.Config, db *sql.DB) (login, refresh, logout http.HandlerFunc) {
	login = func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if db == nil {
			writeError(ctx, w, http.StatusServiceUnavailable, "service_unavailable", "database is not configured", nil)
			return
		}
		if cfg.JWTSigningKey == "" {
			writeError(ctx, w, http.StatusServiceUnavailable, "service_unavailable", "JWT signing key is not configured", nil)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxAuthBodyBytes)
		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(ctx, w, http.StatusBadRequest, "validation_error", "invalid JSON body", nil)
			return
		}
		if strings.TrimSpace(req.Email) == "" || req.Password == "" {
			writeError(ctx, w, http.StatusBadRequest, "validation_error", "email and password are required", nil)
			return
		}

		user, err := repo.GetUserAuthByEmail(ctx, db, req.Email)
		if errors.Is(err, repo.ErrUserNotFound) {
			writeError(ctx, w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password", nil)
			return
		}
		if err != nil {
			writeError(ctx, w, http.StatusInternalServerError, "internal_error", "login failed", nil)
			return
		}

		if err := auth.CheckPassword(user.PasswordHash, req.Password); err != nil {
			writeError(ctx, w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password", nil)
			return
		}

		refreshRaw, err := auth.NewRefreshToken()
		if err != nil {
			writeError(ctx, w, http.StatusInternalServerError, "internal_error", "could not create session", nil)
			return
		}

		familyID := uuid.New()
		expiresAt := time.Now().UTC().Add(cfg.JWTRefreshTTL)
		sessionID, err := repo.InsertAuthSession(ctx, db, user.ID, auth.HashRefreshToken(refreshRaw), familyID, expiresAt)
		if err != nil {
			writeError(ctx, w, http.StatusInternalServerError, "internal_error", "could not create session", nil)
			return
		}

		access, err := auth.IssueAccessToken([]byte(cfg.JWTSigningKey), user.ID.String(), sessionID.String(), cfg.JWTAccessTTL)
		if err != nil {
			writeError(ctx, w, http.StatusInternalServerError, "internal_error", "could not issue access token", nil)
			return
		}

		writeJSON(w, http.StatusOK, tokenResponse{
			AccessToken:  access,
			RefreshToken: refreshRaw,
			ExpiresIn:    int64(cfg.JWTAccessTTL / time.Second),
			TokenType:    "Bearer",
		})
	}

	refresh = func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if db == nil {
			writeError(ctx, w, http.StatusServiceUnavailable, "service_unavailable", "database is not configured", nil)
			return
		}
		if cfg.JWTSigningKey == "" {
			writeError(ctx, w, http.StatusServiceUnavailable, "service_unavailable", "JWT signing key is not configured", nil)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxAuthBodyBytes)
		var req refreshRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(ctx, w, http.StatusBadRequest, "validation_error", "invalid JSON body", nil)
			return
		}
		if strings.TrimSpace(req.RefreshToken) == "" {
			writeError(ctx, w, http.StatusBadRequest, "validation_error", "refresh_token is required", nil)
			return
		}

		hash := auth.HashRefreshToken(req.RefreshToken)
		sess, err := repo.GetAuthSessionByRefreshHash(ctx, db, hash)
		if errors.Is(err, sql.ErrNoRows) {
			writeError(ctx, w, http.StatusUnauthorized, "invalid_token", "refresh token is invalid or expired", nil)
			return
		}
		if err != nil {
			writeError(ctx, w, http.StatusInternalServerError, "internal_error", "refresh failed", nil)
			return
		}

		newRefresh, err := auth.NewRefreshToken()
		if err != nil {
			writeError(ctx, w, http.StatusInternalServerError, "internal_error", "could not rotate session", nil)
			return
		}
		newExpires := time.Now().UTC().Add(cfg.JWTRefreshTTL)
		if err := repo.UpdateAuthSessionRefresh(ctx, db, sess.ID, auth.HashRefreshToken(newRefresh), newExpires); err != nil {
			writeError(ctx, w, http.StatusUnauthorized, "invalid_token", "refresh token is invalid or expired", nil)
			return
		}

		access, err := auth.IssueAccessToken([]byte(cfg.JWTSigningKey), sess.UserID.String(), sess.ID.String(), cfg.JWTAccessTTL)
		if err != nil {
			writeError(ctx, w, http.StatusInternalServerError, "internal_error", "could not issue access token", nil)
			return
		}

		writeJSON(w, http.StatusOK, tokenResponse{
			AccessToken:  access,
			RefreshToken: newRefresh,
			ExpiresIn:    int64(cfg.JWTAccessTTL / time.Second),
			TokenType:    "Bearer",
		})
	}

	logout = func(w http.ResponseWriter, r *http.Request) {
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

		userID, sessionIDStr, err := auth.ParseAccessToken([]byte(cfg.JWTSigningKey), raw)
		if err != nil {
			writeError(ctx, w, http.StatusUnauthorized, "unauthorized", "invalid access token", nil)
			return
		}

		sessionID, err := uuid.Parse(sessionIDStr)
		if err != nil {
			writeError(ctx, w, http.StatusUnauthorized, "unauthorized", "invalid access token", nil)
			return
		}
		uid, err := uuid.Parse(userID)
		if err != nil {
			writeError(ctx, w, http.StatusUnauthorized, "unauthorized", "invalid access token", nil)
			return
		}

		okBelongs, err := repo.SessionBelongsToUser(ctx, db, sessionID, uid)
		if err != nil {
			writeError(ctx, w, http.StatusInternalServerError, "internal_error", "logout failed", nil)
			return
		}
		if !okBelongs {
			writeError(ctx, w, http.StatusUnauthorized, "unauthorized", "invalid access token", nil)
			return
		}

		if err := repo.RevokeAuthSession(ctx, db, sessionID); err != nil {
			writeError(ctx, w, http.StatusInternalServerError, "internal_error", "logout failed", nil)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}

	return login, refresh, logout
}

func bearerToken(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return "", false
	}
	return strings.TrimSpace(strings.TrimPrefix(h, "Bearer ")), true
}

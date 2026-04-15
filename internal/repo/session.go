package repo

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
)

// AuthSession represents a persisted auth session row.
type AuthSession struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	FamilyID  uuid.UUID
	ExpiresAt time.Time
	RevokedAt sql.NullTime
}

// InsertAuthSession creates a new auth session and returns its identifier.
func InsertAuthSession(ctx context.Context, db *sql.DB, userID uuid.UUID, refreshHash string, familyID uuid.UUID, expiresAt time.Time) (uuid.UUID, error) {
	var id uuid.UUID
	err := db.QueryRowContext(ctx,
		`INSERT INTO auth_sessions (user_id, refresh_token_hash, family_id, expires_at) VALUES ($1, $2, $3, $4) RETURNING id`,
		userID, refreshHash, familyID, expiresAt,
	).Scan(&id)
	return id, err
}

// GetAuthSessionByRefreshHash loads an active session by refresh token hash.
func GetAuthSessionByRefreshHash(ctx context.Context, db *sql.DB, refreshHash string) (AuthSession, error) {
	var s AuthSession
	err := db.QueryRowContext(ctx,
		`SELECT id, user_id, family_id, expires_at, revoked_at FROM auth_sessions WHERE refresh_token_hash = $1 AND revoked_at IS NULL AND expires_at > NOW()`,
		refreshHash,
	).Scan(&s.ID, &s.UserID, &s.FamilyID, &s.ExpiresAt, &s.RevokedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return AuthSession{}, err
	}
	return s, err
}

// UpdateAuthSessionRefresh rotates the refresh token hash and extends expiry.
func UpdateAuthSessionRefresh(ctx context.Context, db *sql.DB, sessionID uuid.UUID, newHash string, expiresAt time.Time) error {
	res, err := db.ExecContext(ctx,
		`UPDATE auth_sessions SET refresh_token_hash = $2, expires_at = $3 WHERE id = $1 AND revoked_at IS NULL`,
		sessionID, newHash, expiresAt,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// RevokeAuthSession marks a session as revoked.
func RevokeAuthSession(ctx context.Context, db *sql.DB, sessionID uuid.UUID) error {
	_, err := db.ExecContext(ctx,
		`UPDATE auth_sessions SET revoked_at = NOW() WHERE id = $1 AND revoked_at IS NULL`,
		sessionID,
	)
	return err
}

// SessionBelongsToUser verifies a session identifier belongs to a user.
func SessionBelongsToUser(ctx context.Context, db *sql.DB, sessionID, userID uuid.UUID) (bool, error) {
	var count int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(1) FROM auth_sessions WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL`,
		sessionID, userID,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count == 1, nil
}

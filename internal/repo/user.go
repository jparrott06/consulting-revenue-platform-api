package repo

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/google/uuid"
)

// ErrUserNotFound is returned when a user record does not exist.
var ErrUserNotFound = errors.New("user not found")

// UserAuthRecord contains authentication fields for a user row.
type UserAuthRecord struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
}

// GetUserAuthByEmail loads a user by normalized email for authentication.
func GetUserAuthByEmail(ctx context.Context, db *sql.DB, email string) (UserAuthRecord, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	var rec UserAuthRecord
	err := db.QueryRowContext(ctx,
		`SELECT id, email, password_hash FROM users WHERE email = $1`,
		email,
	).Scan(&rec.ID, &rec.Email, &rec.PasswordHash)
	if errors.Is(err, sql.ErrNoRows) {
		return UserAuthRecord{}, ErrUserNotFound
	}
	return rec, err
}

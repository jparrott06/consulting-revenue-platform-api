package repo

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
)

// ErrMembershipNotFound is returned when no active membership exists for the user and organization.
var ErrMembershipNotFound = errors.New("membership not found")

// GetActiveMembershipRole returns the role for an active membership.
func GetActiveMembershipRole(ctx context.Context, db *sql.DB, userID, organizationID uuid.UUID) (string, error) {
	var role string
	err := db.QueryRowContext(ctx,
		`SELECT role FROM memberships WHERE user_id = $1 AND organization_id = $2 AND status = 'active'`,
		userID, organizationID,
	).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrMembershipNotFound
	}
	return role, err
}

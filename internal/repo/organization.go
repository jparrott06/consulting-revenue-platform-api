package repo

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
)

// ErrOrganizationNotFound is returned when an organization row does not exist.
var ErrOrganizationNotFound = errors.New("organization not found")

// DeactivateOrganization marks the organization as deactivated and suspends all active memberships.
// Financial and accounting rows are not deleted. Idempotent when already deactivated.
func DeactivateOrganization(ctx context.Context, db *sql.DB, organizationID uuid.UUID) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	var deactivatedAt sql.NullTime
	err = tx.QueryRowContext(ctx, `SELECT deactivated_at FROM organizations WHERE id = $1`, organizationID).Scan(&deactivatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrOrganizationNotFound
	}
	if err != nil {
		return err
	}
	if deactivatedAt.Valid {
		if err := tx.Commit(); err != nil {
			return err
		}
		committed = true
		return nil
	}

	if _, err := tx.ExecContext(ctx, `
UPDATE organizations
SET deactivated_at = NOW(), updated_at = NOW()
WHERE id = $1`, organizationID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE memberships
SET status = 'suspended', updated_at = NOW()
WHERE organization_id = $1 AND status = 'active'`, organizationID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

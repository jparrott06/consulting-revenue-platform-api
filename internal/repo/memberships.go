package repo

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// ErrDuplicateMembership is returned when a membership already exists for the user and organization.
var ErrDuplicateMembership = errors.New("membership already exists")

// ErrLastOwnerProtected is returned when an operation would remove or demote the last active owner.
var ErrLastOwnerProtected = errors.New("last owner protected")

// ErrInvalidMembershipRole is returned when a role value is not allowed.
var ErrInvalidMembershipRole = errors.New("invalid membership role")

// MembershipListItem is a membership row joined with user profile fields for listing.
type MembershipListItem struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Email     string
	FullName  string
	Role      string
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
	InvitedAt sql.NullTime
	JoinedAt  sql.NullTime
}

// MembershipRecord is a membership row scoped to an organization.
type MembershipRecord struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	OrganizationID uuid.UUID
	Role           string
	Status         string
}

func normalizeMembershipRole(role string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "owner":
		return "owner", nil
	case "accountant":
		return "accountant", nil
	case "contractor":
		return "contractor", nil
	default:
		return "", ErrInvalidMembershipRole
	}
}

// ListMembershipsForOrganization returns memberships for an organization ordered by creation time.
func ListMembershipsForOrganization(ctx context.Context, db *sql.DB, organizationID uuid.UUID) ([]MembershipListItem, error) {
	rows, err := db.QueryContext(ctx, `
SELECT m.id, m.user_id, u.email, u.full_name, m.role, m.status, m.created_at, m.updated_at, m.invited_at, m.joined_at
FROM memberships m
JOIN users u ON u.id = m.user_id
WHERE m.organization_id = $1
ORDER BY m.created_at ASC`, organizationID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []MembershipListItem
	for rows.Next() {
		var item MembershipListItem
		if err := rows.Scan(
			&item.ID,
			&item.UserID,
			&item.Email,
			&item.FullName,
			&item.Role,
			&item.Status,
			&item.CreatedAt,
			&item.UpdatedAt,
			&item.InvitedAt,
			&item.JoinedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// GetMembershipInOrganization loads a membership by id constrained to an organization.
func GetMembershipInOrganization(ctx context.Context, db *sql.DB, organizationID, membershipID uuid.UUID) (MembershipRecord, error) {
	var rec MembershipRecord
	err := db.QueryRowContext(ctx, `
SELECT id, user_id, organization_id, role, status
FROM memberships
WHERE id = $1 AND organization_id = $2`, membershipID, organizationID,
	).Scan(&rec.ID, &rec.UserID, &rec.OrganizationID, &rec.Role, &rec.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return MembershipRecord{}, ErrMembershipNotFound
	}
	return rec, err
}

// CountActiveOwnersInOrganization returns how many active owner memberships exist for an organization.
func CountActiveOwnersInOrganization(ctx context.Context, db *sql.DB, organizationID uuid.UUID) (int, error) {
	var n int
	err := db.QueryRowContext(ctx, `
SELECT COUNT(*) FROM memberships
WHERE organization_id = $1 AND role = 'owner' AND status = 'active'`, organizationID).Scan(&n)
	return n, err
}

// GetUserIDByEmail returns a user id for an existing email (case-insensitive).
func GetUserIDByEmail(ctx context.Context, db *sql.DB, email string) (uuid.UUID, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	var id uuid.UUID
	err := db.QueryRowContext(ctx, `SELECT id FROM users WHERE email = $1`, email).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, ErrUserNotFound
	}
	return id, err
}

// FindMembershipForUserOrganization returns a membership for the user in the org, if any status exists.
func FindMembershipForUserOrganization(ctx context.Context, db *sql.DB, userID, organizationID uuid.UUID) (MembershipRecord, error) {
	var rec MembershipRecord
	err := db.QueryRowContext(ctx, `
SELECT id, user_id, organization_id, role, status
FROM memberships
WHERE user_id = $1 AND organization_id = $2`, userID, organizationID,
	).Scan(&rec.ID, &rec.UserID, &rec.OrganizationID, &rec.Role, &rec.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return MembershipRecord{}, ErrMembershipNotFound
	}
	return rec, err
}

// CreateActiveMembership inserts a new active membership.
func CreateActiveMembership(ctx context.Context, db *sql.DB, organizationID, userID uuid.UUID, role string) (uuid.UUID, error) {
	role, err := normalizeMembershipRole(role)
	if err != nil {
		return uuid.Nil, err
	}
	now := time.Now().UTC()
	var id uuid.UUID
	err = db.QueryRowContext(ctx, `
INSERT INTO memberships (user_id, organization_id, role, status, invited_at, joined_at)
VALUES ($1, $2, $3, 'active', NULL, $4)
RETURNING id`, userID, organizationID, role, now).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			return uuid.Nil, ErrDuplicateMembership
		}
		return uuid.Nil, err
	}
	return id, nil
}

// UpdateMembershipRole updates a membership role within an organization.
func UpdateMembershipRole(ctx context.Context, db *sql.DB, organizationID, membershipID uuid.UUID, newRole string) error {
	newRole, err := normalizeMembershipRole(newRole)
	if err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	rec, err := getMembershipInOrganizationTx(ctx, tx, organizationID, membershipID)
	if err != nil {
		return err
	}

	if rec.Role == "owner" && newRole != "owner" {
		n, err := countActiveOwnersInOrganizationTx(ctx, tx, organizationID)
		if err != nil {
			return err
		}
		if n <= 1 {
			return ErrLastOwnerProtected
		}
	}

	_, err = tx.ExecContext(ctx, `
UPDATE memberships SET role = $1, updated_at = NOW() WHERE id = $2 AND organization_id = $3`,
		newRole, membershipID, organizationID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// DeleteMembership removes a membership row within an organization.
func DeleteMembership(ctx context.Context, db *sql.DB, organizationID, membershipID uuid.UUID) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	rec, err := getMembershipInOrganizationTx(ctx, tx, organizationID, membershipID)
	if err != nil {
		return err
	}

	if rec.Role == "owner" && rec.Status == "active" {
		n, err := countActiveOwnersInOrganizationTx(ctx, tx, organizationID)
		if err != nil {
			return err
		}
		if n <= 1 {
			return ErrLastOwnerProtected
		}
	}

	res, err := tx.ExecContext(ctx, `DELETE FROM memberships WHERE id = $1 AND organization_id = $2`, membershipID, organizationID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrMembershipNotFound
	}
	return tx.Commit()
}

func getMembershipInOrganizationTx(ctx context.Context, tx *sql.Tx, organizationID, membershipID uuid.UUID) (MembershipRecord, error) {
	var rec MembershipRecord
	err := tx.QueryRowContext(ctx, `
SELECT id, user_id, organization_id, role, status
FROM memberships
WHERE id = $1 AND organization_id = $2`, membershipID, organizationID,
	).Scan(&rec.ID, &rec.UserID, &rec.OrganizationID, &rec.Role, &rec.Status)
	if errors.Is(err, sql.ErrNoRows) {
		return MembershipRecord{}, ErrMembershipNotFound
	}
	return rec, err
}

func countActiveOwnersInOrganizationTx(ctx context.Context, tx *sql.Tx, organizationID uuid.UUID) (int, error) {
	var n int
	err := tx.QueryRowContext(ctx, `
SELECT COUNT(*) FROM memberships
WHERE organization_id = $1 AND role = 'owner' AND status = 'active'`, organizationID).Scan(&n)
	return n, err
}

func isUniqueViolation(err error) bool {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		return pqErr.Code == "23505"
	}
	return false
}

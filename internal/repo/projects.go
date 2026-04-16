package repo

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ErrProjectNotFound is returned when a project is missing in the organization scope.
var ErrProjectNotFound = errors.New("project not found")

// ErrInvalidBillingMode is returned when billing_mode is not allowed.
var ErrInvalidBillingMode = errors.New("invalid billing mode")

// ProjectRecord is a tenant-scoped project row.
type ProjectRecord struct {
	ID               uuid.UUID
	OrganizationID   uuid.UUID
	ClientID         uuid.UUID
	Name             string
	BillingMode      string
	DefaultRateMinor int64
	Archived         bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func normalizeBillingMode(s string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "hourly":
		return "hourly", nil
	case "fixed":
		return "fixed", nil
	case "non_billable":
		return "non_billable", nil
	default:
		return "", ErrInvalidBillingMode
	}
}

// CreateProject inserts a project; client must belong to the org and not be soft-deleted.
func CreateProject(ctx context.Context, db *sql.DB, organizationID, clientID uuid.UUID, name, billingMode string, defaultRateMinor int64) (uuid.UUID, error) {
	if _, err := GetClient(ctx, db, organizationID, clientID); err != nil {
		return uuid.Nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return uuid.Nil, errors.New("name is required")
	}
	bm, err := normalizeBillingMode(billingMode)
	if err != nil {
		return uuid.Nil, err
	}
	if defaultRateMinor < 0 {
		return uuid.Nil, errors.New("default_rate_minor must be non-negative")
	}

	var id uuid.UUID
	err = db.QueryRowContext(ctx, `
INSERT INTO projects (organization_id, client_id, name, billing_mode, default_rate_minor)
VALUES ($1, $2, $3, $4, $5)
RETURNING id`, organizationID, clientID, name, bm, defaultRateMinor).Scan(&id)
	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

// ListProjects returns projects; when includeArchived is false, only active (non-archived) projects.
func ListProjects(ctx context.Context, db *sql.DB, organizationID uuid.UUID, includeArchived bool) ([]ProjectRecord, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if includeArchived {
		rows, err = db.QueryContext(ctx, `
SELECT id, organization_id, client_id, name, billing_mode, default_rate_minor, archived, created_at, updated_at
FROM projects
WHERE organization_id = $1
ORDER BY created_at ASC, id ASC`, organizationID)
	} else {
		rows, err = db.QueryContext(ctx, `
SELECT id, organization_id, client_id, name, billing_mode, default_rate_minor, archived, created_at, updated_at
FROM projects
WHERE organization_id = $1 AND archived = false
ORDER BY created_at ASC, id ASC`, organizationID)
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []ProjectRecord
	for rows.Next() {
		var rec ProjectRecord
		if err := rows.Scan(
			&rec.ID,
			&rec.OrganizationID,
			&rec.ClientID,
			&rec.Name,
			&rec.BillingMode,
			&rec.DefaultRateMinor,
			&rec.Archived,
			&rec.CreatedAt,
			&rec.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

// GetProject returns a project scoped to the organization.
func GetProject(ctx context.Context, db *sql.DB, organizationID, projectID uuid.UUID) (ProjectRecord, error) {
	var rec ProjectRecord
	err := db.QueryRowContext(ctx, `
SELECT id, organization_id, client_id, name, billing_mode, default_rate_minor, archived, created_at, updated_at
FROM projects
WHERE id = $1 AND organization_id = $2`, projectID, organizationID,
	).Scan(
		&rec.ID,
		&rec.OrganizationID,
		&rec.ClientID,
		&rec.Name,
		&rec.BillingMode,
		&rec.DefaultRateMinor,
		&rec.Archived,
		&rec.CreatedAt,
		&rec.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ProjectRecord{}, ErrProjectNotFound
	}
	return rec, err
}

// UpdateProject applies optional field updates.
func UpdateProject(ctx context.Context, db *sql.DB, organizationID, projectID uuid.UUID, name *string, billingMode *string, defaultRateMinor *int64, archived *bool) error {
	rec, err := GetProject(ctx, db, organizationID, projectID)
	if err != nil {
		return err
	}
	newName := rec.Name
	if name != nil {
		newName = strings.TrimSpace(*name)
		if newName == "" {
			return errors.New("name cannot be empty")
		}
	}
	newBM := rec.BillingMode
	if billingMode != nil {
		bm, err := normalizeBillingMode(*billingMode)
		if err != nil {
			return err
		}
		newBM = bm
	}
	newRate := rec.DefaultRateMinor
	if defaultRateMinor != nil {
		if *defaultRateMinor < 0 {
			return errors.New("default_rate_minor must be non-negative")
		}
		newRate = *defaultRateMinor
	}
	newArchived := rec.Archived
	if archived != nil {
		newArchived = *archived
	}

	res, err := db.ExecContext(ctx, `
UPDATE projects
SET name = $1, billing_mode = $2, default_rate_minor = $3, archived = $4, updated_at = NOW()
WHERE id = $5 AND organization_id = $6`,
		newName, newBM, newRate, newArchived, projectID, organizationID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrProjectNotFound
	}
	return nil
}

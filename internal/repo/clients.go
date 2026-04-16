package repo

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ErrClientNotFound is returned when a client row is missing or not visible in the org scope.
var ErrClientNotFound = errors.New("client not found")

// ClientRecord is a tenant-scoped client row.
type ClientRecord struct {
	ID                 uuid.UUID
	OrganizationID     uuid.UUID
	Name               string
	BillingEmail       string
	CurrencyPreference string
	DeletedAt          sql.NullTime
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// NormalizeBillingEmail trims and lowercases email for storage (citext compares case-insensitively).
func NormalizeBillingEmail(s string) string {
	return strings.TrimSpace(strings.ToLower(s))
}

// CreateClient inserts a new active client.
func CreateClient(ctx context.Context, db *sql.DB, organizationID uuid.UUID, name, billingEmail, currency string) (uuid.UUID, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return uuid.Nil, errors.New("name is required")
	}
	billingEmail = NormalizeBillingEmail(billingEmail)
	if billingEmail == "" {
		return uuid.Nil, errors.New("billing_email is required")
	}
	currency = strings.TrimSpace(currency)
	if currency == "" {
		currency = "USD"
	}
	currency, err := NormalizeCurrencyCode(currency)
	if err != nil {
		return uuid.Nil, err
	}

	var id uuid.UUID
	err = db.QueryRowContext(ctx, `
INSERT INTO clients (organization_id, name, billing_email, currency_preference)
VALUES ($1, $2, $3, $4)
RETURNING id`, organizationID, name, billingEmail, currency).Scan(&id)
	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

// ListClients returns clients for an organization; when includeDeleted is false, only active rows.
func ListClients(ctx context.Context, db *sql.DB, organizationID uuid.UUID, includeDeleted bool) ([]ClientRecord, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if includeDeleted {
		rows, err = db.QueryContext(ctx, `
SELECT id, organization_id, name, billing_email, currency_preference, deleted_at, created_at, updated_at
FROM clients
WHERE organization_id = $1
ORDER BY created_at ASC, id ASC`, organizationID)
	} else {
		rows, err = db.QueryContext(ctx, `
SELECT id, organization_id, name, billing_email, currency_preference, deleted_at, created_at, updated_at
FROM clients
WHERE organization_id = $1 AND deleted_at IS NULL
ORDER BY created_at ASC, id ASC`, organizationID)
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []ClientRecord
	for rows.Next() {
		var rec ClientRecord
		if err := rows.Scan(
			&rec.ID,
			&rec.OrganizationID,
			&rec.Name,
			&rec.BillingEmail,
			&rec.CurrencyPreference,
			&rec.DeletedAt,
			&rec.CreatedAt,
			&rec.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

// GetClient loads a client by id scoped to the organization. Soft-deleted rows are not returned.
func GetClient(ctx context.Context, db *sql.DB, organizationID, clientID uuid.UUID) (ClientRecord, error) {
	var rec ClientRecord
	err := db.QueryRowContext(ctx, `
SELECT id, organization_id, name, billing_email, currency_preference, deleted_at, created_at, updated_at
FROM clients
WHERE id = $1 AND organization_id = $2 AND deleted_at IS NULL`,
		clientID, organizationID,
	).Scan(
		&rec.ID,
		&rec.OrganizationID,
		&rec.Name,
		&rec.BillingEmail,
		&rec.CurrencyPreference,
		&rec.DeletedAt,
		&rec.CreatedAt,
		&rec.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ClientRecord{}, ErrClientNotFound
	}
	return rec, err
}

// GetClientIncludingDeleted loads a client including soft-deleted rows (for authorized list flows).
func GetClientIncludingDeleted(ctx context.Context, db *sql.DB, organizationID, clientID uuid.UUID) (ClientRecord, error) {
	var rec ClientRecord
	err := db.QueryRowContext(ctx, `
SELECT id, organization_id, name, billing_email, currency_preference, deleted_at, created_at, updated_at
FROM clients
WHERE id = $1 AND organization_id = $2`,
		clientID, organizationID,
	).Scan(
		&rec.ID,
		&rec.OrganizationID,
		&rec.Name,
		&rec.BillingEmail,
		&rec.CurrencyPreference,
		&rec.DeletedAt,
		&rec.CreatedAt,
		&rec.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ClientRecord{}, ErrClientNotFound
	}
	return rec, err
}

// UpdateClient patches non-nil fields.
func UpdateClient(ctx context.Context, db *sql.DB, organizationID, clientID uuid.UUID, name *string, billingEmail *string, currency *string) error {
	rec, err := GetClient(ctx, db, organizationID, clientID)
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
	newEmail := rec.BillingEmail
	if billingEmail != nil {
		newEmail = NormalizeBillingEmail(*billingEmail)
		if newEmail == "" {
			return errors.New("billing_email cannot be empty")
		}
	}
	newCurr := rec.CurrencyPreference
	if currency != nil {
		c, err := NormalizeCurrencyCode(*currency)
		if err != nil {
			return err
		}
		newCurr = c
	}

	res, err := db.ExecContext(ctx, `
UPDATE clients
SET name = $1, billing_email = $2, currency_preference = $3, updated_at = NOW()
WHERE id = $4 AND organization_id = $5 AND deleted_at IS NULL`,
		newName, newEmail, newCurr, clientID, organizationID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrClientNotFound
	}
	return nil
}

// SoftDeleteClient sets deleted_at for an active client.
func SoftDeleteClient(ctx context.Context, db *sql.DB, organizationID, clientID uuid.UUID) error {
	res, err := db.ExecContext(ctx, `
UPDATE clients
SET deleted_at = NOW(), updated_at = NOW()
WHERE id = $1 AND organization_id = $2 AND deleted_at IS NULL`,
		clientID, organizationID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrClientNotFound
	}
	return nil
}

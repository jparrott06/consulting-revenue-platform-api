package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// ErrDuplicateEmail is returned when a user email already exists.
var ErrDuplicateEmail = errors.New("duplicate email")

// RegisterUserAndOrganization creates a user, default organization, and owner membership in one transaction.
func RegisterUserAndOrganization(ctx context.Context, db *sql.DB, email, passwordHash, fullName string) (userID, orgID string, err error) {
	email = strings.TrimSpace(strings.ToLower(email))
	fullName = strings.TrimSpace(fullName)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = tx.Rollback() }()

	var uid uuid.UUID
	err = tx.QueryRowContext(ctx,
		`INSERT INTO users (email, password_hash, full_name) VALUES ($1, $2, $3) RETURNING id`,
		email, passwordHash, fullName,
	).Scan(&uid)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return "", "", ErrDuplicateEmail
		}
		return "", "", err
	}

	orgName := fmt.Sprintf("%s's Organization", fullName)
	if strings.TrimSpace(orgName) == "'s Organization" {
		orgName = "Organization"
	}

	var oid uuid.UUID
	err = tx.QueryRowContext(ctx,
		`INSERT INTO organizations (name) VALUES ($1) RETURNING id`,
		orgName,
	).Scan(&oid)
	if err != nil {
		return "", "", err
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO memberships (user_id, organization_id, role, status, joined_at) VALUES ($1, $2, 'owner', 'active', NOW())`,
		uid, oid,
	)
	if err != nil {
		return "", "", err
	}

	if err := tx.Commit(); err != nil {
		return "", "", err
	}

	return uid.String(), oid.String(), nil
}

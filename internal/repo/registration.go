package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jparrott06/consulting-revenue-platform-api/internal/db"
	"github.com/lib/pq"
)

// ErrDuplicateEmail is returned when a user email already exists.
var ErrDuplicateEmail = errors.New("duplicate email")

// RegisterUserAndOrganization creates a user, default organization, and owner membership in one transaction.
func RegisterUserAndOrganization(ctx context.Context, pool *sql.DB, email, passwordHash, fullName string) (userID, orgID string, err error) {
	email = strings.TrimSpace(strings.ToLower(email))
	fullName = strings.TrimSpace(fullName)

	var uid uuid.UUID
	var oid uuid.UUID
	err = db.RunInTx(ctx, pool, nil, func(tx *sql.Tx) error {
		e := tx.QueryRowContext(ctx,
			`INSERT INTO users (email, password_hash, full_name) VALUES ($1, $2, $3) RETURNING id`,
			email, passwordHash, fullName,
		).Scan(&uid)
		if e != nil {
			var pqErr *pq.Error
			if errors.As(e, &pqErr) && pqErr.Code == "23505" {
				return ErrDuplicateEmail
			}
			return e
		}

		orgName := fmt.Sprintf("%s's Organization", fullName)
		if strings.TrimSpace(orgName) == "'s Organization" {
			orgName = "Organization"
		}

		e = tx.QueryRowContext(ctx,
			`INSERT INTO organizations (name) VALUES ($1) RETURNING id`,
			orgName,
		).Scan(&oid)
		if e != nil {
			return e
		}

		_, e = tx.ExecContext(ctx,
			`INSERT INTO memberships (user_id, organization_id, role, status, joined_at) VALUES ($1, $2, 'owner', 'active', NOW())`,
			uid, oid,
		)
		return e
	})
	if err != nil {
		if errors.Is(err, ErrDuplicateEmail) {
			return "", "", ErrDuplicateEmail
		}
		return "", "", err
	}
	return uid.String(), oid.String(), nil
}

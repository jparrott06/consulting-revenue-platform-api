package repo

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
)

func TestDeactivateOrganization_IdempotentWhenAlreadyDeactivated(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	orgID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT deactivated_at FROM organizations WHERE id = \$1`).
		WithArgs(orgID).
		WillReturnRows(sqlmock.NewRows([]string{"deactivated_at"}).AddRow(time.Unix(100, 0).UTC()))
	mock.ExpectCommit()

	if err := DeactivateOrganization(context.Background(), db, orgID); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDeactivateOrganization_SuspendsMemberships(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	orgID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT deactivated_at FROM organizations WHERE id = \$1`).
		WithArgs(orgID).
		WillReturnRows(sqlmock.NewRows([]string{"deactivated_at"}).AddRow(nil))
	mock.ExpectExec(`UPDATE organizations`).
		WithArgs(orgID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE memberships`).
		WithArgs(orgID).
		WillReturnResult(sqlmock.NewResult(0, 3))
	mock.ExpectCommit()

	if err := DeactivateOrganization(context.Background(), db, orgID); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDeactivateOrganization_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	orgID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT deactivated_at FROM organizations WHERE id = \$1`).
		WithArgs(orgID).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	err = DeactivateOrganization(context.Background(), db, orgID)
	if !errors.Is(err, ErrOrganizationNotFound) {
		t.Fatalf("expected ErrOrganizationNotFound, got %v", err)
	}
}

package seed

import (
	"context"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestResetDemoOrganization_BlockedByLedger(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM ledger_entries WHERE organization_id = \$1`).
		WithArgs(DemoOrganizationID).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	resetErr := ResetDemoOrganization(context.Background(), db)
	if resetErr != ErrLedgerBlocksReset {
		t.Fatalf("expected ErrLedgerBlocksReset, got %v", resetErr)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

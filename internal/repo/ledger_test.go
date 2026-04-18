package repo

import (
	"context"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
)

func TestListLedgerEntries_DefaultLimit(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	orgID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	rows := sqlmock.NewRows([]string{
		"id", "organization_id", "event_type", "entity_type", "entity_id", "amount_minor", "currency", "metadata_json", "created_at",
	})
	mock.ExpectQuery(`SELECT id, organization_id, event_type, entity_type, entity_id, amount_minor, currency, metadata_json, created_at
FROM ledger_entries
WHERE organization_id = $1
ORDER BY created_at DESC, id DESC
LIMIT $2`).WithArgs(orgID, 50).WillReturnRows(rows)

	got, err := ListLedgerEntries(context.Background(), db, orgID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty, got %d", len(got))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestListLedgerEntries_MaxCap(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	orgID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	eid := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	rows := sqlmock.NewRows([]string{
		"id", "organization_id", "event_type", "entity_type", "entity_id", "amount_minor", "currency", "metadata_json", "created_at",
	}).AddRow(eid, orgID, "invoice_issued", "invoice", orgID, int64(10), "USD", []byte(`{}`), time.Now())
	mock.ExpectQuery(`SELECT id, organization_id, event_type, entity_type, entity_id, amount_minor, currency, metadata_json, created_at
FROM ledger_entries
WHERE organization_id = $1
ORDER BY created_at DESC, id DESC
LIMIT $2`).WithArgs(orgID, 200).WillReturnRows(rows)

	got, err := ListLedgerEntries(context.Background(), db, orgID, 9999)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d rows", len(got))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

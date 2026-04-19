package repo

import (
	"context"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestPurgeAuditLogsOlderThan(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	cutoff := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	mock.ExpectExec(`DELETE FROM audit_logs WHERE created_at < \$1`).
		WithArgs(cutoff).
		WillReturnResult(sqlmock.NewResult(0, 5))

	n, err := PurgeAuditLogsOlderThan(context.Background(), db, cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if n != 5 {
		t.Fatalf("rows: got %d", n)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPurgeWebhookEventsOlderThan(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	cutoff := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	mock.ExpectExec(`DELETE FROM webhook_events WHERE received_at < \$1`).
		WithArgs(cutoff).
		WillReturnResult(sqlmock.NewResult(0, 2))

	n, err := PurgeWebhookEventsOlderThan(context.Background(), db, cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("rows: got %d", n)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
